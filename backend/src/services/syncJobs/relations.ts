import type { Db } from "../../db/client.js";
import { invalidJob } from "../../domain/errors.js";
import type { Relation } from "../../domain/types.js";
import * as relationsRepo from "../../repositories/relations.js";
import {
  configObject,
  type RelationIds,
  type RelationInput,
} from "./inputs.js";
import { existingId, removeMissing } from "./reconcile.js";
import { validateRelations } from "./validation.js";

export async function upsertRelations(
  db: Db,
  jobId: number,
  inputs: RelationInput[],
): Promise<RelationIds> {
  validateRelations(inputs);
  const existing = new Set(
    (await relationsRepo.listRelations(db, jobId)).map((r) => r.id),
  );
  const ids: RelationIds = {
    byID: new Map([...existing].map((id) => [id, id])),
    byName: new Map(),
  };
  const keep = new Set<number>();

  const save = async (parentId: number | null, nodes: RelationInput[]) => {
    for (const input of nodes) {
      if ((input.id ?? 0) < 0)
        throw invalidJob("relation id must be a positive existing id");
      const relation = toRelation(jobId, parentId, input);
      const id = existingId("relation", existing, input.id);
      if (id) {
        relation.id = id;
        await relationsRepo.updateRelation(db, relation);
      } else {
        relation.id = await relationsRepo.insertRelation(db, relation);
      }
      keep.add(relation.id);
      ids.byID.set(id ?? relation.id, relation.id);
      ids.byName.set(relation.name, relation.id);
      await save(relation.id, input.relations ?? []);
    }
  };

  await save(null, inputs);
  await removeMissing(existing, keep, (id) =>
    relationsRepo.deleteRelation(db, id),
  );
  return ids;
}

export async function existingRelationIds(
  db: Db,
  jobId: number,
): Promise<RelationIds> {
  const existing = await relationsRepo.listRelations(db, jobId);
  return {
    byID: new Map(existing.map((r) => [r.id, r.id])),
    byName: new Map(existing.map((r) => [r.name, r.id])),
  };
}

function toRelation(
  jobId: number,
  parentId: number | null,
  input: RelationInput,
): Relation {
  return {
    id: 0,
    sync_job_id: jobId,
    parent_id: parentId,
    name: input.name,
    type: input.type,
    table: input.table,
    foreign_key: input.foreign_key ?? "",
    related_key: input.related_key ?? "",
    config: configObject(input.config),
    active: input.active ?? true,
    fields: [],
    relations: [],
    created_at: "",
    updated_at: "",
  };
}
