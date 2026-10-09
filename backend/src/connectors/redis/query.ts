import type { Doc, Filter, Order } from "../types.js";
import { pageRows, sortRows } from "../shared/rows.js";
import type { TableKeys } from "./keys.js";

/** Pages a table; plain id-ordered browsing only fetches the keys on the page. */
export async function queryTable(
  keys: TableKeys,
  filters: Filter[],
  limit: number,
  offset: number,
  order: Order | null,
): Promise<{ rows: Doc[]; total: number }> {
  const lookup = keys.idLookup(filters);
  if (lookup) {
    const docs = await keys.read(lookup);
    sortRows(docs, order);
    return { rows: pageRows(docs, limit, offset), total: docs.length };
  }

  const column = order?.column.trim() ?? "";
  if (filters.length === 0 && (column === "" || column === "id")) {
    const all = (await keys.all()).sort();
    if (order?.desc) all.reverse();
    const start = Math.max(0, offset);
    const page = all.slice(start, start + (limit <= 0 ? 50 : limit));
    return { rows: await keys.read(page), total: all.length };
  }

  const matched = await keys.matching(filters);
  sortRows(matched, order);
  return { rows: pageRows(matched, limit, offset), total: matched.length };
}
