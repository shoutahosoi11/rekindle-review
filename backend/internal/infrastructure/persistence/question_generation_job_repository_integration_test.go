package persistence

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/shout/ai-study-tool/backend/internal/domain"
)

func TestQuestionGenerationJobRepositoryIntegration(t *testing.T) {
	db := openQuestionGenerationJobIntegrationDB(t)
	defer db.Close()

	ctx := context.Background()
	applyQuestionGenerationJobRollback(t, db)
	applyQuestionGenerationJobMigrations(t, db)
	applyQuestionGenerationJobMigrations(t, db)
	resetIntegrationDB(t, db)

	userID := insertQuestionGenerationJobUser(t, db)
	highlightIDs := []uuid.UUID{
		insertQuestionGenerationJobHighlight(t, db, userID),
		insertQuestionGenerationJobHighlight(t, db, userID),
	}

	repo := NewQuestionGenerationJobRepository(db)
	job, err := repo.Create(ctx, domain.CreateQuestionGenerationJobInput{
		UserID:       userID,
		BookKey:      "book-1",
		Reason:       domain.JobReasonHighlightBatchThreshold,
		HighlightIDs: highlightIDs,
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	if job.Status != domain.JobStatusQueued {
		t.Fatalf("expected queued job, got %s", job.Status)
	}
	if len(job.HighlightIDs) != len(highlightIDs) {
		t.Fatalf("expected %d highlights, got %d", len(highlightIDs), len(job.HighlightIDs))
	}

	_, err = repo.Create(ctx, domain.CreateQuestionGenerationJobInput{
		UserID:       userID,
		BookKey:      "book-1",
		Reason:       domain.JobReasonHighlightBatchThreshold,
		HighlightIDs: []uuid.UUID{insertQuestionGenerationJobHighlight(t, db, userID)},
	})
	if !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("expected active duplicate to return ErrAlreadyExists, got %v", err)
	}

	claimed, ok, err := repo.ClaimQueued(ctx, job.ID, userID)
	if err != nil {
		t.Fatalf("claim queued: %v", err)
	}
	if !ok || claimed.Status != domain.JobStatusProcessing || claimed.ProcessingStartedAt == nil {
		t.Fatalf("expected processing claim, ok=%v job=%#v", ok, claimed)
	}

	_, ok, err = repo.ClaimQueued(ctx, job.ID, userID)
	if err != nil {
		t.Fatalf("claim queued second time: %v", err)
	}
	if ok {
		t.Fatal("expected second claim to be no-op")
	}

	failed, err := repo.RecordFailure(ctx, job.ID, userID, "temporary", domain.JobMaxRetryCount)
	if err != nil {
		t.Fatalf("record failure: %v", err)
	}
	if failed.Status != domain.JobStatusQueued || failed.RetryCount != 1 || failed.LastError != "temporary" {
		t.Fatalf("unexpected failed retry state: %#v", failed)
	}

	if err := repo.MarkEnqueueFailed(ctx, job.ID, userID, "enqueue unavailable"); err != nil {
		t.Fatalf("mark enqueue failed: %v", err)
	}
	if err := repo.MarkQueued(ctx, job.ID, userID); err != nil {
		t.Fatalf("mark queued: %v", err)
	}
	if err := repo.MarkCompleted(ctx, job.ID, userID); err != nil {
		t.Fatalf("mark completed: %v", err)
	}

	completed, err := repo.Get(ctx, job.ID, userID)
	if err != nil {
		t.Fatalf("get completed: %v", err)
	}
	if completed.Status != domain.JobStatusCompleted || completed.CompletedAt == nil {
		t.Fatalf("expected completed job, got %#v", completed)
	}
}

func openQuestionGenerationJobIntegrationDB(t *testing.T) *sql.DB {
	t.Helper()
	databaseURL := os.Getenv("INTEGRATION_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("INTEGRATION_DATABASE_URL is not set")
	}

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.PingContext(context.Background()); err != nil {
		t.Fatalf("ping db: %v", err)
	}
	return db
}

func applyQuestionGenerationJobMigrations(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, file := range []string{
		"032_create_question_generation_jobs.sql",
		"033_add_questions_superseded_at.sql",
		"034_add_users_last_sync_at.sql",
		"035_add_highlights_book_status_index.sql",
	} {
		sqlBytes, err := os.ReadFile(filepath.Join("..", "..", "..", "db", "migrations", file))
		if err != nil {
			t.Fatalf("read migration %s: %v", file, err)
		}
		if _, err := db.Exec(string(sqlBytes)); err != nil {
			t.Fatalf("apply migration %s: %v", file, err)
		}
	}
}

func applyQuestionGenerationJobRollback(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`
DROP INDEX IF EXISTS idx_highlights_user_book_status;
ALTER TABLE highlights DROP COLUMN IF EXISTS book_key;
ALTER TABLE users DROP COLUMN IF EXISTS last_sync_at;
DROP INDEX IF EXISTS idx_questions_active_by_highlight;
ALTER TABLE questions DROP COLUMN IF EXISTS superseded_at;
DROP TABLE IF EXISTS question_generation_job_highlights;
DROP INDEX IF EXISTS idx_question_generation_jobs_enqueue_failed;
DROP INDEX IF EXISTS uq_question_generation_jobs_active;
DROP TABLE IF EXISTS question_generation_jobs;
`); err != nil {
		t.Fatalf("rollback phase1 migrations: %v", err)
	}
}

func insertQuestionGenerationJobUser(t *testing.T, db *sql.DB) uuid.UUID {
	t.Helper()
	userID := uuid.New()
	if _, err := db.Exec(`
INSERT INTO users (id, firebase_uid, username, display_name)
VALUES ($1, $2, $3, $4)
`, userID, "firebase-"+userID.String(), "user_"+userID.String()[:8], "Integration User"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return userID
}

func insertQuestionGenerationJobHighlight(t *testing.T, db *sql.DB, userID uuid.UUID) uuid.UUID {
	t.Helper()
	highlightID := uuid.New()
	if _, err := db.Exec(`
INSERT INTO highlights (
    id,
    user_id,
    book_title,
    book_author,
    asin,
    content,
    content_hash,
    source,
    status,
    book_key
) VALUES ($1, $2, 'Integration Book', 'Author', 'book-1', $3, $4, 'extension', 'pending', 'book-1')
`, highlightID, userID, "integration highlight "+highlightID.String(), "hash-"+highlightID.String()); err != nil {
		t.Fatalf("insert highlight: %v", err)
	}
	return highlightID
}
