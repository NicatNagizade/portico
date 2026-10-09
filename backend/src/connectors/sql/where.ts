import type { Filter, RelationSubquery } from "../types.js";
import { coerceFilterValue } from "../shared/filters.js";
import { splitList } from "../../utils/values.js";
import type { SqlStyle } from "./connection.js";

export type Quote = (name: string) => string;

export type Dialect = {
  style: SqlStyle;
  quote: Quote;
  quoteTable: Quote;
};

export function quoteIdent(name: string, mark: string): string {
  return mark + name.replaceAll(mark, mark + mark) + mark;
}

/** Collects bound parameters and returns the matching placeholder (`$n` or `?`). */
export class Binder {
  params: unknown[] = [];
  constructor(readonly style: SqlStyle) {}

  add(value: unknown): string {
    this.params.push(value);
    return this.style === "pg" ? `$${this.params.length}` : "?";
  }
}

export function whereSQL(
  dialect: Dialect,
  filters: Filter[],
  binder = new Binder(dialect.style),
): { sql: string; params: unknown[] } {
  const parts = filters.map((filter) => {
    const column = filter.column.trim();
    if (!column) throw new Error("filter field is required");
    const quoted = dialect.quote(column);
    return filter.rel
      ? `${quoted} IN (${subquery(dialect, filter.rel, binder)})`
      : predicate(binder, quoted, filter);
  });
  return { sql: parts.join(" AND "), params: binder.params };
}

export function withWhere(sql: string, where: { sql: string }): string {
  return where.sql ? `${sql} WHERE ${where.sql}` : sql;
}

function subquery(
  dialect: Dialect,
  rel: RelationSubquery,
  binder: Binder,
): string {
  if (!rel.table || !rel.select)
    throw new Error("relation subquery requires table and select");
  const select = `SELECT ${dialect.quote(rel.select)} FROM ${dialect.quoteTable(rel.table)}`;
  return withWhere(select, whereSQL(dialect, rel.where, binder));
}

const COMPARISONS: Record<string, string> = {
  eq: "=",
  neq: "<>",
  gt: ">",
  gte: ">=",
  lt: "<",
  lte: "<=",
};

function predicate(binder: Binder, column: string, filter: Filter): string {
  const op = COMPARISONS[filter.operator];
  if (op)
    return `${column} ${op} ${binder.add(coerceFilterValue(filter.value))}`;
  switch (filter.operator) {
    case "like":
      return `${column} LIKE ${binder.add(filter.value)}`;
    case "in":
    case "not_in": {
      const values = splitList(filter.value).map(coerceFilterValue);
      if (values.length === 0)
        throw new Error(
          `operator ${JSON.stringify(filter.operator)} requires at least one value`,
        );
      const list = values.map((v) => binder.add(v)).join(", ");
      return `${column} ${filter.operator === "in" ? "IN" : "NOT IN"} (${list})`;
    }
    case "is_null":
      return `${column} IS NULL`;
    case "is_not_null":
      return `${column} IS NOT NULL`;
    default:
      throw new Error(
        `unsupported filter operator ${JSON.stringify(filter.operator)}`,
      );
  }
}
