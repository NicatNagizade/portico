import type { Doc, FieldType, TableSchema } from "../types.js";
import type { SqlTable } from "./queries.js";
import { Binder, type Quote } from "./where.js";

export function createTableSQL(
  table: string,
  schema: TableSchema,
  quote: Quote,
  columnType: (type: FieldType) => string,
): string {
  const columns = schema.columns.filter((c) => c.name.trim());
  if (!columns.length) throw new Error("schema has no valid columns");
  const defs = columns.map((c) => `${quote(c.name)} ${columnType(c.type)}`);
  const keys = columns.filter((c) => c.primaryKey).map((c) => quote(c.name));
  if (keys.length) defs.push(`PRIMARY KEY (${keys.join(", ")})`);
  return `CREATE TABLE ${table} (${defs.join(", ")})`;
}

/** Keeps schema columns only and serializes nested values as JSON text. */
export function rowForWrite(doc: Doc, schema: TableSchema | null): Doc {
  const columns = schema?.columns.length ? schema.columns : null;
  const types = new Map(columns?.map((c) => [c.name, c.type]));
  const keys = columns
    ? columns.map((c) => c.name).filter((name) => name in doc)
    : Object.keys(doc);
  const out: Doc = {};
  for (const key of keys) {
    const value = doc[key];
    const type = types.get(key);
    const isJson =
      type === "object" ||
      type === "object_array" ||
      (!columns && value && typeof value === "object");
    if (isJson)
      out[key] =
        value == null || typeof value === "string"
          ? value
          : JSON.stringify(value);
    else out[key] = value instanceof Date ? value.toISOString() : value;
  }
  return out;
}

export async function insertRows(
  t: SqlTable,
  docs: Doc[],
  schema: TableSchema | null,
): Promise<void> {
  const rows = docs.map((doc) => rowForWrite(doc, schema));
  const keys = Object.keys(rows[0] ?? {});
  if (keys.length === 0) return;
  const binder = new Binder(t.dialect.style);
  const values = rows
    .map((row) => `(${keys.map((k) => binder.add(row[k] ?? null)).join(", ")})`)
    .join(", ");
  const columns = keys.map(t.dialect.quote).join(", ");
  await t.db.exec(
    `INSERT INTO ${t.table} (${columns}) VALUES ${values}`,
    binder.params,
  );
}
