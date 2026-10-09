import type {
  DestinationWriter,
  FieldType,
  SourceReader,
  TableSchema,
} from "../types.js";
import type { SqlConn } from "./connection.js";
import {
  countRows,
  queryDestination,
  queryPage,
  queryRows,
  readFiltered,
  type SqlTable,
} from "./queries.js";
import type { Dialect } from "./where.js";
import { createTableSQL, insertRows } from "./write.js";

export type SqlSourceSpec = {
  dialect: Dialect;
  connect(): Promise<SqlConn>;
  listTables(db: SqlConn): Promise<string[]>;
  schema(db: SqlConn, table: string): Promise<TableSchema>;
};

export type SqlDestinationSpec = {
  dialect: Dialect;
  connect(): Promise<SqlConn>;
  columnType(type: FieldType): string;
};

/** Shared SourceReader for MySQL / Postgres / SQLite; only discovery differs. */
export function sqlSource(spec: SqlSourceSpec): SourceReader {
  let db: SqlConn | null = null;
  const at = (table: string): SqlTable => ({
    db: db!,
    dialect: spec.dialect,
    table: spec.dialect.quoteTable(table),
  });
  return {
    async open() {
      db = await spec.connect();
    },
    async close() {
      await db?.close();
      db = null;
    },
    listTables: () => spec.listTables(db!),
    async schema(table) {
      const schema = await spec.schema(db!, table);
      if (schema.columns.length === 0)
        throw new Error(
          `table ${JSON.stringify(table)} not found or has no columns`,
        );
      return schema;
    },
    count: (table, filters) => countRows(at(table), filters),
    readChunks: (table, chunkSize, filters, fn) =>
      readFiltered(at(table), chunkSize, filters, fn),
    query: (table, columns, filters, limit, offset, order) =>
      queryPage(at(table), columns, filters, limit, offset, order),
    queryRows: (table, columns, whereColumn, whereValues) =>
      queryRows(at(table), columns, whereColumn, whereValues),
    supportsRelationFilters: () => true,
  };
}

/** Shared DestinationWriter: drop + create the table, then multi-row INSERTs. */
export function sqlDestination(spec: SqlDestinationSpec): DestinationWriter {
  let db: SqlConn | null = null;
  let schema: TableSchema | null = null;
  const at = (name: string): SqlTable => ({
    db: db!,
    dialect: spec.dialect,
    table: spec.dialect.quoteTable(name),
  });
  return {
    async open() {
      db = await spec.connect();
    },
    async close() {
      await db?.close();
      db = null;
      schema = null;
    },
    async prepare(name, outSchema) {
      if (!outSchema.columns.length)
        throw new Error(
          `prepare ${JSON.stringify(name)}: schema has no columns`,
        );
      const table = at(name).table;
      await db!.exec(`DROP TABLE IF EXISTS ${table}`);
      await db!.exec(
        createTableSQL(table, outSchema, spec.dialect.quote, spec.columnType),
      );
      schema = outSchema;
    },
    writeBatch: (name, docs) => insertRows(at(name), docs, schema),
    query: (name, filters, limit, offset, order) =>
      queryDestination(at(name), filters, limit, offset, order),
  };
}
