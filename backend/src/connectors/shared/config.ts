import type { Connection } from "../../domain/types.js";
import { asObject } from "../../utils/values.js";

export type RawConfig = Record<string, unknown>;

export function configOf(conn: Connection): RawConfig {
  return asObject(conn.config) ?? {};
}

export function requiredString(
  cfg: RawConfig,
  key: string,
  label: string,
): string {
  const value = cfg[key];
  if (value == null || String(value).trim() === "")
    throw new Error(`parse ${label} config: ${key} is required`);
  return String(value);
}

export function optionalString(
  cfg: RawConfig,
  key: string,
  fallback = "",
): string {
  return String(cfg[key] ?? fallback) || fallback;
}

export function port(cfg: RawConfig, fallback: number): number {
  return Number(cfg.port ?? fallback) || fallback;
}
