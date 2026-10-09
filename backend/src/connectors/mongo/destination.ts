import type { MongoClient } from "mongodb";
import type { Connection } from "../../domain/types.js";
import type { DestinationWriter, Doc } from "../types.js";
import { DEFAULT_SORTABLE_ID, sortableIdField } from "../primaryKey.js";
import { asObject } from "../../utils/values.js";
import { connect, parseConfig } from "./client.js";
import { fromMongoDoc, toMongoDoc } from "./documents.js";
import { buildMongoFilter, findOptions } from "./filter.js";
import { validatorProperties } from "./validator.js";

export function newMongoDestination(conn: Connection): DestinationWriter {
  const cfg = parseConfig(conn);
  let client: MongoClient | null = null;
  let sortField = DEFAULT_SORTABLE_ID;
  const db = () => client!.db(cfg.database);

  return {
    async open() {
      client = await connect(cfg);
    },
    async close() {
      await client?.close();
      client = null;
    },
    async applyJobConfig(config) {
      sortField = sortableIdField(config);
    },
    /** Drops the collection; with `config.apply_schema` recreates it with a validator. */
    async prepare(name, schema, config) {
      await db()
        .collection(name)
        .drop()
        .catch((error: { codeName?: string }) => {
          if (error?.codeName !== "NamespaceNotFound") throw error;
        });
      sortField = sortableIdField(config);
      if (!asObject(config)?.apply_schema) return;
      const properties = validatorProperties(schema.columns, true, sortField);
      if (sortField)
        properties[sortField] = { bsonType: ["int", "long", "null"] };
      await db().createCollection(name, {
        validator: { $jsonSchema: { bsonType: "object", properties } },
      });
    },
    async writeBatch(name, docs) {
      if (!docs.length) return;
      await db()
        .collection(name)
        .insertMany(docs.map((doc) => toMongoDoc(doc, sortField)));
    },
    async query(name, filters, limit, offset, order) {
      const filter = buildMongoFilter(filters, sortField);
      const collection = db().collection(name);
      const total = await collection.countDocuments(filter);
      const rows = await collection
        .find(filter, findOptions(limit, offset, order, sortField))
        .toArray();
      return {
        rows: rows.map((row) => fromMongoDoc(row as Doc, sortField)),
        total,
      };
    },
  };
}
