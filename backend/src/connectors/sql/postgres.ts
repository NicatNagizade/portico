import type { Connection } from "../../domain/types.js";
import type { DestinationWriter, SourceReader } from "../types.js";
import {
  configOf,
  optionalString,
  port,
  requiredString,
} from "../shared/config.js";
import { sqlDestination, sqlSource } from "./base.js";
import { openPostgres } from "./connection.js";
import { fieldTypeFromSql, postgresColumnType } from "./types.js";
import { quoteIdent, type Dialect } from "./where.js";

const quote = (name: string) => quoteIdent(name, `"`);

const COLUMNS_SQL = `
  SELECT DISTINCT ON (c.ordinal_position) c.column_name AS name, c.data_type,
    EXISTS (
      SELECT 1 FROM information_schema.table_constraints tc
      JOIN information_schema.key_column_usage kcu
        ON tc.constraint_name = kcu.constraint_name AND tc.table_schema = kcu.table_schema
      WHERE tc.constraint_type = 'PRIMARY KEY'
        AND tc.table_schema = c.table_schema AND tc.table_name = c.table_name
        AND kcu.column_name = c.column_name
    ) AS is_pk
  FROM information_schema.columns c
  WHERE c.table_schema = $1 AND c.table_name = $2
  ORDER BY c.ordinal_position`;

function parseConfig(conn: Connection) {
  const cfg = configOf(conn);
  return {
    host: requiredString(cfg, "host", "postgres"),
    port: port(cfg, 5432),
    user: optionalString(cfg, "user"),
    password: optionalString(cfg, "password"),
    database: requiredString(cfg, "database", "postgres"),
    sslmode: optionalString(cfg, "sslmode", "disable"),
    schema: optionalString(cfg, "schema", "public"),
  };
}

function dialectFor(schema: string): Dialect {
  return {
    style: "pg",
    quote,
    quoteTable: (name) => `${quote(schema)}.${quote(name)}`,
  };
}

export function newPostgresSource(conn: Connection): SourceReader {
  const { schema: pgSchema, ...cfg } = parseConfig(conn);
  return sqlSource({
    dialect: dialectFor(pgSchema),
    connect: () => openPostgres(cfg),
    async listTables(db) {
      const rows = await db.query(
        `SELECT table_name AS name FROM information_schema.tables
         WHERE table_schema = $1 AND table_type = 'BASE TABLE' ORDER BY table_name`,
        [pgSchema],
      );
      return rows.map((r) => String(r.name));
    },
    async schema(db, table) {
      const rows = await db.query(COLUMNS_SQL, [pgSchema, table]);
      return {
        columns: rows.map((r) => ({
          name: String(r.name),
          type: fieldTypeFromSql(String(r.data_type)),
          primaryKey: r.is_pk === true || r.is_pk === "t" || r.is_pk === 1,
        })),
      };
    },
  });
}

export function newPostgresDestination(conn: Connection): DestinationWriter {
  const { schema: pgSchema, ...cfg } = parseConfig(conn);
  return sqlDestination({
    dialect: dialectFor(pgSchema),
    connect: () => openPostgres(cfg),
    columnType: postgresColumnType,
  });
}
