import type Typesense from "typesense";
import type { Connection } from "../../domain/types.js";
import type { SourceReader, TableSchema } from "../types.js";
import { DEFAULT_SORTABLE_ID } from "../primaryKey.js";
import {
  queryInMemory,
  readChunksInMemory,
  selectColumns,
} from "../shared/rows.js";
import {
  mergeNestedFrom,
  nestDottedFields,
  schemaFromDocs,
} from "../shared/schema.js";
import { openClient } from "./client.js";
import { fromTypesenseType } from "./schema.js";
import {
  compileFilters,
  concreteFilters,
  loadAll,
  runSearch,
  scanExported,
  searchPage,
} from "./search.js";

const SORTABLE = DEFAULT_SORTABLE_ID;

export function newTypesenseSource(conn: Connection): SourceReader {
  let client: Typesense.Client | null = null;
  const all = (table: string) => loadAll(client!, table, SORTABLE);

  return {
    async open() {
      client = await openClient(conn);
    },
    async close() {
      client = null;
    },
    async listTables() {
      const collections = await client!.collections().retrieve();
      return collections.map((c) => c.name).filter(Boolean);
    },
    /** Declared fields, with nested object shapes filled in from a sample page. */
    async schema(table) {
      const collection = await client!.collections(table).retrieve();
      const declared = (collection.fields ?? [])
        .filter((f) => f.name && f.name !== "id" && f.name !== SORTABLE)
        .map((f) => ({ name: f.name, type: fromTypesenseType(f.type) }));
      const schema: TableSchema = {
        columns: [
          { name: "id", type: "string", primaryKey: true },
          ...nestDottedFields(declared),
        ],
      };
      const sample = await searchPage(client!, table, "", 50, 0, SORTABLE);
      if (!sample.rows.length) return schema;
      return mergeNestedFrom(schema, schemaFromDocs(sample.rows)) ?? schema;
    },
    async count(table, filters) {
      const collection = await client!.collections(table).retrieve();
      const fields = collection.fields ?? [];
      const prepared = await concreteFilters(
        client!,
        table,
        filters,
        fields,
        SORTABLE,
        collection.num_documents ?? 0,
      );
      if (prepared == null) return 0;
      const compiled = compileFilters(prepared, SORTABLE, fields);
      if (compiled == null)
        return (
          await scanExported(
            client!,
            table,
            prepared,
            fields,
            1,
            0,
            SORTABLE,
            null,
          )
        ).total;
      return (await runSearch(client!, table, compiled, fields, 1, 0, SORTABLE))
        .total;
    },
    async readChunks(table, chunkSize, filters, fn) {
      return readChunksInMemory(await all(table), chunkSize, filters, fn);
    },
    async query(table, columns, filters, limit, offset, order) {
      const collection = await client!.collections(table).retrieve();
      const fields = collection.fields ?? [];
      const prepared = await concreteFilters(
        client!,
        table,
        filters,
        fields,
        SORTABLE,
        collection.num_documents ?? 0,
      );
      if (prepared == null) return [];
      const compiled = compileFilters(prepared, SORTABLE, fields);
      if (compiled == null) {
        const { rows } = await scanExported(
          client!,
          table,
          prepared,
          fields,
          limit,
          offset,
          SORTABLE,
          order,
        );
        return selectColumns(rows, columns);
      }
      if (order) {
        const { rows } = queryInMemory(
          await all(table),
          filters,
          limit,
          offset,
          order,
        );
        return selectColumns(rows, columns);
      }
      const page = await runSearch(
        client!,
        table,
        compiled,
        fields,
        limit,
        offset,
        SORTABLE,
      );
      return selectColumns(page.rows, columns);
    },
    async queryRows(table, columns, whereColumn, whereValues) {
      if (!whereValues.length) return [];
      const want = new Set(whereValues.map(String));
      const docs = (await all(table)).filter((doc) =>
        want.has(String(doc[whereColumn])),
      );
      return selectColumns(docs, columns);
    },
    supportsRelationFilters: () => false,
  };
}
