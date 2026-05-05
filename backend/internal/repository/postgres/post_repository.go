package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/shout/ai-study-tool/backend/internal/domain"
	"github.com/shout/ai-study-tool/backend/internal/repository/sqlcgen"
)

type postRepository struct {
	db      *sql.DB
	queries *sqlcgen.Queries
}

func NewPostRepository(db *sql.DB) domain.PostRepository {
	return &postRepository{
		db:      db,
		queries: sqlcgen.New(db),
	}
}

func (r *postRepository) GetTimeline(ctx context.Context, params domain.TimelineParams) ([]*domain.TimelinePost, error) {
	query := `
WITH scored AS (
  SELECT
    p.id,
    MIN(
      CASE
        WHEN p.user_id = $1          THEN 0
        WHEN f.follower_id IS NOT NULL THEN 1
        WHEN ub.user_id    IS NOT NULL THEN 2
        WHEN hb.user_id    IS NOT NULL THEN 2
        WHEN ui.user_id    IS NOT NULL THEN 3
      END
    ) AS score
  FROM posts p
  LEFT JOIN follows        f  ON p.user_id  = f.followee_id  AND f.follower_id = $1
  LEFT JOIN user_books     ub ON p.book_id  = ub.book_id     AND ub.user_id    = $1
  LEFT JOIN highlights     hb ON p.book_title IS NOT NULL
                             AND lower(trim(coalesce(hb.book_title, ''))) = lower(trim(coalesce(p.book_title, '')))
                             AND hb.user_id = $1
  LEFT JOIN user_interests ui ON p.field_id = ui.field_id    AND ui.user_id    = $1
  WHERE p.user_id = $1
     OR f.follower_id IS NOT NULL
     OR ub.user_id IS NOT NULL
     OR hb.user_id IS NOT NULL
     OR ui.user_id IS NOT NULL
  GROUP BY p.id
)
SELECT
  p.id, p.user_id, p.question_id, p.book_id, p.field_id,
  p.body, COALESCE(p.book_title, b.title) AS book_title, p.question_count,
  p.type, p.repost_count, p.like_count, p.comment_count,
  p.created_at, p.updated_at,
  s.score,
  u.username, u.display_name, u.avatar_url,
  fi.name   AS field_name
FROM scored s
JOIN  posts  p  ON s.id        = p.id
JOIN  users  u  ON p.user_id   = u.id
LEFT JOIN books  b  ON p.book_id   = b.id
LEFT JOIN fields fi ON p.field_id  = fi.id
ORDER BY s.score ASC, p.created_at DESC
LIMIT  $2
OFFSET $3`

	rows, err := r.db.QueryContext(ctx, query, params.UserID, params.Limit, params.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []*domain.TimelinePost
	for rows.Next() {
		tp := &domain.TimelinePost{}
		err := rows.Scan(
			&tp.ID, &tp.UserID, &tp.QuestionID, &tp.BookID, &tp.FieldID,
			&tp.Body, &tp.BookTitle, &tp.QuestionCount,
			&tp.Type, &tp.RepostCount, &tp.LikeCount, &tp.CommentCount,
			&tp.CreatedAt, &tp.UpdatedAt,
			&tp.Score,
			&tp.Username, &tp.DisplayName, &tp.AvatarURL,
			&tp.FieldName,
		)
		if err != nil {
			return nil, err
		}
		posts = append(posts, tp)
	}
	return posts, rows.Err()
}

func (r *postRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.TimelinePost, error) {
	row, err := r.queries.GetTimelinePostByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("post repo: get by id: %w", err)
	}

	return &domain.TimelinePost{
		Post: domain.Post{
			ID:            row.ID,
			UserID:        row.UserID,
			QuestionID:    fromNullUUID(row.QuestionID),
			BookID:        fromNullUUID(row.BookID),
			FieldID:       fromNullUUID(row.FieldID),
			Body:          fromNullString(row.Body),
			BookTitle:     nullableString(row.BookTitle),
			QuestionCount: int(row.QuestionCount),
			Type:          row.Type,
			RepostCount:   int(row.RepostCount),
			LikeCount:     int(row.LikeCount),
			CommentCount:  int(row.CommentCount),
			CreatedAt:     row.CreatedAt,
			UpdatedAt:     row.UpdatedAt,
		},
		Score:       int(row.Score),
		Username:    row.Username,
		DisplayName: row.DisplayName,
		AvatarURL:   fromNullString(row.AvatarUrl),
		FieldName:   fromNullString(row.FieldName),
	}, nil
}

