package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shout/ai-study-tool/backend/internal/domain"
)

type mockQuestionSyncHighlightRepository struct {
	candidates             []domain.QuestionGenerationBookCandidate
	pendingByBook          map[string][]*domain.Highlight
	markedProcessing       []uuid.UUID
	markedPendingBookKey   string
	markedPendingQuestion  uuid.UUID
	listCandidatesChanged  *time.Time
	listCandidatesCalled   int
	markPendingForQuestion func(ctx context.Context, userID uuid.UUID, questionID uuid.UUID) (string, error)
}

func (m *mockQuestionSyncHighlightRepository) ListBookStockByUserID(ctx context.Context, userID uuid.UUID) ([]domain.BookStock, error) {
	return []domain.BookStock{}, nil
}

func (m *mockQuestionSyncHighlightRepository) ListUnusedHighlightsByBook(ctx context.Context, userID uuid.UUID, bookKey string, limit int) ([]*domain.Highlight, error) {
	return []*domain.Highlight{}, nil
}

func (m *mockQuestionSyncHighlightRepository) ListUsedHighlightsWithUncoveredPerspectives(ctx context.Context, userID uuid.UUID, bookKey string, limit int) ([]*domain.Highlight, error) {
	return []*domain.Highlight{}, nil
}

func (m *mockQuestionSyncHighlightRepository) RequeueStaleProcessingByUserID(ctx context.Context, userID uuid.UUID, cutoff time.Time) (int, error) {
	return 0, nil
}

func (m *mockQuestionSyncHighlightRepository) ListQuestionGenerationCandidates(ctx context.Context, userID uuid.UUID, changedSince *time.Time) ([]domain.QuestionGenerationBookCandidate, error) {
	m.listCandidatesCalled++
	m.listCandidatesChanged = changedSince
	return append([]domain.QuestionGenerationBookCandidate(nil), m.candidates...), nil
}

func (m *mockQuestionSyncHighlightRepository) ListPendingHighlightsByBook(ctx context.Context, userID uuid.UUID, bookKey string, limit int) ([]*domain.Highlight, error) {
	highlights := m.pendingByBook[bookKey]
	if len(highlights) > limit {
		highlights = highlights[:limit]
	}
	return append([]*domain.Highlight(nil), highlights...), nil
}

func (m *mockQuestionSyncHighlightRepository) MarkHighlightsProcessing(ctx context.Context, userID uuid.UUID, highlightIDs []uuid.UUID) error {
	m.markedProcessing = append(m.markedProcessing, highlightIDs...)
	return nil
}

func (m *mockQuestionSyncHighlightRepository) MarkHighlightPendingForQuestion(ctx context.Context, userID uuid.UUID, questionID uuid.UUID) (string, error) {
	m.markedPendingQuestion = questionID
	if m.markPendingForQuestion != nil {
		return m.markPendingForQuestion(ctx, userID, questionID)
	}
	return m.markedPendingBookKey, nil
}

type mockQuestionSyncQuestionRepository struct {
	dailyGeneratedCount int
	lastSyncAt          *time.Time
	updatedSyncAt       *time.Time
	findQuestion        *domain.Question
	findMeta            *domain.QuestionMeta
	supersededHighlight uuid.UUID
}

func (m *mockQuestionSyncQuestionRepository) GetDailyGeneratedCount(ctx context.Context, userID uuid.UUID, day time.Time) (int, error) {
	return m.dailyGeneratedCount, nil
}

func (m *mockQuestionSyncQuestionRepository) ReserveDailyGeneratedCount(ctx context.Context, userID uuid.UUID, day time.Time, delta int, limit int) (bool, error) {
	return true, nil
}

func (m *mockQuestionSyncQuestionRepository) QueueHighlightsWithinDailyLimit(ctx context.Context, userID uuid.UUID, day time.Time, limit int, highlightIDs []uuid.UUID, questionCountByHighlightID map[uuid.UUID]int, requestedAt time.Time) ([]uuid.UUID, bool, error) {
	return append([]uuid.UUID(nil), highlightIDs...), true, nil
}

func (m *mockQuestionSyncQuestionRepository) GetUserLastQuestionSyncAt(ctx context.Context, userID uuid.UUID) (*time.Time, error) {
	return m.lastSyncAt, nil
}

