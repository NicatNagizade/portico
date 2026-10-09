import { isActive, type Field } from "../../domain/types.js";
import type { ColumnSchema, Doc, TableSchema } from "../../connectors/types.js";

export function destinationName(field: Field): string {
  return field.destination_name || field.source_name;
}

export function applyFields(docs: Doc[], fields: Field[]): void {
  const rules = rulesBySource(fields);
  if (rules.size === 0) return;

  for (const doc of docs) {
    for (const [source, rule] of rules) {
      if (!isActive(rule.active)) {
        delete doc[source];
        continue;
      }
      if (!(source in doc)) continue;

      let value = mapValue(doc[source], rule);
      if (rule.destination_type) {
        value = coerce(value, rule.destination_type);
      }

      const destination = destinationName(rule);
      doc[destination] = value;
      if (destination !== source) delete doc[source];
    }
  }
}

export function schemaWithFields(
  base: TableSchema | null,
  fields: Field[],
): TableSchema | null {
  if (!base || fields.length === 0) return base;
  const rules = rulesBySource(fields);
  if (rules.size === 0) return base;

  const columns: ColumnSchema[] = [];
  for (const column of base.columns) {
    const rule = rules.get(column.name);
    if (!rule) {
      columns.push(column);
      continue;
    }
    if (!isActive(rule.active)) continue;

    columns.push({
      ...column,
      name: destinationName(rule),
      type: (rule.destination_type || column.type) as ColumnSchema["type"],
    });
  }
  return { columns };
}

function rulesBySource(fields: Field[]): Map<string, Field> {
  return new Map(
    fields
      .filter((field) => field.source_name)
      .map((field) => [field.source_name, field]),
  );
}

function mapValue(value: unknown, field: Field): unknown {
  const mapping = field.values.find(
    (item) => item.source_value === String(value),
  );
  return mapping ? mapping.destination_value : value;
}

function coerce(value: unknown, type: string): unknown {
  if (value == null) return null;
  if (type === "string") {
    if (typeof value === "string") return value;
    return typeof value === "object" ? JSON.stringify(value) : String(value);
  }
  if (type === "int64") {
    const parsed = Number.parseInt(String(value), 10);
    return Number.isFinite(parsed) ? parsed : value;
  }
  if (type === "float64") {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : value;
  }
  if (type === "bool") {
    if (value === true || value === "true") return true;
    if (value === false || value === "false") return false;
    return value;
  }
  if (
    (type === "object" || type === "object_array") &&
    typeof value === "string"
  ) {
    try {
      return JSON.parse(value);
    } catch {
      return value;
    }
  }
  return value;
}
