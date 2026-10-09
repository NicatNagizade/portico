import type { CollectionFieldSchema } from "typesense/lib/Typesense/Collection.js";
import type { FieldType, TableSchema } from "../types.js";
import { sortColumn } from "../shared/filters.js";
import { asObject } from "../../utils/values.js";

const TO_TYPESENSE: Record<FieldType, string> = {
  string: "string",
  int64: "int64",
  float64: "float",
  bool: "bool",
  object: "object",
  object_array: "object[]",
};

export function fromTypesenseType(type: string): FieldType {
  if (type === "int32" || type === "int64") return "int64";
  if (type === "float") return "float64";
  if (type === "bool") return "bool";
  if (type === "object") return "object";
  if (type === "object[]") return "object_array";
  return "string";
}

export function isSortable(field: CollectionFieldSchema | undefined): boolean {
  if (!field || field.name === "id") return false;
  return (
    field.sort === true ||
    field.type === "int32" ||
    field.type === "int64" ||
    field.type === "float"
  );
}

/** Collection create body; optional keys come from the sync job config. */
export function collectionSchema(
  name: string,
  schema: TableSchema,
  config: unknown,
  sortField: string,
): Record<string, unknown> {
  const raw = asObject(config) ?? {};
  const fields: Array<Record<string, unknown>> = schema.columns
    .filter((col) => col.name && col.name !== "id" && col.name !== sortField)
    .map((col) => ({
      name: col.name,
      type: TO_TYPESENSE[col.type],
      optional: true,
    }));
  if (sortField)
    fields.push({ name: sortField, type: "int64", optional: true, sort: true });

  const body: Record<string, unknown> = {
    name,
    fields,
    enable_nested_fields:
      raw.enable_nested_fields == null
        ? true
        : Boolean(raw.enable_nested_fields),
  };
  const defaultSort =
    typeof raw.default_sorting_field === "string"
      ? raw.default_sorting_field.trim()
      : "";
  if (defaultSort)
    body.default_sorting_field = sortField
      ? sortColumn(defaultSort, sortField)
      : defaultSort;
  for (const key of ["symbols_to_index", "token_separators"]) {
    const list = raw[key];
    if (Array.isArray(list) && list.length) body[key] = list;
  }
  return body;
}
