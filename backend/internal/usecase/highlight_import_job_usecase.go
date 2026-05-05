package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/shout/ai-study-tool/backend/internal/domain"
)

type HighlightImportJobUsecase struct {
	queueRepo     domain.HighlightImportQueueRepository
	highlightRepo domain.HighlightImportRepository
}

func NewHighlightImportJobUsecase(
	queueRepo domain.HighlightImportQueueRepository,
	highlightRepo domain.HighlightImportRepository,
) *HighlightImportJobUsecase {
	return &HighlightImportJobUsecase{
		queueRepo:     queueRepo,
		highlightRepo: highlightRepo,
	}
}

const importJobBatchSize = 50

// ProcessAll は queued 状態のキューを全件処理する。
// Cloud Run Job のエントリーポイントから呼ばれる。
func (u *HighlightImportJobUsecase) ProcessAll(ctx context.Context) error {
	// stale な processing を queued に戻す
	cutoff := time.Now().UTC().Add(-domain.ImportQueueStaleTimeout)
	if requeued, err := u.queueRepo.RequeueStale(ctx, cutoff); err != nil {
		log.Printf("highlight import job: requeue stale error: %v", err)
	} else if requeued > 0 {
		log.Printf("highlight import job: requeued %d stale items", requeued)
	}

	processed := 0
	for {
		batch, err := u.queueRepo.DequeueBatch(ctx, importJobBatchSize)
		if err != nil {
			return fmt.Errorf("highlight import job: dequeue batch: %w", err)
		}
		if len(batch) == 0 {
			break
		}

		for _, item := range batch {
			if err := u.processOne(ctx, item); err != nil {
				log.Printf("highlight import job: process item %s error: %v", item.ID, err)
			}
		}
		processed += len(batch)
	}

	log.Printf("highlight import job: processed %d items", processed)
	return nil
}

func (u *HighlightImportJobUsecase) processOne(ctx context.Context, item *domain.HighlightImportQueue) error {
	// CAS: queued → processing（他の実行との競合防止）
	ok, err := u.queueRepo.ClaimProcessing(ctx, item.ID)
	if err != nil {
		return fmt.Errorf("claim processing: %w", err)
	}
	if !ok {
		return nil // 別の実行がすでに処理中
	}

	highlights, err := u.deserializePayload(item)
	if err != nil {
		return u.fail(ctx, item, fmt.Sprintf("deserialize payload: %v", err))
	}

	if _, err := u.highlightRepo.BulkUpsert(ctx, highlights); err != nil {
		if item.RetryCount >= domain.ImportQueueMaxRetry {
			return u.fail(ctx, item, fmt.Sprintf("bulk upsert (max retry reached): %v", err))
		}
		if requeueErr := u.queueRepo.RequeueWithRetry(ctx, item.ID, err.Error()); requeueErr != nil {
			log.Printf("highlight import job: requeue error for %s: %v", item.ID, requeueErr)
		}
		return nil
	}

	if err := u.queueRepo.MarkCompleted(ctx, item.ID); err != nil {
		log.Printf("highlight import job: mark completed error for %s: %v", item.ID, err)
	}
	return nil
}

func (u *HighlightImportJobUsecase) deserializePayload(item *domain.HighlightImportQueue) ([]*domain.Highlight, error) {
	var highlights []*domain.Highlight
	if err := json.Unmarshal(item.RawPayload, &highlights); err != nil {
		return nil, fmt.Errorf("unmarshal highlights: %w", err)
	}
	if len(highlights) == 0 {
		return nil, fmt.Errorf("empty highlights payload for queue_id=%s", item.ID)
	}
	// UserID の整合性確認
	for i := range highlights {
		highlights[i].UserID = item.UserID
	}
	return highlights, nil
}

func (u *HighlightImportJobUsecase) fail(ctx context.Context, item *domain.HighlightImportQueue, reason string) error {
	if err := u.queueRepo.MarkFailed(ctx, item.ID, reason); err != nil {
		log.Printf("highlight import job: mark failed error for %s: %v", item.ID, err)
	}
	return fmt.Errorf("queue_id=%s failed: %s", item.ID, reason)
}

// ProcessSingle は特定の queue_id を処理する（テスト・デバッグ用）。
func (u *HighlightImportJobUsecase) ProcessSingle(ctx context.Context, queueID uuid.UUID) error {
	batch, err := u.queueRepo.DequeueBatch(ctx, 1)
	if err != nil {
		return err
	}
	for _, item := range batch {
		if item.ID == queueID {
			return u.processOne(ctx, item)
		}
	}
	return nil
}
