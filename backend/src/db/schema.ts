/**
 * App database tables (Postgres DDL). SQLite gets the same column names from
 * `schema.sqlite.ts`. Repositories query these tables on both drivers:
 * Drizzle emits `$1` or `?` from the connection, and the json/bool columns
 * below bind values either driver accepts.
 */
import {
  bigint,
  bigserial,
  customType,
  index,
  pgTable,
  text,
  timestamp,
  varchar,
} from "drizzle-orm/pg-core";

/**
 * JSON stored as jsonb in Postgres and as text in SQLite. Values are
 * stringified on write so node:sqlite can bind them, and parsed on read
 * when the driver has not already returned an object.
 */
const jsonObject = customType<{
  data: Record<string, unknown>;
  driverData: string;
}>({
  dataType: () => "jsonb",
  toDriver: (value) => JSON.stringify(value),
  fromDriver: (value) =>
    typeof value === "string"
      ? (JSON.parse(value) as Record<string, unknown>)
      : (value as Record<string, unknown>),
});

/** Boolean stored as boolean in Postgres and 0/1 in SQLite. */
const flag = customType<{ data: boolean; driverData: number | boolean }>({
  dataType: () => "boolean",
  toDriver: (value) => (value ? 1 : 0),
  fromDriver: (value) => value === true || value === 1,
});

const at = (name: string) =>
  timestamp(name, { withTimezone: true, mode: "string" });

export const connections = pgTable(
  "connections",
  {
    id: bigserial("id", { mode: "number" }).primaryKey(),
    name: varchar("name", { length: 255 }).notNull(),
    type: varchar("type", { length: 50 }).notNull(),
    config: jsonObject("config").notNull(),
    created_at: at("created_at"),
    updated_at: at("updated_at"),
  },
  (table) => [index("idx_connections_type").on(table.type)],
);

export const syncJobs = pgTable(
  "sync_jobs",
  {
    id: bigserial("id", { mode: "number" }).primaryKey(),
    name: varchar("name", { length: 255 }).notNull(),
    source_connection_id: bigint("source_connection_id", {
      mode: "number",
    }).notNull(),
    source_table: varchar("source_table", { length: 255 }).notNull(),
    destination_connection_id: bigint("destination_connection_id", {
      mode: "number",
    }).notNull(),
    destination_table: varchar("destination_table", { length: 255 }).notNull(),
    chunk_size: bigint("chunk_size", { mode: "number" }).notNull().default(500),
    workers: bigint("workers", { mode: "number" }).notNull().default(2),
    config: jsonObject("config"),
    created_at: at("created_at"),
    updated_at: at("updated_at"),
  },
  (table) => [
    index("idx_sync_jobs_source_connection_id").on(table.source_connection_id),
    index("idx_sync_jobs_destination_connection_id").on(
      table.destination_connection_id,
    ),
  ],
);

export const syncJobRelations = pgTable(
  "sync_job_relations",
  {
    id: bigserial("id", { mode: "number" }).primaryKey(),
    sync_job_id: bigint("sync_job_id", { mode: "number" }).notNull(),
    parent_id: bigint("parent_id", { mode: "number" }),
    name: varchar("name", { length: 255 }).notNull(),
    type: varchar("type", { length: 50 }).notNull(),
    table: varchar("table", { length: 255 }).notNull(),
    foreign_key: varchar("foreign_key", { length: 255 }),
    related_key: varchar("related_key", { length: 255 }),
    config: jsonObject("config"),
    active: flag("active").notNull().default(true),
    created_at: at("created_at"),
    updated_at: at("updated_at"),
  },
  (table) => [
    index("idx_sync_job_relations_sync_job_id").on(table.sync_job_id),
    index("idx_sync_job_relations_parent_id").on(table.parent_id),
  ],
);

export const syncJobFields = pgTable(
  "sync_job_fields",
  {
    id: bigserial("id", { mode: "number" }).primaryKey(),
    sync_job_id: bigint("sync_job_id", { mode: "number" }).notNull(),
    sync_job_relation_id: bigint("sync_job_relation_id", { mode: "number" }),
    source_name: varchar("source_name", { length: 255 }).notNull(),
    destination_name: varchar("destination_name", { length: 255 }),
    destination_type: varchar("destination_type", { length: 50 }),
    destination_config: jsonObject("destination_config"),
    active: flag("active").notNull().default(true),
    created_at: at("created_at"),
    updated_at: at("updated_at"),
  },
  (table) => [
    index("idx_sync_job_fields_sync_job_id").on(table.sync_job_id),
    index("idx_sync_job_fields_sync_job_relation_id").on(
      table.sync_job_relation_id,
    ),
  ],
);

export const syncJobFieldValues = pgTable(
  "sync_job_fields_values",
  {
    id: bigserial("id", { mode: "number" }).primaryKey(),
    sync_job_field_id: bigint("sync_job_field_id", {
      mode: "number",
    }).notNull(),
    source_value: varchar("source_value", { length: 255 }).notNull(),
    destination_value: varchar("destination_value", { length: 255 }).notNull(),
    created_at: at("created_at"),
    updated_at: at("updated_at"),
  },
  (table) => [
    index("idx_sync_job_fields_values_sync_job_field_id").on(
      table.sync_job_field_id,
    ),
  ],
);

export const syncJobRules = pgTable(
  "sync_job_rules",
  {
    id: bigserial("id", { mode: "number" }).primaryKey(),
    sync_job_id: bigint("sync_job_id", { mode: "number" }).notNull(),
    field: varchar("field", { length: 255 }).notNull(),
    operator: varchar("operator", { length: 20 }).notNull(),
    value: text("value"),
    active: flag("active").notNull().default(true),
    created_at: at("created_at"),
    updated_at: at("updated_at"),
  },
  (table) => [index("idx_sync_job_rules_sync_job_id").on(table.sync_job_id)],
);

export const syncLogs = pgTable(
  "sync_logs",
  {
    id: bigserial("id", { mode: "number" }).primaryKey(),
    sync_job_id: bigint("sync_job_id", { mode: "number" }).notNull(),
    status: varchar("status", { length: 50 }).notNull(),
    message: text("message"),
    rows_total: bigint("rows_total", { mode: "number" }),
    rows_synced: bigint("rows_synced", { mode: "number" }),
    duration_ms: bigint("duration_ms", { mode: "number" }),
    started_at: at("started_at"),
    finished_at: at("finished_at"),
  },
  (table) => [
    index("idx_sync_logs_sync_job_id").on(table.sync_job_id),
    index("idx_sync_logs_status").on(table.status),
  ],
);

/** Tables dropped by `migrate refresh`, children first. */
export const APP_TABLES = [
  "sync_logs",
  "sync_job_rules",
  "sync_job_fields_values",
  "sync_job_fields",
  "sync_job_relations",
  "sync_jobs",
  "connections",
] as const;