func (m *mockQuestionSyncQuestionRepository) UpdateUserLastQuestionSyncAt(ctx context.Context, userID uuid.UUID, syncedAt time.Time) error {
	m.updatedSyncAt = &syncedAt
	return nil
}

func (m *mockQuestionSyncQuestionRepository) ListPerspectivesByHighlightID(ctx context.Context, userID string, highlightID uuid.UUID) ([]string, error) {
	return []string{}, nil
}

func (m *mockQuestionSyncQuestionRepository) FindByID(ctx context.Context, id string) (*domain.Question, *domain.QuestionMeta, *domain.QuestionStats, error) {
	return m.findQuestion, m.findMeta, nil, nil
}

func (m *mockQuestionSyncQuestionRepository) SupersedeActiveQuestionsForHighlight(ctx context.Context, userID uuid.UUID, highlightID uuid.UUID) error {
	m.supersededHighlight = highlightID
	return nil
}

type mockQuestionGenerationJobRepository struct {
	createErr           error
	createdInputs       []domain.CreateQuestionGenerationJobInput
	enqueueFailedJobs   []*domain.QuestionGenerationJob
	claimJob            *domain.QuestionGenerationJob
	claimOK             bool
	claimCalls          int
	markedQueued        []uuid.UUID
	markedCompleted     []uuid.UUID
	markedEnqueueFailed []uuid.UUID
	requeuedStale       int
}

func (m *mockQuestionGenerationJobRepository) Create(ctx context.Context, input domain.CreateQuestionGenerationJobInput) (*domain.QuestionGenerationJob, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	m.createdInputs = append(m.createdInputs, input)
	return &domain.QuestionGenerationJob{
		ID:           uuid.New(),
		UserID:       input.UserID,
		BookKey:      input.BookKey,
		Reason:       input.Reason,
		Status:       domain.JobStatusQueued,
		HighlightIDs: input.HighlightIDs,
	}, nil
}

func (m *mockQuestionGenerationJobRepository) Get(ctx context.Context, jobID, userID uuid.UUID) (*domain.QuestionGenerationJob, error) {
	return nil, domain.ErrNotFound
}

func (m *mockQuestionGenerationJobRepository) ListEnqueueFailedByUserID(ctx context.Context, userID uuid.UUID, limit int) ([]*domain.QuestionGenerationJob, error) {
	return append([]*domain.QuestionGenerationJob(nil), m.enqueueFailedJobs...), nil
}

func (m *mockQuestionGenerationJobRepository) ClaimQueued(ctx context.Context, jobID, userID uuid.UUID) (*domain.QuestionGenerationJob, bool, error) {
	m.claimCalls++
	return m.claimJob, m.claimOK, nil
}

func (m *mockQuestionGenerationJobRepository) RequeueStaleProcessing(ctx context.Context, cutoff time.Time) (int, error) {
	return m.requeuedStale, nil
}

func (m *mockQuestionGenerationJobRepository) MarkQueued(ctx context.Context, jobID, userID uuid.UUID) error {
	m.markedQueued = append(m.markedQueued, jobID)
	return nil
}

func (m *mockQuestionGenerationJobRepository) MarkCompleted(ctx context.Context, jobID, userID uuid.UUID) error {
	m.markedCompleted = append(m.markedCompleted, jobID)
	return nil
}

func (m *mockQuestionGenerationJobRepository) MarkEnqueueFailed(ctx context.Context, jobID, userID uuid.UUID, lastError string) error {
	m.markedEnqueueFailed = append(m.markedEnqueueFailed, jobID)
	return nil
}

func (m *mockQuestionGenerationJobRepository) RecordFailure(ctx context.Context, jobID, userID uuid.UUID, lastError string, maxRetry int) (*domain.QuestionGenerationJob, error) {
	return nil, nil
}

type mockQuestionGenerationTaskEnqueuer struct {
	err      error
	enqueued []uuid.UUID
}

func (m *mockQuestionGenerationTaskEnqueuer) EnqueueQuestionGeneration(ctx context.Context, jobID uuid.UUID, userID uuid.UUID) error {
	if m.err != nil {
		return m.err
	}
	m.enqueued = append(m.enqueued, jobID)
	return nil
}

func newQuestionSyncTestUsecase(
	highlightRepo *mockQuestionSyncHighlightRepository,
	questionRepo *mockQuestionSyncQuestionRepository,
	jobRepo *mockQuestionGenerationJobRepository,
	enqueuer *mockQuestionGenerationTaskEnqueuer,
) *QuestionSyncUsecase {
	uc := NewQuestionSyncUsecase(highlightRepo, questionRepo, jobRepo, enqueuer)
	uc.now = func() time.Time { return time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC) }
	return uc
}

