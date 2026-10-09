import type { Connection } from "../../domain/types.js";
import type { DestinationWriter, SourceReader } from "../types.js";
import { configOf, requiredString } from "../shared/config.js";
import { sqlDestination, sqlSource } from "./base.js";
import { openSqlite } from "./connection.js";
import { fieldTypeFromSqlite, sqliteColumnType } from "./types.js";
import { quoteIdent, type Dialect } from "./where.js";

const quote = (name: string) => quoteIdent(name, `"`);
const dialect: Dialect = { style: "sqlite", quote, quoteTable: quote };

function filePath(conn: Connection): string {
  return requiredString(configOf(conn), "path", "sqlite");
}

export function newSqliteSource(conn: Connection): SourceReader {
  const path = filePath(conn);
  return sqlSource({
    dialect,
    connect: () => openSqlite(path),
    async listTables(db) {
      const rows = await db.query(
        `SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`,
      );
      return rows.map((r) => String(r.name));
    },
    async schema(db, table) {
      const rows = await db.query(`PRAGMA table_info(${quote(table)})`);
      return {
        columns: rows.map((r) => ({
          name: String(r.name),
          type: fieldTypeFromSqlite(String(r.type ?? "")),
          primaryKey: Number(r.pk) > 0,
        })),
      };
    },
  });
}

export function newSqliteDestination(conn: Connection): DestinationWriter {
  const path = filePath(conn);
  return sqlDestination({
    dialect,
    connect: () => openSqlite(path),
    columnType: sqliteColumnType,
  });
}
