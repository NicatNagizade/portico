import type { Db } from "../../db/client.js";
import { ErrSyncJobNotFound } from "../../domain/errors.js";
import type { SyncJob } from "../../domain/types.js";
import * as fieldsRepo from "../../repositories/fields.js";
import * as jobsRepo from "../../repositories/syncJobs.js";
import { positiveOr } from "../../utils/values.js";
import { upsertFields } from "./fields.js";
import {
  configObject,
  fieldsFromRelations,
  fieldToInput,
  type FieldInput,
  type JobInput,
  type RelationIds,
  type RelationInput,
  type RuleInput,
} from "./inputs.js";
import { existingRelationIds, upsertRelations } from "./relations.js";
import { upsertRules } from "./rules.js";

export type {
  FieldInput,
  JobInput,
  RelationInput,
  RuleInput,
} from "./inputs.js";

export async function createJob(db: Db, input: JobInput): Promise<SyncJob> {
  const id = await db.transaction(async (tx) => {
    const jobId = await jobsRepo.insertJob(tx, {
      name: input.name,
      source_connection_id: input.source_connection_id,
      source_table: input.source_table,
      destination_connection_id: input.destination_connection_id,
      destination_table: input.destination_table,
      chunk_size: positiveOr(input.chunk_size, 500),
      workers: positiveOr(input.workers, 2),
      config: configObject(input.config),
    });
    const relations = input.relations ?? [];
    const relationIds = await upsertRelations(tx, jobId, relations);
    await upsertFields(
      tx,
      jobId,
      [...(input.fields ?? []), ...fieldsFromRelations(relations)],
      relationIds,
    );
    await upsertRules(tx, jobId, input.rules ?? []);
    return jobId;
  });
  return (await jobsRepo.getJob(db, id))!;
}

/** Partial update: only keys present in `body` are changed. */
export async function updateJob(
  db: Db,
  id: number,
  body: Partial<JobInput>,
): Promise<SyncJob> {
  const job = await jobsRepo.getJob(db, id);
  if (!job) throw ErrSyncJobNotFound;
  applyJobColumns(job, body);

  await db.transaction(async (tx) => {
    await jobsRepo.updateJob(tx, job);
    const relationIds =
      body.relations !== undefined
        ? await upsertRelations(tx, id, body.relations ?? [])
        : await existingRelationIds(tx, id);
    if (body.fields !== undefined || body.relations !== undefined)
      await upsertFields(
        tx,
        id,
        await fieldInputs(tx, id, body, relationIds),
        relationIds,
      );
    if (body.rules !== undefined) await upsertRules(tx, id, body.rules ?? []);
  });
  return (await jobsRepo.getJob(db, id))!;
}

function applyJobColumns(job: SyncJob, body: Partial<JobInput>): void {
  if ("name" in body) job.name = String(body.name ?? "");
  if ("source_connection_id" in body)
    job.source_connection_id = Number(body.source_connection_id);
  if ("source_table" in body)
    job.source_table = String(body.source_table ?? "");
  if ("destination_connection_id" in body)
    job.destination_connection_id = Number(body.destination_connection_id);
  if ("destination_table" in body)
    job.destination_table = String(body.destination_table ?? "");
  if ("chunk_size" in body) job.chunk_size = Number(body.chunk_size);
  if ("workers" in body) job.workers = Number(body.workers);
  if ("config" in body) job.config = configObject(body.config);
}

/**
 * Root and relation fields are saved together, so a body that only changes one
 * side keeps the other side's stored fields.
 */
async function fieldInputs(
  db: Db,
  jobId: number,
  body: { fields?: FieldInput[]; relations?: RelationInput[] },
  relationIds: RelationIds,
): Promise<FieldInput[]> {
  const root =
    body.fields !== undefined
      ? (body.fields ?? [])
      : (await fieldsRepo.listFields(db, jobId, true)).map((f) =>
          fieldToInput(f),
        );
  if (body.relations !== undefined)
    return [...root, ...fieldsFromRelations(body.relations ?? [])];

  const nameById = new Map(
    [...relationIds.byName].map(([name, relationId]) => [relationId, name]),
  );
  const nested = (await fieldsRepo.listFields(db, jobId, false)).map((f) =>
    fieldToInput(f, nameById.get(f.sync_job_relation_id!)),
  );
  return [...root, ...nested];
}
