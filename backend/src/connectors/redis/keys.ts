import type { Redis } from "ioredis";
import type { Doc, Filter } from "../types.js";
import { filterRows } from "../shared/filters.js";
import { splitList } from "../../utils/values.js";
import type { RedisConfig } from "./client.js";

const SCAN_COUNT = 1000;
/** Keys per MGET / UNLINK call; spreading more as arguments overflows the call stack. */
const KEY_BATCH = 500;

/** Calls `fn` once per SCAN page of matching keys; `fn` returns false to stop early. */
export async function scanKeys(
  client: Redis,
  pattern: string,
  fn: (keys: string[]) => Promise<boolean | void>,
): Promise<void> {
  let cursor = "0";
  do {
    const [next, keys] = await client.scan(
      cursor,
      "MATCH",
      pattern,
      "COUNT",
      SCAN_COUNT,
    );
    cursor = next;
    if (keys.length && (await fn(keys)) === false) return;
  } while (cursor !== "0");
}

/** Documents live at `{table}{sep}{id}`. */
export class TableKeys {
  constructor(
    private client: Redis,
    private cfg: RedisConfig,
    readonly table: string,
  ) {}

  key(id: string): string {
    return this.table + this.cfg.sep + id;
  }

  get pattern(): string {
    return this.key("*");
  }

  idFromKey(key: string): string {
    const prefix = this.key("");
    return key.startsWith(prefix) ? key.slice(prefix.length) : key;
  }

  /** All keys of this table, or the first `limit` found. */
  async all(limit = 0): Promise<string[]> {
    const out: string[] = [];
    await scanKeys(this.client, this.pattern, async (keys) => {
      for (const key of keys) out.push(key);
      return limit <= 0 || out.length < limit;
    });
    return limit > 0 ? out.slice(0, limit) : out;
  }

  async count(): Promise<number> {
    let n = 0;
    await scanKeys(this.client, this.pattern, async (keys) => {
      n += keys.length;
    });
    return n;
  }

  /** Deletes page by page so the full key list is never held in memory. */
  async deleteAll(): Promise<void> {
    await scanKeys(this.client, this.pattern, async (keys) => {
      for (let i = 0; i < keys.length; i += KEY_BATCH)
        await this.client.unlink(...keys.slice(i, i + KEY_BATCH));
    });
  }

  async read(keys: string[]): Promise<Doc[]> {
    const docs: Doc[] = [];
    for (let i = 0; i < keys.length; i += KEY_BATCH) {
      const chunk = keys.slice(i, i + KEY_BATCH);
      const values = await this.client.mget(...chunk);
      values.forEach((value, j) => {
        if (value == null) return;
        const doc = JSON.parse(value) as Doc;
        doc.id ??= this.idFromKey(chunk[j]);
        docs.push(doc);
      });
    }
    return docs;
  }

  async loadAll(): Promise<Doc[]> {
    return this.read((await this.all()).sort());
  }

  /** Keeps only matching documents, so a filter does not hold the whole table. */
  async matching(filters: Filter[]): Promise<Doc[]> {
    const matched: Doc[] = [];
    await scanKeys(this.client, this.pattern, async (batch) => {
      matched.push(...filterRows(await this.read(batch), filters));
    });
    return matched;
  }

  /** Keys for a lone `id eq` / `id in` filter, which can skip the full scan. */
  idLookup(filters: Filter[]): string[] | null {
    if (filters.length !== 1 || filters[0].column.trim() !== "id") return null;
    const { operator, value } = filters[0];
    const ids =
      operator === "eq"
        ? [value.trim()].filter(Boolean)
        : operator === "in"
          ? splitList(value)
          : [];
    return ids.length ? ids.map((id) => this.key(id)) : null;
  }
}
