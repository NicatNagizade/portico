import { asc, eq } from "drizzle-orm";
import type { Db } from "../db/client.js";
import { syncJobRules } from "../db/schema.js";
import type { Rule } from "../domain/types.js";
import { now } from "../utils/values.js";
import { toRule } from "./mappers.js";

type RuleRow = Omit<Rule, "id" | "created_at" | "updated_at">;

function columns(rule: RuleRow) {
  return {
    field: rule.field,
    operator: rule.operator,
    value: rule.value,
    active: rule.active,
  };
}

export async function listRules(db: Db, jobId: number): Promise<Rule[]> {
  const rows = await db
    .select()
    .from(syncJobRules)
    .where(eq(syncJobRules.sync_job_id, jobId))
    .orderBy(asc(syncJobRules.id));
  return rows.map(toRule);
}

export async function insertRule(db: Db, rule: RuleRow): Promise<number> {
  const at = now();
  const [row] = await db
    .insert(syncJobRules)
    .values({
      sync_job_id: rule.sync_job_id,
      ...columns(rule),
      created_at: at,
      updated_at: at,
    })
    .returning({ id: syncJobRules.id });
  return row.id;
}

export async function updateRule(db: Db, rule: Rule): Promise<void> {
  await db
    .update(syncJobRules)
    .set({ ...columns(rule), updated_at: now() })
    .where(eq(syncJobRules.id, rule.id));
}

export async function deleteRule(db: Db, id: number): Promise<void> {
  await db.delete(syncJobRules).where(eq(syncJobRules.id, id));
}
