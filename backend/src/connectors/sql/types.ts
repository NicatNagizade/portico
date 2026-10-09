import type { FieldType } from "../types.js";

const INT_TYPES = new Set([
  "tinyint",
  "smallint",
  "mediumint",
  "int",
  "integer",
  "bigint",
  "int2",
  "int4",
  "int8",
]);
const FLOAT_TYPES = new Set([
  "float",
  "double",
  "decimal",
  "numeric",
  "real",
  "double precision",
  "float4",
  "float8",
]);

/** information_schema DATA_TYPE (MySQL or Postgres) to a portable field type. */
export function fieldTypeFromSql(dataType: string): FieldType {
  const type = dataType.toLowerCase();
  if (type === "json" || type === "jsonb") return "object";
  if (INT_TYPES.has(type)) return "int64";
  if (FLOAT_TYPES.has(type)) return "float64";
  if (type === "bool" || type === "boolean") return "bool";
  return "string";
}

/** SQLite declared types use affinity rules, so match by substring. */
export function fieldTypeFromSqlite(declared: string): FieldType {
  const type = declared.toUpperCase();
  if (type.includes("INT")) return "int64";
  if (["REAL", "FLOA", "DOUB"].some((t) => type.includes(t))) return "float64";
  if (type.includes("BOOL")) return "bool";
  if (type.includes("JSON")) return "object";
  return "string";
}

type ColumnTypes = Record<
  "int64" | "float64" | "bool" | "json" | "text",
  string
>;

function columnType(types: ColumnTypes) {
  return (type: FieldType): string => {
    if (type === "object" || type === "object_array") return types.json;
    if (type === "int64" || type === "float64" || type === "bool")
      return types[type];
    return types.text;
  };
}

export const mysqlColumnType = columnType({
  int64: "BIGINT",
  float64: "DOUBLE",
  bool: "TINYINT(1)",
  json: "JSON",
  text: "TEXT",
});

export const postgresColumnType = columnType({
  int64: "BIGINT",
  float64: "DOUBLE PRECISION",
  bool: "BOOLEAN",
  json: "JSONB",
  text: "TEXT",
});

export const sqliteColumnType = columnType({
  int64: "INTEGER",
  float64: "REAL",
  bool: "INTEGER",
  json: "TEXT",
  text: "TEXT",
});
