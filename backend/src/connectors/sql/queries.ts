import type { Doc, Filter, Order } from "../types.js";
import { queryInMemory } from "../shared/rows.js";
import type { SqlConn } from "./connection.js";
import { Binder, whereSQL, withWhere, type Dialect } from "./where.js";

/** A connection plus the already-quoted table name queries run against. */
export type SqlTable = { db: SqlConn; dialect: Dialect; table: string };

type OnChunk = (docs: Doc[]) => Promise<void>;

function selectList(t: SqlTable, columns: string[] | null): string {
  return columns?.length ? columns.map(t.dialect.quote).join(", ") : "*";
}

export async function countRows(t: SqlTable, filters: Filter[]) {
  const where = whereSQL(t.dialect, filters);
  const sql = withWhere(`SELECT COUNT(*) AS n FROM ${t.table}`, where);
  const [row] = await t.db.query(sql, where.params);
  return Number(row?.n ?? 0);
}

export async function queryPage(
  t: SqlTable,
  columns: string[] | null,
  filters: Filter[],
  limit: number,
  offset: number,
  order: Order | null,
): Promise<Doc[]> {
  const where = whereSQL(t.dialect, filters);
  let sql = withWhere(
    `SELECT ${selectList(t, columns)} FROM ${t.table}`,
    where,
  );
  const column = order?.column.trim();
  if (column)
    sql += ` ORDER BY ${t.dialect.quote(column)} ${order!.desc ? "DESC" : "ASC"}`;
  sql += ` LIMIT ${limit > 0 ? limit : 50} OFFSET ${Math.max(0, offset)}`;
  return t.db.query(sql, where.params);
}

export async function queryRows(
  t: SqlTable,
  columns: string[] | null,
  whereColumn: string,
  whereValues: unknown[],
): Promise<Doc[]> {
  if (whereValues.length === 0) return [];
  const binder = new Binder(t.dialect.style);
  const list = whereValues.map((v) => binder.add(v)).join(", ");
  const sql = `SELECT ${selectList(t, columns)} FROM ${t.table} WHERE ${t.dialect.quote(whereColumn)} IN (${list})`;
  return t.db.query(sql, binder.params);
}

export async function readFiltered(
  t: SqlTable,
  chunkSize: number,
  filters: Filter[],
  fn: OnChunk,
): Promise<void> {
  const where = whereSQL(t.dialect, filters);
  await t.db.read(
    withWhere(`SELECT * FROM ${t.table}`, where),
    where.params,
    chunkSize,
    fn,
  );
}

/**
 * Destination browse: plain filters run in SQL; dotted (nested JSON) filters or
 * sorts load the root-filtered table and finish in memory.
 */
export async function queryDestination(
  t: SqlTable,
  filters: Filter[],
  limit: number,
  offset: number,
  order: Order | null,
): Promise<{ rows: Doc[]; total: number }> {
  const root = filters.filter((f) => !f.column.includes("."));
  const nested = filters.filter((f) => f.column.includes("."));
  if (nested.length === 0 && !order?.column.includes(".")) {
    return {
      total: await countRows(t, root),
      rows: await queryPage(t, null, root, limit, offset, order),
    };
  }
  const where = whereSQL(t.dialect, root);
  const rows = await t.db.query(
    withWhere(`SELECT * FROM ${t.table}`, where),
    where.params,
  );
  return queryInMemory(rows, nested, limit, offset, order);
}
