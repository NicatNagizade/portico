import type { Request, Response } from "express";
import { HttpError, invalid } from "../domain/errors.js";
import { errorMessage } from "../utils/values.js";

function positiveInt(raw: unknown): number | null {
  const n = Number(raw);
  return Number.isInteger(n) && n > 0 ? n : null;
}

export function idParam(req: Request): number {
  const id = positiveInt(req.params.id);
  if (id == null) throw invalid("invalid id");
  return id;
}

/** Optional `?name=<id>` filter; null when absent. */
export function idQuery(req: Request, name: string): number | null {
  if (!req.query[name]) return null;
  const id = positiveInt(req.query[name]);
  if (id == null) throw invalid(`invalid ${name}`);
  return id;
}

/** Optional `?name=<iso instant>` filter; null when absent. */
export function instantQuery(req: Request, name: string): string | null {
  const raw = req.query[name];
  if (raw == null || raw === "") return null;
  if (typeof raw !== "string" || Number.isNaN(Date.parse(raw))) {
    throw invalid(`invalid ${name}`);
  }
  return new Date(raw).toISOString();
}

export function requireBody(req: Request): Record<string, any> {
  if (!req.body || typeof req.body !== "object")
    throw invalid("invalid json body");
  return req.body;
}

/** Throws a 400 naming the first missing key. */
export function requireKeys(body: Record<string, any>, keys: string[]): void {
  const missing = keys.find((key) => body[key] == null || body[key] === "");
  if (missing) throw invalid(`${missing} is required`);
}

export function orThrow<T>(value: T | null | undefined, error: HttpError): T {
  if (value == null) throw error;
  return value;
}

export function sendError(res: Response, error: unknown): void {
  if (error instanceof HttpError) {
    res.status(error.status).json({ error: error.message });
    return;
  }
  const message = errorMessage(error);
  res
    .status(message.includes("not found") ? 404 : 500)
    .json({ error: message });
}
