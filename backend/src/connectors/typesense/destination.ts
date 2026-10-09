import type Typesense from "typesense";
import type { Connection } from "../../domain/types.js";
import type { DestinationWriter, Doc, Order } from "../types.js";
import { DEFAULT_SORTABLE_ID, sortableIdField } from "../primaryKey.js";
import { sortColumn } from "../shared/filters.js";
import { sortRows } from "../shared/rows.js";
import { parseInt64 } from "../../utils/values.js";
import { openClient } from "./client.js";
import { collectionSchema, isSortable } from "./schema.js";
import {
  compileFilters,
  concreteFilters,
  runSearch,
  scanExported,
} from "./search.js";

type ImportResult = { success?: boolean; error?: string };

export function newTypesenseDestination(conn: Connection): DestinationWriter {
  let client: Typesense.Client | null = null;
  let sortField = DEFAULT_SORTABLE_ID;

  return {
    async open() {
      client = await openClient(conn);
    },
    async close() {
      client = null;
    },
    async applyJobConfig(config) {
      sortField = sortableIdField(config);
    },
    async prepare(name, schema, config) {
      await client!
        .collections(name)
        .delete()
        .catch(() => undefined);
      sortField = sortableIdField(config);
      const body = collectionSchema(name, schema, config, sortField);
      await client!.collections().create(body as never);
    },
    async writeBatch(name, docs) {
      if (!docs.length) return;
      const payload = docs.map((doc) => withSortableId(doc, sortField));
      const results = await client!
        .collections(name)
        .documents()
        .import(payload, { action: "create" });
      const failed = importResults(results).find((r) => r?.success === false);
      if (failed)
        throw new Error(
          `typesense import document failed: ${failed.error || "unknown error"}`,
        );
    },
    async query(name, filters, limit, offset, order) {
      const collection = await client!.collections(name).retrieve();
      const fields = collection.fields ?? [];
      const prepared = await concreteFilters(
        client!,
        name,
        filters,
        fields,
        sortField,
        collection.num_documents ?? 0,
      );
      if (prepared == null) return { rows: [], total: 0 };
      const compiled = compileFilters(prepared, sortField, fields);
      if (compiled == null)
        return scanExported(
          client!,
          name,
          prepared,
          fields,
          limit,
          offset,
          sortField,
          order,
        );

      const extra: Record<string, unknown> = {};
      let localOrder: Order | null = null;
      if (order?.column.trim()) {
        const column = sortColumn(order.column, sortField);
        if (isSortable(fields.find((f) => f.name === column)))
          extra.sort_by = `${column}:${order.desc ? "desc" : "asc"}`;
        else localOrder = order;
      }
      const { rows, total } = await runSearch(
        client!,
        name,
        compiled,
        fields,
        limit,
        offset,
        sortField,
        extra,
      );
      sortRows(rows, localOrder);
      return { rows, total };
    },
  };
}

function withSortableId(doc: Doc, sortField: string): Doc {
  if (!sortField) return doc;
  const n = parseInt64(doc.id);
  return n == null ? doc : { ...doc, [sortField]: n };
}

/** The client returns parsed results or raw JSONL depending on the call. */
function importResults(results: unknown): ImportResult[] {
  if (Array.isArray(results)) return results as ImportResult[];
  return String(results)
    .split("\n")
    .filter(Boolean)
    .map((line) => JSON.parse(line) as ImportResult);
}
