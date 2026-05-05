package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/shout/ai-study-tool/backend/internal/domain"
)

type QuestionGenerationJobRepository struct {
	db *sql.DB
}

func NewQuestionGenerationJobRepository(db *sql.DB) *QuestionGenerationJobRepository {
	return &QuestionGenerationJobRepository{db: db}
}

func (r *QuestionGenerationJobRepository) Create(ctx context.Context, input domain.CreateQuestionGenerationJobInput) (*domain.QuestionGenerationJob, error) {
	bookKey := strings.TrimSpace(input.BookKey)
	if input.UserID == uuid.Nil || bookKey == "" || input.Reason == "" || len(input.HighlightIDs) == 0 {
		return nil, domain.ErrInvalidInput
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("question generation job repo: begin create: %w", err)
	}
	defer tx.Rollback()

	job, err := scanQuestionGenerationJob(tx.QueryRowContext(ctx, `
INSERT INTO question_generation_jobs (user_id, book_key, reason)
VALUES ($1, $2, $3)
RETURNING id, user_id, book_key, status, reason, retry_count, last_error, created_at,
          processing_started_at, completed_at, failed_at
`, input.UserID, bookKey, string(input.Reason)))
	if err != nil {
		return nil, wrapQuestionGenerationJobError("create", err)
	}

	for _, highlightID := range uniqueUUIDs(input.HighlightIDs) {
		if highlightID == uuid.Nil {
			return nil, domain.ErrInvalidInput
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO question_generation_job_highlights (job_id, highlight_id, planned_question_count)
VALUES ($1, $2, 1)
ON CONFLICT (job_id, highlight_id) DO NOTHING
`, job.ID, highlightID); err != nil {
			return nil, wrapQuestionGenerationJobError("attach highlight", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, wrapQuestionGenerationJobError("commit create", err)
	}

	return r.Get(ctx, job.ID, input.UserID)
}

func (r *QuestionGenerationJobRepository) Get(ctx context.Context, jobID, userID uuid.UUID) (*domain.QuestionGenerationJob, error) {
	job, err := scanQuestionGenerationJob(r.db.QueryRowContext(ctx, `
SELECT id, user_id, book_key, status, reason, retry_count, last_error, created_at,
       processing_started_at, completed_at, failed_at
FROM question_generation_jobs
WHERE id = $1
  AND user_id = $2
`, jobID, userID))
	if err != nil {
		return nil, wrapQuestionGenerationJobError("get", err)
	}

	if err := r.loadHighlightIDs(ctx, job); err != nil {
		return nil, err
	}

	return job, nil
}

func (r *QuestionGenerationJobRepository) ListEnqueueFailedByUserID(ctx context.Context, userID uuid.UUID, limit int) ([]*domain.QuestionGenerationJob, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := r.db.QueryContext(ctx, `
SELECT id, user_id, book_key, status, reason, retry_count, last_error, created_at,
       processing_started_at, completed_at, failed_at
FROM question_generation_jobs
WHERE user_id = $1
  AND status = $2
ORDER BY created_at ASC
LIMIT $3
`, userID, string(domain.JobStatusEnqueueFailed), limit)
	if err != nil {
		return nil, wrapQuestionGenerationJobError("list enqueue failed", err)
	}
	defer rows.Close()

	jobs := make([]*domain.QuestionGenerationJob, 0)
	for rows.Next() {
		job, err := scanQuestionGenerationJob(rows)
		if err != nil {
			return nil, wrapQuestionGenerationJobError("scan enqueue failed", err)
		}
		if err := r.loadHighlightIDs(ctx, job); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapQuestionGenerationJobError("rows enqueue failed", err)
	}
	return jobs, nil
}

func (r *QuestionGenerationJobRepository) ClaimQueued(ctx context.Context, jobID, userID uuid.UUID) (*domain.QuestionGenerationJob, bool, error) {
	job, err := scanQuestionGenerationJob(r.db.QueryRowContext(ctx, `
UPDATE question_generation_jobs
SET status = $3,
    processing_started_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND user_id = $2
  AND status = $4
RETURNING id, user_id, book_key, status, reason, retry_count, last_error, created_at,
          processing_started_at, completed_at, failed_at
`, jobID, userID, string(domain.JobStatusProcessing), string(domain.JobStatusQueued)))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, wrapQuestionGenerationJobError("claim queued", err)
	}

	if err := r.loadHighlightIDs(ctx, job); err != nil {
		return nil, false, err
	}

	return job, true, nil
}

func (r *QuestionGenerationJobRepository) RequeueStaleProcessing(ctx context.Context, cutoff time.Time) (int, error) {
	result, err := r.db.ExecContext(ctx, `
UPDATE question_generation_jobs
SET status = $1,
    processing_started_at = NULL,
    updated_at = NOW()
WHERE status = $2
  AND processing_started_at < $3
`, string(domain.JobStatusQueued), string(domain.JobStatusProcessing), cutoff.UTC())
	if err != nil {
		return 0, wrapQuestionGenerationJobError("requeue stale processing", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("question generation job repo: requeue stale processing rows affected: %w", err)
	}
	return int(rowsAffected), nil
}

func (r *QuestionGenerationJobRepository) MarkQueued(ctx context.Context, jobID, userID uuid.UUID) error {
	return r.updateStatus(ctx, jobID, userID, `
UPDATE question_generation_jobs
SET status = $3,
    last_error = NULL,
    processing_started_at = NULL,
    updated_at = NOW()
WHERE id = $1
  AND user_id = $2
`, domain.JobStatusQueued, "mark queued")
}

func (r *QuestionGenerationJobRepository) MarkCompleted(ctx context.Context, jobID, userID uuid.UUID) error {
	return r.updateStatus(ctx, jobID, userID, `
UPDATE question_generation_jobs
SET status = $3,
    completed_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND user_id = $2
`, domain.JobStatusCompleted, "mark completed")
}

func (r *QuestionGenerationJobRepository) MarkEnqueueFailed(ctx context.Context, jobID, userID uuid.UUID, lastError string) error {
	result, err := r.db.ExecContext(ctx, `
UPDATE question_generation_jobs
SET status = $3,
    last_error = LEFT($4, 500),
    updated_at = NOW()
WHERE id = $1
  AND user_id = $2
`, jobID, userID, string(domain.JobStatusEnqueueFailed), strings.TrimSpace(lastError))
	if err != nil {
		return wrapQuestionGenerationJobError("mark enqueue failed", err)
	}
	return ensureQuestionGenerationJobUpdated(result)
}

func (r *QuestionGenerationJobRepository) RecordFailure(ctx context.Context, jobID, userID uuid.UUID, lastError string, maxRetry int) (*domain.QuestionGenerationJob, error) {
	if maxRetry <= 0 {
		maxRetry = domain.JobMaxRetryCount
	}

	job, err := scanQuestionGenerationJob(r.db.QueryRowContext(ctx, `
UPDATE question_generation_jobs
SET retry_count = retry_count + 1,
    status = CASE WHEN retry_count + 1 >= $3 THEN $4 ELSE $5 END,
    last_error = LEFT($6, 500),
    processing_started_at = NULL,
    failed_at = CASE WHEN retry_count + 1 >= $3 THEN NOW() ELSE failed_at END,
    updated_at = NOW()
WHERE id = $1
  AND user_id = $2
RETURNING id, user_id, book_key, status, reason, retry_count, last_error, created_at,
          processing_started_at, completed_at, failed_at
`, jobID, userID, maxRetry, string(domain.JobStatusFailed), string(domain.JobStatusQueued), strings.TrimSpace(lastError)))
	if err != nil {
		return nil, wrapQuestionGenerationJobError("record failure", err)
	}

	if err := r.loadHighlightIDs(ctx, job); err != nil {
		return nil, err
	}

	return job, nil
}

func (r *QuestionGenerationJobRepository) updateStatus(
	ctx context.Context,
	jobID uuid.UUID,
	userID uuid.UUID,
	query string,
	status domain.QuestionGenerationJobStatus,
	action string,
) error {
	result, err := r.db.ExecContext(ctx, query, jobID, userID, string(status))
	if err != nil {
		return wrapQuestionGenerationJobError(action, err)
	}
	return ensureQuestionGenerationJobUpdated(result)
}

func (r *QuestionGenerationJobRepository) loadHighlightIDs(ctx context.Context, job *domain.QuestionGenerationJob) error {
	rows, err := r.db.QueryContext(ctx, `
SELECT highlight_id
FROM question_generation_job_highlights
WHERE job_id = $1
ORDER BY highlight_id
`, job.ID)
	if err != nil {
		return fmt.Errorf("question generation job repo: load highlights: %w", err)
	}
	defer rows.Close()

	highlightIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var highlightID uuid.UUID
		if err := rows.Scan(&highlightID); err != nil {
			return fmt.Errorf("question generation job repo: scan highlight: %w", err)
		}
		highlightIDs = append(highlightIDs, highlightID)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("question generation job repo: rows highlights: %w", err)
	}

	job.HighlightIDs = highlightIDs
	return nil
}

type questionGenerationJobScanner interface {
	Scan(dest ...any) error
}

func scanQuestionGenerationJob(scanner questionGenerationJobScanner) (*domain.QuestionGenerationJob, error) {
	var (
		job                 domain.QuestionGenerationJob
		status              string
		reason              string
		lastError           sql.NullString
		processingStartedAt sql.NullTime
		completedAt         sql.NullTime
		failedAt            sql.NullTime
	)

	if err := scanner.Scan(
		&job.ID,
		&job.UserID,
		&job.BookKey,
		&status,
		&reason,
		&job.RetryCount,
		&lastError,
		&job.CreatedAt,
		&processingStartedAt,
		&completedAt,
		&failedAt,
	); err != nil {
		return nil, err
	}

	job.Status = domain.QuestionGenerationJobStatus(status)
	job.Reason = domain.QuestionGenerationJobReason(reason)
	job.LastError = lastError.String
	job.ProcessingStartedAt = nullableTimePtr(processingStartedAt)
	job.CompletedAt = nullableTimePtr(completedAt)
	job.FailedAt = nullableTimePtr(failedAt)
	return &job, nil
}

func nullableTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}

func uniqueUUIDs(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(ids))
	result := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func ensureQuestionGenerationJobUpdated(result sql.Result) error {
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("question generation job repo: rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func wrapQuestionGenerationJobError(action string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("question generation job repo: %s: %w", action, domain.ErrNotFound)
	}

	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		return fmt.Errorf("question generation job repo: %s: %w", action, domain.ErrAlreadyExists)
	}

	return fmt.Errorf("question generation job repo: %s: %w", action, err)
}
