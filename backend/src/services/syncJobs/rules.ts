import type { Db } from "../../db/client.js";
import * as rulesRepo from "../../repositories/rules.js";
import type { RuleInput } from "./inputs.js";
import { reconcile } from "./reconcile.js";
import { normalizeRule } from "./validation.js";

export async function upsertRules(
  db: Db,
  jobId: number,
  inputs: RuleInput[],
): Promise<void> {
  const existing = await rulesRepo.listRules(db, jobId);
  await reconcile(
    "rule",
    new Set(existing.map((r) => r.id)),
    inputs,
    async (input, id) => {
      const rule = {
        id: id ?? 0,
        sync_job_id: jobId,
        ...normalizeRule(input),
        active: input.active ?? true,
        created_at: "",
        updated_at: "",
      };
      if (!id) return rulesRepo.insertRule(db, rule);
      await rulesRepo.updateRule(db, rule);
      return id;
    },
    (id) => rulesRepo.deleteRule(db, id),
  );
}
