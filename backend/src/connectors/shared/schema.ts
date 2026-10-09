import type { ColumnSchema, Doc, FieldType, TableSchema } from "../types.js";

export function inferFieldType(value: unknown): FieldType {
  if (typeof value === "boolean") return "bool";
  if (typeof value === "number")
    return Number.isInteger(value) ? "int64" : "float64";
  if (Array.isArray(value)) return "object_array";
  if (value && typeof value === "object") return "object";
  return "string";
}

type ColumnNode = { type: FieldType; children: Map<string, ColumnNode> };

/** Infers a schema from sample docs: keys sorted with `id` first, nested objects merged. */
export function schemaFromDocs(docs: Doc[]): TableSchema {
  const root = new Map<string, ColumnNode>();
  for (const doc of docs)
    for (const [key, value] of Object.entries(doc))
      mergeValue(root, key, value);
  const sorted = [...root.keys()].sort();
  const names = root.has("id")
    ? ["id", ...sorted.filter((name) => name !== "id")]
    : sorted;
  return {
    columns: nestDottedFields(
      names.map((name) => toColumn(name, root.get(name)!, name === "id")),
    ),
  };
}

function mergeValue(
  into: Map<string, ColumnNode>,
  key: string,
  value: unknown,
): void {
  const type = inferFieldType(value);
  let node = into.get(key);
  if (!node) {
    node = { type, children: new Map() };
    into.set(key, node);
  } else if (node.type === "string" && type !== "string") {
    node.type = type;
  }
  const nested =
    type === "object"
      ? [value]
      : type === "object_array"
        ? (value as unknown[])
        : [];
  for (const item of nested) {
    if (!item || typeof item !== "object" || Array.isArray(item)) continue;
    for (const [k, v] of Object.entries(item as Doc))
      mergeValue(node.children, k, v);
  }
}

function toColumn(name: string, node: ColumnNode, pk: boolean): ColumnSchema {
  const column: ColumnSchema = { name, type: node.type, primaryKey: pk };
  if (node.children.size)
    column.columns = [...node.children].map(([k, child]) =>
      toColumn(k, child, false),
    );
  return column;
}

/** Turns flat `a.b` column names into nested object columns. */
export function nestDottedFields(columns: ColumnSchema[]): ColumnSchema[] {
  const roots = new Map<string, ColumnSchema>();
  const nested = new Map<string, ColumnSchema[]>();
  for (const column of columns) {
    if (!column.name) continue;
    const dot = column.name.indexOf(".");
    if (dot < 0) {
      roots.set(column.name, column);
      continue;
    }
    const name = column.name.slice(0, dot);
    const kids = nested.get(name) ?? [];
    kids.push({ ...column, name: column.name.slice(dot + 1) });
    nested.set(name, kids);
    if (!roots.has(name)) roots.set(name, { name, type: "object" });
  }
  return [...roots.values()].map((root) => {
    const kids = nested.get(root.name);
    if (!kids?.length) return { ...root };
    const type =
      root.type === "object" || root.type === "object_array"
        ? root.type
        : "object";
    return { ...root, type, columns: nestDottedFields(kids) };
  });
}

/** Fills nested column definitions missing from `base` using an inferred `sample` schema. */
export function mergeNestedFrom(
  base: TableSchema | null,
  sample: TableSchema | null,
): TableSchema | null {
  if (!base || !sample) return base ?? sample;
  const byName = new Map(sample.columns.map((c) => [c.name, c]));
  const columns = base.columns.map((column) => {
    const fromSample = byName.get(column.name);
    const isObject = column.type === "object" || column.type === "object_array";
    if (column.columns?.length || !isObject || !fromSample?.columns?.length)
      return { ...column };
    return { ...column, columns: fromSample.columns };
  });
  const seen = new Set(columns.map((c) => c.name));
  return {
    columns: [...columns, ...sample.columns.filter((c) => !seen.has(c.name))],
  };
}
