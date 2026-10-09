import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import type { AddressInfo } from "node:net";
import { DatabaseSync } from "node:sqlite";
import test from "node:test";
import { createApp } from "../src/app.js";
import { defaultRegistry } from "../src/connectors/registry.js";
import { openDb } from "../src/db/client.js";
import { setKey } from "../src/utils/secretbox.js";

function seedSource(file: string): void {
  const db = new DatabaseSync(file);
  db.exec(`
    CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT);
    CREATE TABLE posts (id INTEGER PRIMARY KEY, user_id INTEGER, title TEXT, status INTEGER);
    CREATE TABLE comments (id INTEGER PRIMARY KEY, post_id INTEGER, body TEXT);
    INSERT INTO users VALUES (1, 'ada'), (2, 'bea');
    INSERT INTO posts VALUES (1, 1, 'first', 1), (2, 2, 'second', 0), (3, 1, 'third', 1);
    INSERT INTO comments VALUES (1, 1, 'nice'), (2, 1, 'great'), (3, 3, 'ok');
  `);
  db.close();
}

test("api end to end on sqlite", async (t) => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "portico-express-"));
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }));
  seedSource(path.join(dir, "source.db"));
  setKey("test-key");

  const db = await openDb({
    httpAddr: ":0",
    dbDriver: "sqlite",
    dbHost: "",
    dbPort: "",
    dbUser: "",
    dbPassword: "",
    dbName: path.join(dir, "app.db"),
    dbSSLMode: "disable",
    openAIAPIKey: "",
    openAIBaseURL: "",
    openAIModel: "",
    envFile: "",
  });
  const server = createApp(db, defaultRegistry(), null).listen(0);
  t.after(async () => {
    server.close();
    await db.close();
  });
  const base = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  const call = async (method: string, url: string, body?: unknown) => {
    const res = await fetch(base + url, {
      method,
      headers: { "content-type": "application/json" },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    const text = await res.text();
    const type = res.headers.get("content-type") ?? "";
    return {
      status: res.status,
      body: type.includes("json") && text ? JSON.parse(text) : text,
    };
  };

  const health = await call("GET", "/health");
  assert.deepEqual(health.body, { status: "ok", ai_enabled: false });

  const src = await call("POST", "/connections", {
    name: "src",
    type: "sqlite",
    config: { path: path.join(dir, "source.db"), password: "hidden" },
  });
  assert.equal(src.status, 201);
  assert.equal(src.body.config.password, "");
  const dst = await call("POST", "/connections", {
    name: "dst",
    type: "sqlite",
    config: { path: path.join(dir, "dest.db") },
  });
  assert.equal(dst.status, 201);

  const tables = await call("GET", `/connections/${src.body.id}/tables`);
  assert.deepEqual(tables.body.tables, ["comments", "posts", "users"]);
  const columns = await call(
    "GET",
    `/connections/${src.body.id}/columns?table=posts`,
  );
  assert.deepEqual(
    columns.body.columns.map((c: { name: string }) => c.name),
    ["id", "user_id", "title", "status"],
  );

  const created = await call("POST", "/sync-jobs", {
    name: "posts sync",
    source_connection_id: src.body.id,
    source_table: "posts",
    destination_connection_id: dst.body.id,
    destination_table: "posts_out",
    chunk_size: 2,
    workers: 2,
    relations: [
      { name: "user", type: "belongs_to", table: "users" },
      { name: "comments", type: "has_many", table: "comments" },
    ],
    fields: [
      { source_name: "title", destination_name: "headline" },
      {
        source_name: "status",
        destination_type: "string",
        values: [{ source_value: "1", destination_value: "published" }],
      },
    ],
    rules: [{ field: "id", operator: "gt", value: "0" }],
  });
  assert.equal(created.status, 201);
  const job = created.body;
  assert.equal(job.relations.length, 2);
  assert.equal(job.fields.length, 2);
  assert.equal(job.fields[1].values[0].destination_value, "published");
  assert.equal(job.rules[0].operator, "gt");

  const invalid = await call("POST", "/sync-jobs", {
    ...job,
    id: undefined,
    relations: [{ name: "x", type: "nope", table: "users" }],
  });
  assert.equal(invalid.status, 400);

  const listed = await call("GET", "/sync-jobs?page=1&page_size=10");
  assert.equal(listed.body.total, 1);
  assert.equal(listed.body.items[0].source_connection.name, "src");

  const run = await call("POST", `/sync-jobs/${job.id}/run`);
  assert.equal(run.status, 200);
  assert.equal(run.body.status, "success");
  assert.equal(run.body.rows_total, 3);
  assert.equal(run.body.rows_synced, 3);

  const destination = await call("POST", `/sync-jobs/${job.id}/explore`, {
    side: "destination",
    sort_by: "id",
    sort_dir: "asc",
  });
  assert.equal(destination.status, 200);
  assert.equal(destination.body.total, 3);
  const first = destination.body.rows[0];
  assert.equal(first.headline, "first");
  assert.equal(first.status, "published");
  assert.equal(first.user.name, "ada");
  assert.equal(first.comments.length, 2);
  assert.equal(destination.body.rows[1].status, "0");

  const nested = await call("POST", `/sync-jobs/${job.id}/explore`, {
    side: "source",
    filters: [{ field: "user.name", operator: "eq", value: "ada" }],
  });
  assert.equal(nested.status, 200, JSON.stringify(nested.body));
  assert.equal(nested.body.total, 2);
  assert.deepEqual(
    nested.body.rows.map((r: { headline: string }) => r.headline),
    ["first", "third"],
  );

  const badSort = await call("POST", `/sync-jobs/${job.id}/explore`, {
    side: "source",
    sort_by: "comments",
  });
  assert.equal(badSort.status, 400);

  const csv = await call("POST", `/sync-jobs/${job.id}/explore/export`, {
    side: "source",
    fields: ["id", "headline"],
  });
  assert.equal(csv.status, 200);
  assert.equal(csv.body, "id,headline\n1,first\n2,second\n3,third\n");

  const logs = await call("GET", `/sync-logs?sync_job_id=${job.id}`);
  assert.equal(logs.body.total, 1);
  assert.equal(logs.body.items[0].sync_job.name, "posts sync");
  const byStatus = await call(
    "GET",
    `/sync-logs?sync_job_id=${job.id}&status=success`,
  );
  assert.equal(byStatus.body.total, 1);
  assert.equal(
    (await call("GET", `/sync-logs?sync_job_id=${job.id}&status=failed`)).body
      .total,
    0,
  );
  assert.equal((await call("GET", "/sync-logs?status=nope")).status, 400);
  assert.equal(
    (await call("GET", "/sync-logs?from=2000-01-01T00:00:00.000Z")).body.total,
    1,
  );
  assert.equal(
    (await call("GET", "/sync-logs?from=2999-01-01T00:00:00.000Z")).body.total,
    0,
  );
  assert.equal((await call("GET", "/sync-logs?from=yesterday")).status, 400);

  const updated = await call("PUT", `/sync-jobs/${job.id}`, {
    name: "renamed",
    rules: [],
  });
  assert.equal(updated.status, 200);
  assert.equal(updated.body.name, "renamed");
  assert.equal(updated.body.rules, undefined);
  assert.equal(updated.body.fields.length, 2);
  assert.equal(updated.body.relations.length, 2);

  assert.equal((await call("DELETE", `/sync-jobs/${job.id}`)).status, 204);
  assert.equal((await call("GET", `/sync-jobs/${job.id}`)).status, 404);
  assert.equal(
    (await call("DELETE", `/connections/${dst.body.id}`)).status,
    204,
  );
});
