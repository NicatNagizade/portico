import { loadConfig } from "../config.js";
import { defaultRegistry } from "../connectors/registry.js";
import { openDb } from "../db/client.js";
import { Orchestrator } from "../services/sync/orchestrator.js";
import { setKey } from "../utils/secretbox.js";

const raw = process.argv[2];
const id = Number(raw);
if (!raw || !Number.isInteger(id) || id <= 0) {
  console.error("usage: run-sync <sync-job-id>");
  process.exit(2);
}
const cfg = loadConfig();
setKey(process.env.APP_KEY ?? "");
const db = await openDb(cfg);
const ac = new AbortController();
process.on("SIGINT", () => ac.abort());
try {
  const sync = new Orchestrator(db, defaultRegistry());
  const { log, error } = await sync.run(id, ac.signal);
  console.log(
    `sync job ${id} log=${log.id} status=${log.status} rows=${log.rows_synced ?? 0}/${log.rows_total ?? 0} duration_ms=${log.duration_ms ?? 0}\n${log.message}`,
  );
  if (error) process.exitCode = 1;
} finally {
  await db.close();
}
