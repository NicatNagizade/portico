import type { Doc, Filter, Order } from "../types.js";
import { forEachChunk } from "../../utils/collections.js";
import { compareNumeric, filterRows, pathValues } from "./filters.js";

export { filterRows } from "./filters.js";

export function cloneRows(rows: Doc[]): Doc[] {
  return rows.map((row) => ({ ...row }));
}

export function sortRows(rows: Doc[], order: Order | null): void {
  const column = order?.column.trim();
  if (!order || !column) return;
  const parts = column.split(".");
  const valueOf = (row: Doc) =>
    parts.length > 1 ? (pathValues(row, parts)[0] ?? null) : row[column];
  rows.sort((a, b) => {
    const cmp = compareValues(valueOf(a), valueOf(b));
    return order.desc ? -cmp : cmp;
  });
}

function compareValues(a: unknown, b: unknown): number {
  if (a == null || b == null) return a == null ? (b == null ? 0 : -1) : 1;
  return compareNumeric(a, String(b)) ?? String(a).localeCompare(String(b));
}

export function pageRows(rows: Doc[], limit: number, offset: number): Doc[] {
  const size = limit <= 0 ? 50 : limit;
  const start = Math.max(0, offset);
  return rows.slice(start, start + size);
}

export function selectColumns(rows: Doc[], columns: string[] | null): Doc[] {
  if (!columns?.length) return rows;
  return rows.map((row) => {
    const doc: Doc = {};
    for (const column of columns) if (column in row) doc[column] = row[column];
    return doc;
  });
}

export function readChunksInMemory(
  rows: Doc[],
  chunkSize: number,
  filters: Filter[],
  fn: (docs: Doc[]) => Promise<void>,
): Promise<void> {
  return forEachChunk(filterRows(rows, filters), chunkSize, fn);
}

export function queryInMemory(
  rows: Doc[],
  filters: Filter[],
  limit: number,
  offset: number,
  order: Order | null,
): { rows: Doc[]; total: number } {
  const matched = filterRows(rows, filters);
  sortRows(matched, order);
  return { rows: pageRows(matched, limit, offset), total: matched.length };
}
