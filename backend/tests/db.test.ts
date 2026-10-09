import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { DatabaseSync } from "node:sqlite";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { getTableColumns } from "drizzle-orm";
import type { Config } from "../src/config.js";
import { openDb } from "../src/db/client.js";
import { connections, syncJobs } from "../src/db/schema.js";
import * as pgSchema from "../src/db/schema.js";
import * as sqliteSchema from "../src/db/schema.sqlite.js";
import { setKey } from "../src/utils/secretbox.js";

const tables = [
  "connections",
  "syncJobs",
  "syncJobRelations",
  "syncJobFields",
  "syncJobFieldValues",
  "syncJobRules",
  "syncLogs",
] as const;

test("sqlite schema uses the same columns as postgres", () => {
  for (const name of tables) {
    const pgColumns = Object.keys(getTableColumns(pgSchema[name])).sort();
    const sqliteColumns = Object.keys(
      getTableColumns(sqliteSchema[name]),
    ).sort();
    assert.deepEqual(sqliteColumns, pgColumns, name);
  }
});

test("sqlite transaction rolls back", async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "portico-drizzle-"));
  setKey("test-key");
  const db = await openDb(sqliteConfig(path.join(dir, "app.db")));
  try {
    const row = {
      name: "a",
      type: "sqlite",
      config: {},
      created_at: "2026-01-01T00:00:00.000Z",
      updated_at: "2026-01-01T00:00:00.000Z",
    };
    await db.insert(connections).values(row);
    await assert.rejects(
      db.transaction(async (tx) => {
        await tx.insert(connections).values({ ...row, name: "b" });
        throw new Error("nope");
      }),
      /nope/,
    );
    const left = await db.select().from(connections);
    assert.equal(left.length, 1);
    assert.equal(left[0].name, "a");
  } finally {
    await db.close();
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test("existing sqlite tables are kept", async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "portico-drizzle-"));
  const file = path.join(dir, "app.db");
  const sqlite = new DatabaseSync(file);
  const migration = fs
    .readdirSync(sqliteMigrations)
    .map((name) => path.join(sqliteMigrations, name, "migration.sql"))
    .find((sqlPath) => fs.existsSync(sqlPath));
  if (!migration) throw new Error("sqlite migration missing");
  for (const statement of fs
    .readFileSync(migration, "utf8")
    .split("--> statement-breakpoint")) {
    if (statement.trim()) sqlite.exec(statement);
  }
  sqlite.exec(
    `INSERT INTO connections (name, type, config, created_at, updated_at) VALUES ('old', 'sqlite', '{}', '2026-01-01T00:00:00.000Z', '2026-01-01T00:00:00.000Z')`,
  );
  sqlite.close();

  const db = await openDb(sqliteConfig(file));
  try {
    const [kept] = await db.select().from(connections);
    assert.equal(kept.name, "old");
    assert.deepEqual(kept.config, {});
    assert.deepEqual(await db.select().from(syncJobs), []);
  } finally {
    await db.close();
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

const sqliteMigrations = fileURLToPath(
  new URL("../src/db/migrations/sqlite", import.meta.url),
);

function sqliteConfig(file: string): Config {
  return {
    httpAddr: ":0",
    dbDriver: "sqlite",
    dbHost: "",
    dbPort: "",
    dbUser: "",
    dbPassword: "",
    dbName: file,
    dbSSLMode: "disable",
    openAIAPIKey: "",
    openAIBaseURL: "",
    openAIModel: "",
    envFile: "",
  };
}
