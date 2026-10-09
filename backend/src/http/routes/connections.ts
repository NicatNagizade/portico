import { Router } from "express";
import type { Registry } from "../../connectors/registry.js";
import type { ColumnSchema, SourceReader } from "../../connectors/types.js";
import type { Db } from "../../db/client.js";
import { ErrConnectionNotFound, invalid } from "../../domain/errors.js";
import * as connectionsRepo from "../../repositories/connections.js";
import { merge } from "../../utils/secretbox.js";
import { asObject } from "../../utils/values.js";
import { idParam, orThrow } from "../helpers.js";
import { presentConnection } from "../present.js";

export function connectionRoutes(db: Db, registry: Registry): Router {
  const router = Router();
  const getConnection = async (id: number) =>
    orThrow(await connectionsRepo.getConnection(db, id), ErrConnectionNotFound);

  /** Opens the connection as a source, runs `fn`, and always closes it. */
  const withSource = async <T>(
    id: number,
    fn: (source: SourceReader) => Promise<T>,
  ): Promise<T> => {
    const source = registry.newSource(await getConnection(id));
    await source.open();
    try {
      return await fn(source);
    } finally {
      await source.close();
    }
  };

  router.get("/", async (_req, res) => {
    const items = await connectionsRepo.listConnections(db);
    res.json(items.map(presentConnection));
  });

  router.post("/check", async (req, res) => {
    const body = req.body ?? {};
    if (!body.type || body.config == null)
      throw invalid("type and config are required");

    let config = asObject(body.config) ?? {};
    if (body.id != null) {
      const existing = await getConnection(Number(body.id));
      config = merge(existing.config, config);
    }
    await registry.check({
      id: 0,
      name: "",
      type: String(body.type),
      config,
      created_at: "",
      updated_at: "",
    });
    res.json({ ok: true });
  });

  router.post("/", async (req, res) => {
    const body = req.body ?? {};
    if (!body.name || !body.type || body.config == null)
      throw invalid("name, type, and config are required");

    const item = await connectionsRepo.insertConnection(
      db,
      String(body.name),
      String(body.type),
      asObject(body.config) ?? {},
    );
    res.status(201).json(presentConnection(item));
  });

  router.get("/:id", async (req, res) => {
    res.json(presentConnection(await getConnection(idParam(req))));
  });

  router.put("/:id", async (req, res) => {
    const item = await getConnection(idParam(req));
    const body = req.body ?? {};
    if ("name" in body) item.name = String(body.name ?? "");
    if ("type" in body) item.type = String(body.type ?? "");
    if (body.config != null) item.config = merge(item.config, body.config);
    res.json(presentConnection(await connectionsRepo.saveConnection(db, item)));
  });

  router.delete("/:id", async (req, res) => {
    if (!(await connectionsRepo.deleteConnection(db, idParam(req))))
      throw ErrConnectionNotFound;
    res.status(204).end();
  });

  router.get("/:id/tables", async (req, res) => {
    const tables = await withSource(idParam(req), (s) => s.listTables());
    res.json({ tables: tables ?? [] });
  });

  router.get("/:id/columns", async (req, res) => {
    const table = String(req.query.table ?? "");
    if (!table) throw invalid("table is required");
    const schema = await withSource(idParam(req), (s) => s.schema(table));
    res.json({ columns: schema.columns.map(columnInfo) });
  });

  return router;
}

function columnInfo(column: ColumnSchema): Record<string, unknown> {
  return {
    name: column.name,
    type: column.type,
    ...(column.columns?.length
      ? { columns: column.columns.map(columnInfo) }
      : {}),
  };
}
