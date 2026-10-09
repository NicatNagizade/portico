import type { Field } from "../../domain/types.js";
import { asObject } from "../../utils/values.js";

export type ValueInput = {
  id?: number;
  source_value: string;
  destination_value: string;
};

export type FieldInput = {
  id?: number;
  sync_job_relation_id?: number;
  relation?: string;
  source_name: string;
  destination_name?: string;
  destination_type?: string;
  destination_config?: unknown;
  active?: boolean;
  values?: ValueInput[];
};

export type RelationInput = {
  id?: number;
  name: string;
  type: string;
  table: string;
  foreign_key?: string;
  related_key?: string;
  config?: unknown;
  active?: boolean;
  fields?: FieldInput[];
  relations?: RelationInput[];
};

export type RuleInput = {
  id?: number;
  field: string;
  operator: string;
  value?: string;
  active?: boolean;
};

export type JobInput = {
  name: string;
  source_connection_id: number;
  source_table: string;
  destination_connection_id: number;
  destination_table: string;
  chunk_size?: number;
  workers?: number;
  config?: unknown;
  relations?: RelationInput[];
  fields?: FieldInput[];
  rules?: RuleInput[];
};

/** Saved relation ids, looked up by client id or by relation name. */
export type RelationIds = {
  byID: Map<number, number>;
  byName: Map<string, number>;
};

export function configObject(raw: unknown): Record<string, unknown> | null {
  return raw == null ? null : asObject(raw);
}

export function pivotTable(config: Record<string, unknown> | null): string {
  return typeof config?.pivot_table === "string" ? config.pivot_table : "";
}

/** Flattens `relations[].fields[]` into field inputs tagged with their relation name. */
export function fieldsFromRelations(inputs: RelationInput[]): FieldInput[] {
  return inputs.flatMap((relation) => [
    ...(relation.fields ?? []).map((field) => ({
      ...field,
      relation: relation.name,
      sync_job_relation_id: undefined,
    })),
    ...fieldsFromRelations(relation.relations ?? []),
  ]);
}

export function fieldToInput(field: Field, relation?: string): FieldInput {
  return {
    id: field.id,
    ...(relation || field.sync_job_relation_id == null
      ? {}
      : { sync_job_relation_id: field.sync_job_relation_id }),
    ...(relation ? { relation } : {}),
    source_name: field.source_name,
    destination_name: field.destination_name,
    destination_type: field.destination_type,
    destination_config: field.destination_config,
    active: field.active,
    values: field.values.map((v) => ({
      id: v.id,
      source_value: v.source_value,
      destination_value: v.destination_value,
    })),
  };
}
