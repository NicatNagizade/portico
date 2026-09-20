package seed

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/portico/exampledata/internal/config"
)

var reactionTypes = []string{"like", "dislike", "smile", "angry"}

var firstNames = []string{
	"Alex", "Sam", "Jordan", "Taylor", "Casey", "Morgan", "Riley", "Avery",
	"Quinn", "Jamie", "Cameron", "Drew", "Blake", "Reese", "Skyler", "Parker",
	"Nina", "Omar", "Elena", "Kai", "Mila", "Leo", "Zara", "Noor",
}

var lastNames = []string{
	"Smith", "Johnson", "Brown", "Garcia", "Miller", "Davis", "Wilson", "Moore",
	"Taylor", "Anderson", "Thomas", "Jackson", "White", "Harris", "Martin", "Lee",
	"Clark", "Lewis", "Walker", "Hall", "Allen", "Young", "King", "Wright",
}

var postTitles = []string{
	"Morning thoughts", "Weekend plans", "Quick update", "What I learned today",
	"A small win", "Looking ahead", "Notes from the road", "Behind the scenes",
	"Something curious", "Building in public", "Quiet evening", "Hot take",
}

var commentBodies = []string{
	"Great point!", "Totally agree.", "Interesting perspective.", "Thanks for sharing.",
	"Could you expand on that?", "This helped a lot.", "Nice write-up.", "Well said.",
	"I had a similar experience.", "Not sure I follow.", "Love this.", "Bookmarked.",
}

// Run truncates existing example tables and bulk-loads fake users/posts/comments/reactions.
func Run(ctx context.Context, cfg *config.Config) error {
	conn, err := pgx.Connect(ctx, cfg.DSN())
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(ctx)

	start := time.Now()
	log.Printf(
		"seeding %d users (posts %d–%d, comments %d–%d, reaction chance %.0f%%)…",
		cfg.UserCount, cfg.PostsMin, cfg.PostsMax, cfg.CommentsMin, cfg.CommentsMax,
		cfg.ReactionChance*100,
	)

	if err := prepareForLoad(ctx, conn); err != nil {
		return err
	}

	rng := rand.New(rand.NewSource(42))
	var (
		nextPostID     int64 = 1
		nextCommentID  int64 = 1
		nextReactionID int64 = 1
		totalPosts     int64
		totalComments  int64
		totalReactions int64
	)

	for from := 1; from <= cfg.UserCount; from += cfg.UserBatchSize {
		to := from + cfg.UserBatchSize - 1
		if to > cfg.UserCount {
			to = cfg.UserCount
		}

		stats, err := seedUserBatch(ctx, conn, cfg, rng, from, to, nextPostID, nextCommentID, nextReactionID)
		if err != nil {
			return fmt.Errorf("seed users %d–%d: %w", from, to, err)
		}
		nextPostID = stats.nextPostID
		nextCommentID = stats.nextCommentID
		nextReactionID = stats.nextReactionID
		totalPosts += stats.posts
		totalComments += stats.comments
		totalReactions += stats.reactions

		log.Printf(
			"users %d–%d done | posts=%d comments=%d reactions=%d elapsed=%s",
			from, to, totalPosts, totalComments, totalReactions, time.Since(start).Round(time.Second),
		)
	}

	if err := finalizeLoad(ctx, conn, int64(cfg.UserCount), nextPostID-1, nextCommentID-1, nextReactionID-1); err != nil {
		return err
	}

	log.Printf(
		"seed complete in %s: users=%d posts=%d comments=%d reactions=%d",
		time.Since(start).Round(time.Second),
		cfg.UserCount, totalPosts, totalComments, totalReactions,
	)
	return nil
}

type batchStats struct {
	posts          int64
	comments       int64
	reactions      int64
	nextPostID     int64
	nextCommentID  int64
	nextReactionID int64
}

func prepareForLoad(ctx context.Context, conn *pgx.Conn) error {
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
	for _, s := range stmts {
		if _, err := conn.Exec(ctx, s); err != nil {
			return fmt.Errorf("prepare: %w", err)
		}
	}
	return nil
}

