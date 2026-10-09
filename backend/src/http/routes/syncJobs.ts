import { Router } from "express";
import type { Db } from "../../db/client.js";
import { ErrSyncJobNotFound, invalid } from "../../domain/errors.js";
import * as jobsRepo from "../../repositories/syncJobs.js";
import type { Explorer } from "../../services/explore/index.js";
import { suggestExplore, type Completer } from "../../services/llm.js";
import type { Orchestrator } from "../../services/sync/orchestrator.js";
import { createJob, updateJob } from "../../services/syncJobs/index.js";
import { pageOf, parsePage } from "../../utils/pagination.js";
import {
  idParam,
  idQuery,
  orThrow,
  requireBody,
  requireKeys,
} from "../helpers.js";
import { presentJob, presentLog } from "../present.js";

const REQUIRED_ON_CREATE = [
  "name",
  "source_connection_id",
  "source_table",
  "destination_connection_id",
  "destination_table",
];

export function syncJobRoutes(
  db: Db,
  sync: Orchestrator,
  explorer: Explorer,
  llm: Completer | null,
): Router {
  const router = Router();
  const getJob = async (id: number, full = true) =>
    orThrow(await jobsRepo.getJob(db, id, full), ErrSyncJobNotFound);

  router.get("/", async (req, res) => {
    const connectionId = idQuery(req, "connection_id");
    const page = parsePage(req.query as Record<string, unknown>);
    const result = await jobsRepo.listJobs(
      db,
      connectionId,
      page.page,
      page.pageSize,
    );
    const items = result.items.map((job) => presentJob(job, false));
    res.json(pageOf(items, result.total, page));
  });

  router.post("/", async (req, res) => {
    const body = req.body ?? {};
    requireKeys(body, REQUIRED_ON_CREATE);
    res.status(201).json(presentJob(await createJob(db, body), true));
  });

  router.get("/:id", async (req, res) => {
    res.json(presentJob(await getJob(idParam(req)), true));
  });

  router.put("/:id", async (req, res) => {
    const job = await updateJob(db, idParam(req), req.body ?? {});
    res.json(presentJob(job, true));
  });

  router.delete("/:id", async (req, res) => {
    if (!(await jobsRepo.deleteJob(db, idParam(req)))) throw ErrSyncJobNotFound;
    res.status(204).end();
  });

  router.post("/:id/run", async (req, res) => {
    const id = idParam(req);
    await getJob(id, false);
    const { log, error } = await sync.run(id, req.signal);
    res.status(error ? 500 : 200).json(presentLog(log));
  });

  router.post("/:id/start", async (req, res) => {
    const id = idParam(req);
    await getJob(id, false);
    res.status(202).json(presentLog(await sync.start(id)));
  });

  router.post("/:id/explore", async (req, res) => {
    const body = requireBody(req);
    const result = await explorer.preview(idParam(req), {
      side: String(body.side ?? ""),
      page: Number(body.page ?? 0),
      pageSize: Number(body.page_size ?? 0),
      sortBy: String(body.sort_by ?? ""),
      sortDir: String(body.sort_dir ?? ""),
      filters: body.filters ?? [],
    });
    res.json(result);
  });

  router.post("/:id/explore/suggest", async (req, res) => {
    await getJob(idParam(req), false);
    const body = requireBody(req);
    if (!String(body.prompt ?? "").trim()) throw invalid("prompt is required");
    res.json(await suggestExplore(llm, body));
  });

  router.post("/:id/explore/export", async (req, res) => {
    const body = requireBody(req);
    const file = await explorer.exportCSV(
      idParam(req),
      String(body.side ?? ""),
      body.filters ?? [],
      "fields" in body ? (body.fields ?? []) : null,
    );
    res.setHeader(
      "Content-Disposition",
      `attachment; filename="${file.filename}"`,
    );
    res.type("text/csv; charset=utf-8").send(file.body);
  });

  return router;
}
