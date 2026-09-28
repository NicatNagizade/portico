package seed

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/portico/exampledata/internal/config"
)

const mysqlInsertChunk = 500

type mysqlStore struct {
	db *sql.DB
}

func openMySQLStore(ctx context.Context, cfg *config.Config) (*mysqlStore, error) {
	db, err := sql.Open("mysql", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect: %w", err)
	}
	return &mysqlStore{db: db}, nil
}

func (s *mysqlStore) Close() {
	_ = s.db.Close()
}

func (s *mysqlStore) Prepare(ctx context.Context) error {
	log.Println("truncating tables and dropping secondary indexes for faster load…")
	stmts := []string{
		`SET FOREIGN_KEY_CHECKS=0`,
		`TRUNCATE TABLE reactions`,
		`TRUNCATE TABLE comments`,
		`TRUNCATE TABLE posts`,
		`TRUNCATE TABLE users`,
		`DROP INDEX users_username_idx ON users`,
		`DROP INDEX users_email_idx ON users`,
		`DROP INDEX posts_user_id_idx ON posts`,
		`DROP INDEX posts_created_at_idx ON posts`,
		`DROP INDEX comments_post_id_idx ON comments`,
		`DROP INDEX comments_user_id_idx ON comments`,
		`DROP INDEX reactions_comment_id_idx ON reactions`,
		`DROP INDEX reactions_user_id_idx ON reactions`,
		`DROP INDEX reactions_type_idx ON reactions`,
		`ALTER TABLE reactions DROP INDEX reactions_comment_user_unique`,
		`SET FOREIGN_KEY_CHECKS=1`,
	}
	for _, q := range stmts {
		if _, err := s.db.ExecContext(ctx, q); err != nil && !isMySQLMissingKey(err) {
			return fmt.Errorf("prepare: %w", err)
		}
	}
	return nil
}

func isMySQLMissingKey(err error) bool {
	var me *mysqldriver.MySQLError
	return errors.As(err, &me) && me.Number == 1091
}

func (s *mysqlStore) Copy(ctx context.Context, table string, columns []string, rows [][]any) error {
	if len(rows) == 0 {
		return nil
	}
	colList := "`" + strings.Join(columns, "`,`") + "`"
	placeholders := "(" + strings.Repeat("?,", len(columns)-1) + "?)"

	for start := 0; start < len(rows); start += mysqlInsertChunk {
		end := start + mysqlInsertChunk
		if end > len(rows) {
			end = len(rows)
		}
		chunk := rows[start:end]
		var b strings.Builder
		args := make([]any, 0, len(chunk)*len(columns))
		b.WriteString("INSERT INTO `")
		b.WriteString(table)
		b.WriteString("` (")
		b.WriteString(colList)
		b.WriteString(") VALUES ")
		for i, row := range chunk {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(placeholders)
			args = append(args, row...)
		}
		if _, err := s.db.ExecContext(ctx, b.String(), args...); err != nil {
			return fmt.Errorf("insert %s: %w", table, err)
		}
	}
	return nil
}

func (s *mysqlStore) Finalize(ctx context.Context, maxUser, maxPost, maxComment, maxReaction int64) error {
	log.Println("restoring indexes and auto-increment…")

	if err := setMySQLAutoIncrement(ctx, s.db, "users", maxUser); err != nil {
		return err
	}
	if err := setMySQLAutoIncrement(ctx, s.db, "posts", maxPost); err != nil {
		return err
	}
	if err := setMySQLAutoIncrement(ctx, s.db, "comments", maxComment); err != nil {
		return err
	}
	if err := setMySQLAutoIncrement(ctx, s.db, "reactions", maxReaction); err != nil {
		return err
	}

	indexStmts := []string{
		`CREATE UNIQUE INDEX users_username_idx ON users (username)`,
		`CREATE UNIQUE INDEX users_email_idx ON users (email)`,
		`CREATE INDEX posts_user_id_idx ON posts (user_id)`,
		`CREATE INDEX posts_created_at_idx ON posts (created_at)`,
		`CREATE INDEX comments_post_id_idx ON comments (post_id)`,
		`CREATE INDEX comments_user_id_idx ON comments (user_id)`,
		`ALTER TABLE reactions ADD CONSTRAINT reactions_comment_user_unique UNIQUE (comment_id, user_id)`,
		`CREATE INDEX reactions_comment_id_idx ON reactions (comment_id)`,
		`CREATE INDEX reactions_user_id_idx ON reactions (user_id)`,
		`CREATE INDEX reactions_type_idx ON reactions (type)`,
		`ANALYZE TABLE users, posts, comments, reactions`,
	}
	for _, q := range indexStmts {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("finalize: %w", err)
		}
	}
	return nil
}

func setMySQLAutoIncrement(ctx context.Context, db *sql.DB, table string, maxID int64) error {
	next := maxID + 1
	if next < 1 {
		next = 1
	}
	q := fmt.Sprintf("ALTER TABLE `%s` AUTO_INCREMENT = %d", table, next)
	if _, err := db.ExecContext(ctx, q); err != nil {
		return fmt.Errorf("set auto_increment %s: %w", table, err)
	}
	return nil
}
