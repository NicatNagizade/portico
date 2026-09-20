package migrate

import (
	"context"
	"fmt"
	"log"
	"regexp"

	"github.com/jackc/pgx/v5"
	"github.com/portico/exampledata/internal/config"
)

var dbNamePattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

const schemaSQL = `
CREATE TABLE IF NOT EXISTS users (
    id          BIGSERIAL PRIMARY KEY,
    username    VARCHAR(64)  NOT NULL,
    email       VARCHAR(255) NOT NULL,
    full_name   VARCHAR(255) NOT NULL,
    bio         TEXT         NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS posts (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT       NOT NULL REFERENCES users(id),
    title       VARCHAR(255) NOT NULL,
    body        TEXT         NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS comments (
    id          BIGSERIAL PRIMARY KEY,
    post_id     BIGINT       NOT NULL REFERENCES posts(id),
    user_id     BIGINT       NOT NULL REFERENCES users(id),
    body        TEXT         NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS reactions (
    id          BIGSERIAL PRIMARY KEY,
    comment_id  BIGINT       NOT NULL REFERENCES comments(id),
    user_id     BIGINT       NOT NULL REFERENCES users(id),
    type        VARCHAR(32)  NOT NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT reactions_type_check CHECK (type IN ('like', 'dislike', 'smile', 'angry')),
    CONSTRAINT reactions_comment_user_unique UNIQUE (comment_id, user_id)
);
`

const indexesSQL = `
CREATE UNIQUE INDEX IF NOT EXISTS users_username_idx ON users (username);
CREATE UNIQUE INDEX IF NOT EXISTS users_email_idx ON users (email);
CREATE INDEX IF NOT EXISTS posts_user_id_idx ON posts (user_id);
CREATE INDEX IF NOT EXISTS posts_created_at_idx ON posts (created_at);
CREATE INDEX IF NOT EXISTS comments_post_id_idx ON comments (post_id);
CREATE INDEX IF NOT EXISTS comments_user_id_idx ON comments (user_id);
CREATE INDEX IF NOT EXISTS reactions_comment_id_idx ON reactions (comment_id);
CREATE INDEX IF NOT EXISTS reactions_user_id_idx ON reactions (user_id);
CREATE INDEX IF NOT EXISTS reactions_type_idx ON reactions (type);
`

// Run creates the database if missing, then creates tables and indexes.
func Run(ctx context.Context, cfg *config.Config) error {
	if !dbNamePattern.MatchString(cfg.DBName) {
		return fmt.Errorf("invalid database name %q", cfg.DBName)
	}

	if err := ensureDatabase(ctx, cfg); err != nil {
		return err
	}

	conn, err := pgx.Connect(ctx, cfg.DSN())
	if err != nil {
		return fmt.Errorf("connect to %s: %w", cfg.DBName, err)
	}
	defer conn.Close(ctx)

	if _, err := conn.Exec(ctx, schemaSQL); err != nil {
		return fmt.Errorf("create tables: %w", err)
	}
	if _, err := conn.Exec(ctx, indexesSQL); err != nil {
		return fmt.Errorf("create indexes: %w", err)
	}

	log.Printf("database %q is ready (users, posts, comments, reactions)", cfg.DBName)
	return nil
}

func ensureDatabase(ctx context.Context, cfg *config.Config) error {
	admin, err := pgx.Connect(ctx, cfg.AdminDSN())
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer admin.Close(ctx)

	var exists bool
	err = admin.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)`,
		cfg.DBName,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check database: %w", err)
	}
	if exists {
		log.Printf("database %q already exists", cfg.DBName)
		return nil
	}

	ident := pgx.Identifier{cfg.DBName}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+ident); err != nil {
		return fmt.Errorf("create database: %w", err)
	}
	log.Printf("created database %q", cfg.DBName)
	return nil
}
