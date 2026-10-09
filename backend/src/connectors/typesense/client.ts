import Typesense from "typesense";
import type { Connection } from "../../domain/types.js";
import {
  configOf,
  optionalString,
  port,
  requiredString,
} from "../shared/config.js";

export async function openClient(conn: Connection): Promise<Typesense.Client> {
  const cfg = configOf(conn);
  const client = new Typesense.Client({
    nodes: [
      {
        host: requiredString(cfg, "host", "typesense"),
        port: port(cfg, 8108),
        protocol: optionalString(cfg, "protocol", "http"),
      },
    ],
    apiKey: requiredString(cfg, "api_key", "typesense"),
    connectionTimeoutSeconds: 5,
  });
  await client.health.retrieve();
  return client;
}
