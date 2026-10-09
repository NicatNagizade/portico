import type { Doc, Filter } from "../types.js";
import { splitList } from "../../utils/values.js";

/** In-memory filter matching for document stores; dotted columns walk nested docs/arrays. */
export function filterRows(rows: Doc[], filters: Filter[]): Doc[] {
  if (filters.length === 0) return rows;
  return rows.filter((row) => filters.every((f) => matchFilter(row, f)));
}

function matchFilter(row: Doc, f: Filter): boolean {
  const values = pathValues(row, f.column.split("."));
  if (f.operator === "is_null")
    return values.length === 0 || values.some((v) => v == null);
  if (f.operator === "is_not_null") return values.some((v) => v != null);
  if (values.length === 0) return false;

  const present = values.filter((v) => v != null);
  const equals = (want: string) => (raw: unknown) =>
    String(raw) === want || compareNumeric(raw, want) === 0;
  const inList = (raw: unknown) =>
    splitList(f.value).some((v) => equals(v)(raw));

  switch (f.operator) {
    case "eq":
      return present.some(equals(f.value));
    case "neq":
      return !present.some(equals(f.value));
    case "in":
      return present.some(inList);
    case "not_in":
      return !present.some(inList);
    case "like": {
      const pattern = likeToRegex(f.value);
      return present.some((raw) => pattern.test(String(raw)));
    }
    case "gt":
    case "gte":
    case "lt":
    case "lte":
      return present.some((raw) => {
        const n =
          compareNumeric(raw, f.value) ?? String(raw).localeCompare(f.value);
        if (f.operator === "gt") return n > 0;
        if (f.operator === "gte") return n >= 0;
        if (f.operator === "lt") return n < 0;
        return n <= 0;
      });
    default:
      return false;
  }
}

export function pathValues(value: unknown, parts: string[]): unknown[] {
  if (parts.length === 0) return value == null ? [] : [value];
  if (Array.isArray(value))
    return value.flatMap((item) => pathValues(item, parts));
  if (value && typeof value === "object") {
    const [key, ...rest] = parts;
    const child = (value as Doc)[key];
    return child === undefined ? [] : pathValues(child, rest);
  }
  return [];
}

/** -1 / 0 / 1 when both sides are numeric, otherwise null. */
export function compareNumeric(raw: unknown, want: string): number | null {
  const text = String(raw);
  if (text.trim() === "" || want.trim() === "") return null;
  const left = Number(text);
  const right = Number(want);
  if (!Number.isFinite(left) || !Number.isFinite(right)) return null;
  return Math.sign(left - right);
}

/** SQL LIKE pattern (`%`, `_`, `\` escape) as an anchored RegExp. */
export function likeToRegex(pattern: string): RegExp {
  let out = "^";
  for (let i = 0; i < pattern.length; i++) {
    const ch = pattern[i];
    if (ch === "%") out += ".*";
    else if (ch === "_") out += ".";
    else if (ch === "\\" && i + 1 < pattern.length)
      out += escapeRegExp(pattern[++i]);
    else out += escapeRegExp(ch);
  }
  return new RegExp(out + "$");
}

function escapeRegExp(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

/** Filter values arrive as strings; numbers and booleans are compared as such. */
export function coerceFilterValue(value: string): unknown {
  if (/^-?\d+(\.\d+)?$/.test(value)) return Number(value);
  if (value === "true") return true;
  if (value === "false") return false;
  return value;
}

/**
 * Document stores keep `id` as a string plus a numeric copy for range queries;
 * range operators on `id` use the numeric column.
 */
export function idFilterColumn(
  column: string,
  operator: string,
  eqColumn: string,
  rangeColumn: string,
): string {
  const col = column.trim();
  if (col !== "id") return col;
  if (["gt", "gte", "lt", "lte"].includes(operator)) return rangeColumn;
  return eqColumn.trim() || col;
}

export function sortColumn(column: string, sortableId: string): string {
  const col = column.trim();
  return col === "id" ? sortableId : col;
}
