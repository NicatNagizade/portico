import { asc, eq } from "drizzle-orm";
import type { Db } from "../db/client.js";
import { syncJobRelations } from "../db/schema.js";
import type { Relation } from "../domain/types.js";
import { now } from "../utils/values.js";
import { toRelation } from "./mappers.js";

type RelationRow = Omit<
  Relation,
  "id" | "fields" | "relations" | "created_at" | "updated_at"
>;

function columns(rel: RelationRow) {
  return {
    parent_id: rel.parent_id,
    name: rel.name,
    type: rel.type,
    table: rel.table,
    foreign_key: rel.foreign_key,
    related_key: rel.related_key,
    config: rel.config,
    active: rel.active,
  };
}

export async function listRelations(
  db: Db,
  jobId: number,
): Promise<Relation[]> {
  const rows = await db
    .select()
    .from(syncJobRelations)
    .where(eq(syncJobRelations.sync_job_id, jobId))
    .orderBy(asc(syncJobRelations.id));
  return rows.map((row) => toRelation(row));
}

export async function insertRelation(
  db: Db,
  rel: RelationRow,
): Promise<number> {
  const at = now();
  const [row] = await db
    .insert(syncJobRelations)
    .values({
      sync_job_id: rel.sync_job_id,
      ...columns(rel),
      created_at: at,
      updated_at: at,
    })
    .returning({ id: syncJobRelations.id });
  return row.id;
}

export async function updateRelation(db: Db, rel: Relation): Promise<void> {
  await db
    .update(syncJobRelations)
    .set({ ...columns(rel), updated_at: now() })
    .where(eq(syncJobRelations.id, rel.id));
}

export async function deleteRelation(db: Db, id: number): Promise<void> {
  await db.delete(syncJobRelations).where(eq(syncJobRelations.id, id));
}
