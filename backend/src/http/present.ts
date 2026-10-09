import type {
  Connection,
  Field,
  FieldValue,
  Relation,
  Rule,
  SyncJob,
  SyncLog,
} from "../domain/types.js";
import { childrenByParent } from "../services/sync/relations/tree.js";
import { redact } from "../utils/secretbox.js";

/** `{ [key]: value }` when value is set, otherwise `{}` — for optional JSON keys. */
function optional(key: string, value: unknown, when = Boolean(value)) {
  return when ? { [key]: value } : {};
}

export function presentConnection(c: Connection) {
  return {
    id: c.id,
    name: c.name,
    type: c.type,
    config: redact(c.config),
    created_at: c.created_at,
    updated_at: c.updated_at,
  };
}

function presentValue(v: FieldValue) {
  return {
    id: v.id,
    sync_job_field_id: v.sync_job_field_id,
    source_value: v.source_value,
    destination_value: v.destination_value,
    created_at: v.created_at,
    updated_at: v.updated_at,
  };
}

function presentField(f: Field) {
  return {
    id: f.id,
    sync_job_id: f.sync_job_id,
    ...optional(
      "sync_job_relation_id",
      f.sync_job_relation_id,
      f.sync_job_relation_id != null,
    ),
    source_name: f.source_name,
    destination_name: f.destination_name,
    destination_type: f.destination_type,
    ...optional("destination_config", f.destination_config),
    active: f.active,
    ...optional("values", f.values.map(presentValue), f.values.length > 0),
    created_at: f.created_at,
    updated_at: f.updated_at,
  };
}

function presentRelation(
  rel: Relation,
  children: Map<number, Relation[]>,
): Record<string, unknown> {
  const nested = children.get(rel.id) ?? [];
  return {
    id: rel.id,
    sync_job_id: rel.sync_job_id,
    name: rel.name,
    type: rel.type,
    table: rel.table,
    foreign_key: rel.foreign_key,
    related_key: rel.related_key,
    ...optional("config", rel.config),
    active: rel.active,
    ...optional("fields", rel.fields.map(presentField), rel.fields.length > 0),
    ...optional(
      "relations",
      nested.map((child) => presentRelation(child, children)),
      nested.length > 0,
    ),
    created_at: rel.created_at,
    updated_at: rel.updated_at,
  };
}

function presentRule(rule: Rule) {
  return {
    id: rule.id,
    sync_job_id: rule.sync_job_id,
    field: rule.field,
    operator: rule.operator,
    value: rule.value,
    active: rule.active,
    created_at: rule.created_at,
    updated_at: rule.updated_at,
  };
}

/** `full` adds the nested relations, field overrides, and rules. */
export function presentJob(job: SyncJob, full: boolean) {
  const children = childrenByParent(job.relations);
  const roots = job.relations.filter((rel) => rel.parent_id == null);
  return {
    id: job.id,
    name: job.name,
    source_connection_id: job.source_connection_id,
    source_table: job.source_table,
    destination_connection_id: job.destination_connection_id,
    destination_table: job.destination_table,
    chunk_size: job.chunk_size,
    workers: job.workers,
    ...optional("config", job.config),
    ...optional(
      "source_connection",
      job.source_connection && presentConnection(job.source_connection),
    ),
    ...optional(
      "destination_connection",
      job.destination_connection &&
        presentConnection(job.destination_connection),
    ),
    ...optional(
      "relations",
      roots.map((rel) => presentRelation(rel, children)),
      full && roots.length > 0,
    ),
    ...optional(
      "fields",
      job.fields.map(presentField),
      full && job.fields.length > 0,
    ),
    ...optional(
      "rules",
      job.rules.map(presentRule),
      full && job.rules.length > 0,
    ),
    created_at: job.created_at,
    updated_at: job.updated_at,
  };
}

export function presentLog(log: SyncLog, withJob = false) {
  return {
    id: log.id,
    sync_job_id: log.sync_job_id,
    status: log.status,
    message: log.message,
    rows_total: log.rows_total,
    rows_synced: log.rows_synced,
    duration_ms: log.duration_ms,
    started_at: log.started_at,
    finished_at: log.finished_at,
    ...optional(
      "sync_job",
      log.sync_job && presentJob(log.sync_job, false),
      withJob && log.sync_job != null,
    ),
  };
}
