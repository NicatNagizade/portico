import { count, type SQL } from "drizzle-orm";
import type { PgTable } from "drizzle-orm/pg-core";
import type { Db, Row } from "./client.js";

export async function selectPage(
  db: Db,
  table: PgTable,
  where: SQL | undefined,
  order: SQL,
  page: number,
  pageSize: number,
): Promise<{ rows: Row[]; total: number }> {
  const [counted] = await db.select({ n: count() }).from(table).where(where);
  const rows = await db
    .select()
    .from(table)
    .where(where)
    .orderBy(order)
    .limit(pageSize)
    .offset((page - 1) * pageSize);
  return { rows, total: Number(counted?.n ?? 0) };
}
