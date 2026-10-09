import type { SyncJob } from "../../domain/types.js";
import type { Registry } from "../../connectors/registry.js";
import {
  filterRows,
  pageRows,
  sortRows,
} from "../../connectors/shared/rows.js";
import type {
  Doc,
  Filter,
  Order,
  SourceReader,
} from "../../connectors/types.js";
import {
  activeFilters,
  buildDocs,
  loadSourceShape,
  type SourceShape,
} from "../sync/documents.js";
import { exploreColumns } from "./columns.js";
import { toSourceFilters, toSourceOrder } from "./fieldNames.js";
import { pushDownRelationFilters } from "./relationFilters.js";

export type Page = { rows: Doc[]; columns: string[]; total: number };

const SCAN_CHUNK = 250;

/**
 * Pages through source rows shaped like destination docs. Root filters run in
 * the source; relation filters are pushed down when possible, otherwise the
 * whole filtered table is shaped and filtered in memory.
 */
export async function previewSource(
  registry: Registry,
  job: SyncJob,
  limit: number,
  offset: number,
  order: Order | null,
  extra: Filter[],
): Promise<Page> {
  if (!job.source_connection) throw new Error("source connection missing");
  const source = registry.newSource(job.source_connection);
  await source.open();
  try {
    const shape = await loadSourceShape(source, job);
    const sourceOrder = toSourceOrder(shape.schema, job.fields, order);
    const mapped = toSourceFilters(job.fields, extra);
    let nested = mapped.filter((f) => f.column.includes("."));
    let root = [
      ...activeFilters(job.rules),
      ...mapped.filter((f) => !f.column.includes(".")),
    ];
    if (nested.length && source.supportsRelationFilters()) {
      const { pushed, remaining } = await pushDownRelationFilters(
        source,
        job,
        shape.schema,
        nested,
      );
      if (!remaining.length) root = [...root, ...pushed];
      nested = remaining;
    }

    if (nested.length)
      return await scanInMemory(
        source,
        job,
        shape,
        root,
        nested,
        order,
        limit,
        offset,
      );
    return await queryPage(
      source,
      job,
      shape,
      root,
      sourceOrder,
      limit,
      offset,
    );
  } finally {
    await source.close();
  }
}

async function queryPage(
  source: SourceReader,
  job: SyncJob,
  shape: SourceShape,
  filters: Filter[],
  order: Order | null,
  limit: number,
  offset: number,
): Promise<Page> {
  const total = await source.count(job.source_table, filters);
  const rows = await source.query(
    job.source_table,
    null,
    filters,
    limit,
    offset,
    order,
  );
  const docs = await buildDocs(source, job, shape, rows, offset);
  return {
    rows: docs,
    columns: exploreColumns(shape.schema, job, docs),
    total,
  };
}

async function scanInMemory(
  source: SourceReader,
  job: SyncJob,
  shape: SourceShape,
  root: Filter[],
  nested: Filter[],
  order: Order | null,
  limit: number,
  offset: number,
): Promise<Page> {
  const matched: Doc[] = [];
  await source.readChunks(job.source_table, SCAN_CHUNK, root, async (rows) => {
    const docs = await buildDocs(source, job, shape, rows, 0);
    matched.push(...filterRows(docs, nested));
  });
  sortRows(matched, order);
  const rows = pageRows(matched, limit, offset);
  return {
    rows,
    columns: exploreColumns(shape.schema, job, rows),
    total: matched.length,
  };
}
