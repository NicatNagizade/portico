import type { Document } from "mongodb";
import type { ColumnSchema, FieldType } from "../types.js";

const BSON_TYPES: Record<FieldType, string[]> = {
  int64: ["int", "long", "null"],
  float64: ["double", "int", "long", "null"],
  bool: ["bool", "null"],
  object: ["object", "null"],
  object_array: ["array", "null"],
  string: ["string", "date", "null"],
};

/** `$jsonSchema` properties for a collection validator built from the destination schema. */
export function validatorProperties(
  columns: ColumnSchema[],
  root: boolean,
  sortField: string,
): Document {
  const props: Document = {};
  for (const column of columns) {
    if (!column.name || column.name === sortField) continue;
    if (root && column.name === "id") {
      props._id = { bsonType: ["string", "null"] };
      continue;
    }
    const nested = validatorProperties(column.columns ?? [], false, "");
    const properties = Object.keys(nested).length ? { properties: nested } : {};
    if (column.type === "object")
      props[column.name] = { bsonType: ["object", "null"], ...properties };
    else if (column.type === "object_array")
      props[column.name] = {
        bsonType: ["array", "null"],
        items: { bsonType: "object", ...properties },
      };
    else props[column.name] = { bsonType: BSON_TYPES[column.type] };
  }
  return props;
}

/** Reads a column back from a validator property spec. */
export function columnFromSpec(
  name: string,
  spec: Document,
  primaryKey: boolean,
): ColumnSchema {
  const types = Array.isArray(spec.bsonType)
    ? spec.bsonType.map(String)
    : [String(spec.bsonType ?? "string")];
  const type = fieldType(types);
  const column: ColumnSchema = { name, type, primaryKey };
  const props = (
    type === "object" ? spec.properties : (spec.items as Document)?.properties
  ) as Document | undefined;
  if (props)
    column.columns = Object.entries(props).map(([n, s]) =>
      columnFromSpec(n, s as Document, false),
    );
  return column;
}

function fieldType(types: string[]): FieldType {
  if (types.includes("object")) return "object";
  if (types.includes("array")) return "object_array";
  if (types.includes("bool")) return "bool";
  if (types.includes("double")) return "float64";
  if (types.includes("int") || types.includes("long")) return "int64";
  return "string";
}
