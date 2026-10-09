import type { Document } from "mongodb";
import type { Filter, Order } from "../types.js";
import {
  coerceFilterValue,
  idFilterColumn,
  likeToRegex,
  sortColumn,
} from "../shared/filters.js";
import { DEFAULT_SORTABLE_ID } from "../primaryKey.js";
import { splitList } from "../../utils/values.js";

const COMPARISONS: Record<string, string> = {
  neq: "$ne",
  gt: "$gt",
  gte: "$gte",
  lt: "$lt",
  lte: "$lte",
};

/** Builds a Mongo query; `id` maps to `_id` (equality) or the numeric sortable id (ranges). */
export function buildMongoFilter(
  filters: Filter[],
  sortableId: string,
): Document {
  const clauses = filters.map((filter) => clause(filter, sortableId));
  if (clauses.length === 0) return {};
  return clauses.length === 1 ? clauses[0] : { $and: clauses };
}

function clause(filter: Filter, sortableId: string): Document {
  const column = idFilterColumn(
    filter.column,
    filter.operator,
    "_id",
    sortableId || DEFAULT_SORTABLE_ID,
  );
  if (!column) throw new Error("filter field is required");
  const value = (raw: string) =>
    column === "_id" ? raw : coerceFilterValue(raw);

  const op = COMPARISONS[filter.operator];
  if (op) return { [column]: { [op]: value(filter.value) } };
  switch (filter.operator) {
    case "eq":
      return { [column]: value(filter.value) };
    case "in":
    case "not_in": {
      const values = splitList(filter.value).map(value);
      if (!values.length)
        throw new Error(
          `operator ${JSON.stringify(filter.operator)} requires at least one value`,
        );
      return {
        [column]: { [filter.operator === "in" ? "$in" : "$nin"]: values },
      };
    }
    case "like":
      return { [column]: { $regex: likeToRegex(filter.value).source } };
    case "is_null":
      return { [column]: null };
    case "is_not_null":
      return { [column]: { $exists: true, $ne: null } };
    default:
      throw new Error(
        `unsupported filter operator ${JSON.stringify(filter.operator)}`,
      );
  }
}

export function findOptions(
  limit: number,
  offset: number,
  order: Order | null,
  sortableId: string,
) {
  const options: { skip: number; limit: number; sort?: Document } = {
    skip: Math.max(0, offset),
    limit: limit <= 0 ? 50 : limit,
  };
  if (order?.column.trim())
    options.sort = {
      [sortColumn(order.column, sortableId)]: order.desc ? -1 : 1,
    };
  return options;
}
