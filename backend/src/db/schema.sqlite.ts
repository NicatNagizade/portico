import { index, integer, sqliteTable, text } from "drizzle-orm/sqlite-core";

/**
 * SQLite DDL twin of `schema.ts`. Drizzle Kit generates `migrations/sqlite`
 * from this file. Repositories query through `schema.ts`; column names here
 * must stay the same.
 */

const json = (name: string) => text(name, { mode: "json" });
const flag = (name: string) =>
  integer(name, { mode: "boolean" }).notNull().default(true);

export const connections = sqliteTable(
  "connections",
  {
    id: integer("id").primaryKey({ autoIncrement: true }),
    name: text("name").notNull(),
    type: text("type").notNull(),
    config: json("config").notNull(),
    created_at: text("created_at"),
    updated_at: text("updated_at"),
  },
  (table) => [index("idx_connections_type").on(table.type)],
);

export const syncJobs = sqliteTable(
  "sync_jobs",
  {
    id: integer("id").primaryKey({ autoIncrement: true }),
    name: text("name").notNull(),
    source_connection_id: integer("source_connection_id").notNull(),
    source_table: text("source_table").notNull(),
    destination_connection_id: integer("destination_connection_id").notNull(),
    destination_table: text("destination_table").notNull(),
    chunk_size: integer("chunk_size").notNull().default(500),
    workers: integer("workers").notNull().default(2),
    config: json("config"),
    created_at: text("created_at"),
    updated_at: text("updated_at"),
  },
  (table) => [
    index("idx_sync_jobs_source_connection_id").on(table.source_connection_id),
    index("idx_sync_jobs_destination_connection_id").on(
      table.destination_connection_id,
    ),
  ],
);

export const syncJobRelations = sqliteTable(
  "sync_job_relations",
  {
    id: integer("id").primaryKey({ autoIncrement: true }),
    sync_job_id: integer("sync_job_id").notNull(),
    parent_id: integer("parent_id"),
    name: text("name").notNull(),
    type: text("type").notNull(),
    table: text("table").notNull(),
    foreign_key: text("foreign_key"),
    related_key: text("related_key"),
    config: json("config"),
    active: flag("active"),
    created_at: text("created_at"),
    updated_at: text("updated_at"),
  },
  (table) => [
    index("idx_sync_job_relations_sync_job_id").on(table.sync_job_id),
    index("idx_sync_job_relations_parent_id").on(table.parent_id),
  ],
);

export const syncJobFields = sqliteTable(
  "sync_job_fields",
  {
    id: integer("id").primaryKey({ autoIncrement: true }),
    sync_job_id: integer("sync_job_id").notNull(),
    sync_job_relation_id: integer("sync_job_relation_id"),
    source_name: text("source_name").notNull(),
    destination_name: text("destination_name"),
    destination_type: text("destination_type"),
    destination_config: json("destination_config"),
    active: flag("active"),
    created_at: text("created_at"),
    updated_at: text("updated_at"),
  },
  (table) => [
    index("idx_sync_job_fields_sync_job_id").on(table.sync_job_id),
    index("idx_sync_job_fields_sync_job_relation_id").on(
      table.sync_job_relation_id,
    ),
  ],
);

export const syncJobFieldValues = sqliteTable(
  "sync_job_fields_values",
  {
    id: integer("id").primaryKey({ autoIncrement: true }),
    sync_job_field_id: integer("sync_job_field_id").notNull(),
    source_value: text("source_value").notNull(),
    destination_value: text("destination_value").notNull(),
    created_at: text("created_at"),
    updated_at: text("updated_at"),
  },
  (table) => [
    index("idx_sync_job_fields_values_sync_job_field_id").on(
      table.sync_job_field_id,
    ),
  ],
);

export const syncJobRules = sqliteTable(
  "sync_job_rules",
  {
    id: integer("id").primaryKey({ autoIncrement: true }),
    sync_job_id: integer("sync_job_id").notNull(),
    field: text("field").notNull(),
    operator: text("operator").notNull(),
    value: text("value"),
    active: flag("active"),
    created_at: text("created_at"),
    updated_at: text("updated_at"),
  },
  (table) => [index("idx_sync_job_rules_sync_job_id").on(table.sync_job_id)],
);

export const syncLogs = sqliteTable(
  "sync_logs",
  {
    id: integer("id").primaryKey({ autoIncrement: true }),
    sync_job_id: integer("sync_job_id").notNull(),
    status: text("status").notNull(),
    message: text("message"),
    rows_total: integer("rows_total"),
    rows_synced: integer("rows_synced"),
    duration_ms: integer("duration_ms"),
    started_at: text("started_at"),
    finished_at: text("finished_at"),
  },
  (table) => [
    index("idx_sync_logs_sync_job_id").on(table.sync_job_id),
    index("idx_sync_logs_status").on(table.status),
  ],
);
