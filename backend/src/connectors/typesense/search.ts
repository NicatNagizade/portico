import readline from "node:readline";
import type Typesense from "typesense";
import type { CollectionFieldSchema } from "typesense/lib/Typesense/Collection.js";
import type { Doc, Filter, Order } from "../types.js";
import { idFilterColumn } from "../shared/filters.js";
import { filterRows, pageRows, sortRows } from "../shared/rows.js";
import { DEFAULT_SORTABLE_ID } from "../primaryKey.js";
import { splitList } from "../../utils/values.js";

const PAGE_SIZE = 250;
const RANGE_OPS: Record<string, string> = {
  gt: ">",
  gte: ">=",
  lt: "<",
  lte: "<=",
};

type Hits = Array<{ document?: Doc }> | undefined;
type FilterField = Pick<CollectionFieldSchema, "name" | "type">;

/** Search args for a filter list, or null when one of them cannot be expressed. */
export type CompiledFilter = {
  filterBy: string;
  q: string;
  /** Empty uses every string field. */
  queryBy: string;
};

export function compileFilters(
  filters: Filter[],
  sortableId: string,
  fields: FilterField[] = [],
): CompiledFilter | null {
  const filterBy = buildFilterBy(filters, sortableId, fields);
  if (filterBy == null) return null;
  return { filterBy, q: "*", queryBy: "" };
}

/** Typesense `filter_by` string, or null when a filter cannot be expressed. */
export function buildFilterBy(
  filters: Filter[],
  sortableId: string,
  fields: FilterField[] = [],
): string | null {
  const parts: string[] = [];
  for (const filter of filters) {
    const part = filterPart(filter, sortableId, fields);
    if (part == null) return null;
    if (part) parts.push(part);
  }
  return parts.join(" && ");
}

function filterPart(
  filter: Filter,
  sortableId: string,
  fields: FilterField[],
): string | null {
  const column = idFilterColumn(
    filter.column.trim(),
    filter.operator,
    filter.column.trim(),
    sortableId || DEFAULT_SORTABLE_ID,
  );
  if (!column) return null;
  if (filter.operator === "eq")
    return `${column}:=${literal(column, filter.value, fields)}`;
  if (filter.operator === "neq")
    return `${column}:!=${literal(column, filter.value, fields)}`;
  if (RANGE_OPS[filter.operator]) {
    // String ranges are token matches, not lexicographic, unless we scan.
    if (!isScalar(fieldType(fields, column))) return null;
    return `${column}:${RANGE_OPS[filter.operator]}${filter.value}`;
  }
  // `like` is a real substring match (`%`, `_`). Typesense tokens cannot
  // express that, so the caller scans the field.
  if (filter.operator === "like") return null;
  if (filter.operator === "is_null" || filter.operator === "is_not_null") {
    // Numeric fields reject `_missing`. String fields accept it.
    if (isScalar(fieldType(fields, column))) return null;
    const mark = filter.operator === "is_null" ? "_missing" : "!_missing";
    return `${column}:${mark}`;
  }
  if (filter.operator === "in" || filter.operator === "not_in") {
    const scalar = isScalar(fieldType(fields, column));
    const list = splitList(filter.value).map((value) =>
      literal(column, value, fields),
    );
    if (!list.length) return null;
    const op = filter.operator === "in" ? (scalar ? "" : "=") : "!=";
    return `${column}:${op}[${list.join(", ")}]`;
  }
  return null;
}

function fieldType(fields: FilterField[], column: string): string {
  return fields.find((field) => field.name === column)?.type ?? "";
}

function baseType(type: string): string {
  return type.endsWith("[]") ? type.slice(0, -2) : type;
}

function isScalar(type: string): boolean {
  const base = baseType(type);
  return (
    base === "int32" || base === "int64" || base === "float" || base === "bool"
  );
}

/** Numbers and bools stay bare; strings are backtick-quoted. */
function literal(column: string, value: string, fields: FilterField[]): string {
  const type = fieldType(fields, column);
  if (!isScalar(type)) return "`" + value.replaceAll("`", "") + "`";
  if (baseType(type) === "bool") return value === "true" ? "true" : "false";
  return value;
}

/** String fields to search with `q=*` (Typesense requires at least one). */
export function queryBy(
  fields: CollectionFieldSchema[],
  sortableId: string,
): string {
  return fields
    .filter(
      (f) =>
        f.name &&
        f.name !== "id" &&
        f.name !== sortableId &&
        f.index !== false &&
        (f.type === "string" || f.type === "string[]"),
    )
    .map((f) => f.name)
    .join(",");
}

/** Hit documents without the internal sortable id. */
export function hitDocs(hits: Hits, sortableId: string): Doc[] {
  return (hits ?? []).flatMap((hit) => {
    if (!hit.document) return [];
    const { [sortableId]: _, ...doc } = hit.document;
    return [doc];
  });
}

export async function searchPage(
  client: Typesense.Client,
  table: string,
  filterBy: string,
  limit: number,
  offset: number,
  sortableId: string,
) {
  const collection = await client.collections(table).retrieve();
  const fields = collection.fields ?? [];
  const result = await runSearch(
    client,
    table,
    { filterBy, q: "*", queryBy: "" },
    fields,
    limit,
    offset,
    sortableId,
  );
  return { ...result, fields };
}