func makePendingHighlights(userID uuid.UUID, count int) []*domain.Highlight {
	highlights := make([]*domain.Highlight, 0, count)
	for index := 0; index < count; index++ {
		highlights = append(highlights, &domain.Highlight{
			ID:      uuid.New(),
			UserID:  userID,
			Content: "ハイライト本文",
			Status:  domain.HighlightStatusPending,
		})
	}
	return highlights
}

func TestSyncQuestionStockCreatesJobForConditionA(t *testing.T) {
	userID := uuid.New()
	highlightRepo := &mockQuestionSyncHighlightRepository{
		candidates: []domain.QuestionGenerationBookCandidate{{
			BookKey:               "book-a",
			PendingHighlightCount: 10,
			LatestHighlightAt:     time.Now(),
		}},
		pendingByBook: map[string][]*domain.Highlight{
			"book-a": makePendingHighlights(userID, 10),
		},
	}
	jobRepo := &mockQuestionGenerationJobRepository{}
	enqueuer := &mockQuestionGenerationTaskEnqueuer{}
	uc := newQuestionSyncTestUsecase(highlightRepo, &mockQuestionSyncQuestionRepository{}, jobRepo, enqueuer)

	result, err := uc.SyncQuestionStock(context.Background(), &domain.User{ID: userID})
	if err != nil {
		t.Fatalf("SyncQuestionStock failed: %v", err)
	}

	if result.QueuedCount != 1 {
		t.Fatalf("expected 1 queued job, got %d", result.QueuedCount)
	}
	if len(jobRepo.createdInputs) != 1 || jobRepo.createdInputs[0].Reason != domain.JobReasonHighlightBatchThreshold {
		t.Fatalf("unexpected created job: %#v", jobRepo.createdInputs)
	}
	if len(highlightRepo.markedProcessing) != domain.MaxHighlightsPerJob {
		t.Fatalf("expected 10 processing highlights, got %d", len(highlightRepo.markedProcessing))
	}
	if len(enqueuer.enqueued) != 1 {
		t.Fatalf("expected task enqueue once, got %d", len(enqueuer.enqueued))
	}
}

func TestSyncQuestionStockDoesNotCreateJobForNinePendingWithUnansweredQuestion(t *testing.T) {
	userID := uuid.New()
	highlightRepo := &mockQuestionSyncHighlightRepository{
		candidates: []domain.QuestionGenerationBookCandidate{{
			BookKey:                 "book-a",
			PendingHighlightCount:   9,
			UnansweredQuestionCount: 1,
			LatestHighlightAt:       time.Now(),
		}},
		pendingByBook: map[string][]*domain.Highlight{
			"book-a": makePendingHighlights(userID, 9),
		},
	}
	jobRepo := &mockQuestionGenerationJobRepository{}
	uc := newQuestionSyncTestUsecase(highlightRepo, &mockQuestionSyncQuestionRepository{}, jobRepo, &mockQuestionGenerationTaskEnqueuer{})

	result, err := uc.SyncQuestionStock(context.Background(), &domain.User{ID: userID})
	if err != nil {
		t.Fatalf("SyncQuestionStock failed: %v", err)
	}

	if result.QueuedCount != 0 || len(jobRepo.createdInputs) != 0 {
		t.Fatalf("expected no job, result=%#v created=%#v", result, jobRepo.createdInputs)
	}
}

func TestSyncQuestionStockCreatesJobForConditionB(t *testing.T) {
	userID := uuid.New()
	highlightRepo := &mockQuestionSyncHighlightRepository{
		candidates: []domain.QuestionGenerationBookCandidate{{
			BookKey:                 "book-a",
			PendingHighlightCount:   5,
			UnansweredQuestionCount: 0,
			LatestHighlightAt:       time.Now(),
		}},
		pendingByBook: map[string][]*domain.Highlight{
			"book-a": makePendingHighlights(userID, 5),
		},
	}
	jobRepo := &mockQuestionGenerationJobRepository{}
	uc := newQuestionSyncTestUsecase(highlightRepo, &mockQuestionSyncQuestionRepository{}, jobRepo, &mockQuestionGenerationTaskEnqueuer{})

	result, err := uc.SyncQuestionStock(context.Background(), &domain.User{ID: userID})
	if err != nil {
		t.Fatalf("SyncQuestionStock failed: %v", err)
	}

	if result.QueuedCount != 1 {
		t.Fatalf("expected 1 queued job, got %d", result.QueuedCount)
	}
	if len(jobRepo.createdInputs) != 1 || jobRepo.createdInputs[0].Reason != domain.JobReasonAllUnansweredConsumed {
		t.Fatalf("unexpected created job: %#v", jobRepo.createdInputs)
	}
}

