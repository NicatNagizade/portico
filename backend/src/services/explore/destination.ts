import { ErrDestinationReadUnsupported } from "../../domain/errors.js";
import type { SyncJob } from "../../domain/types.js";
import type { Registry } from "../../connectors/registry.js";
import type { Filter, Order, TableSchema } from "../../connectors/types.js";
import { loadSourceShape } from "../sync/documents.js";
import { exploreColumns } from "./columns.js";
import type { Page } from "./source.js";

export async function previewDestination(
  registry: Registry,
  job: SyncJob,
  limit: number,
  offset: number,
  order: Order | null,
  filters: Filter[],
): Promise<Page> {
  if (!job.destination_connection)
    throw new Error("destination connection missing");
  const destination = registry.newDestination(job.destination_connection);
  if (!destination.query) throw ErrDestinationReadUnsupported;
  await destination.open();
  try {
    await destination.applyJobConfig?.(job.config);
    const { rows, total } = await destination.query(
      job.destination_table,
      filters,
      limit,
      offset,
      order,
    );
    const schema = await trySourceSchema(registry, job);
    return { rows, columns: exploreColumns(schema, job, rows), total };
  } finally {
    await destination.close();
  }
}

/** Source schema for column ordering; null when the source is unreachable. */
export async function trySourceSchema(
  registry: Registry,
  job: SyncJob,
): Promise<TableSchema | null> {
  if (!job.source_connection) return null;
  const source = registry.newSource(job.source_connection);
  try {
    await source.open();
    return (await loadSourceShape(source, job)).schema;
  } catch {
    return null;
  } finally {
    await source.close();
  }
}
