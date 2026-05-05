package persistence

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/shout/ai-study-tool/backend/internal/domain"
)

func TestQuestionSyncRepositoriesIntegration(t *testing.T) {
	databaseURL := os.Getenv("INTEGRATION_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("INTEGRATION_DATABASE_URL is not set")
	}

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping db: %v", err)
	}
	resetIntegrationDB(t, db)

	userID := uuid.New()
	if _, err := db.ExecContext(ctx, `
INSERT INTO users (id, firebase_uid, username, display_name)
VALUES ($1, $2, $3, $4)
`, userID, "firebase-"+userID.String(), "user_"+userID.String()[:8], "Integration User"); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	questionRepo := NewQuestionRepository(db)
	highlightRepo := NewHighlightRepository(db)
	day := time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC)

	count, err := questionRepo.GetDailyGeneratedCount(ctx, userID, day)
	if err != nil {
		t.Fatalf("get daily count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected initial daily count 0, got %d", count)
	}

	completedHighlightID := uuid.New()
	pendingHighlightID := uuid.New()
	insertIntegrationHighlight(t, db, userID, completedHighlightID, domain.HighlightStatusCompleted, 2, nil)
	insertIntegrationHighlight(t, db, userID, pendingHighlightID, domain.HighlightStatusPending, 0, nil)

	queuedIDs, reserved, err := questionRepo.QueueHighlightsWithinDailyLimit(
		ctx,
		userID,
		day,
		100,
		[]uuid.UUID{completedHighlightID, pendingHighlightID},
		map[uuid.UUID]int{
			completedHighlightID: 3,
			pendingHighlightID:   3,
		},
		time.Now(),
	)
	if err != nil {
		t.Fatalf("queue highlights within daily limit: %v", err)
	}
	if !reserved || len(queuedIDs) != 1 || queuedIDs[0] != completedHighlightID {
		t.Fatalf("expected transactional queue to reserve and queue highlight, reserved=%v ids=%#v", reserved, queuedIDs)
	}

	count, err = questionRepo.GetDailyGeneratedCount(ctx, userID, day)
	if err != nil {
		t.Fatalf("get daily count after transactional queue: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected daily count 3 after transactional queue, got %d", count)
	}

	var status string
	var retryCount int
	if err := db.QueryRowContext(ctx, `
SELECT status, retry_count
FROM highlights
WHERE id = $1
`, completedHighlightID).Scan(&status, &retryCount); err != nil {
		t.Fatalf("read transactionally queued highlight: %v", err)
	}
	if status != string(domain.HighlightStatusPending) || retryCount != 0 {
		t.Fatalf("expected transactionally queued highlight pending with retry_count=0, got status=%s retry=%d", status, retryCount)
	}

	deniedHighlightID := uuid.New()
	insertIntegrationHighlight(t, db, userID, deniedHighlightID, domain.HighlightStatusCompleted, 2, nil)

	queuedIDs, reserved, err = questionRepo.QueueHighlightsWithinDailyLimit(
		ctx,
		userID,
		day,
		100,
		[]uuid.UUID{deniedHighlightID},
		map[uuid.UUID]int{deniedHighlightID: 98},
		time.Now(),
	)
	if err != nil {
		t.Fatalf("queue highlights within denied daily limit: %v", err)
	}
	if reserved || len(queuedIDs) != 0 {
		t.Fatalf("expected transactional queue to be denied, reserved=%v ids=%#v", reserved, queuedIDs)
	}

	count, err = questionRepo.GetDailyGeneratedCount(ctx, userID, day)
	if err != nil {
		t.Fatalf("get daily count after denied transactional queue: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected daily count to remain 3 after denied queue, got %d", count)
	}

	if err := db.QueryRowContext(ctx, `
SELECT status, retry_count
FROM highlights
WHERE id = $1
`, deniedHighlightID).Scan(&status, &retryCount); err != nil {
		t.Fatalf("read denied transactional highlight: %v", err)
	}
	if status != string(domain.HighlightStatusCompleted) || retryCount != 2 {
		t.Fatalf("expected denied highlight rollback to completed retry=2, got status=%s retry=%d", status, retryCount)
	}

	oldProcessingID := uuid.New()
	newProcessingID := uuid.New()
	oldStartedAt := time.Now().Add(-20 * time.Minute)
	newStartedAt := time.Now().Add(-1 * time.Minute)
	insertIntegrationHighlight(t, db, userID, oldProcessingID, domain.HighlightStatusProcessing, 0, &oldStartedAt)
	insertIntegrationHighlight(t, db, userID, newProcessingID, domain.HighlightStatusProcessing, 0, &newStartedAt)

	requeued, err := highlightRepo.RequeueStaleProcessing(ctx, time.Now().Add(-10*time.Minute))
	if err != nil {
		t.Fatalf("requeue stale processing: %v", err)
	}
	if requeued != 1 {
		t.Fatalf("expected 1 stale highlight requeued, got %d", requeued)
	}

	if err := db.QueryRowContext(ctx, `
SELECT status
FROM highlights
WHERE id = $1
`, oldProcessingID).Scan(&status); err != nil {
		t.Fatalf("read old processing highlight: %v", err)
	}
	if status != string(domain.HighlightStatusPending) {
		t.Fatalf("expected old processing highlight pending, got %s", status)
	}

	if err := db.QueryRowContext(ctx, `
SELECT status
FROM highlights
WHERE id = $1
`, newProcessingID).Scan(&status); err != nil {
		t.Fatalf("read new processing highlight: %v", err)
	}
	if status != string(domain.HighlightStatusProcessing) {
		t.Fatalf("expected new processing highlight to stay processing, got %s", status)
	}
}

func resetIntegrationDB(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`
TRUNCATE users, books RESTART IDENTITY CASCADE
`); err != nil {
		t.Fatalf("reset integration db: %v", err)
	}
}

func insertIntegrationHighlight(t *testing.T, db *sql.DB, userID uuid.UUID, highlightID uuid.UUID, status domain.HighlightStatus, retryCount int, processingStartedAt *time.Time) {
	t.Helper()
	if _, err := db.Exec(`
INSERT INTO highlights (
    id,
    user_id,
    book_title,
    asin,
    content,
    content_hash,
    source,
    status,
    retry_count,
    processing_started_at
) VALUES ($1, $2, $3, $4, $5, $6, 'extension', $7, $8, $9)
`, highlightID, userID, "Integration Book", "ASIN-"+highlightID.String()[:8], "integration highlight "+highlightID.String(), "hash-"+highlightID.String(), string(status), retryCount, processingStartedAt); err != nil {
		t.Fatalf("insert highlight %s: %v", highlightID, err)
	}
}