export async function runSearch(
  client: Typesense.Client,
  table: string,
  compiled: CompiledFilter,
  fields: CollectionFieldSchema[],
  limit: number,
  offset: number,
  sortableId: string,
  extra: Record<string, unknown> = {},
) {
  const size = limit <= 0 ? 50 : limit;
  const queryByFields = compiled.queryBy || queryBy(fields, sortableId);
  if (!queryByFields)
    throw new Error(
      `typesense collection ${JSON.stringify(table)} has no searchable string fields to browse`,
    );
  const params: Record<string, unknown> = {
    q: compiled.q,
    query_by: queryByFields,
    page: Math.floor(Math.max(0, offset) / size) + 1,
    per_page: size,
    ...extra,
  };
  if (compiled.filterBy) params.filter_by = compiled.filterBy;
  const result = await client
    .collections(table)
    .documents()
    .search(params as never);
  return {
    rows: hitDocs(result.hits as Hits, sortableId),
    total: result.found ?? 0,
  };
}

/**
 * Drops numeric `is_null` / `is_not_null` when every document has that field
 * (or none do). Returns null when the filter matches nothing.
 */
export async function concreteFilters(
  client: Typesense.Client,
  table: string,
  filters: Filter[],
  fields: CollectionFieldSchema[],
  sortableId: string,
  numDocuments: number,
): Promise<Filter[] | null> {
  const kept: Filter[] = [];
  for (const filter of filters) {
    if (filter.operator !== "is_null" && filter.operator !== "is_not_null") {
      kept.push(filter);
      continue;
    }
    if (filter.column.trim() === "id") {
      if (filter.operator === "is_null") return null;
      continue;
    }
    const column = idFilterColumn(
      filter.column,
      filter.operator,
      filter.column,
      sortableId || DEFAULT_SORTABLE_ID,
    );
    const type = fieldType(fields, column);
    if (!isScalar(type)) {
      kept.push(filter);
      continue;
    }
    const present = await presentCount(
      client,
      table,
      column,
      type,
      fields,
      sortableId,
    );
    if (present == null) {
      kept.push(filter);
      continue;
    }
    const filled = present === numDocuments;
    const empty = present === 0;
    if (!filled && !empty) {
      kept.push(filter);
      continue;
    }
    const wantNull = filter.operator === "is_null";
    if (wantNull === filled) return null;
  }
  return kept;
}

/** Fields a streamed export needs so in-memory filters and sorts can run. */
function exportFields(filters: Filter[], order: Order | null): string {
  const names = new Set(["id"]);
  for (const filter of filters) {
    const top = filter.column.trim().split(".")[0];
    if (top) names.add(top);
  }
  const sort = order?.column.trim().split(".")[0];
  if (sort) names.add(sort);
  return [...names].join(",");
}

/**
 * Filters Typesense cannot apply in `filter_by` (`like`, string ranges).
 * Streams the compared fields, then loads the full documents for one page.
 */
export async function scanExported(
  client: Typesense.Client,
  table: string,
  filters: Filter[],
  fields: CollectionFieldSchema[],
  limit: number,
  offset: number,
  sortableId: string,
  order: Order | null,
): Promise<{ rows: Doc[]; total: number }> {
  const stream = await client
    .collections(table)
    .documents()
    .exportStream({ include_fields: exportFields(filters, order) });
  const matched: Doc[] = [];
  const lines = readline.createInterface({
    input: stream,
    crlfDelay: Infinity,
  });
  for await (const line of lines) {
    if (!line) continue;
    const doc = JSON.parse(line) as Doc;
    if (filterRows([doc], filters).length) matched.push(doc);
  }
  sortRows(matched, order);
  const page = pageRows(matched, limit, offset);
  const ids = page.map((doc) => String(doc.id ?? "")).filter(Boolean);
  return {
    rows: await docsById(client, table, ids, fields, sortableId),
    total: matched.length,
  };
}

async function presentCount(
  client: Typesense.Client,
  table: string,
  column: string,
  type: string,
  fields: CollectionFieldSchema[],
  sortableId: string,
): Promise<number | null> {
  const base = baseType(type);
  const filterBy =
    base === "bool"
      ? `(${column}:=true || ${column}:=false)`
      : base === "float"
        ? `${column}:>=-1e308`
        : `${column}:>=-9223372036854775808`;
  const q = queryBy(fields, sortableId);
  if (!q) return null;
  try {
    const result = await client.collections(table).documents().search({
      q: "*",
      query_by: q,
      filter_by: filterBy,
      per_page: 1,
    });
    return result.found ?? 0;
  } catch {
    return null;
  }
}

async function docsById(
  client: Typesense.Client,
  table: string,
  ids: string[],
  fields: CollectionFieldSchema[],
  sortableId: string,
): Promise<Doc[]> {
  if (!ids.length) return [];
  const list = ids.map((id) => "`" + id.replaceAll("`", "") + "`").join(", ");
  const { rows } = await runSearch(
    client,
    table,
    { filterBy: `id:=[${list}]`, q: "*", queryBy: "" },
    fields,
    ids.length,
    0,
    sortableId,
  );
  const byId = new Map(rows.map((row) => [String(row.id), row]));
  return ids.flatMap((id) => {
    const doc = byId.get(id);
    return doc ? [doc] : [];
  });
}

export async function loadAll(
  client: Typesense.Client,
  table: string,
  sortableId: string,
): Promise<Doc[]> {
  const collection = await client.collections(table).retrieve();
  const q = queryBy(collection.fields ?? [], sortableId);
  if (!q) return [];
  const rows: Doc[] = [];
  for (let page = 1; ; page++) {
    const result = await client
      .collections(table)
      .documents()
      .search({ q: "*", query_by: q, page, per_page: PAGE_SIZE });
    const hits = (result.hits ?? []) as Hits;
    rows.push(...hitDocs(hits, sortableId));
    if ((hits?.length ?? 0) < PAGE_SIZE) break;
  }
  return rows;
}
