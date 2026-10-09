import { loadConfig } from "../config.js";
import { openDb } from "../db/client.js";
import { importConnections } from "../services/importConnections.js";
import { setKey } from "../utils/secretbox.js";

const cfg = loadConfig();
setKey(process.env.APP_KEY ?? "");
const db = await openDb(cfg);
const path = process.env.CONNECTIONS_FILE || "connections.json";
try {
  const out = await importConnections(db, path);
  for (const row of out.connections)
    console.log(
      `${row.action} connection ${row.name} (${row.type}) id=${row.id}`,
    );
  for (const row of out.syncJobs)
    console.log(`${row.action} sync_job ${row.name} id=${row.id}`);
  console.log(
    `imported ${out.connections.length} connection(s), ${out.syncJobs.length} sync job(s) from ${path}`,
  );
} finally {
  await db.close();
}
