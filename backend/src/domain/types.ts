export type Connection = {
  id: number;
  name: string;
  type: string;
  config: Record<string, unknown>;
  created_at: string;
  updated_at: string;
};

export type FieldValue = {
  id: number;
  sync_job_field_id: number;
  source_value: string;
  destination_value: string;
  created_at: string;
  updated_at: string;
};

export type Field = {
  id: number;
  sync_job_id: number;
  sync_job_relation_id: number | null;
  source_name: string;
  destination_name: string;
  destination_type: string;
  destination_config: Record<string, unknown> | null;
  active: boolean;
  values: FieldValue[];
  created_at: string;
  updated_at: string;
};

export type Relation = {
  id: number;
  sync_job_id: number;
  parent_id: number | null;
  name: string;
  type: string;
  table: string;
  foreign_key: string;
  related_key: string;
  config: Record<string, unknown> | null;
  active: boolean;
  fields: Field[];
  relations: Relation[];
  created_at: string;
  updated_at: string;
};

export type Rule = {
  id: number;
  sync_job_id: number;
  field: string;
  operator: string;
  value: string;
  active: boolean;
  created_at: string;
  updated_at: string;
};

export type SyncJob = {
  id: number;
  name: string;
  source_connection_id: number;
  source_table: string;
  destination_connection_id: number;
  destination_table: string;
  chunk_size: number;
  workers: number;
  config: Record<string, unknown> | null;
  source_connection?: Connection;
  destination_connection?: Connection;
  relations: Relation[];
  fields: Field[];
  rules: Rule[];
  created_at: string;
  updated_at: string;
};

export type SyncLog = {
  id: number;
  sync_job_id: number;
  status: string;
  message: string;
  rows_total: number | null;
  rows_synced: number | null;
  duration_ms: number | null;
  started_at: string;
  finished_at: string | null;
  sync_job?: SyncJob;
};

export const RULE_OPERATORS = [
  "eq",
  "neq",
  "gt",
  "gte",
  "lt",
  "lte",
  "in",
  "not_in",
  "like",
  "is_null",
  "is_not_null",
] as const;

export function validRuleOperator(op: string): boolean {
  return (RULE_OPERATORS as readonly string[]).includes(op);
}

export const LOG_STATUSES = [
  "running",
  "success",
  "failed",
  "stopped",
] as const;

export function validLogStatus(status: string): boolean {
  return (LOG_STATUSES as readonly string[]).includes(status);
}

export function ruleNeedsValue(op: string): boolean {
  return op !== "is_null" && op !== "is_not_null";
}

export function isActive(active: boolean | null | undefined): boolean {
  return active == null || active;
}
