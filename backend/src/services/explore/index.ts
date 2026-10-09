import type { Db } from "../../db/client.js";
import { ErrSyncJobNotFound } from "../../domain/errors.js";
import type { SyncJob } from "../../domain/types.js";
import type { Registry } from "../../connectors/registry.js";
import type { Filter, Order } from "../../connectors/types.js";
import * as jobsRepo from "../../repositories/syncJobs.js";
import { csvCell, toCSV } from "../../utils/csv.js";
import { pickColumns } from "./columns.js";
import { previewDestination, trySourceSchema } from "./destination.js";
import {
  parseFilters,
  parseSide,
  parseSort,
  type FilterInput,
  type Side,
} from "./params.js";
import { previewSource, type Page } from "./source.js";

export type PreviewInput = {
  side: string;
  page: number;
  pageSize: number;
  sortBy: string;
  sortDir: string;
  filters: FilterInput[];
};

const EXPORT_PAGE = 250;

/** Browses source or destination rows of a sync job, and exports them as CSV. */
export class Explorer {
  constructor(
    private db: Db,
    private registry: Registry,
  ) {}

  async preview(jobId: number, input: PreviewInput) {
    const side = parseSide(input.side);
    const page = Math.max(1, input.page);
    const pageSize = input.pageSize < 1 ? 50 : Math.min(input.pageSize, 100);
    const filters = parseFilters(input.filters);
    const job = await this.job(jobId);
    const order =
      parseSort(input.sortBy, input.sortDir) ??
      (await this.defaultOrder(job, side));
    const result = await this.page(
      job,
      side,
      pageSize,
      (page - 1) * pageSize,
      order,
      filters,
    );
    return {
      ...result,
      page,
      page_size: pageSize,
      side,
      ...(order.sortBy
        ? { sort_by: order.sortBy, sort_dir: order.sortDir }
        : {}),
    };
  }

  async exportCSV(
    jobId: number,
    rawSide: string,
    rawFilters: FilterInput[],
    fields: string[] | null,
  ): Promise<{ filename: string; body: string }> {
    const side = parseSide(rawSide);
    const filters = parseFilters(rawFilters);
    const job = await this.job(jobId);
    const records: string[][] = [];
    let headers: string[] = [];
    for (let offset = 0; ; offset += EXPORT_PAGE) {
      const page = await this.page(
        job,
        side,
        EXPORT_PAGE,
        offset,
        null,
        filters,
      );
      if (offset === 0) {
        headers = pickColumns(page.columns, fields);
        records.push(headers);
      }
      for (const row of page.rows)
        records.push(headers.map((h) => csvCell(row[h])));
      if (page.rows.length < EXPORT_PAGE) break;
    }
    const name =
      job.name.replace(/[^a-zA-Z0-9_-]/g, "-") || `sync-job-${job.id}`;
    return { filename: `${name}-${side}.csv`, body: toCSV(records) };
  }

  private page(
    job: SyncJob,
    side: Side,
    limit: number,
    offset: number,
    order: Order | null,
    filters: Filter[],
  ): Promise<Page> {
    return side === "source"
      ? previewSource(this.registry, job, limit, offset, order, filters)
      : previewDestination(this.registry, job, limit, offset, order, filters);
  }

  private async job(id: number): Promise<SyncJob> {
    const job = await jobsRepo.getJob(this.db, id);
    if (!job) throw ErrSyncJobNotFound;
    return job;
  }

  private async defaultOrder(job: SyncJob, side: Side) {
    const schema =
      side === "source" ? await trySourceSchema(this.registry, job) : null;
    const column =
      side === "source"
        ? schema?.columns.find((c) => c.primaryKey)?.name || "id"
        : "id";
    return { column, desc: false, sortBy: column, sortDir: "asc" };
  }
}
