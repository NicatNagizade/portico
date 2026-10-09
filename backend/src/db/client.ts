import { readMigrationFiles } from "drizzle-orm/migrator";
import {
  drizzle as drizzlePg,
  type NodePgDatabase,
} from "drizzle-orm/node-postgres";
import { migrate as migratePg } from "drizzle-orm/node-postgres/migrator";
import {
  drizzle as drizzleSqlite,
  type NodeSQLiteDatabase,
} from "drizzle-orm/node-sqlite";
import { migrate as migrateSqlite } from "drizzle-orm/node-sqlite/migrator";
import { DatabaseSync, type SQLInputValue } from "node:sqlite";
import { fileURLToPath } from "node:url";
import pg from "pg";
import type { Config } from "../config.js";
import { APP_TABLES } from "./schema.js";

export type Row = Record<string, any>;

type Query = Pick<NodePgDatabase, "select" | "insert" | "update" | "delete">;

export type Db = Query & {
  driver: "postgres" | "sqlite";
  transaction<T>(fn: (tx: Db) => Promise<T>): Promise<T>;
  close(): Promise<void>;
};

type Driver = Db["driver"];
type Orm = NodePgDatabase | NodeSQLiteDatabase;

type State = {
  driver: Driver;
  orm: Orm;
  exec(sql: string, params?: unknown[]): Promise<void>;
  query(sql: string, params?: unknown[]): Promise<Row[]>;
};

const stateOf = new WeakMap<Db, State>();

for (const oid of [20, 21, 23, 1700])
  pg.types.setTypeParser(oid, (value) => Number(value));

export async function openDb(cfg: Config): Promise<Db> {
  const { db, state } = await connect(cfg);
  stateOf.set(db, state);
  await applyMigrations(state);
  return db;
}

/** Drops the app tables and applies the migrations again. */
export async function refresh(db: Db): Promise<void> {
  const state = stateOf.get(db);
  if (!state) throw new Error("refresh needs the root database");
  for (const table of APP_TABLES)
    await state.exec(`DROP TABLE IF EXISTS ${quote(table)}`);
  if (state.driver === "postgres")
    await state.exec(`DROP SCHEMA IF EXISTS drizzle CASCADE`);
  else await state.exec(`DROP TABLE IF EXISTS __drizzle_migrations`);
  await applyMigrations(state);
}

function connect(cfg: Config): Promise<{ db: Db; state: State }> {
  if (cfg.dbDriver === "sqlite") return connectSqlite(cfg.dbName);
  if (cfg.dbDriver !== "postgres")
    throw new Error(`unsupported db driver: ${cfg.dbDriver}`);
  return connectPostgres(cfg);
}

async function connectPostgres(cfg: Config) {
  await ensurePostgresDatabase(cfg);
  const pool = new pg.Pool(pgOptions(cfg, cfg.dbName));
  const orm = drizzlePg({ client: pool });
  const state: State = {
    driver: "postgres",
    orm,
    exec: async (statement, params = []) => {
      await pool.query(statement, params);
    },
    query: async (statement, params = []) =>
      (await pool.query(statement, params)).rows as Row[],
  };
  return {
    state,
    db: wrap(orm, "postgres", async () => {
      await pool.end();
    }),
  };
}

function connectSqlite(file: string): Promise<{ db: Db; state: State }> {
  const sqlite = new DatabaseSync(file);
  const orm = drizzleSqlite({ client: sqlite });
  const state: State = {
    driver: "sqlite",
    orm,
    exec: async (statement, params = []) => {
      sqlite.prepare(statement).run(...(params as SQLInputValue[]));
    },
    query: async (statement, params = []) =>
      sqlite.prepare(statement).all(...(params as SQLInputValue[])) as Row[],
  };
  return Promise.resolve({
    state,
    db: wrap(
      orm,
      "sqlite",
      async () => {
        sqlite.close();
      },
      sqlite,
    ),
  });
}

function wrap(
  orm: Orm,
  driver: Driver,
  close: () => Promise<void>,
  sqlite?: DatabaseSync,
): Db {
  const query = orm as Query;
  const db: Db = {
    driver,
    select: query.select.bind(query),
    insert: query.insert.bind(query),
    update: query.update.bind(query),
    delete: query.delete.bind(query),
    async transaction(fn) {
      if (sqlite) return sqliteTransaction(sqlite, db, fn);
      return (orm as NodePgDatabase).transaction(async (tx) =>
        fn(wrap(tx, driver, async () => {})),
      );
    },
    close,
  };
  return db;
}

