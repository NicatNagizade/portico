import { MongoClient } from "mongodb";
import type { Connection } from "../../domain/types.js";
import {
  configOf,
  optionalString,
  port,
  requiredString,
} from "../shared/config.js";

export type MongoConfig = {
  host: string;
  port: number;
  user: string;
  password: string;
  database: string;
  authSource: string;
};

export function parseConfig(conn: Connection): MongoConfig {
  const cfg = configOf(conn);
  return {
    host: requiredString(cfg, "host", "mongodb"),
    database: requiredString(cfg, "database", "mongodb"),
    port: port(cfg, 27017),
    user: optionalString(cfg, "user"),
    password: optionalString(cfg, "password"),
    authSource: optionalString(cfg, "auth_source", "admin"),
  };
}

function uri(cfg: MongoConfig): string {
  const auth = cfg.user
    ? `${encodeURIComponent(cfg.user)}:${encodeURIComponent(cfg.password)}@`
    : "";
  const query = new URLSearchParams({ directConnection: "true" });
  if (cfg.user && cfg.authSource) query.set("authSource", cfg.authSource);
  return `mongodb://${auth}${cfg.host}:${cfg.port}/${cfg.database}?${query}`;
}

export async function connect(cfg: MongoConfig): Promise<MongoClient> {
  const client = new MongoClient(uri(cfg), {
    serverSelectionTimeoutMS: 2000,
    connectTimeoutMS: 2000,
  });
  await client.connect();
  await client.db(cfg.database).command({ ping: 1 });
  return client;
}