func TestSyncQuestionStockDoesNotCreateJobBelowMinimum(t *testing.T) {
	userID := uuid.New()
	highlightRepo := &mockQuestionSyncHighlightRepository{
		candidates: []domain.QuestionGenerationBookCandidate{{
			BookKey:               "book-a",
			PendingHighlightCount: 4,
			LatestHighlightAt:     time.Now(),
		}},
		pendingByBook: map[string][]*domain.Highlight{
			"book-a": makePendingHighlights(userID, 4),
		},
	}
	jobRepo := &mockQuestionGenerationJobRepository{}
	uc := newQuestionSyncTestUsecase(highlightRepo, &mockQuestionSyncQuestionRepository{}, jobRepo, &mockQuestionGenerationTaskEnqueuer{})

	result, err := uc.SyncQuestionStock(context.Background(), &domain.User{ID: userID})
	if err != nil {
		t.Fatalf("SyncQuestionStock failed: %v", err)
	}

	if result.QueuedCount != 0 || len(jobRepo.createdInputs) != 0 {
		t.Fatalf("expected no job, result=%#v created=%#v", result, jobRepo.createdInputs)
	}
}

func TestSyncQuestionStockDoesNotDuplicateActiveJob(t *testing.T) {
	userID := uuid.New()
	highlightRepo := &mockQuestionSyncHighlightRepository{
		candidates: []domain.QuestionGenerationBookCandidate{{
			BookKey:               "book-a",
			PendingHighlightCount: 10,
			LatestHighlightAt:     time.Now(),
		}},
		pendingByBook: map[string][]*domain.Highlight{
			"book-a": makePendingHighlights(userID, 10),
		},
	}
	jobRepo := &mockQuestionGenerationJobRepository{createErr: domain.ErrAlreadyExists}
	uc := newQuestionSyncTestUsecase(highlightRepo, &mockQuestionSyncQuestionRepository{}, jobRepo, &mockQuestionGenerationTaskEnqueuer{})

	result, err := uc.SyncQuestionStock(context.Background(), &domain.User{ID: userID})
	if err != nil {
		t.Fatalf("SyncQuestionStock failed: %v", err)
	}

	if result.QueuedCount != 0 || len(highlightRepo.markedProcessing) != 0 {
		t.Fatalf("expected duplicate active job to no-op, result=%#v marked=%#v", result, highlightRepo.markedProcessing)
	}
}

func TestSyncQuestionStockMarksEnqueueFailedWhenTaskEnqueueFails(t *testing.T) {
	userID := uuid.New()
	highlightRepo := &mockQuestionSyncHighlightRepository{
		candidates: []domain.QuestionGenerationBookCandidate{{
			BookKey:               "book-a",
			PendingHighlightCount: 10,
			LatestHighlightAt:     time.Now(),
		}},
		pendingByBook: map[string][]*domain.Highlight{
			"book-a": makePendingHighlights(userID, 10),
		},
	}
	jobRepo := &mockQuestionGenerationJobRepository{}
	uc := newQuestionSyncTestUsecase(highlightRepo, &mockQuestionSyncQuestionRepository{}, jobRepo, &mockQuestionGenerationTaskEnqueuer{err: errors.New("cloud tasks unavailable")})

	result, err := uc.SyncQuestionStock(context.Background(), &domain.User{ID: userID})
	if err != nil {
		t.Fatalf("SyncQuestionStock failed: %v", err)
	}

	if result.QueuedCount != 1 {
		t.Fatalf("expected job to be counted after enqueue failure, got %d", result.QueuedCount)
	}
	if len(jobRepo.markedEnqueueFailed) != 1 {
		t.Fatalf("expected enqueue_failed mark, got %#v", jobRepo.markedEnqueueFailed)
	}
}

