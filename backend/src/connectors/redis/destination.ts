import type { Redis } from "ioredis";
import type { Connection } from "../../domain/types.js";
import type { DestinationWriter } from "../types.js";
import { CATALOG, openClient, parseConfig } from "./client.js";
import { TableKeys } from "./keys.js";
import { queryTable } from "./query.js";

/** go-redis gives each in-flight pipeline its own pooled connection; match that. */
const WRITE_CONNECTIONS = 8;

export function newRedisDestination(conn: Connection): DestinationWriter {
  const cfg = parseConfig(conn);
  const clients: Redis[] = [];
  let turn = 0;
  const keys = (table: string) => new TableKeys(clients[0], cfg, table);

  return {
    async open() {
      const results = await Promise.allSettled(
        Array.from({ length: WRITE_CONNECTIONS }, () => openClient(cfg)),
      );
      for (const result of results)
        if (result.status === "fulfilled") clients.push(result.value);
      const failed = results.find((result) => result.status === "rejected");
      if (failed) {
        clients.splice(0).forEach((client) => client.disconnect());
        throw failed.reason;
      }
    },
    async close() {
      await Promise.all(clients.splice(0).map((client) => client.quit()));
    },
    async prepare(name) {
      await keys(name).deleteAll();
      await clients[0].sadd(CATALOG, name);
    },
    async writeBatch(name, docs) {
      if (docs.length === 0) return;
      const t = keys(name);
      const pipeline = clients[turn++ % clients.length].pipeline();
      for (const doc of docs) {
        const id = String(doc.id ?? "");
        if (!id || id === "undefined" || id === "null")
          throw new Error(
            `redis write into ${JSON.stringify(name)}: document missing id`,
          );
        pipeline.set(t.key(id), JSON.stringify(doc));
      }
      await pipeline.exec();
    },
    query: (name, filters, limit, offset, order) =>
      queryTable(keys(name), filters, limit, offset, order),
  };
}
