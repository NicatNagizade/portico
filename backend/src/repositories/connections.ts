import { asc, eq, and } from "drizzle-orm";
import type { Db } from "../db/client.js";
import { connections } from "../db/schema.js";
import type { Connection } from "../domain/types.js";
import { needsSeal, seal } from "../utils/secretbox.js";
import { now } from "../utils/values.js";
import { toConnection } from "./mappers.js";

export async function listConnections(db: Db): Promise<Connection[]> {
  const rows = await db.select().from(connections).orderBy(asc(connections.id));
  return rows.map(toConnection);
}

export async function getConnection(
  db: Db,
  id: number,
): Promise<Connection | null> {
  const [row] = await db
    .select()
    .from(connections)
    .where(eq(connections.id, id));
  return row ? toConnection(row) : null;
}

export async function findConnections(
  db: Db,
  name: string,
  type?: string,
): Promise<Connection[]> {
  const where =
    type == null
      ? eq(connections.name, name)
      : and(eq(connections.name, name), eq(connections.type, type));
  const rows = await db
    .select()
    .from(connections)
    .where(where)
    .orderBy(asc(connections.id));
  return rows.map(toConnection);
}

export async function insertConnection(
  db: Db,
  name: string,
  type: string,
  config: Record<string, unknown>,
): Promise<Connection> {
  const at = now();
  const [row] = await db
    .insert(connections)
    .values({
      name,
      type,
      config: seal(config),
      created_at: at,
      updated_at: at,
    })
    .returning();
  return toConnection(row);
}

export async function saveConnection(
  db: Db,
  item: Connection,
): Promise<Connection> {
  await db
    .update(connections)
    .set({
      name: item.name,
      type: item.type,
      config: seal(item.config),
      updated_at: now(),
    })
    .where(eq(connections.id, item.id));
  return (await getConnection(db, item.id))!;
}

export async function deleteConnection(db: Db, id: number): Promise<boolean> {
  if (!(await getConnection(db, id))) return false;
  await db.delete(connections).where(eq(connections.id, id));
  return true;
}

/** Re-saves rows whose secrets were stored before APP_KEY sealing existed. */
export async function sealPlaintextConfigs(db: Db): Promise<void> {
  const rows = await db
    .select({ id: connections.id, config: connections.config })
    .from(connections);
  for (const row of rows) {
    if (!needsSeal(row.config)) continue;
    const item = await getConnection(db, row.id);
    if (item) await saveConnection(db, item);
  }
}
