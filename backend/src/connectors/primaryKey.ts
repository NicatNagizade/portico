import type { ColumnSchema, TableSchema } from "./types.js";

export const DEFAULT_SORTABLE_ID = "id_int";

export type PrimaryKeyConfig = {
  source: string[];
  destination: string;
  intField: string;
  configured: boolean;
};

export function parsePrimaryKeyConfig(raw: unknown): PrimaryKeyConfig {
  const out: PrimaryKeyConfig = {
    source: [],
    destination: "id",
    intField: DEFAULT_SORTABLE_ID,
    configured: false,
  };
  if (raw == null) return out;
  const top =
    typeof raw === "string"
      ? (JSON.parse(raw) as Record<string, unknown>)
      : (raw as Record<string, unknown>);
  if (!top || typeof top !== "object")
    throw new Error("parse sync job config: invalid json");

  let legacyInt = "";
  if ("primary_key_int" in top) {
    if (typeof top.primary_key_int !== "string")
      throw new Error("parse primary_key_int");
    legacyInt = top.primary_key_int.trim();
  }
  if (!("primary_key" in top) || top.primary_key == null) {
    if (legacyInt) out.intField = legacyInt;
    return out;
  }

  out.configured = true;
  out.intField = legacyInt;
  const pk = top.primary_key;
  if (typeof pk === "string") {
    if (pk.trim()) out.source = [pk.trim()];
  } else if (Array.isArray(pk)) {
    out.source = pk.map((v) => String(v).trim()).filter(Boolean);
  } else if (pk && typeof pk === "object") {
    const obj = pk as Record<string, unknown>;
    out.source = parseSourceList(obj.source);
    if (typeof obj.destination === "string" && obj.destination.trim())
      out.destination = obj.destination.trim();
    if ("int" in obj)
      out.intField = obj.int == null ? "" : String(obj.int).trim();
  } else {
    throw new Error("parse primary_key: expected string, array, or object");
  }
  if (!out.destination) out.destination = "id";
  return out;
}

/**
 * Numeric copy of `id` that document destinations (Mongo, Typesense) store for
 * range filters and sorting. Empty string disables it.
 */
export function sortableIdField(config: unknown): string {
  const pk = parsePrimaryKeyConfig(config ?? {});
  if (pk.configured) return pk.intField;
  return pk.intField || DEFAULT_SORTABLE_ID;
}

function parseSourceList(raw: unknown): string[] {
  if (raw == null) return [];
  if (typeof raw === "string") return raw.trim() ? [raw.trim()] : [];
  if (Array.isArray(raw))
    return raw.map((v) => String(v).trim()).filter(Boolean);
  throw new Error("parse primary_key.source");
}

export function applyPrimaryKeyOverride(
  schema: TableSchema | null,
  cfg: PrimaryKeyConfig,
): TableSchema | null {
  if (!cfg.configured || cfg.source.length === 0 || !schema) return schema;
  const want = new Set(cfg.source);
  let found = 0;
  const columns = schema.columns.map((col) => {
    const primaryKey = want.has(col.name);
    if (primaryKey) found++;
    return { ...col, primaryKey };
  });
  if (found !== want.size) {
    const have = new Set(
      columns.filter((c) => c.primaryKey).map((c) => c.name),
    );
    const missing = cfg.source.filter((name) => !have.has(name));
    throw new Error(
      `primary_key source ${JSON.stringify(missing)} not found in source schema`,
    );
  }
  return { columns };
}

export function applyDestinationPrimaryKey(
  schema: TableSchema | null,
  cfg: PrimaryKeyConfig,
): TableSchema | null {
  if (!cfg.configured || !schema) return schema;
  const dest = cfg.destination.trim() || "id";
  let found = false;
  const columns: ColumnSchema[] = schema.columns.map((col) => {
    const primaryKey = col.name === dest;
    if (primaryKey) found = true;
    return { ...col, primaryKey };
  });
  if (!found) columns.unshift({ name: dest, type: "string", primaryKey: true });
  return { columns };
}
