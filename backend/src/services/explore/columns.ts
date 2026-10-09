import { isActive, type SyncJob } from "../../domain/types.js";
import type { Doc, TableSchema } from "../../connectors/types.js";
import { destinationName, schemaWithFields } from "../sync/fields.js";
import { relationColumnType } from "../sync/relations/schema.js";
import { rootRelations } from "../sync/relations/tree.js";

/** Column order for the explore table: schema order first, then extra row keys sorted. */
export function exploreColumns(
  schema: TableSchema | null,
  job: SyncJob,
  rows: Doc[],
): string[] {
  const known = knownColumns(schema, job);
  const seen = new Set(known);
  const extra = new Set<string>();
  for (const row of rows)
    for (const key of Object.keys(row))
      if (key && !seen.has(key)) extra.add(key);
  return [...known, ...[...extra].sort()];
}

function knownColumns(schema: TableSchema | null, job: SyncJob): string[] {
  const roots = rootRelations(job.relations);
  const shaped = schemaWithFields(
    schema
      ? {
          columns: [
            ...schema.columns,
            ...roots.map((r) => ({
              name: r.name,
              type: relationColumnType(r),
            })),
          ],
        }
      : null,
    job.fields,
  );
  const names = shaped
    ? shaped.columns.map((c) => c.name)
    : [
        ...job.fields.filter((f) => isActive(f.active)).map(destinationName),
        ...roots.map((r) => r.name),
      ];
  return [...new Set(names.filter(Boolean))];
}

export function pickColumns(columns: string[], fields: string[] | null) {
  if (fields == null) return columns;
  const want = new Set(fields.map((f) => f.trim()).filter(Boolean));
  return columns.filter((column) => want.has(column));
}
