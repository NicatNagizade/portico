import pg from "pg";
import mysql from "mysql2/promise";
import { DatabaseSync, type SQLInputValue } from "node:sqlite";
import type { Doc } from "../types.js";
import { forEachChunk } from "../../utils/collections.js";

export type SqlStyle = "pg" | "mysql" | "sqlite";

type OnChunk = (rows: Doc[]) => Promise<void>;

export interface SqlConn {
  style: SqlStyle;
  query(sql: string, params?: unknown[]): Promise<Doc[]>;
  exec(sql: string, params?: unknown[]): Promise<void>;
  /** Streams a SELECT in chunks so large tables are not loaded at once. */
  read(
    sql: string,
    params: unknown[],
    chunkSize: number,
    fn: OnChunk,
  ): Promise<void>;
  close(): Promise<void>;
}

export type ServerConfig = {
  host: string;
  port: number;
  user: string;
  password: string;
  database: string;
};

class PgConn implements SqlConn {
  style = "pg" as const;
  constructor(private pool: pg.Pool) {}

  async query(sql: string, params: unknown[] = []): Promise<Doc[]> {
    const res = await this.pool.query(sql, params);
    return (res.rows as Doc[]).map(normalizeRow);
  }

  async exec(sql: string, params: unknown[] = []): Promise<void> {
    await this.pool.query(sql, params);
  }

  async read(sql: string, params: unknown[], chunkSize: number, fn: OnChunk) {
    const size = chunkSize > 0 ? Math.trunc(chunkSize) : 500;
    const client = await this.pool.connect();
    try {
      await client.query("BEGIN");
      await client.query(
        `DECLARE portico_read NO SCROLL CURSOR FOR ${sql}`,
        params,
      );
      for (;;) {
        const res = await client.query<Doc>(`FETCH ${size} FROM portico_read`);
        if (res.rows.length === 0) break;
        await fn(res.rows.map(normalizeRow));
      }
      await client.query("COMMIT");
    } catch (error) {
      await client.query("ROLLBACK").catch(() => undefined);
      throw error;
    } finally {
      client.release();
    }
  }

  async close(): Promise<void> {
    await this.pool.end();
  }
}

class MysqlConn implements SqlConn {
  style = "mysql" as const;
  constructor(private pool: mysql.Pool) {}

  async query(sql: string, params: unknown[] = []): Promise<Doc[]> {
    const [rows] = await this.pool.query(sql, params);
    return (rows as Doc[]).map(normalizeRow);
  }

  async exec(sql: string, params: unknown[] = []): Promise<void> {
    await this.pool.query(sql, params);
  }

  async read(sql: string, params: unknown[], chunkSize: number, fn: OnChunk) {
    await forEachChunk(await this.query(sql, params), chunkSize, fn);
  }

  async close(): Promise<void> {
    await this.pool.end();
  }
}

class SqliteConn implements SqlConn {
  style = "sqlite" as const;
  constructor(private db: DatabaseSync) {}

  async query(sql: string, params: unknown[] = []): Promise<Doc[]> {
    const rows = this.db.prepare(sql).all(...(params as SQLInputValue[]));
    return (rows as Doc[]).map(normalizeRow);
  }

  async exec(sql: string, params: unknown[] = []): Promise<void> {
    this.db.prepare(sql).run(...(params as SQLInputValue[]));
  }

  async read(sql: string, params: unknown[], chunkSize: number, fn: OnChunk) {
    await forEachChunk(await this.query(sql, params), chunkSize, fn);
  }

  async close(): Promise<void> {
    this.db.close();
  }
}

export async function openPostgres(
  cfg: ServerConfig & { sslmode: string },
): Promise<SqlConn> {
  const pool = new pg.Pool({
    ...cfg,
    ssl: cfg.sslmode === "disable" ? false : { rejectUnauthorized: false },
  });
  await pool.query("SELECT 1");
  return new PgConn(pool);
}

export async function openMysql(cfg: ServerConfig): Promise<SqlConn> {
  const pool = mysql.createPool({
    ...cfg,
    supportBigNumbers: true,
    bigNumberStrings: false,
  });
  await pool.query("SELECT 1");
  return new MysqlConn(pool);
}

export async function openSqlite(file: string): Promise<SqlConn> {
  const db = new DatabaseSync(file);
  db.prepare("SELECT 1").get();
  return new SqliteConn(db);
}

/** Dates become ISO strings, JSON text becomes objects, bigints become numbers. */
function normalizeRow(row: Doc): Doc {
  for (const key of Object.keys(row)) {
    const value = row[key];
    const next = normalizeValue(value);
    if (next !== value) row[key] = next;
  }
  return row;
}

function normalizeValue(value: unknown): unknown {
  if (value == null) return null;
  if (value instanceof Date) return value.toISOString();
  if (typeof value === "bigint") return Number(value);
  const text = Buffer.isBuffer(value)
    ? value.toString("utf8")
    : typeof value === "string"
      ? value
      : null;
  if (text == null) return value;
  return text[0] === "{" || text[0] === "[" ? parseJSON(text) : text;
}

function parseJSON(text: string): unknown {
  try {
    return JSON.parse(text);
  } catch {
    return text;
  }
}
