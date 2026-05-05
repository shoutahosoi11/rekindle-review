package persistence

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/shout/ai-study-tool/backend/internal/domain"
	"github.com/shout/ai-study-tool/backend/internal/repository/sqlcgen"
)

type socialRepository struct {
	db      *sql.DB
	queries *sqlcgen.Queries
}

func NewSocialRepository(db *sql.DB) domain.SocialRepository {
	return &socialRepository{
		db:      db,
		queries: sqlcgen.New(db),
	}
}

func (r *socialRepository) Follow(ctx context.Context, followerID, followeeID string) error {
	parsedFollowerID, err := parseSocialUUID(followerID)
	if err != nil {
		return fmt.Errorf("social repo: parse follower id: %w", err)
	}

	parsedFolloweeID, err := parseSocialUUID(followeeID)
	if err != nil {
		return fmt.Errorf("social repo: parse followee id: %w", err)
	}

	_, err = r.queries.FollowUser(ctx, sqlcgen.FollowUserParams{
		FollowerID: parsedFollowerID,
		FolloweeID: parsedFolloweeID,
	})
	if err != nil {
		return fmt.Errorf("social repo: follow insert: %w", err)
	}

	return nil
}

func (r *socialRepository) Unfollow(ctx context.Context, followerID, followeeID string) error {
	parsedFollowerID, err := parseSocialUUID(followerID)
	if err != nil {
		return fmt.Errorf("social repo: parse follower id: %w", err)
	}

	parsedFolloweeID, err := parseSocialUUID(followeeID)
	if err != nil {
		return fmt.Errorf("social repo: parse followee id: %w", err)
	}

	_, err = r.queries.UnfollowUser(ctx, sqlcgen.UnfollowUserParams{
		FollowerID: parsedFollowerID,
		FolloweeID: parsedFolloweeID,
	})
	if err != nil {
		return fmt.Errorf("social repo: unfollow delete: %w", err)
	}

	return nil
}

func (r *socialRepository) Like(ctx context.Context, userID, postID string) error {
	return r.adjustPostCounter(ctx, `
INSERT INTO likes (user_id, post_id)
VALUES ($1, $2)
ON CONFLICT (user_id, post_id) DO NOTHING
`, userID, postID, "like_count", 1)
}

func (r *socialRepository) Unlike(ctx context.Context, userID, postID string) error {
	return r.adjustPostCounter(ctx, `
DELETE FROM likes
WHERE user_id = $1 AND post_id = $2
`, userID, postID, "like_count", -1)
}

func (r *socialRepository) Repost(ctx context.Context, userID, postID string) error {
	return r.adjustPostCounter(ctx, `
INSERT INTO reposts (user_id, post_id)
VALUES ($1, $2)
ON CONFLICT (user_id, post_id) DO NOTHING
`, userID, postID, "repost_count", 1)
}

func (r *socialRepository) Unrepost(ctx context.Context, userID, postID string) error {
	return r.adjustPostCounter(ctx, `
DELETE FROM reposts
WHERE user_id = $1 AND post_id = $2
`, userID, postID, "repost_count", -1)
}

func (r *socialRepository) CreateComment(ctx context.Context, comment *domain.Comment) (*domain.Comment, error) {
	var created *domain.Comment

	err := r.withTx(ctx, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
WITH inserted AS (
	INSERT INTO comments (post_id, user_id, body, content)
	VALUES ($1, $2, $3, $3)
	RETURNING id, post_id, user_id, content, created_at
)
SELECT
	inserted.id,
	inserted.post_id,
	inserted.user_id,
	u.username,
	u.display_name,
	u.avatar_url,
	inserted.content,
	inserted.created_at
FROM inserted
JOIN users u ON u.id = inserted.user_id
`, comment.PostID, comment.UserID, comment.Content)

		var (
			id          uuid.UUID
			postID      uuid.UUID
			userID      uuid.UUID
			username    string
			displayName string
			avatarURL   sql.NullString
			content     string
			createdAt   sql.NullTime
		)

		if err := row.Scan(&id, &postID, &userID, &username, &displayName, &avatarURL, &content, &createdAt); err != nil {
			return fmt.Errorf("social repo: insert comment: %w", err)
		}

		if _, err := tx.ExecContext(ctx, `
UPDATE posts
SET comment_count = GREATEST(comment_count + 1, 0)
WHERE id = $1
`, comment.PostID); err != nil {
			return fmt.Errorf("social repo: increment comment count: %w", err)
		}

		created = &domain.Comment{
			ID:          id.String(),
			PostID:      postID.String(),
			UserID:      userID.String(),
			Username:    username,
			DisplayName: displayName,
			AvatarURL:   fromNullString(avatarURL),
			Content:     content,
			CreatedAt:   createdAt.Time,
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return created, nil
}

func (r *socialRepository) ListComments(ctx context.Context, input domain.ListCommentsInput) ([]*domain.Comment, error) {
	postID, err := uuid.Parse(input.PostID)
	if err != nil {
		return nil, fmt.Errorf("social repo: parse post id for list comments: %w", err)
	}

	rows, err := r.queries.ListCommentsByPostID(ctx, sqlcgen.ListCommentsByPostIDParams{
		PostID: postID,
		Limit:  int32(input.Limit),
		Offset: int32(input.Offset),
	})
	if err != nil {
		return nil, fmt.Errorf("social repo: list comments query: %w", err)
	}

	comments := make([]*domain.Comment, 0, len(rows))
	for _, row := range rows {
		comments = append(comments, &domain.Comment{
			ID:          row.ID.String(),
			PostID:      row.PostID.String(),
			UserID:      row.UserID.String(),
			Username:    row.Username,
			DisplayName: row.DisplayName,
			AvatarURL:   fromNullString(row.AvatarUrl),
			Content:     row.Content,
			CreatedAt:   row.CreatedAt,
		})
	}

	return comments, nil
}

func (r *socialRepository) adjustPostCounter(ctx context.Context, mutation, userID, postID, column string, delta int) error {
	return r.withTx(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, mutation, userID, postID)
		if err != nil {
			return fmt.Errorf("social repo: mutate relation: %w", err)
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("social repo: mutate rows affected: %w", err)
		}

		if rowsAffected == 0 {
			return nil
		}

		query := fmt.Sprintf(`
UPDATE posts
SET %s = GREATEST(%s + $2, 0)
WHERE id = $1
`, column, column)

		if _, err := tx.ExecContext(ctx, query, postID, delta); err != nil {
			return fmt.Errorf("social repo: update post counter: %w", err)
		}

		return nil
	})
}

func (r *socialRepository) withTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("social repo: begin tx: %w", err)
	}

	if err := fn(tx); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("%w: rollback: %v", err, rollbackErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("social repo: commit tx: %w", err)
	}

	return nil
}

func parseSocialUUID(value string) (uuid.UUID, error) {
	return uuid.Parse(value)
}
