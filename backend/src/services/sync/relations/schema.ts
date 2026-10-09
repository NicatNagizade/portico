import type { Relation, SyncJob } from "../../../domain/types.js";
import type {
  ColumnSchema,
  SourceReader,
  TableSchema,
} from "../../../connectors/types.js";
import { schemaWithFields } from "../fields.js";
import { isSingle } from "./keys.js";
import { activeRelations, childrenByParent, rootRelations } from "./tree.js";

/** Source schema plus one nested column per root relation (with its children). */
export async function destinationSchema(
  source: SourceReader,
  base: TableSchema,
  job: SyncJob,
): Promise<TableSchema> {
  const active = activeRelations(job.relations);
  const related = new Map<string, TableSchema>();
  for (const relation of active) {
    if (relation.table && !related.has(relation.table))
      related.set(relation.table, await source.schema(relation.table));
  }

  const children = childrenByParent(active);
  const columns = rootRelations(active).map((relation) =>
    relationColumn(relation, children, related),
  );
  return (
    schemaWithFields({ columns: [...base.columns, ...columns] }, job.fields) ??
    base
  );
}

export function relationColumnType(relation: Relation): ColumnSchema["type"] {
  return isSingle(relation) ? "object" : "object_array";
}

function relationColumn(
  relation: Relation,
  children: Map<number, Relation[]>,
  related: Map<string, TableSchema>,
  ancestors = new Set<number>(),
): ColumnSchema {
  if (ancestors.has(relation.id))
    throw new Error(
      `cyclic parent_id involving relation ${JSON.stringify(relation.name)}`,
    );
  const next = new Set(ancestors).add(relation.id);
  const own =
    schemaWithFields(related.get(relation.table) ?? null, relation.fields)
      ?.columns ?? [];
  const nested = (children.get(relation.id) ?? []).map((child) =>
    relationColumn(child, children, related, next),
  );
  return {
    name: relation.name,
    type: relationColumnType(relation),
    columns: [...own, ...nested],
  };
}
