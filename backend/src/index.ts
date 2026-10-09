import { createApp } from "./app.js";
import { listenHostPort, loadConfig } from "./config.js";
import { defaultRegistry } from "./connectors/registry.js";
import { openDb } from "./db/client.js";
import { sealPlaintextConfigs } from "./repositories/connections.js";
import { newClient } from "./services/llm.js";
import { setKey } from "./utils/secretbox.js";

const cfg = loadConfig();
setKey(process.env.APP_KEY ?? "");
const db = await openDb(cfg);
await sealPlaintextConfigs(db);
const app = createApp(
  db,
  defaultRegistry(),
  newClient(cfg.openAIAPIKey, cfg.openAIBaseURL, cfg.openAIModel),
);
const { host, port } = listenHostPort(cfg.httpAddr);
app.listen(port, host, () => {
  console.log(
    `express api listening on ${cfg.httpAddr}${cfg.envFile ? ` (env ${cfg.envFile})` : ""}`,
  );
});
