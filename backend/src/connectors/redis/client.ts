import { Redis } from "ioredis";
import type { Connection } from "../../domain/types.js";
import {
  configOf,
  optionalString,
  port,
  requiredString,
} from "../shared/config.js";

/** Set of table names written by Portico, so empty tables still list. */
export const CATALOG = "__portico:tables";

export type RedisConfig = {
  host: string;
  port: number;
  password: string;
  db: number;
  sep: string;
};

export function parseConfig(conn: Connection): RedisConfig {
  const cfg = configOf(conn);
  return {
    host: requiredString(cfg, "host", "redis"),
    port: port(cfg, 6379),
    password: optionalString(cfg, "password"),
    db: Number(cfg.db ?? 0) || 0,
    sep: optionalString(cfg, "key_separator", ":"),
  };
}

export function createClient(cfg: RedisConfig): Redis {
  return new Redis({
    host: cfg.host,
    port: cfg.port,
    password: cfg.password || undefined,
    db: cfg.db,
    lazyConnect: true,
    enableReadyCheck: false,
  });
}

/** Connected and pinged client; disconnected again if either step fails. */
export async function openClient(cfg: RedisConfig): Promise<Redis> {
  const client = createClient(cfg);
  try {
    await client.connect();
    await client.ping();
    return client;
  } catch (error) {
    client.disconnect();
    throw error;
  }
}
