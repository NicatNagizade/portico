import type { Db } from "../../db/client.js";
import { invalidJob } from "../../domain/errors.js";
import type { Field } from "../../domain/types.js";
import * as fieldsRepo from "../../repositories/fields.js";
import {
  configObject,
  type FieldInput,
  type RelationIds,
  type ValueInput,
} from "./inputs.js";
import { reconcile } from "./reconcile.js";
import { validateField } from "./validation.js";

export async function upsertFields(
  db: Db,
  jobId: number,
  inputs: FieldInput[],
  relations: RelationIds,
): Promise<void> {
  const existing = await fieldsRepo.listFields(db, jobId);
  await reconcile(
    "field",
    new Set(existing.map((f) => f.id)),
    inputs,
    async (input, id) => {
      validateField(input);
      const field = toField(jobId, input, relationId(input, relations));
      if (id) {
        field.id = id;
        await fieldsRepo.updateField(db, field);
      } else {
        field.id = await fieldsRepo.insertField(db, field);
      }
      await upsertFieldValues(db, field.id, input.values ?? []);
      return field.id;
    },
    (id) => fieldsRepo.deleteField(db, id),
  );
}

async function upsertFieldValues(
  db: Db,
  fieldId: number,
  inputs: ValueInput[],
): Promise<void> {
  const existing = await fieldsRepo.listFieldValues(db, fieldId);
  const seen = new Set<string>();
  await reconcile(
    "field value",
    new Set(existing.map((v) => v.id)),
    inputs,
    async (input, id) => {
      const source = input.source_value.trim();
      if (!source) throw invalidJob("field value source_value is required");
      if (seen.has(source))
        throw invalidJob(
          `duplicate field value source_value ${JSON.stringify(source)}`,
        );
      seen.add(source);
      if (!id)
        return fieldsRepo.insertFieldValue(
          db,
          fieldId,
          source,
          input.destination_value,
        );
      await fieldsRepo.updateFieldValue(
        db,
        id,
        source,
        input.destination_value,
      );
      return id;
    },
    (id) => fieldsRepo.deleteFieldValue(db, id),
  );
}

/** Resolves a field's relation by name or by client id to the saved relation id. */
function relationId(input: FieldInput, relations: RelationIds): number | null {
  if (input.relation) {
    const id = relations.byName.get(input.relation);
    if (id == null)
      throw invalidJob(`relation ${JSON.stringify(input.relation)} not found`);
    return id;
  }
  if (input.sync_job_relation_id == null) return null;
  if (input.sync_job_relation_id <= 0)
    throw invalidJob(
      "sync_job_relation_id must be a positive existing id (use relation by name)",
    );
  const id = relations.byID.get(input.sync_job_relation_id);
  if (id == null)
    throw invalidJob(
      `sync_job_relation_id ${input.sync_job_relation_id} not found`,
    );
  return id;
}

function toField(
  jobId: number,
  input: FieldInput,
  relationId: number | null,
): Field {
  return {
    id: 0,
    sync_job_id: jobId,
    sync_job_relation_id: relationId,
    source_name: input.source_name,
    destination_name: input.destination_name ?? "",
    destination_type: input.destination_type ?? "",
    destination_config: configObject(input.destination_config),
    active: input.active ?? true,
    values: [],
    created_at: "",
    updated_at: "",
  };
}
