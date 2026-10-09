import type { Connection } from "../../domain/types.js";
import type { DestinationWriter, SourceReader } from "../types.js";
import {
  configOf,
  optionalString,
  port,
  requiredString,
} from "../shared/config.js";
import { sqlDestination, sqlSource } from "./base.js";
import { openMysql } from "./connection.js";
import { fieldTypeFromSql, mysqlColumnType } from "./types.js";
import { quoteIdent, type Dialect } from "./where.js";

const quote = (name: string) => quoteIdent(name, "`");
const dialect: Dialect = { style: "mysql", quote, quoteTable: quote };

function parseConfig(conn: Connection) {
  const cfg = configOf(conn);
  return {
    host: requiredString(cfg, "host", "mysql"),
    port: port(cfg, 3306),
    user: optionalString(cfg, "user"),
    password: optionalString(cfg, "password"),
    database: requiredString(cfg, "database", "mysql"),
  };
}

export function newMysqlSource(conn: Connection): SourceReader {
  const cfg = parseConfig(conn);
  return sqlSource({
    dialect,
    connect: () => openMysql(cfg),
    async listTables(db) {
      const rows = await db.query(
        `SELECT TABLE_NAME AS name FROM information_schema.TABLES
         WHERE TABLE_SCHEMA = ? AND TABLE_TYPE = 'BASE TABLE' ORDER BY TABLE_NAME`,
        [cfg.database],
      );
      return rows.map((r) => String(r.name ?? r.TABLE_NAME));
    },
    async schema(db, table) {
      const rows = await db.query(
        `SELECT COLUMN_NAME AS name, DATA_TYPE AS data_type, COLUMN_KEY AS column_key
         FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
         ORDER BY ORDINAL_POSITION`,
        [cfg.database, table],
      );
      return {
        columns: rows.map((r) => ({
          name: String(r.name),
          type: fieldTypeFromSql(String(r.data_type)),
          primaryKey: String(r.column_key) === "PRI",
        })),
      };
    },
  });
}

export function newMysqlDestination(conn: Connection): DestinationWriter {
  const cfg = parseConfig(conn);
  return sqlDestination({
    dialect,
    connect: () => openMysql(cfg),
    columnType: mysqlColumnType,
  });
}