/** node:sqlite's Drizzle transaction does not await async callbacks. */
async function sqliteTransaction<T>(
  sqlite: DatabaseSync,
  db: Db,
  fn: (tx: Db) => Promise<T>,
): Promise<T> {
  sqlite.exec("BEGIN");
  try {
    const out = await fn(db);
    sqlite.exec("COMMIT");
    return out;
  } catch (error) {
    sqlite.exec("ROLLBACK");
    throw error;
  }
}

async function applyMigrations(state: State): Promise<void> {
  const folder = migrationsDir(state.driver);
  await baseline(state, folder);
  if (state.driver === "postgres")
    await migratePg(state.orm as NodePgDatabase, { migrationsFolder: folder });
  else
    migrateSqlite(state.orm as NodeSQLiteDatabase, {
      migrationsFolder: folder,
    });
}

/**
 * App tables may already exist without a drizzle migration record.
 * Record the current migrations as applied instead of running CREATE TABLE again.
 */
async function baseline(state: State, folder: string): Promise<void> {
  if (!(await appTablesExist(state)) || (await migrationsExist(state))) return;
  const migrations = readMigrationFiles({ migrationsFolder: folder });
  if (state.driver === "postgres") {
    await state.exec(`CREATE SCHEMA IF NOT EXISTS drizzle`);
    await state.exec(`CREATE TABLE drizzle.__drizzle_migrations (
      id SERIAL PRIMARY KEY,
      hash text NOT NULL,
      created_at bigint,
      name text,
      applied_at timestamp with time zone DEFAULT now()
    )`);
    for (const migration of migrations)
      await state.exec(
        `INSERT INTO drizzle.__drizzle_migrations (hash, created_at, name) VALUES ($1, $2, $3)`,
        [migration.hash, migration.folderMillis, migration.name],
      );
    return;
  }
  await state.exec(`CREATE TABLE __drizzle_migrations (
    id INTEGER PRIMARY KEY,
    hash text NOT NULL,
    created_at numeric,
    name text,
    applied_at TEXT
  )`);
  for (const migration of migrations)
    await state.exec(
      `INSERT INTO __drizzle_migrations (hash, created_at, name, applied_at) VALUES (?, ?, ?, ?)`,
      [migration.hash, migration.folderMillis, migration.name, nowStamp()],
    );
}

/** True only when every app table is already there, as on a Go-created database. */
async function appTablesExist(state: State): Promise<boolean> {
  if (state.driver === "sqlite") {
    const placeholders = APP_TABLES.map(() => "?").join(", ");
    const rows = await state.query(
      `SELECT name FROM sqlite_master WHERE type = 'table' AND name IN (${placeholders})`,
      [...APP_TABLES],
    );
    return rows.length === APP_TABLES.length;
  }
  const names = APP_TABLES.map((name) => `'${name}'`).join(", ");
  const [row] = await state.query(
    `SELECT count(*) AS n FROM pg_tables WHERE schemaname = 'public' AND tablename IN (${names})`,
  );
  return Number(row?.n) === APP_TABLES.length;
}

async function migrationsExist(state: State): Promise<boolean> {
  if (state.driver === "sqlite")
    return tableExists(state, "__drizzle_migrations");
  const [row] = await state.query(
    `SELECT to_regclass('drizzle.__drizzle_migrations') AS rel`,
  );
  return row?.rel != null;
}

async function tableExists(state: State, name: string): Promise<boolean> {
  const rows = await state.query(
    `SELECT 1 AS ok FROM sqlite_master WHERE type = 'table' AND name = ?`,
    [name],
  );
  return rows.length > 0;
}

function migrationsDir(driver: Driver): string {
  return fileURLToPath(new URL(`./migrations/${driver}`, import.meta.url));
}

function quote(name: string): string {
  return `"${name}"`;
}

function nowStamp(): string {
  return new Date().toISOString();
}

function pgOptions(cfg: Config, database: string): pg.ClientConfig {
  return {
    host: cfg.dbHost,
    port: Number(cfg.dbPort),
    user: cfg.dbUser,
    password: cfg.dbPassword,
    database,
    ssl: cfg.dbSSLMode === "disable" ? false : { rejectUnauthorized: false },
  };
}

async function ensurePostgresDatabase(cfg: Config): Promise<void> {
  if (!/^[a-zA-Z_][a-zA-Z0-9_]*$/.test(cfg.dbName))
    throw new Error(`invalid database name ${JSON.stringify(cfg.dbName)}`);
  const admin = new pg.Client(pgOptions(cfg, "template1"));
  await admin.connect();
  try {
    const exists = await admin.query(
      "SELECT 1 FROM pg_database WHERE datname = $1",
      [cfg.dbName],
    );
    if (exists.rowCount === 0)
      await admin.query(`CREATE DATABASE "${cfg.dbName}"`);
  } finally {
    await admin.end();
  }
}