func finalizeLoad(ctx context.Context, conn *pgx.Conn, maxUser, maxPost, maxComment, maxReaction int64) error {
	log.Println("restoring indexes and sequences…")

	if err := setSerial(ctx, conn, "users", maxUser); err != nil {
		return err
	}
	if err := setSerial(ctx, conn, "posts", maxPost); err != nil {
		return err
	}
	if err := setSerial(ctx, conn, "comments", maxComment); err != nil {
		return err
	}
	if err := setSerial(ctx, conn, "reactions", maxReaction); err != nil {
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
	for _, s := range indexStmts {
		if _, err := conn.Exec(ctx, s); err != nil {
			return fmt.Errorf("finalize: %w", err)
		}
	}
	return nil
}

func setSerial(ctx context.Context, conn *pgx.Conn, table string, maxID int64) error {
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

func seedUserBatch(
	ctx context.Context,
	conn *pgx.Conn,
	cfg *config.Config,
	rng *rand.Rand,
	fromUser, toUser int,
	nextPostID, nextCommentID, nextReactionID int64,
) (batchStats, error) {
	userCount := toUser - fromUser + 1
	now := time.Now().UTC()

	users := make([][]any, 0, userCount)
	for id := fromUser; id <= toUser; id++ {
		fn := firstNames[rng.Intn(len(firstNames))]
		ln := lastNames[rng.Intn(len(lastNames))]
		users = append(users, []any{
			int64(id),
			fmt.Sprintf("user_%d", id),
			fmt.Sprintf("user_%d@example.com", id),
			fn + " " + ln,
			fmt.Sprintf("Bio for %s %s (#%d)", fn, ln, id),
			now.Add(-time.Duration(rng.Intn(365*24)) * time.Hour),
		})
	}
	if err := copyRows(ctx, conn, "users",
		[]string{"id", "username", "email", "full_name", "bio", "created_at"}, users); err != nil {
		return batchStats{}, err
	}

	posts := make([][]any, 0, userCount*cfg.PostsMax)
	postIDs := make([]int64, 0, userCount*cfg.PostsMax)
	postAuthors := make([]int64, 0, userCount*cfg.PostsMax)

	postID := nextPostID
	for userID := fromUser; userID <= toUser; userID++ {
		nPosts := cfg.PostsMin + rng.Intn(cfg.PostsMax-cfg.PostsMin+1)
		for i := 0; i < nPosts; i++ {
			title := postTitles[rng.Intn(len(postTitles))]
			posts = append(posts, []any{
				postID,
				int64(userID),
				fmt.Sprintf("%s #%d", title, postID),
				fmt.Sprintf("Body of post %d by user %d. Lorem ipsum dolor sit amet.", postID, userID),
				now.Add(-time.Duration(rng.Intn(180*24)) * time.Hour),
			})
			postIDs = append(postIDs, postID)
			postAuthors = append(postAuthors, int64(userID))
			postID++
		}
	}
	if err := copyRows(ctx, conn, "posts",
		[]string{"id", "user_id", "title", "body", "created_at"}, posts); err != nil {
		return batchStats{}, err
	}

	comments := make([][]any, 0, len(postIDs)*cfg.CommentsMax)
	commentIDs := make([]int64, 0, len(postIDs)*cfg.CommentsMax)

	commentID := nextCommentID
	for i, pid := range postIDs {
		nComments := cfg.CommentsMin + rng.Intn(cfg.CommentsMax-cfg.CommentsMin+1)
		for c := 0; c < nComments; c++ {
			// Only reference users that already exist (created in this or earlier batches).
			commenter := int64(1 + rng.Intn(toUser))
			if commenter == postAuthors[i] && toUser > 1 {
				commenter = commenter%int64(toUser) + 1
				if commenter == postAuthors[i] {
					commenter = 1
					if postAuthors[i] == 1 {
						commenter = 2
					}
				}
			}
			comments = append(comments, []any{
				commentID,
				pid,
				commenter,
				commentBodies[rng.Intn(len(commentBodies))],
				now.Add(-time.Duration(rng.Intn(90*24)) * time.Hour),
			})
			commentIDs = append(commentIDs, commentID)
			commentID++
		}
	}
	if err := copyRows(ctx, conn, "comments",
		[]string{"id", "post_id", "user_id", "body", "created_at"}, comments); err != nil {
		return batchStats{}, err
	}

	reactions := make([][]any, 0, len(commentIDs)/2)
	reactionID := nextReactionID
	for _, cid := range commentIDs {
		if rng.Float64() > cfg.ReactionChance {
			continue
		}
		n := cfg.ReactionsMin + rng.Intn(cfg.ReactionsMax-cfg.ReactionsMin+1)
		seen := make(map[int64]struct{}, n)
		for len(seen) < n {
			uid := int64(1 + rng.Intn(toUser))
			if _, ok := seen[uid]; ok {
				continue
			}
			seen[uid] = struct{}{}
			reactions = append(reactions, []any{
				reactionID,
				cid,
				uid,
				reactionTypes[rng.Intn(len(reactionTypes))],
				now.Add(-time.Duration(rng.Intn(60*24)) * time.Hour),
			})
			reactionID++
		}
	}
	if len(reactions) > 0 {
		if err := copyRows(ctx, conn, "reactions",
			[]string{"id", "comment_id", "user_id", "type", "created_at"}, reactions); err != nil {
			return batchStats{}, err
		}
	}

	return batchStats{
		posts:          int64(len(posts)),
		comments:       int64(len(comments)),
		reactions:      int64(len(reactions)),
		nextPostID:     postID,
		nextCommentID:  commentID,
		nextReactionID: reactionID,
	}, nil
}

func copyRows(ctx context.Context, conn *pgx.Conn, table string, columns []string, rows [][]any) error {
	if len(rows) == 0 {
		return nil
	}
	_, err := conn.CopyFrom(ctx, pgx.Identifier{table}, columns, pgx.CopyFromRows(rows))
	if err != nil {
		return fmt.Errorf("copy %s: %w", table, err)
	}
	return nil
}
