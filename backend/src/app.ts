import express, {
  type NextFunction,
  type Request,
  type Response,
} from "express";
import type { Registry } from "./connectors/registry.js";
import type { Db } from "./db/client.js";
import { sendError } from "./http/helpers.js";
import { connectionRoutes } from "./http/routes/connections.js";
import { syncJobRoutes } from "./http/routes/syncJobs.js";
import { syncLogRoutes } from "./http/routes/syncLogs.js";
import { Explorer } from "./services/explore/index.js";
import type { Completer } from "./services/llm.js";
import { Orchestrator } from "./services/sync/orchestrator.js";

export function createApp(db: Db, registry: Registry, llm: Completer | null) {
  const app = express();
  const sync = new Orchestrator(db, registry);
  const explorer = new Explorer(db, registry);

  app.use(express.json());
  app.use(
    (error: unknown, _req: Request, res: Response, next: NextFunction) => {
      if (error instanceof SyntaxError) {
        res.status(400).json({ error: "invalid json body" });
        return;
      }
      next(error);
    },
  );
  app.use(logRequests);

  app.get("/health", (_req, res) => {
    res.json({ status: "ok", ai_enabled: llm != null });
  });

  app.use("/connections", connectionRoutes(db, registry));
  app.use("/sync-jobs", syncJobRoutes(db, sync, explorer, llm));
  app.use("/sync-logs", syncLogRoutes(db, sync));

  app.use((_req: Request, res: Response) => {
    res.status(404).json({ error: "not found" });
  });
  app.use(
    (error: unknown, _req: Request, res: Response, _next: NextFunction) => {
      sendError(res, error);
    },
  );
  return app;
}

function logRequests(req: Request, res: Response, next: NextFunction) {
  const started = Date.now();
  res.on("finish", () => {
    console.log(
      `${req.method} ${req.originalUrl} ${res.statusCode} ${Date.now() - started}ms`,
    );
  });
  next();
}
