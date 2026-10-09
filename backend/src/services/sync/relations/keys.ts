import type { Relation } from "../../../domain/types.js";
import type { TableSchema } from "../../../connectors/types.js";

/** Empty foreign/related keys fall back to `<singular table>_id`. */
export function resolveRelationKeys(
  sourceTable: string,
  relation: Relation,
): Relation {
  const result = { ...relation };
  if (relation.type === "belongs_to") {
    result.foreign_key ||= `${singular(relation.table)}_id`;
  } else if (relation.type === "belongs_to_many") {
    result.foreign_key ||= `${singular(sourceTable)}_id`;
    result.related_key ||= `${singular(relation.table)}_id`;
  } else {
    result.foreign_key ||= `${singular(sourceTable)}_id`;
  }
  return result;
}

export function primaryKey(schema: TableSchema): string {
  const keys = schema.columns.filter((column) => column.primaryKey);
  if (keys.length === 0) throw new Error("table has no primary key");
  if (keys.length > 1)
    throw new Error(
      "composite primary keys are not supported for related tables",
    );
  return keys[0].name;
}

export function pivotTable(relation: Relation): string {
  const pivot = relation.config?.pivot_table;
  return typeof pivot === "string" ? pivot : "";
}

export function isSingle(relation: Relation): boolean {
  return relation.type === "has_one" || relation.type === "belongs_to";
}

function singular(name: string): string {
  return name.length > 1 && name.endsWith("s") ? name.slice(0, -1) : name;
}
