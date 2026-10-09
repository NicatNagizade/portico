import { asc, eq, inArray, or } from "drizzle-orm";
import type { Db } from "../db/client.js";
import { selectPage } from "../db/page.js";
import {
  syncJobFields,
  syncJobFieldValues,
  syncJobRelations,
  syncJobRules,
  syncJobs,
  syncLogs,
} from "../db/schema.js";
import type { SyncJob } from "../domain/types.js";
import { groupBy } from "../utils/collections.js";
import { now } from "../utils/values.js";
import { getConnection } from "./connections.js";
import { listFields } from "./fields.js";
import { toJob, toRelation } from "./mappers.js";
import { listRules } from "./rules.js";

export type JobRow = Pick<
  SyncJob,
  | "name"
  | "source_connection_id"
  | "source_table"
  | "destination_connection_id"
  | "destination_table"
  | "chunk_size"
  | "workers"
  | "config"
>;

function columns(job: JobRow) {
  return {
    name: job.name,
    source_connection_id: job.source_connection_id,
    source_table: job.source_table,
    destination_connection_id: job.destination_connection_id,
    destination_table: job.destination_table,
    chunk_size: job.chunk_size,
    workers: job.workers,
    config: job.config,
  };
}

/** Attaches connections, and with `full` also relations, fields, and rules. */
async function loadGraph(
  db: Db,
  job: SyncJob,
  full: boolean,
): Promise<SyncJob> {
  job.source_connection =
    (await getConnection(db, job.source_connection_id)) ?? undefined;
  job.destination_connection =
    (await getConnection(db, job.destination_connection_id)) ?? undefined;
  if (!full) return job;

  const fields = await listFields(db, job.id);
  const fieldsByRelation = groupBy(
    fields,
    (field) => field.sync_job_relation_id,
  );
  const relationRows = await db
    .select()
    .from(syncJobRelations)
    .where(eq(syncJobRelations.sync_job_id, job.id))
    .orderBy(asc(syncJobRelations.id));
  job.fields = fieldsByRelation.get(null) ?? [];
  job.relations = relationRows.map((row) =>
    toRelation(row, fieldsByRelation.get(row.id) ?? []),
  );
  job.rules = await listRules(db, job.id);
  return job;
}

export async function listJobs(
  db: Db,
  connectionId: number | null,
  page: number,
  pageSize: number,
): Promise<{ items: SyncJob[]; total: number }> {
  const where =
    connectionId == null
      ? undefined
      : or(
          eq(syncJobs.source_connection_id, connectionId),
          eq(syncJobs.destination_connection_id, connectionId),
        );
  const { rows, total } = await selectPage(
    db,
    syncJobs,
    where,
    asc(syncJobs.id),
    page,
    pageSize,
  );
  const items: SyncJob[] = [];
  for (const row of rows) items.push(await loadGraph(db, toJob(row), false));
  return { items, total };
}

export async function getJob(
  db: Db,
  id: number,
  full = true,
): Promise<SyncJob | null> {
  const [row] = await db.select().from(syncJobs).where(eq(syncJobs.id, id));
  return row ? loadGraph(db, toJob(row), full) : null;
}

export async function findJobsByName(db: Db, name: string): Promise<SyncJob[]> {
  const rows = await db
    .select()
    .from(syncJobs)
    .where(eq(syncJobs.name, name))
    .orderBy(asc(syncJobs.id));
  return Promise.all(rows.map((row) => loadGraph(db, toJob(row), true)));
}

export async function insertJob(db: Db, job: JobRow): Promise<number> {
  const at = now();
  const [row] = await db
    .insert(syncJobs)
    .values({ ...columns(job), created_at: at, updated_at: at })
    .returning({ id: syncJobs.id });
  return row.id;
}

export async function updateJob(db: Db, job: SyncJob): Promise<void> {
  await db
    .update(syncJobs)
    .set({ ...columns(job), updated_at: now() })
    .where(eq(syncJobs.id, job.id));
}

export async function deleteJob(db: Db, id: number): Promise<boolean> {
  if (!(await getJob(db, id, false))) return false;
  await db
    .delete(syncJobFieldValues)
    .where(
      inArray(
        syncJobFieldValues.sync_job_field_id,
        db
          .select({ id: syncJobFields.id })
          .from(syncJobFields)
          .where(eq(syncJobFields.sync_job_id, id)),
      ),
    );
  await db.delete(syncJobFields).where(eq(syncJobFields.sync_job_id, id));
  await db.delete(syncJobRules).where(eq(syncJobRules.sync_job_id, id));
  await db.delete(syncLogs).where(eq(syncLogs.sync_job_id, id));
  await db.delete(syncJobRelations).where(eq(syncJobRelations.sync_job_id, id));
  await db.delete(syncJobs).where(eq(syncJobs.id, id));
  return true;
}