func (r *postRepository) Create(ctx context.Context, input domain.CreatePostInput) (*domain.Post, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("post repo: begin tx: %w", err)
	}

	query := `
INSERT INTO posts (user_id, question_id, book_id, field_id, body, book_title, question_count, type)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, user_id, question_id, book_id, field_id, body, book_title, question_count, type, repost_count, like_count, comment_count, created_at, updated_at`

	row := tx.QueryRowContext(ctx, query,
		input.UserID, input.QuestionID, input.BookID, input.FieldID, nullableString(input.Body), nullableString(input.BookTitle), input.QuestionCount, input.Type,
	)
	p := &domain.Post{}
	err = row.Scan(
		&p.ID, &p.UserID, &p.QuestionID, &p.BookID, &p.FieldID, &p.Body, &p.BookTitle, &p.QuestionCount,
		&p.Type, &p.RepostCount, &p.LikeCount, &p.CommentCount,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("post repo: create post: %w", err)
	}

	for _, question := range input.Questions {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO post_questions (post_id, question_id, sort_order, note)
VALUES ($1, $2, $3, $4)
`, p.ID, question.QuestionID, question.SortOrder, nullableString(question.Note)); err != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("post repo: create post question: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("post repo: commit tx: %w", err)
	}
	return p, nil
}

func (r *postRepository) ListQuestionsByPostID(ctx context.Context, postID uuid.UUID) ([]*domain.PostedQuestion, error) {
	rows, err := r.queries.ListPostedQuestionsByPostID(ctx, postID)
	if err != nil {
		return nil, fmt.Errorf("post repo: list questions by post id: %w", err)
	}

	questions := make([]*domain.PostedQuestion, 0, len(rows))
	for _, row := range rows {
		var options []string
		if row.Options.Valid {
			if err := json.Unmarshal(row.Options.RawMessage, &options); err != nil {
				options = []string{}
			}
		}

		questions = append(questions, &domain.PostedQuestion{
			Question: domain.Question{
				ID:            row.ID.String(),
				QuestionType:  domain.QuestionType(row.QuestionType),
				Content:       row.Body,
				Options:       options,
				CorrectAnswer: row.CorrectAnswer,
				Explanation:   row.Explanation.String,
			},
			Note:      row.Note.String,
			SortOrder: int(row.SortOrder),
		})
	}

	if len(questions) == 0 {
		exists, err := r.queries.PostExists(ctx, postID)
		if err != nil {
			return nil, fmt.Errorf("post repo: check post exists: %w", err)
		}
		if !exists {
			return nil, domain.ErrNotFound
		}
	}

	return questions, nil
}

func (r *postRepository) CanView(ctx context.Context, viewerID, postID uuid.UUID) (bool, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT EXISTS (
	SELECT 1
	FROM posts p
	LEFT JOIN follows f ON p.user_id = f.followee_id AND f.follower_id = $1
	LEFT JOIN user_books ub ON p.book_id = ub.book_id AND ub.user_id = $1
	LEFT JOIN highlights hb ON p.book_title IS NOT NULL
		AND lower(trim(coalesce(hb.book_title, ''))) = lower(trim(coalesce(p.book_title, '')))
		AND hb.user_id = $1
	LEFT JOIN user_interests ui ON p.field_id = ui.field_id AND ui.user_id = $1
	WHERE p.id = $2
		AND (
			p.user_id = $1
			OR f.follower_id IS NOT NULL
			OR ub.user_id IS NOT NULL
			OR hb.user_id IS NOT NULL
			OR ui.user_id IS NOT NULL
		)
)`, viewerID, postID)

	var canView bool
	if err := row.Scan(&canView); err != nil {
		return false, fmt.Errorf("post repo: can view: %w", err)
	}

	return canView, nil
}

func nullableString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func fromNullUUID(value uuid.NullUUID) *uuid.UUID {
	if !value.Valid {
		return nil
	}
	return &value.UUID
}
