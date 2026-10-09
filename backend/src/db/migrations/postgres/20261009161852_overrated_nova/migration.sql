CREATE TABLE "connections" (
	"id" bigserial PRIMARY KEY,
	"name" varchar(255) NOT NULL,
	"type" varchar(50) NOT NULL,
	"config" jsonb NOT NULL,
	"created_at" timestamp with time zone,
	"updated_at" timestamp with time zone
);
--> statement-breakpoint
CREATE TABLE "sync_job_fields_values" (
	"id" bigserial PRIMARY KEY,
	"sync_job_field_id" bigint NOT NULL,
	"source_value" varchar(255) NOT NULL,
	"destination_value" varchar(255) NOT NULL,
	"created_at" timestamp with time zone,
	"updated_at" timestamp with time zone
);
--> statement-breakpoint
CREATE TABLE "sync_job_fields" (
	"id" bigserial PRIMARY KEY,
	"sync_job_id" bigint NOT NULL,
	"sync_job_relation_id" bigint,
	"source_name" varchar(255) NOT NULL,
	"destination_name" varchar(255),
	"destination_type" varchar(50),
	"destination_config" jsonb,
	"active" boolean DEFAULT true NOT NULL,
	"created_at" timestamp with time zone,
	"updated_at" timestamp with time zone
);
--> statement-breakpoint
CREATE TABLE "sync_job_relations" (
	"id" bigserial PRIMARY KEY,
	"sync_job_id" bigint NOT NULL,
	"parent_id" bigint,
	"name" varchar(255) NOT NULL,
	"type" varchar(50) NOT NULL,
	"table" varchar(255) NOT NULL,
	"foreign_key" varchar(255),
	"related_key" varchar(255),
	"config" jsonb,
	"active" boolean DEFAULT true NOT NULL,
	"created_at" timestamp with time zone,
	"updated_at" timestamp with time zone
);
--> statement-breakpoint
CREATE TABLE "sync_job_rules" (
	"id" bigserial PRIMARY KEY,
	"sync_job_id" bigint NOT NULL,
	"field" varchar(255) NOT NULL,
	"operator" varchar(20) NOT NULL,
	"value" text,
	"active" boolean DEFAULT true NOT NULL,
	"created_at" timestamp with time zone,
	"updated_at" timestamp with time zone
);
--> statement-breakpoint
CREATE TABLE "sync_jobs" (
	"id" bigserial PRIMARY KEY,
	"name" varchar(255) NOT NULL,
	"source_connection_id" bigint NOT NULL,
	"source_table" varchar(255) NOT NULL,
	"destination_connection_id" bigint NOT NULL,
	"destination_table" varchar(255) NOT NULL,
	"chunk_size" bigint DEFAULT 500 NOT NULL,
	"workers" bigint DEFAULT 2 NOT NULL,
	"config" jsonb,
	"created_at" timestamp with time zone,
	"updated_at" timestamp with time zone
);
--> statement-breakpoint
CREATE TABLE "sync_logs" (
	"id" bigserial PRIMARY KEY,
	"sync_job_id" bigint NOT NULL,
	"status" varchar(50) NOT NULL,
	"message" text,
	"rows_total" bigint,
	"rows_synced" bigint,
	"duration_ms" bigint,
	"started_at" timestamp with time zone,
	"finished_at" timestamp with time zone
);
--> statement-breakpoint
CREATE INDEX "idx_connections_type" ON "connections" ("type");--> statement-breakpoint
CREATE INDEX "idx_sync_job_fields_values_sync_job_field_id" ON "sync_job_fields_values" ("sync_job_field_id");--> statement-breakpoint
CREATE INDEX "idx_sync_job_fields_sync_job_id" ON "sync_job_fields" ("sync_job_id");--> statement-breakpoint
CREATE INDEX "idx_sync_job_fields_sync_job_relation_id" ON "sync_job_fields" ("sync_job_relation_id");--> statement-breakpoint
CREATE INDEX "idx_sync_job_relations_sync_job_id" ON "sync_job_relations" ("sync_job_id");--> statement-breakpoint
CREATE INDEX "idx_sync_job_relations_parent_id" ON "sync_job_relations" ("parent_id");--> statement-breakpoint
CREATE INDEX "idx_sync_job_rules_sync_job_id" ON "sync_job_rules" ("sync_job_id");--> statement-breakpoint
CREATE INDEX "idx_sync_jobs_source_connection_id" ON "sync_jobs" ("source_connection_id");--> statement-breakpoint
CREATE INDEX "idx_sync_jobs_destination_connection_id" ON "sync_jobs" ("destination_connection_id");--> statement-breakpoint
CREATE INDEX "idx_sync_logs_sync_job_id" ON "sync_logs" ("sync_job_id");--> statement-breakpoint
CREATE INDEX "idx_sync_logs_status" ON "sync_logs" ("status");