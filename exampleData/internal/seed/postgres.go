package seed

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/portico/exampledata/internal/config"
)

type postgresStore struct {
	conn *pgx.Conn
}

func openPostgresStore(ctx context.Context, cfg *config.Config) (*postgresStore, error) {
	conn, err := pgx.Connect(ctx, cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	return &postgresStore{conn: conn}, nil
}

func (s *postgresStore) Close() {
	s.conn.Close(context.Background())
}

func (s *postgresStore) Prepare(ctx context.Context) error {
	log.Println("truncating tables and dropping secondary indexes for faster load…")
	stmts := []string{
		`TRUNCATE reactions, comments, posts, users RESTART IDENTITY CASCADE`,
		`DROP INDEX IF EXISTS users_username_idx`,
		`DROP INDEX IF EXISTS users_email_idx`,
		`DROP INDEX IF EXISTS posts_user_id_idx`,
		`DROP INDEX IF EXISTS posts_created_at_idx`,
		`DROP INDEX IF EXISTS comments_post_id_idx`,
		`DROP INDEX IF EXISTS comments_user_id_idx`,
		`DROP INDEX IF EXISTS reactions_comment_id_idx`,
		`DROP INDEX IF EXISTS reactions_user_id_idx`,
		`DROP INDEX IF EXISTS reactions_type_idx`,
		`ALTER TABLE reactions DROP CONSTRAINT IF EXISTS reactions_comment_user_unique`,
	}
	for _, q := range stmts {
		if _, err := s.conn.Exec(ctx, q); err != nil {
			return fmt.Errorf("prepare: %w", err)
		}
	}
	return nil
}

func (s *postgresStore) Copy(ctx context.Context, table string, columns []string, rows [][]any) error {
	if len(rows) == 0 {
		return nil
	}
	_, err := s.conn.CopyFrom(ctx, pgx.Identifier{table}, columns, pgx.CopyFromRows(rows))
	if err != nil {
		return fmt.Errorf("copy %s: %w", table, err)
	}
	return nil
}

func (s *postgresStore) Finalize(ctx context.Context, maxUser, maxPost, maxComment, maxReaction int64) error {
	log.Println("restoring indexes and sequences…")

	if err := setPostgresSerial(ctx, s.conn, "users", maxUser); err != nil {
		return err
	}
	if err := setPostgresSerial(ctx, s.conn, "posts", maxPost); err != nil {
		return err
	}
	if err := setPostgresSerial(ctx, s.conn, "comments", maxComment); err != nil {
		return err
	}
	if err := setPostgresSerial(ctx, s.conn, "reactions", maxReaction); err != nil {
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
		`ANALYZE users`,
		`ANALYZE posts`,
		`ANALYZE comments`,
		`ANALYZE reactions`,
	}
	for _, q := range indexStmts {
		if _, err := s.conn.Exec(ctx, q); err != nil {
			return fmt.Errorf("finalize: %w", err)
		}
	}
	return nil
}

func setPostgresSerial(ctx context.Context, conn *pgx.Conn, table string, maxID int64) error {
	var q string
	if maxID < 1 {
		q = fmt.Sprintf(`SELECT setval(pg_get_serial_sequence('%s', 'id'), 1, false)`, table)
	} else {
		q = fmt.Sprintf(`SELECT setval(pg_get_serial_sequence('%s', 'id'), %d, true)`, table, maxID)
	}
	if _, err := conn.Exec(ctx, q); err != nil {
		return fmt.Errorf("set sequence %s: %w", table, err)
	}
	return nil
}
