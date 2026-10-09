import type { Db } from "../../db/client.js";
import { ErrNotRunning } from "../../domain/errors.js";
import type { SyncLog } from "../../domain/types.js";
import type { Registry } from "../../connectors/registry.js";
import * as jobsRepo from "../../repositories/syncJobs.js";
import * as logsRepo from "../../repositories/syncLogs.js";
import { isAbort } from "../../utils/abort.js";
import { errorMessage } from "../../utils/values.js";
import { runSyncJob } from "./runner.js";

type Outcome = { rowsTotal: number; rowsSynced: number; error: unknown };

/** Owns sync log rows and cancellation; the actual copy is in `runSyncJob`. */
export class Orchestrator {
  private cancels = new Map<number, AbortController>();

  constructor(
    private db: Db,
    private registry: Registry,
  ) {}

  /** Starts a sync in the background and returns its running log. */
  async start(jobId: number): Promise<SyncLog> {
    const log = await logsRepo.insertLog(this.db, jobId, now());
    void this.track(log).catch((error) =>
      console.error("finalize sync failed", error),
    );
    return log;
  }

  /** Runs a sync to completion; `signal` aborts it (e.g. client disconnect). */
  async run(
    jobId: number,
    signal?: AbortSignal,
  ): Promise<{ log: SyncLog; error: unknown }> {
    const log = await logsRepo.insertLog(this.db, jobId, now());
    return this.track(log, signal);
  }

  async stop(logId: number): Promise<SyncLog> {
    const log = await logsRepo.getLog(this.db, logId);
    if (log?.status !== "running") throw ErrNotRunning;
    this.cancels.get(logId)?.abort();
    const finished = new Date();
    log.status = "stopped";
    log.message = "sync stopped";
    log.finished_at = finished.toISOString();
    log.duration_ms = finished.getTime() - Date.parse(log.started_at);
    await logsRepo.saveLog(this.db, log);
    return log;
  }

  private async track(log: SyncLog, signal?: AbortSignal) {
    const controller = new AbortController();
    const abort = () => controller.abort();
    signal?.addEventListener("abort", abort);
    this.cancels.set(log.id, controller);
    try {
      const outcome = await this.execute(log, controller.signal);
      return {
        log: await this.finalize(log, outcome),
        error: outcome.error,
      };
    } finally {
      signal?.removeEventListener("abort", abort);
      this.cancels.delete(log.id);
    }
  }

  private async execute(log: SyncLog, signal: AbortSignal): Promise<Outcome> {
    let rowsTotal = 0;
    let rowsSynced = 0;
    try {
      const job = await jobsRepo.getJob(this.db, log.sync_job_id);
      if (!job) throw new Error("source or destination connection missing");
      const result = await runSyncJob(this.registry, job, signal, {
        onTotal: async (total) => {
          rowsTotal = total;
          await logsRepo.setRowsTotal(this.db, log.id, total);
        },
        onChunk: async (rows) => {
          rowsSynced += rows;
          await logsRepo.addRowsSynced(
            this.db,
            log.id,
            rows,
            Date.now() - Date.parse(log.started_at),
          );
        },
      });
      return { ...result, error: null };
    } catch (error) {
      console.error("sync failed", error);
      return { rowsTotal, rowsSynced, error };
    }
  }

  private async finalize(started: SyncLog, outcome: Outcome) {
    const log = await logsRepo.getLog(this.db, started.id);
    if (!log) throw outcome.error ?? new Error("sync log not found");
    const finished = new Date();
    log.duration_ms = finished.getTime() - Date.parse(started.started_at);
    log.rows_total = outcome.rowsTotal;
    log.rows_synced = outcome.rowsSynced;
    if (log.status !== "running") {
      log.finished_at ??= finished.toISOString();
    } else {
      log.finished_at = finished.toISOString();
      Object.assign(log, outcomeStatus(outcome));
    }
    await logsRepo.saveLog(this.db, log);
    return log;
  }
}

function outcomeStatus({ rowsTotal, rowsSynced, error }: Outcome) {
  if (isAbort(error)) return { status: "stopped", message: "sync stopped" };
  if (error) return { status: "failed", message: errorMessage(error) };
  return {
    status: "success",
    message: `synced ${rowsSynced} of ${rowsTotal} rows`,
  };
}

function now(): string {
  return new Date().toISOString();
}
