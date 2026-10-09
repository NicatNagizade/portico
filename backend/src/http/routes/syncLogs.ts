import { Router, type Request } from "express";
import type { Db } from "../../db/client.js";
import { ErrSyncLogNotFound, invalid } from "../../domain/errors.js";
import { validLogStatus } from "../../domain/types.js";
import * as logsRepo from "../../repositories/syncLogs.js";
import type { Orchestrator } from "../../services/sync/orchestrator.js";
import { pageOf, parsePage } from "../../utils/pagination.js";
import { idParam, idQuery, instantQuery, orThrow } from "../helpers.js";
import { presentLog } from "../present.js";

export function syncLogRoutes(db: Db, sync: Orchestrator): Router {
  const router = Router();
  const getLog = async (id: number) =>
    orThrow(await logsRepo.getLog(db, id), ErrSyncLogNotFound);

  router.get("/", async (req, res) => {
    const jobId = idQuery(req, "sync_job_id");
    const status = statusQuery(req);
    const from = instantQuery(req, "from");
    const to = instantQuery(req, "to");
    const page = parsePage(req.query as Record<string, unknown>);
    const result = await logsRepo.listLogs(
      db,
      { jobId, status, from, to },
      page.page,
      page.pageSize,
    );
    const items = result.items.map((log) => presentLog(log, true));
    res.json(pageOf(items, result.total, page));
  });

  router.get("/:id", async (req, res) => {
    res.json(presentLog(await getLog(idParam(req))));
  });

  router.post("/:id/stop", async (req, res) => {
    const id = idParam(req);
    await getLog(id);
    res.json(presentLog(await sync.stop(id)));
  });

  return router;
}

function statusQuery(req: Request): string | null {
  const raw = req.query.status;
  if (raw == null || raw === "") return null;
  if (typeof raw !== "string" || !validLogStatus(raw))
    throw invalid("invalid status");
  return raw;
}
