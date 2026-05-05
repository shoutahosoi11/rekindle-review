package persistence

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/shout/ai-study-tool/backend/internal/domain"
	"github.com/shout/ai-study-tool/backend/internal/repository/sqlcgen"
)

type answerRepository struct {
	db      *sql.DB
	queries *sqlcgen.Queries
}

func NewAnswerRepository(db *sql.DB) domain.AnswerRepository {
	return &answerRepository{
		db:      db,
		queries: sqlcgen.New(db),
	}
}

func (r *answerRepository) Upsert(ctx context.Context, input domain.AnswerUpsertInput) (*domain.Answer, error) {
	userID, err := parseAnswerUUID(input.UserID)
	if err != nil {
		return nil, fmt.Errorf("answer repo: parse user id: %w", err)
	}

	questionID, err := parseAnswerUUID(input.QuestionID)
	if err != nil {
		return nil, fmt.Errorf("answer repo: parse question id: %w", err)
	}

	answer, err := r.queries.UpsertAnswer(ctx, sqlcgen.UpsertAnswerParams{
		UserID:     userID,
		QuestionID: questionID,
		UserAnswer: input.UserAnswer,
		IsCorrect:  input.IsCorrect,
	})
	if err != nil {
		return nil, fmt.Errorf("answer repo: upsert: %w", err)
	}

	return toDomainAnswer(answer), nil
}

func (r *answerRepository) UpsertAndUpdateStats(ctx context.Context, input domain.AnswerUpsertInput) (*domain.Answer, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("answer repo: begin tx: %w", err)
	}
	defer tx.Rollback()

	userID, err := parseAnswerUUID(input.UserID)
	if err != nil {
		return nil, fmt.Errorf("answer repo: parse user id: %w", err)
	}

	inputQuestionID, err := parseAnswerUUID(input.QuestionID)
	if err != nil {
		return nil, fmt.Errorf("answer repo: parse question id: %w", err)
	}

	txQueries := r.queries.WithTx(tx)

	var previousCorrect bool
	hadPreviousAnswer := true
	err = tx.QueryRowContext(ctx, `
SELECT is_correct
FROM answers
WHERE user_id = $1
  AND question_id = $2
FOR UPDATE`, userID, inputQuestionID).Scan(&previousCorrect)
	if err != nil {
		if err == sql.ErrNoRows {
			hadPreviousAnswer = false
		} else {
			return nil, fmt.Errorf("answer repo: lock existing answer: %w", err)
		}
	}

	answer, err := txQueries.UpsertAnswer(ctx, sqlcgen.UpsertAnswerParams{
		UserID:     userID,
		QuestionID: inputQuestionID,
		UserAnswer: input.UserAnswer,
		IsCorrect:  input.IsCorrect,
	})
	if err != nil {
		return nil, fmt.Errorf("answer repo: upsert: %w", err)
	}

	answerDelta, correctDelta := answerStatDeltas(hadPreviousAnswer, previousCorrect, input.IsCorrect)
	if _, err := tx.ExecContext(ctx, `
UPDATE questions
SET
    answer_count = GREATEST(answer_count + $2, 0),
    correct_count = GREATEST(correct_count + $3, 0),
    updated_at = NOW()
WHERE id = $1`, inputQuestionID, answerDelta, correctDelta); err != nil {
		return nil, fmt.Errorf("answer repo: update stats: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("answer repo: commit: %w", err)
	}

	return toDomainAnswer(answer), nil
}

func answerStatDeltas(hadPreviousAnswer bool, previousCorrect bool, currentCorrect bool) (int, int) {
	if !hadPreviousAnswer {
		if currentCorrect {
			return 1, 1
		}
		return 1, 0
	}
	if previousCorrect == currentCorrect {
		return 0, 0
	}
	if currentCorrect {
		return 0, 1
	}
	return 0, -1
}

func toDomainAnswer(answer sqlcgen.Answer) *domain.Answer {
	return &domain.Answer{
		ID:         answer.ID.String(),
		UserID:     answer.UserID.String(),
		QuestionID: answer.QuestionID.String(),
		UserAnswer: answer.UserAnswer,
		IsCorrect:  answer.IsCorrect,
		CreatedAt:  answer.CreatedAt,
		UpdatedAt:  answer.UpdatedAt,
	}
}

func parseAnswerUUID(value string) (uuid.UUID, error) {
	return uuid.Parse(value)
}
