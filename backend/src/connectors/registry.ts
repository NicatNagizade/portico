import type { Connection } from "../domain/types.js";
import type { DestinationWriter, SourceReader } from "./types.js";
import { newMongoDestination } from "./mongo/destination.js";
import { newMongoSource } from "./mongo/source.js";
import { newRedisDestination } from "./redis/destination.js";
import { newRedisSource } from "./redis/source.js";
import { newMysqlDestination, newMysqlSource } from "./sql/mysql.js";
import { newPostgresDestination, newPostgresSource } from "./sql/postgres.js";
import { newSqliteDestination, newSqliteSource } from "./sql/sqlite.js";
import { newTypesenseDestination } from "./typesense/destination.js";
import { newTypesenseSource } from "./typesense/source.js";

type SourceFactory = (conn: Connection) => SourceReader;
type DestinationFactory = (conn: Connection) => DestinationWriter;

export class Registry {
  private sources = new Map<string, SourceFactory>();
  private destinations = new Map<string, DestinationFactory>();

  register(
    type: string,
    source: SourceFactory,
    destination: DestinationFactory,
  ): void {
    this.sources.set(type, source);
    this.destinations.set(type, destination);
  }

  newSource(conn: Connection): SourceReader {
    return factory(this.sources, conn.type, "source")(conn);
  }

  newDestination(conn: Connection): DestinationWriter {
    return factory(this.destinations, conn.type, "destination")(conn);
  }

  /** Opens and closes a connection to prove the config works. */
  async check(conn: Connection): Promise<void> {
    if (!this.sources.has(conn.type) && !this.destinations.has(conn.type))
      throw new Error(
        `no connector registered for type ${JSON.stringify(conn.type)}`,
      );
    const connector = this.sources.has(conn.type)
      ? this.newSource(conn)
      : this.newDestination(conn);
    try {
      await connector.open();
    } finally {
      await connector.close();
    }
  }
}

function factory<F>(factories: Map<string, F>, type: string, kind: string): F {
  const found = factories.get(type);
  if (!found)
    throw new Error(
      `no ${kind} connector registered for type ${JSON.stringify(type)}`,
    );
  return found;
}

export function defaultRegistry(): Registry {
  const registry = new Registry();
  registry.register("mysql", newMysqlSource, newMysqlDestination);
  registry.register("postgres", newPostgresSource, newPostgresDestination);
  registry.register("sqlite", newSqliteSource, newSqliteDestination);
  registry.register("mongodb", newMongoSource, newMongoDestination);
  registry.register("typesense", newTypesenseSource, newTypesenseDestination);
  registry.register("redis", newRedisSource, newRedisDestination);
  return registry;
}
