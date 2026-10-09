import type { Relation, SyncJob } from "../../../domain/types.js";
import type {
  Doc,
  SourceReader,
  TableSchema,
} from "../../../connectors/types.js";
import { groupBy, uniqueBy } from "../../../utils/collections.js";
import { applyFields } from "../fields.js";
import { pivotTable, primaryKey, resolveRelationKeys } from "./keys.js";
import { collectRelationDocs, orderRelations } from "./tree.js";

type SchemaOf = (table: string) => Promise<TableSchema>;

/** Loads every active relation into `docs` as nested objects / arrays. */
export async function enrichRelations(
  source: SourceReader,
  job: SyncJob,
  parentSchema: TableSchema,
  docs: Doc[],
): Promise<void> {
  const ordered = orderRelations(job.relations);
  const byId = new Map(ordered.map((relation) => [relation.id, relation]));
  const schemaOf = cachedSchemas(source, job.source_table, parentSchema);

  for (const relation of ordered) {
    const parent =
      relation.parent_id == null ? null : byId.get(relation.parent_id);
    if (relation.parent_id != null && !parent)
      throw new Error(
        `parent_id ${relation.parent_id} not found for relation ${JSON.stringify(relation.name)}`,
      );

    const parentTable = parent?.table ?? job.source_table;
    const targets = parent ? collectRelationDocs(docs, parent.name) : docs;
    const context = {
      source,
      parentTable,
      parentSchema: await schemaOf(parentTable),
      schemaOf,
    };
    await enrichOne(context, targets, relation);
    applyFields(collectRelationDocs(docs, relation.name), relation.fields);
  }
}

type Context = {
  source: SourceReader;
  parentTable: string;
  parentSchema: TableSchema;
  schemaOf: SchemaOf;
};

function cachedSchemas(
  source: SourceReader,
  table: string,
  schema: TableSchema,
): SchemaOf {
  const cache = new Map([[table, schema]]);
  return async (name) => {
    let found = cache.get(name);
    if (!found) {
      found = await source.schema(name);
      cache.set(name, found);
    }
    return found;
  };
}

function enrichOne(
  context: Context,
  docs: Doc[],
  relation: Relation,
): Promise<void> {
  switch (relation.type) {
    case "belongs_to_many":
      return enrichBelongsToMany(context, docs, relation);
    case "has_many":
      return enrichHas(context, docs, relation, true);
    case "has_one":
      return enrichHas(context, docs, relation, false);
    case "belongs_to":
      return enrichBelongsTo(context, docs, relation);
    default:
      throw new Error(
        `unsupported relation type ${JSON.stringify(relation.type)}`,
      );
  }
}

async function enrichBelongsToMany(
  { source, parentTable, parentSchema, schemaOf }: Context,
  docs: Doc[],
  input: Relation,
): Promise<void> {
  const relation = resolveRelationKeys(parentTable, input);
  const pivot = pivotTable(relation);
  if (!pivot)
    throw new Error(
      `relation ${JSON.stringify(relation.name)}: config.pivot_table is required`,
    );

  const parentKey = primaryKey(parentSchema);
  const ids = uniqueValues(docs, parentKey);
  docs.forEach((doc) => (doc[relation.name] = []));
  if (ids.length === 0) return;

  const pivotRows = await source.queryRows(
    pivot,
    [relation.foreign_key, relation.related_key],
    relation.foreign_key,
    ids,
  );
  const relatedIds = uniqueValues(pivotRows, relation.related_key);
  if (relatedIds.length === 0) return;

  const relatedKey = primaryKey(await schemaOf(relation.table));
  const relatedRows = await source.queryRows(
    relation.table,
    null,
    relatedKey,
    relatedIds,
  );
  const relatedById = indexBy(relatedRows, relatedKey);
  const linked = pivotRows.filter(
    (row) =>
      row[relation.foreign_key] != null &&
      relatedById.has(String(row[relation.related_key])),
  );
  const grouped = groupBy(linked, (row) => String(row[relation.foreign_key]));
  for (const doc of docs) {
    const rows = grouped.get(String(doc[parentKey]));
    if (rows)
      doc[relation.name] = rows.map((row) => ({
        ...relatedById.get(String(row[relation.related_key])),
      }));
  }
}

async function enrichHas(
  { source, parentTable, parentSchema }: Context,
  docs: Doc[],
  input: Relation,
  many: boolean,
): Promise<void> {
  const relation = resolveRelationKeys(parentTable, input);
  const localKey = relation.related_key || primaryKey(parentSchema);
  const ids = uniqueValues(docs, localKey);
  docs.forEach((doc) => (doc[relation.name] = many ? [] : null));
  if (ids.length === 0) return;

  const rows = await source.queryRows(
    relation.table,
    null,
    relation.foreign_key,
    ids,
  );
  const grouped = groupBy(
    rows.filter((row) => row[relation.foreign_key] != null),
    (row) => String(row[relation.foreign_key]),
  );
  for (const doc of docs) {
    const children = grouped.get(String(doc[localKey]));
    if (!children?.length) continue;
    const copies = children.map((row) => ({ ...row }));
    doc[relation.name] = many ? copies : copies[0];
  }
}

async function enrichBelongsTo(
  { source, schemaOf }: Context,
  docs: Doc[],
  input: Relation,
): Promise<void> {
  const relation = resolveRelationKeys("", input);
  const keepFk = relation.name === relation.foreign_key;
  const ids = uniqueValues(docs, relation.foreign_key);
  if (ids.length === 0) {
    if (!keepFk) docs.forEach((doc) => (doc[relation.name] = null));
    return;
  }

  const ownerKey =
    relation.related_key || primaryKey(await schemaOf(relation.table));
  const rows = await source.queryRows(relation.table, null, ownerKey, ids);
  const byId = indexBy(rows, ownerKey);
  for (const doc of docs) {
    const related = byId.get(String(doc[relation.foreign_key]));
    if (related) doc[relation.name] = { ...related };
    else if (!keepFk) doc[relation.name] = null;
  }
}

function uniqueValues(docs: Doc[], field: string): unknown[] {
  return uniqueBy(
    docs.filter((doc) => doc[field] != null),
    (doc) => String(doc[field]),
  ).map((doc) => doc[field]);
}

function indexBy(rows: Doc[], key: string): Map<string, Doc> {
  return new Map(
    rows
      .filter((row) => row[key] != null)
      .map((row) => [String(row[key]), row]),
  );
}