func TestSyncQuestionStockReenqueuesFailedJobsBeforeScanning(t *testing.T) {
	userID := uuid.New()
	jobID := uuid.New()
	jobRepo := &mockQuestionGenerationJobRepository{
		enqueueFailedJobs: []*domain.QuestionGenerationJob{{
			ID:     jobID,
			UserID: userID,
			Status: domain.JobStatusEnqueueFailed,
		}},
	}
	uc := newQuestionSyncTestUsecase(&mockQuestionSyncHighlightRepository{}, &mockQuestionSyncQuestionRepository{}, jobRepo, &mockQuestionGenerationTaskEnqueuer{})

	result, err := uc.SyncQuestionStock(context.Background(), &domain.User{ID: userID})
	if err != nil {
		t.Fatalf("SyncQuestionStock failed: %v", err)
	}

	if result.QueuedCount != 1 {
		t.Fatalf("expected recovered enqueue_failed job to count as queued, got %d", result.QueuedCount)
	}
	if len(jobRepo.markedQueued) != 1 || jobRepo.markedQueued[0] != jobID {
		t.Fatalf("expected failed job marked queued, got %#v", jobRepo.markedQueued)
	}
}

func TestSyncQuestionStockSkipsWhenDailyLimitReached(t *testing.T) {
	userID := uuid.New()
	questionRepo := &mockQuestionSyncQuestionRepository{dailyGeneratedCount: defaultQuestionSyncDailyLimit}
	highlightRepo := &mockQuestionSyncHighlightRepository{
		candidates: []domain.QuestionGenerationBookCandidate{{
			BookKey:               "book-a",
			PendingHighlightCount: 10,
			LatestHighlightAt:     time.Now(),
		}},
		pendingByBook: map[string][]*domain.Highlight{
			"book-a": makePendingHighlights(userID, 10),
		},
	}
	jobRepo := &mockQuestionGenerationJobRepository{}
	uc := newQuestionSyncTestUsecase(highlightRepo, questionRepo, jobRepo, &mockQuestionGenerationTaskEnqueuer{})

	result, err := uc.SyncQuestionStock(context.Background(), &domain.User{ID: userID})
	if err != nil {
		t.Fatalf("SyncQuestionStock failed: %v", err)
	}

	if !result.SkippedDueToDailyLimit {
		t.Fatal("expected daily limit skip")
	}
	if highlightRepo.listCandidatesCalled != 0 || len(jobRepo.createdInputs) != 0 {
		t.Fatalf("expected no scan or job creation, scans=%d created=%#v", highlightRepo.listCandidatesCalled, jobRepo.createdInputs)
	}
}

func TestEvaluateBookAfterAnswerReturnsHighlightToPendingAndCreatesJob(t *testing.T) {
	userID := uuid.New()
	questionID := uuid.New()
	highlightID := uuid.New()
	highlightRepo := &mockQuestionSyncHighlightRepository{
		markedPendingBookKey: "book-a",
		candidates: []domain.QuestionGenerationBookCandidate{{
			BookKey:                 "book-a",
			PendingHighlightCount:   5,
			UnansweredQuestionCount: 0,
			LatestHighlightAt:       time.Now(),
		}},
		pendingByBook: map[string][]*domain.Highlight{
			"book-a": makePendingHighlights(userID, 5),
		},
	}
	questionRepo := &mockQuestionSyncQuestionRepository{
		findQuestion: &domain.Question{ID: questionID.String()},
		findMeta:     &domain.QuestionMeta{QuestionID: questionID.String(), HighlightID: highlightID.String()},
	}
	jobRepo := &mockQuestionGenerationJobRepository{}
	uc := newQuestionSyncTestUsecase(highlightRepo, questionRepo, jobRepo, &mockQuestionGenerationTaskEnqueuer{})

	if err := uc.EvaluateBookAfterAnswer(context.Background(), &domain.User{ID: userID}, questionID.String()); err != nil {
		t.Fatalf("EvaluateBookAfterAnswer failed: %v", err)
	}

	if questionRepo.supersededHighlight != highlightID {
		t.Fatalf("expected superseded highlight %s, got %s", highlightID, questionRepo.supersededHighlight)
	}
	if highlightRepo.markedPendingQuestion != questionID {
		t.Fatalf("expected marked pending question %s, got %s", questionID, highlightRepo.markedPendingQuestion)
	}
	if len(jobRepo.createdInputs) != 1 {
		t.Fatalf("expected one follow-up job, got %#v", jobRepo.createdInputs)
	}
}
