import type { SyncJob } from "../../domain/types.js";
import { applyDestinationPrimaryKey } from "../../connectors/primaryKey.js";
import type { Registry } from "../../connectors/registry.js";
import { throwIfAborted } from "../../utils/abort.js";
import { WorkerPool } from "../../utils/workerPool.js";
import { activeFilters, finishDocs, loadSourceShape } from "./documents.js";
import { cloneRows } from "../../connectors/shared/rows.js";
import { enrichRelations } from "./relations/enrich.js";
import { destinationSchema } from "./relations/schema.js";

export type SyncProgress = {
  onTotal(rowsTotal: number): Promise<void>;
  onChunk(rows: number): Promise<void>;
};

/**
 * Prepares the destination, then reads the source in chunks. Relations are
 * loaded while reading; ids, field overrides, and writes run on `job.workers`.
 */
export async function runSyncJob(
  registry: Registry,
  job: SyncJob,
  signal: AbortSignal,
  progress: SyncProgress,
): Promise<{ rowsTotal: number; rowsSynced: number }> {
  if (!job.source_connection || !job.destination_connection)
    throw new Error("source or destination connection missing");
  const source = registry.newSource(job.source_connection);
  const destination = registry.newDestination(job.destination_connection);

  await source.open();
  try {
    await destination.open();
    try {
      const shape = await loadSourceShape(source, job);
      const outSchema = await destinationSchema(source, shape.schema, job);
      await destination.prepare(
        job.destination_table,
        applyDestinationPrimaryKey(outSchema, shape.pk) ?? outSchema,
        job.config,
      );

      const filters = activeFilters(job.rules);
      const rowsTotal = await source.count(job.source_table, filters);
      await progress.onTotal(rowsTotal);

      const pool = new WorkerPool(job.workers > 0 ? job.workers : 1);
      let rowsSynced = 0;
      let nextIndex = 0;
      await source.readChunks(
        job.source_table,
        job.chunk_size > 0 ? job.chunk_size : 500,
        filters,
        async (rows) => {
          throwIfAborted(signal);
          const docs = cloneRows(rows);
          await enrichRelations(source, job, shape.schema, docs);
          const startIndex = nextIndex;
          nextIndex += docs.length;
          pool.add(async () => {
            throwIfAborted(signal);
            finishDocs(docs, job, shape, startIndex);
            await destination.writeBatch(job.destination_table, docs);
            rowsSynced += docs.length;
            await progress.onChunk(docs.length);
          });
        },
      );
      await pool.wait();
      return { rowsTotal, rowsSynced };
    } finally {
      await destination.close();
    }
  } finally {
    await source.close();
  }
}
