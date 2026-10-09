import {
  ErrInvalidFilter,
  ErrInvalidSide,
  ErrInvalidSort,
  withCode,
} from "../../domain/errors.js";
import { ruleNeedsValue, validRuleOperator } from "../../domain/types.js";
import type { Filter, Order } from "../../connectors/types.js";

export type Side = "source" | "destination";

export type FilterInput = {
  field?: string;
  operator?: string;
  value?: string;
};

export type SortedOrder = Order & { sortBy: string; sortDir: string };

export function parseSide(raw: string): Side {
  const side = raw.trim().toLowerCase();
  if (side !== "source" && side !== "destination") throw ErrInvalidSide;
  return side;
}

export function parseSort(sortBy: string, sortDir: string): SortedOrder | null {
  const column = sortBy.trim();
  const dir = sortDir.trim().toLowerCase() || "asc";
  if (!column) return null;
  if (dir !== "asc" && dir !== "desc") throw ErrInvalidSort;
  return { column, desc: dir === "desc", sortBy: column, sortDir: dir };
}

export function parseFilters(input: FilterInput[] | undefined): Filter[] {
  return (input ?? []).map((f) => {
    const field = (f.field ?? "").trim();
    const operator = (f.operator ?? "").trim();
    if (!field) throw invalidFilter("field is required");
    if (!validRuleOperator(operator))
      throw invalidFilter(`unsupported operator ${JSON.stringify(operator)}`);
    const needsValue = ruleNeedsValue(operator);
    const value = needsValue ? (f.value ?? "") : "";
    if (needsValue && !value.trim())
      throw invalidFilter(
        `value is required for operator ${JSON.stringify(operator)}`,
      );
    return {
      column: field,
      operator,
      value: operator === "like" ? containsLike(value) : value,
    };
  });
}

/** A like value with no `%` matches anywhere. A value that already has `%` stays a pattern. */
function containsLike(value: string): string {
  return value.includes("%") ? value : `%${value}%`;
}

function invalidFilter(message: string) {
  return withCode(ErrInvalidFilter, `invalid explore filter: ${message}`);
}
