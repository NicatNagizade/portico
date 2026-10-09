import { asc, eq, inArray } from "drizzle-orm";
import type { Db } from "../db/client.js";
import { syncJobFields, syncJobFieldValues } from "../db/schema.js";
import type { Field, FieldValue } from "../domain/types.js";
import { groupBy } from "../utils/collections.js";
import { now } from "../utils/values.js";
import { toField, toFieldValue } from "./mappers.js";

type FieldRow = Omit<Field, "id" | "values" | "created_at" | "updated_at">;

function columns(field: FieldRow) {
  return {
    sync_job_relation_id: field.sync_job_relation_id,
    source_name: field.source_name,
    destination_name: field.destination_name,
    destination_type: field.destination_type,
    destination_config: field.destination_config,
    active: field.active,
  };
}

/** All fields of a job with their value maps; `rootOnly` null means root and relation fields. */
export async function listFields(
  db: Db,
  jobId: number,
  rootOnly: boolean | null = null,
): Promise<Field[]> {
  const fieldRows = await db
    .select()
    .from(syncJobFields)
    .where(eq(syncJobFields.sync_job_id, jobId))
    .orderBy(asc(syncJobFields.id));
  const valueRows = await db
    .select()
    .from(syncJobFieldValues)
    .where(
      inArray(
        syncJobFieldValues.sync_job_field_id,
        db
          .select({ id: syncJobFields.id })
          .from(syncJobFields)
          .where(eq(syncJobFields.sync_job_id, jobId)),
      ),
    )
    .orderBy(asc(syncJobFieldValues.id));
  const valuesByField = groupBy(
    valueRows.map(toFieldValue),
    (value) => value.sync_job_field_id,
  );
  return fieldRows
    .map((row) => toField(row, valuesByField.get(row.id) ?? []))
    .filter(
      (field) =>
        rootOnly == null || rootOnly === (field.sync_job_relation_id == null),
    );
}

export async function insertField(db: Db, field: FieldRow): Promise<number> {
  const at = now();
  const [row] = await db
    .insert(syncJobFields)
    .values({
      sync_job_id: field.sync_job_id,
      ...columns(field),
      created_at: at,
      updated_at: at,
    })
    .returning({ id: syncJobFields.id });
  return row.id;
}

export async function updateField(db: Db, field: Field): Promise<void> {
  await db
    .update(syncJobFields)
    .set({ ...columns(field), updated_at: now() })
    .where(eq(syncJobFields.id, field.id));
}

export async function deleteField(db: Db, id: number): Promise<void> {
  await db
    .delete(syncJobFieldValues)
    .where(eq(syncJobFieldValues.sync_job_field_id, id));
  await db.delete(syncJobFields).where(eq(syncJobFields.id, id));
}

export async function listFieldValues(
  db: Db,
  fieldId: number,
): Promise<FieldValue[]> {
  const rows = await db
    .select()
    .from(syncJobFieldValues)
    .where(eq(syncJobFieldValues.sync_job_field_id, fieldId))
    .orderBy(asc(syncJobFieldValues.id));
  return rows.map(toFieldValue);
}

export async function insertFieldValue(
  db: Db,
  fieldId: number,
  source: string,
  destination: string,
): Promise<number> {
  const at = now();
  const [row] = await db
    .insert(syncJobFieldValues)
    .values({
      sync_job_field_id: fieldId,
      source_value: source,
      destination_value: destination,
      created_at: at,
      updated_at: at,
    })
    .returning({ id: syncJobFieldValues.id });
  return row.id;
}

export async function updateFieldValue(
  db: Db,
  id: number,
  source: string,
  destination: string,
): Promise<void> {
  await db
    .update(syncJobFieldValues)
    .set({
      source_value: source,
      destination_value: destination,
      updated_at: now(),
    })
    .where(eq(syncJobFieldValues.id, id));
}

export async function deleteFieldValue(db: Db, id: number): Promise<void> {
  await db.delete(syncJobFieldValues).where(eq(syncJobFieldValues.id, id));
}
