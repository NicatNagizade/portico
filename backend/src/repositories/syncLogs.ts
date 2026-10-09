import { and, desc, eq, gte, lt, sql, type SQL } from "drizzle-orm";
import type { Db } from "../db/client.js";
import { selectPage } from "../db/page.js";
import { syncLogs } from "../db/schema.js";
import type { SyncLog } from "../domain/types.js";
import { toLog } from "./mappers.js";
import { getJob } from "./syncJobs.js";

export type LogListFilter = {
  jobId: number | null;
  status: string | null;
  from: string | null;
  to: string | null;
};

function whereLogs(filter: LogListFilter): SQL | undefined {
  const parts: SQL[] = [];
  if (filter.jobId != null) parts.push(eq(syncLogs.sync_job_id, filter.jobId));
  if (filter.status) parts.push(eq(syncLogs.status, filter.status));
  if (filter.from) parts.push(gte(syncLogs.started_at, filter.from));
  if (filter.to) parts.push(lt(syncLogs.started_at, filter.to));
  if (parts.length === 0) return undefined;
  if (parts.length === 1) return parts[0];
  return and(...parts);
}

export async function listLogs(
  db: Db,
  filter: LogListFilter,
  page: number,
  pageSize: number,
): Promise<{ items: SyncLog[]; total: number }> {
  const where = whereLogs(filter);
  const { rows, total } = await selectPage(
    db,
    syncLogs,
    where,
    desc(syncLogs.id),
    page,
    pageSize,
  );
  const items: SyncLog[] = [];
  for (const row of rows) {
    const job = await getJob(db, row.sync_job_id, false);
    items.push(toLog(row, job ?? undefined));
  }
  return { items, total };
}

export async function getLog(db: Db, id: number): Promise<SyncLog | null> {
  const [row] = await db.select().from(syncLogs).where(eq(syncLogs.id, id));
  return row ? toLog(row) : null;
}

export async function insertLog(
  db: Db,
  jobId: number,
  startedAt: string,
): Promise<SyncLog> {
  const [row] = await db
    .insert(syncLogs)
    .values({
      sync_job_id: jobId,
      status: "running",
      message: "sync started",
      started_at: startedAt,
    })
    .returning();
  return toLog(row);
}

export async function saveLog(db: Db, log: SyncLog): Promise<void> {
  await db
    .update(syncLogs)
    .set({
      status: log.status,
      message: log.message,
      rows_total: log.rows_total,
      rows_synced: log.rows_synced,
      duration_ms: log.duration_ms,
      finished_at: log.finished_at,
    })
    .where(eq(syncLogs.id, log.id));
}

export async function setRowsTotal(
  db: Db,
  id: number,
  rowsTotal: number,
): Promise<void> {
  await db
    .update(syncLogs)
    .set({ rows_total: rowsTotal })
    .where(eq(syncLogs.id, id));
}

export async function addRowsSynced(
  db: Db,
  id: number,
  rows: number,
  durationMs: number,
): Promise<void> {
  await db
    .update(syncLogs)
    .set({
      rows_synced: sql`COALESCE(${syncLogs.rows_synced}, 0) + ${rows}`,
      duration_ms: durationMs,
    })
    .where(eq(syncLogs.id, id));
}
