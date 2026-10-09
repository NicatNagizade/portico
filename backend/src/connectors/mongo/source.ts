import type { Collection, Document, MongoClient } from "mongodb";
import type { Connection } from "../../domain/types.js";
import type { Doc, SourceReader, TableSchema } from "../types.js";
import { DEFAULT_SORTABLE_ID } from "../primaryKey.js";
import { selectColumns } from "../shared/rows.js";
import { nestDottedFields, schemaFromDocs } from "../shared/schema.js";
import { connect, parseConfig } from "./client.js";
import { fromMongoDoc } from "./documents.js";
import { buildMongoFilter, findOptions } from "./filter.js";
import { columnFromSpec } from "./validator.js";

const SCHEMA_SAMPLE = 50;
const fromRaw = (doc: Document) =>
  fromMongoDoc(doc as Doc, DEFAULT_SORTABLE_ID);
const filterOf = (filters: Parameters<typeof buildMongoFilter>[0]) =>
  buildMongoFilter(filters, DEFAULT_SORTABLE_ID);

export function newMongoSource(conn: Connection): SourceReader {
  const cfg = parseConfig(conn);
  let client: MongoClient | null = null;
  const collection = (name: string) =>
    client!.db(cfg.database).collection(name);

  return {
    async open() {
      client = await connect(cfg);
    },
    async close() {
      await client?.close();
      client = null;
    },
    async listTables() {
      return client!
        .db(cfg.database)
        .listCollections()
        .map((c) => c.name)
        .toArray();
    },
    async schema(table) {
      return (
        (await validatorSchema(client!, cfg.database, table)) ??
        (await sampleSchema(collection(table)))
      );
    },
    count: (table, filters) =>
      collection(table).countDocuments(filterOf(filters)),
    async readChunks(table, chunkSize, filters, fn) {
      const size = chunkSize > 0 ? chunkSize : 500;
      let batch: Doc[] = [];
      for await (const raw of collection(table).find(filterOf(filters))) {
        batch.push(fromRaw(raw));
        if (batch.length >= size) {
          await fn(batch);
          batch = [];
        }
      }
      if (batch.length) await fn(batch);
    },
    async query(table, columns, filters, limit, offset, order) {
      const options = findOptions(limit, offset, order, DEFAULT_SORTABLE_ID);
      const rows = await collection(table)
        .find(filterOf(filters), options)
        .toArray();
      return selectColumns(rows.map(fromRaw), columns);
    },
    async queryRows(table, columns, whereColumn, whereValues) {
      if (!whereValues.length) return [];
      const column = whereColumn === "id" ? "_id" : whereColumn;
      const rows = await collection(table)
        .find({ [column]: { $in: whereValues } })
        .toArray();
      return selectColumns(rows.map(fromRaw), columns);
    },
    supportsRelationFilters: () => false,
  };
}

async function validatorSchema(
  client: MongoClient,
  database: string,
  table: string,
): Promise<TableSchema | null> {
  const info = (await client
    .db(database)
    .listCollections({ name: table }, { nameOnly: false })
    .next()) as { options?: { validator?: Document } } | null;
  const props = info?.options?.validator?.$jsonSchema?.properties as
    Document | undefined;
  if (!props) return null;
  const columns = Object.entries(props).map(([name, spec]) =>
    columnFromSpec(
      name === "_id" ? "id" : name,
      spec as Document,
      name === "_id",
    ),
  );
  return { columns: nestDottedFields(columns) };
}

async function sampleSchema(collection: Collection): Promise<TableSchema> {
  const docs = await collection.find({}, { limit: SCHEMA_SAMPLE }).toArray();
  if (docs.length === 0)
    return { columns: [{ name: "id", type: "string", primaryKey: true }] };
  return schemaFromDocs(docs.map(fromRaw));
}
