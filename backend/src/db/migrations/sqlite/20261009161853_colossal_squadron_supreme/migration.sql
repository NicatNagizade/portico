CREATE TABLE `connections` (
	`id` integer PRIMARY KEY AUTOINCREMENT,
	`name` text NOT NULL,
	`type` text NOT NULL,
	`config` text NOT NULL,
	`created_at` text,
	`updated_at` text
);
--> statement-breakpoint
CREATE TABLE `sync_job_fields_values` (
	`id` integer PRIMARY KEY AUTOINCREMENT,
	`sync_job_field_id` integer NOT NULL,
	`source_value` text NOT NULL,
	`destination_value` text NOT NULL,
	`created_at` text,
	`updated_at` text
);
--> statement-breakpoint
CREATE TABLE `sync_job_fields` (
	`id` integer PRIMARY KEY AUTOINCREMENT,
	`sync_job_id` integer NOT NULL,
	`sync_job_relation_id` integer,
	`source_name` text NOT NULL,
	`destination_name` text,
	`destination_type` text,
	`destination_config` text,
	`active` integer DEFAULT true NOT NULL,
	`created_at` text,
	`updated_at` text
);
--> statement-breakpoint
CREATE TABLE `sync_job_relations` (
	`id` integer PRIMARY KEY AUTOINCREMENT,
	`sync_job_id` integer NOT NULL,
	`parent_id` integer,
	`name` text NOT NULL,
	`type` text NOT NULL,
	`table` text NOT NULL,
	`foreign_key` text,
	`related_key` text,
	`config` text,
	`active` integer DEFAULT true NOT NULL,
	`created_at` text,
	`updated_at` text
);
--> statement-breakpoint
CREATE TABLE `sync_job_rules` (
	`id` integer PRIMARY KEY AUTOINCREMENT,
	`sync_job_id` integer NOT NULL,
	`field` text NOT NULL,
	`operator` text NOT NULL,
	`value` text,
	`active` integer DEFAULT true NOT NULL,
	`created_at` text,
	`updated_at` text
);
--> statement-breakpoint
CREATE TABLE `sync_jobs` (
	`id` integer PRIMARY KEY AUTOINCREMENT,
	`name` text NOT NULL,
	`source_connection_id` integer NOT NULL,
	`source_table` text NOT NULL,
	`destination_connection_id` integer NOT NULL,
	`destination_table` text NOT NULL,
	`chunk_size` integer DEFAULT 500 NOT NULL,
	`workers` integer DEFAULT 2 NOT NULL,
	`config` text,
	`created_at` text,
	`updated_at` text
);
--> statement-breakpoint
CREATE TABLE `sync_logs` (
	`id` integer PRIMARY KEY AUTOINCREMENT,
	`sync_job_id` integer NOT NULL,
	`status` text NOT NULL,
	`message` text,
	`rows_total` integer,
	`rows_synced` integer,
	`duration_ms` integer,
	`started_at` text,
	`finished_at` text
);
--> statement-breakpoint
CREATE INDEX `idx_connections_type` ON `connections` (`type`);--> statement-breakpoint
CREATE INDEX `idx_sync_job_fields_values_sync_job_field_id` ON `sync_job_fields_values` (`sync_job_field_id`);--> statement-breakpoint
CREATE INDEX `idx_sync_job_fields_sync_job_id` ON `sync_job_fields` (`sync_job_id`);--> statement-breakpoint
CREATE INDEX `idx_sync_job_fields_sync_job_relation_id` ON `sync_job_fields` (`sync_job_relation_id`);--> statement-breakpoint
CREATE INDEX `idx_sync_job_relations_sync_job_id` ON `sync_job_relations` (`sync_job_id`);--> statement-breakpoint
CREATE INDEX `idx_sync_job_relations_parent_id` ON `sync_job_relations` (`parent_id`);--> statement-breakpoint
CREATE INDEX `idx_sync_job_rules_sync_job_id` ON `sync_job_rules` (`sync_job_id`);--> statement-breakpoint
CREATE INDEX `idx_sync_jobs_source_connection_id` ON `sync_jobs` (`source_connection_id`);--> statement-breakpoint
CREATE INDEX `idx_sync_jobs_destination_connection_id` ON `sync_jobs` (`destination_connection_id`);--> statement-breakpoint
CREATE INDEX `idx_sync_logs_sync_job_id` ON `sync_logs` (`sync_job_id`);--> statement-breakpoint
CREATE INDEX `idx_sync_logs_status` ON `sync_logs` (`status`);