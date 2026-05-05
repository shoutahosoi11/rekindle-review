package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shout/ai-study-tool/backend/internal/domain"
)

type HighlightImportQueueRepository struct {
	db *sql.DB
}

func NewHighlightImportQueueRepository(db *sql.DB) domain.HighlightImportQueueRepository {
	return &HighlightImportQueueRepository{db: db}
}

func (r *HighlightImportQueueRepository) Enqueue(ctx context.Context, userID uuid.UUID, source string, payload []byte) (uuid.UUID, error) {
	id := uuid.New()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO highlight_import_queue (id, user_id, source, raw_payload, status)
		VALUES ($1, $2, $3, $4, 'queued')
	`, id, userID, source, payload)
	if err != nil {
		return uuid.Nil, fmt.Errorf("highlight import queue: enqueue: %w", err)
	}
	return id, nil
}

func (r *HighlightImportQueueRepository) DequeueBatch(ctx context.Context, limit int) ([]*domain.HighlightImportQueue, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, source, raw_payload, status, retry_count,
		       COALESCE(last_error, ''), created_at,
		       processing_started_at, completed_at, failed_at
		FROM highlight_import_queue
		WHERE status = 'queued'
		ORDER BY created_at ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("highlight import queue: dequeue batch: %w", err)
	}
	defer rows.Close()

	var items []*domain.HighlightImportQueue
	for rows.Next() {
		item := &domain.HighlightImportQueue{}
		if err := rows.Scan(
			&item.ID, &item.UserID, &item.Source, &item.RawPayload,
			&item.Status, &item.RetryCount, &item.LastError, &item.CreatedAt,
			&item.ProcessingStartedAt, &item.CompletedAt, &item.FailedAt,
		); err != nil {
			return nil, fmt.Errorf("highlight import queue: scan: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *HighlightImportQueueRepository) ClaimProcessing(ctx context.Context, id uuid.UUID) (bool, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE highlight_import_queue
		SET status = 'processing', processing_started_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status = 'queued'
	`, id)
	if err != nil {
		return false, fmt.Errorf("highlight import queue: claim processing: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (r *HighlightImportQueueRepository) MarkCompleted(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE highlight_import_queue
		SET status = 'completed', completed_at = NOW()
		WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("highlight import queue: mark completed: %w", err)
	}
	return nil
}

func (r *HighlightImportQueueRepository) MarkFailed(ctx context.Context, id uuid.UUID, errMsg string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE highlight_import_queue
		SET status = 'failed', failed_at = NOW(), last_error = $2
		WHERE id = $1
	`, id, errMsg)
	if err != nil {
		return fmt.Errorf("highlight import queue: mark failed: %w", err)
	}
	return nil
}

func (r *HighlightImportQueueRepository) RequeueWithRetry(ctx context.Context, id uuid.UUID, errMsg string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE highlight_import_queue
		SET status = 'queued', retry_count = retry_count + 1, last_error = $2,
		    processing_started_at = NULL
		WHERE id = $1
	`, id, errMsg)
	if err != nil {
		return fmt.Errorf("highlight import queue: requeue with retry: %w", err)
	}
	return nil
}

func (r *HighlightImportQueueRepository) RequeueStale(ctx context.Context, cutoff time.Time) (int, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE highlight_import_queue
		SET status = 'queued', processing_started_at = NULL
		WHERE status = 'processing' AND processing_started_at < $1
	`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("highlight import queue: requeue stale: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(n), nil
}
