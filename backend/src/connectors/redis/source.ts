import type { Redis } from "ioredis";
import type { Connection } from "../../domain/types.js";
import type { SourceReader, TableSchema } from "../types.js";
import { readChunksInMemory, selectColumns } from "../shared/rows.js";
import { schemaFromDocs } from "../shared/schema.js";
import { CATALOG, openClient, parseConfig } from "./client.js";
import { scanKeys, TableKeys } from "./keys.js";
import { queryTable } from "./query.js";

const SCHEMA_SAMPLE = 50;

export function newRedisSource(conn: Connection): SourceReader {
  const cfg = parseConfig(conn);
  let client: Redis | null = null;
  const keys = (table: string) => new TableKeys(client!, cfg, table);

  return {
    async open() {
      client = await openClient(cfg);
    },
    async close() {
      await client?.quit();
      client = null;
    },
    /** Catalog entries plus any `{table}{sep}...` key prefix found by scanning. */
    async listTables() {
      const tables = new Set(await client!.smembers(CATALOG));
      await scanKeys(client!, "*", async (found) => {
        for (const key of found) {
          const idx = key.indexOf(cfg.sep);
          if (idx > 0 && !key.startsWith("__portico"))
            tables.add(key.slice(0, idx));
        }
      });
      return [...tables].sort();
    },
    async schema(table): Promise<TableSchema> {
      const t = keys(table);
      const docs = await t.read(await t.all(SCHEMA_SAMPLE));
      if (docs.length === 0)
        return { columns: [{ name: "id", type: "string", primaryKey: true }] };
      return schemaFromDocs(docs);
    },
    async count(table, filters) {
      const t = keys(table);
      const lookup = t.idLookup(filters);
      if (lookup) return (await t.read(lookup)).length;
      if (filters.length === 0) return t.count();
      return (await t.matching(filters)).length;
    },
    async readChunks(table, chunkSize, filters, fn) {
      const t = keys(table);
      const lookup = t.idLookup(filters);
      const docs = lookup ? await t.read(lookup) : await t.loadAll();
      return readChunksInMemory(docs, chunkSize, lookup ? [] : filters, fn);
    },
    async query(table, columns, filters, limit, offset, order) {
      const { rows } = await queryTable(
        keys(table),
        filters,
        limit,
        offset,
        order,
      );
      return selectColumns(rows, columns);
    },
    async queryRows(table, columns, whereColumn, whereValues) {
      if (whereValues.length === 0) return [];
      const t = keys(table);
      if (whereColumn.trim() === "id") {
        const ids = whereValues.map(String).filter((id) => id && id !== "null");
        return selectColumns(await t.read(ids.map((id) => t.key(id))), columns);
      }
      const want = new Set(whereValues.map(String));
      const docs = (await t.loadAll()).filter((doc) =>
        want.has(String(doc[whereColumn])),
      );
      return selectColumns(docs, columns);
    },
    supportsRelationFilters: () => false,
  };
}
