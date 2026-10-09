import { ErrInvalidSort } from "../../domain/errors.js";
import { isActive, type Field } from "../../domain/types.js";
import type { Filter, Order, TableSchema } from "../../connectors/types.js";
import { destinationName } from "../sync/fields.js";

/** Maps a column name shown in the UI (destination or source name) back to the source column. */
export function sourceName(fields: Field[], shown: string): string {
  const want = shown.trim();
  const field = fields.find(
    (f) =>
      isActive(f.active) &&
      f.source_name &&
      (destinationName(f) === want || f.source_name === want),
  );
  return field?.source_name ?? "";
}

export function toSourceFilters(fields: Field[], filters: Filter[]): Filter[] {
  return filters.map((f) => ({
    ...f,
    column: sourceName(fields, f.column) || f.column,
  }));
}

/** Translates a UI sort to a sortable source column; nested columns cannot be sorted in SQL. */
export function toSourceOrder(
  schema: TableSchema,
  fields: Field[],
  order: Order | null,
): Order | null {
  if (!order?.column.trim()) return null;
  const column = sourceName(fields, order.column) || order.column.trim();
  const schemaColumn = schema.columns.find((c) => c.name === column);
  if (schemaColumn) {
    if (schemaColumn.type === "object" || schemaColumn.type === "object_array")
      throw ErrInvalidSort;
    return { column, desc: order.desc };
  }
  if (fields.some((f) => isActive(f.active) && f.source_name === column))
    return { column, desc: order.desc };
  throw ErrInvalidSort;
}
