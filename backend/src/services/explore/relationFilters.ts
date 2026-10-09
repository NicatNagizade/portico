import type { Relation, SyncJob } from "../../domain/types.js";
import type {
  Filter,
  SourceReader,
  TableSchema,
} from "../../connectors/types.js";
import {
  pivotTable,
  primaryKey,
  resolveRelationKeys,
} from "../sync/relations/keys.js";
import { rootRelations } from "../sync/relations/tree.js";
import { sourceName } from "./fieldNames.js";

const PUSHABLE = new Set([
  "eq",
  "in",
  "like",
  "gt",
  "gte",
  "lt",
  "lte",
  "is_not_null",
]);

/** Single-row relations, where `neq` / `not_in` / `is_null` match one value. */
const SINGLE = new Set(["belongs_to", "has_one"]);

function pushable(operator: string, relationType: string): boolean {
  if (PUSHABLE.has(operator)) return true;
  return (
    SINGLE.has(relationType) &&
    (operator === "neq" || operator === "not_in" || operator === "is_null")
  );
}

/**
 * Turns `relation.column` filters into SQL subquery filters on the root table.
 * Filters that cannot be pushed down are returned in `remaining`.
 */
export async function pushDownRelationFilters(
  source: SourceReader,
  job: SyncJob,
  schema: TableSchema,
  nested: Filter[],
): Promise<{ pushed: Filter[]; remaining: Filter[] }> {
  const byName = new Map(rootRelations(job.relations).map((r) => [r.name, r]));
  const pushed: Filter[] = [];
  const remaining: Filter[] = [];
  for (const filter of nested) {
    const [relationName, column, ...deeper] = filter.column.split(".");
    const relation = byName.get(relationName);
    const subquery =
      relation &&
      column &&
      !deeper.length &&
      pushable(filter.operator, relation.type)
        ? await relationFilter(source, job, schema, relation, {
            column: sourceName(relation.fields, column) || column,
            operator: filter.operator,
            value: filter.value,
          })
        : null;
    if (subquery) pushed.push(subquery);
    else remaining.push(filter);
  }
  return { pushed, remaining };
}

async function relationFilter(
  source: SourceReader,
  job: SyncJob,
  parentSchema: TableSchema,
  input: Relation,
  related: Filter,
): Promise<Filter | null> {
  const relation = resolveRelationKeys(job.source_table, input);
  const inSubquery = (
    column: string,
    table: string,
    select: string,
    where: Filter[],
  ): Filter => ({
    column,
    operator: "eq",
    value: "",
    rel: { table, select, where },
  });

  switch (relation.type) {
    case "belongs_to": {
      const owner =
        relation.related_key || primaryKey(await source.schema(relation.table));
      return inSubquery(relation.foreign_key, relation.table, owner, [related]);
    }
    case "has_many":
    case "has_one": {
      const local = relation.related_key || primaryKey(parentSchema);
      return inSubquery(local, relation.table, relation.foreign_key, [related]);
    }
    case "belongs_to_many": {
      const pivot = pivotTable(relation);
      if (!pivot) return null;
      const relatedPK = primaryKey(await source.schema(relation.table));
      const throughRelated = inSubquery(
        relation.related_key,
        relation.table,
        relatedPK,
        [related],
      );
      return inSubquery(primaryKey(parentSchema), pivot, relation.foreign_key, [
        throughRelated,
      ]);
    }
    default:
      return null;
  }
}
