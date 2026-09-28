package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
	"github.com/portico/exampledata/internal/config"
)

const mysqlSchemaSQL = `
CREATE TABLE IF NOT EXISTS users (
    id          BIGINT AUTO_INCREMENT PRIMARY KEY,
    username    VARCHAR(64)  NOT NULL,
    email       VARCHAR(255) NOT NULL,
    full_name   VARCHAR(255) NOT NULL,
    bio         TEXT         NOT NULL,
    created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS posts (
    id          BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id     BIGINT       NOT NULL,
    title       VARCHAR(255) NOT NULL,
    body        TEXT         NOT NULL,
    created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT posts_user_id_fk FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS comments (
    id          BIGINT AUTO_INCREMENT PRIMARY KEY,
    post_id     BIGINT       NOT NULL,
    user_id     BIGINT       NOT NULL,
    body        TEXT         NOT NULL,
    created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT comments_post_id_fk FOREIGN KEY (post_id) REFERENCES posts(id),
    CONSTRAINT comments_user_id_fk FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS reactions (
    id          BIGINT AUTO_INCREMENT PRIMARY KEY,
    comment_id  BIGINT       NOT NULL,
    user_id     BIGINT       NOT NULL,
    type        VARCHAR(32)  NOT NULL,
    created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT reactions_comment_id_fk FOREIGN KEY (comment_id) REFERENCES comments(id),
    CONSTRAINT reactions_user_id_fk FOREIGN KEY (user_id) REFERENCES users(id),
    CONSTRAINT reactions_comment_user_unique UNIQUE (comment_id, user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
`

const mysqlDropTablesSQL = `
SET FOREIGN_KEY_CHECKS=0;
DROP TABLE IF EXISTS reactions;
DROP TABLE IF EXISTS comments;
DROP TABLE IF EXISTS posts;
DROP TABLE IF EXISTS users;
SET FOREIGN_KEY_CHECKS=1;
`

func runMySQL(ctx context.Context, cfg *config.Config, truncate bool) error {
	if err := ensureMySQLDatabase(ctx, cfg); err != nil {
		return err
	}

	db, err := sql.Open("mysql", cfg.DSN())
	if err != nil {
		return fmt.Errorf("connect to %s: %w", cfg.DBName, err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("connect to %s: %w", cfg.DBName, err)
	}

	if truncate {
		if _, err := db.ExecContext(ctx, mysqlDropTablesSQL); err != nil {
			return fmt.Errorf("drop tables: %w", err)
		}
		log.Printf("dropped existing tables in %q", cfg.DBName)
	}

	if _, err := db.ExecContext(ctx, mysqlSchemaSQL); err != nil {
		return fmt.Errorf("create tables: %w", err)
	}
	if err := ensureMySQLIndexes(ctx, db); err != nil {
		return err
	}

	log.Printf("database %q is ready (users, posts, comments, reactions)", cfg.DBName)
	return nil
}

func ensureMySQLDatabase(ctx context.Context, cfg *config.Config) error {
	admin, err := sql.Open("mysql", cfg.AdminDSN())
	if err != nil {
		return fmt.Errorf("connect to mysql: %w", err)
	}
	defer admin.Close()
	if err := admin.PingContext(ctx); err != nil {
		return fmt.Errorf("connect to mysql: %w", err)
	}

	var exists int
	err = admin.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = ?`,
		cfg.DBName,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check database: %w", err)
	}
	if exists > 0 {
		log.Printf("database %q already exists", cfg.DBName)
		return nil
	}

	q := fmt.Sprintf(
		"CREATE DATABASE `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci",
		cfg.DBName,
	)
	if _, err := admin.ExecContext(ctx, q); err != nil {
		return fmt.Errorf("create database: %w", err)
	}
	log.Printf("created database %q", cfg.DBName)
	return nil
}

func ensureMySQLIndexes(ctx context.Context, db *sql.DB) error {
	stmts := []struct {
		table string
		name  string
		sql   string
	}{
		{"users", "users_username_idx", `CREATE UNIQUE INDEX users_username_idx ON users (username)`},
		{"users", "users_email_idx", `CREATE UNIQUE INDEX users_email_idx ON users (email)`},
		{"posts", "posts_user_id_idx", `CREATE INDEX posts_user_id_idx ON posts (user_id)`},
		{"posts", "posts_created_at_idx", `CREATE INDEX posts_created_at_idx ON posts (created_at)`},
		{"comments", "comments_post_id_idx", `CREATE INDEX comments_post_id_idx ON comments (post_id)`},
		{"comments", "comments_user_id_idx", `CREATE INDEX comments_user_id_idx ON comments (user_id)`},
		{"reactions", "reactions_comment_id_idx", `CREATE INDEX reactions_comment_id_idx ON reactions (comment_id)`},
		{"reactions", "reactions_user_id_idx", `CREATE INDEX reactions_user_id_idx ON reactions (user_id)`},
		{"reactions", "reactions_type_idx", `CREATE INDEX reactions_type_idx ON reactions (type)`},
	}
	for _, s := range stmts {
		var n int
		err := db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM information_schema.STATISTICS
			 WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND INDEX_NAME = ?`,
			s.table, s.name,
		).Scan(&n)
		if err != nil {
			return fmt.Errorf("check index %s: %w", s.name, err)
		}
		if n > 0 {
			continue
		}
		if _, err := db.ExecContext(ctx, s.sql); err != nil {
			return fmt.Errorf("create index %s: %w", s.name, err)
		}
	}
	return nil
}
