import fs from "node:fs";
import type { Db } from "../db/client.js";
import {
  ErrAmbiguous,
  ErrConnectionNotFound,
  invalidJob,
  withCode,
} from "../domain/errors.js";
import * as connectionsRepo from "../repositories/connections.js";
import * as jobsRepo from "../repositories/syncJobs.js";
import { createJob, updateJob, type JobInput } from "./syncJobs/index.js";
import { merge } from "../utils/secretbox.js";
import { errorMessage, positiveOr } from "../utils/values.js";

type FileConnection = {
  name?: string;
  type?: string;
  config?: Record<string, unknown>;
};

type FileJob = Partial<
  Omit<JobInput, "source_connection_id" | "destination_connection_id">
> & {
  source_connection?: string;
  destination_connection?: string;
};

type Result = {
  name: string;
  type: string;
  id: number;
  action: "created" | "updated";
};

/** Upserts connections (by name + type) and sync jobs (by name) from a JSON file. */
export async function importConnections(
  db: Db,
  path: string,
): Promise<{ connections: Result[]; syncJobs: Result[] }> {
  const raw = JSON.parse(fs.readFileSync(path, "utf8")) as {
    connections?: FileConnection[];
    sync_jobs?: FileJob[];
  };
  const connections = raw.connections ?? [];
  const jobs = raw.sync_jobs ?? [];
  if (connections.length === 0 && jobs.length === 0)
    throw new Error(`${path}: no connections or sync_jobs to import`);

  const out = { connections: [] as Result[], syncJobs: [] as Result[] };
  for (const [i, input] of connections.entries())
    out.connections.push(
      await importConnection(db, input, `${path}: connections[${i}]`),
    );
  for (const [i, input] of jobs.entries())
    out.syncJobs.push(await importJob(db, input, `${path}: sync_jobs[${i}]`));
  return out;
}

async function importConnection(
  db: Db,
  input: FileConnection,
  where: string,
): Promise<Result> {
  if (!input.name || !input.type || input.config == null)
    throw new Error(`${where}: name, type, and config are required`);

  const found = await connectionsRepo.findConnections(
    db,
    input.name,
    input.type,
  );
  if (found.length > 1) throw ErrAmbiguous;
  if (found.length === 0) {
    const created = await connectionsRepo.insertConnection(
      db,
      input.name,
      input.type,
      input.config,
    );
    return { ...pick(created), action: "created" };
  }

  const existing = found[0];
  existing.config = merge(existing.config, input.config);
  const updated = await connectionsRepo.saveConnection(db, existing);
  return { ...pick(updated), action: "updated" };
}

async function importJob(
  db: Db,
  input: FileJob,
  where: string,
): Promise<Result> {
  if (
    !input.name ||
    !input.source_connection ||
    !input.destination_connection ||
    !input.source_table ||
    !input.destination_table
  )
    throw new Error(
      `${where}: name, source_connection, destination_connection, source_table, and destination_table are required`,
    );

  const label = `${where} ${JSON.stringify(input.name)}`;
  const body: JobInput = {
    name: input.name,
    source_connection_id: await connectionId(
      db,
      input.source_connection,
      `${label}: source_connection`,
    ),
    destination_connection_id: await connectionId(
      db,
      input.destination_connection,
      `${label}: destination_connection`,
    ),
    source_table: input.source_table,
    destination_table: input.destination_table,
    chunk_size: positiveOr(input.chunk_size, 500),
    workers: positiveOr(input.workers, 2),
    config: input.config,
    relations: input.relations ?? [],
    fields: input.fields ?? [],
    rules: input.rules ?? [],
  };

  const existing = await jobsRepo.findJobsByName(db, input.name);
  if (existing.length > 1)
    throw invalidJob(`multiple sync jobs named ${JSON.stringify(input.name)}`);
  if (existing.length === 0) {
    const created = await createJob(db, body);
    return {
      name: created.name,
      type: "sync_job",
      id: created.id,
      action: "created",
    };
  }
  const updated = await updateJob(db, existing[0].id, body);
  return {
    name: updated.name,
    type: "sync_job",
    id: updated.id,
    action: "updated",
  };
}

/** The id of the only connection with this name. */
async function connectionId(
  db: Db,
  name: string,
  label: string,
): Promise<number> {
  try {
    const found = await connectionsRepo.findConnections(db, name);
    if (found.length === 0)
      throw withCode(
        ErrConnectionNotFound,
        `${JSON.stringify(name)}: connection not found`,
      );
    if (found.length > 1) throw ErrAmbiguous;
    return found[0].id;
  } catch (error) {
    throw new Error(`${label} ${JSON.stringify(name)}: ${errorMessage(error)}`);
  }
}

function pick(c: { name: string; type: string; id: number }) {
  return { name: c.name, type: c.type, id: c.id };
}
