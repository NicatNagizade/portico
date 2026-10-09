import { ruleNeedsValue, validRuleOperator } from "../../domain/types.js";
import { invalidJob } from "../../domain/errors.js";
import {
  configObject,
  pivotTable,
  type FieldInput,
  type RelationInput,
  type RuleInput,
} from "./inputs.js";

const FIELD_TYPES = new Set([
  "string",
  "int64",
  "float64",
  "bool",
  "object",
  "object_array",
]);

const RELATION_TYPES = new Set(["has_many", "has_one", "belongs_to"]);

export function validateRelations(
  inputs: RelationInput[],
  names = new Set<string>(),
): void {
  for (const relation of inputs) {
    const name = JSON.stringify(relation.name);
    if (!relation.name) throw invalidJob("relation name is required");
    if (names.has(relation.name))
      throw invalidJob(`duplicate relation name ${name}`);
    names.add(relation.name);

    if (relation.type === "belongs_to_many") {
      if (!pivotTable(configObject(relation.config)))
        throw invalidJob(`config.pivot_table is required for relation ${name}`);
    } else if (!RELATION_TYPES.has(relation.type)) {
      throw invalidJob(
        `unsupported relation type ${JSON.stringify(relation.type)}`,
      );
    }
    validateRelations(relation.relations ?? [], names);
  }
}

export function validateField(input: FieldInput): void {
  if (!input.source_name) throw invalidJob("source_name is required");
  if (input.destination_type && !FIELD_TYPES.has(input.destination_type))
    throw invalidJob(
      `unsupported destination_type ${JSON.stringify(input.destination_type)}`,
    );
  if (input.relation && input.sync_job_relation_id != null)
    throw invalidJob("set relation or sync_job_relation_id, not both");
}

/** Validates a rule and returns its trimmed field and value (empty for null checks). */
export function normalizeRule(input: RuleInput) {
  const field = input.field.trim();
  if (!field) throw invalidJob("rule field is required");
  if (!validRuleOperator(input.operator))
    throw invalidJob(
      `unsupported rule operator ${JSON.stringify(input.operator)}`,
    );
  const needsValue = ruleNeedsValue(input.operator);
  const value = needsValue ? (input.value ?? "") : "";
  if (needsValue && !value.trim())
    throw invalidJob(
      `rule value is required for operator ${JSON.stringify(input.operator)}`,
    );
  return { field, operator: input.operator, value };
}
