import { loadConfig } from "../config.js";
import { openDb, refresh } from "../db/client.js";
import { sealPlaintextConfigs } from "../repositories/connections.js";
import { setKey } from "../utils/secretbox.js";

const arg = process.argv[2] ?? "migrate";
if (arg !== "migrate" && arg !== "refresh" && arg !== "") {
  console.error("usage: migrate [migrate|refresh]");
  process.exit(2);
}
const cfg = loadConfig();
setKey(process.env.APP_KEY ?? "");
const db = await openDb(cfg);
if (arg === "refresh") {
  await refresh(db);
  console.log("migrated (refresh): dropped all tables and recreated schema");
} else {
  await sealPlaintextConfigs(db);
  console.log("migrated: schema up to date");
}
await db.close();
