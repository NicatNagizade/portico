import type { Row } from "../db/client.js";
import type {
  Connection,
  Field,
  FieldValue,
  Relation,
  Rule,
  SyncJob,
  SyncLog,
} from "../domain/types.js";
import { open } from "../utils/secretbox.js";
import { asObject, bool, iso, isoOrNull, num } from "../utils/values.js";

const text = (value: unknown) => String(value ?? "");
const idOrNull = (value: unknown) => (value == null ? null : Number(value));

function timestamps(row: Row) {
  return { created_at: iso(row.created_at), updated_at: iso(row.updated_at) };
}

export function toConnection(row: Row): Connection {
  return {
    id: Number(row.id),
    name: text(row.name),
    type: text(row.type),
    config: open(asObject(row.config) ?? {}),
    ...timestamps(row),
  };
}

export function toFieldValue(row: Row): FieldValue {
  return {
    id: Number(row.id),
    sync_job_field_id: Number(row.sync_job_field_id),
    source_value: text(row.source_value),
    destination_value: text(row.destination_value),
    ...timestamps(row),
  };
}

export function toField(row: Row, values: FieldValue[] = []): Field {
  return {
    id: Number(row.id),
    sync_job_id: Number(row.sync_job_id),
    sync_job_relation_id: idOrNull(row.sync_job_relation_id),
    source_name: text(row.source_name),
    destination_name: text(row.destination_name),
    destination_type: text(row.destination_type),
    destination_config: asObject(row.destination_config),
    active: bool(row.active),
    values,
    ...timestamps(row),
  };
}

export function toRelation(row: Row, fields: Field[] = []): Relation {
  return {
    id: Number(row.id),
    sync_job_id: Number(row.sync_job_id),
    parent_id: idOrNull(row.parent_id),
    name: text(row.name),
    type: text(row.type),
    table: text(row.table),
    foreign_key: text(row.foreign_key),
    related_key: text(row.related_key),
    config: asObject(row.config),
    active: bool(row.active),
    fields,
    relations: [],
    ...timestamps(row),
  };
}

export function toRule(row: Row): Rule {
  return {
    id: Number(row.id),
    sync_job_id: Number(row.sync_job_id),
    field: text(row.field),
    operator: text(row.operator),
    value: text(row.value),
    active: bool(row.active),
    ...timestamps(row),
  };
}

export function toJob(row: Row): SyncJob {
  return {
    id: Number(row.id),
    name: text(row.name),
    source_connection_id: Number(row.source_connection_id),
    source_table: text(row.source_table),
    destination_connection_id: Number(row.destination_connection_id),
    destination_table: text(row.destination_table),
    chunk_size: Number(row.chunk_size ?? 500),
    workers: Number(row.workers ?? 2),
    config: asObject(row.config),
    relations: [],
    fields: [],
    rules: [],
    ...timestamps(row),
  };
}

export function toLog(row: Row, job?: SyncJob): SyncLog {
  return {
    id: Number(row.id),
    sync_job_id: Number(row.sync_job_id),
    status: text(row.status),
    message: text(row.message),
    rows_total: num(row.rows_total),
    rows_synced: num(row.rows_synced),
    duration_ms: num(row.duration_ms),
    started_at: iso(row.started_at),
    finished_at: isoOrNull(row.finished_at),
    ...(job ? { sync_job: job } : {}),
  };
}
