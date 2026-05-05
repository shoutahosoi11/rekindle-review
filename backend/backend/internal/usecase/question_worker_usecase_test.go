package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shout/ai-study-tool/backend/internal/domain"
)

type mockQuestionWorkerRepository struct {
	save             func(ctx context.Context, question *domain.Question, meta *domain.QuestionMeta) error
	reserveDaily     func(ctx context.Context, userID uuid.UUID, day time.Time, delta int, limit int) (bool, error)
	savedHighlight   []uuid.UUID
	superseded       []uuid.UUID
	reservedDelta    int
	saveGenerationID string
}

func (m *mockQuestionWorkerRepository) Save(ctx context.Context, question *domain.Question, meta *domain.QuestionMeta) error {
	if m.save == nil {
		if meta != nil {
			highlightID, _ := uuid.Parse(meta.HighlightID)
			m.savedHighlight = append(m.savedHighlight, highlightID)
		}
		return nil
	}
	return m.save(ctx, question, meta)
}

func (m *mockQuestionWorkerRepository) SupersedeActiveQuestionsForHighlight(ctx context.Context, userID uuid.UUID, highlightID uuid.UUID) error {
	m.superseded = append(m.superseded, highlightID)
	return nil
}

func (m *mockQuestionWorkerRepository) SaveGeneration(ctx context.Context, userID, sourceType, sourceID, promptUsed, modelUsed string) (string, error) {
	if m.saveGenerationID != "" {
		return m.saveGenerationID, nil
	}
	return "generation-id", nil
}

func (m *mockQuestionWorkerRepository) ListPerspectivesByHighlightID(ctx context.Context, userID string, highlightID uuid.UUID) ([]string, error) {
	return make([]string, 0), nil
}

func (m *mockQuestionWorkerRepository) GetDailyGeneratedCount(ctx context.Context, userID uuid.UUID, day time.Time) (int, error) {
	return 0, nil
}

func (m *mockQuestionWorkerRepository) ReserveDailyGeneratedCount(ctx context.Context, userID uuid.UUID, day time.Time, delta int, limit int) (bool, error) {
	m.reservedDelta = delta
	if m.reserveDaily != nil {
		return m.reserveDaily(ctx, userID, day, delta, limit)
	}
	return true, nil
}

func (m *mockQuestionWorkerRepository) EnqueueRegeneration(ctx context.Context, userID string, highlightID uuid.UUID, questionID string) error {
	return nil
}

func (m *mockQuestionWorkerRepository) ClaimPendingRegenerationTasks(ctx context.Context, limit int) ([]*domain.RegenerationTask, error) {
	return make([]*domain.RegenerationTask, 0), nil
}

func (m *mockQuestionWorkerRepository) DeferRegenerationTasks(ctx context.Context, taskIDs []uuid.UUID, lastError string) error {
	return nil
}

func (m *mockQuestionWorkerRepository) MarkRegenerationTasksCompleted(ctx context.Context, taskIDs []uuid.UUID) error {
	return nil
}

func (m *mockQuestionWorkerRepository) MarkRegenerationTasksFailed(ctx context.Context, taskIDs []uuid.UUID, lastError string, maxRetry int) error {
	return nil
}

func TestDynamicMaxWaitTime(t *testing.T) {
	cases := []struct {
		total int
		want  time.Duration
	}{
		{total: 1, want: 5 * time.Second},
		{total: 5, want: 15 * time.Second},
		{total: 6, want: 15 * time.Second},
		{total: 20, want: 30 * time.Second},
		{total: 21, want: 30 * time.Second},
		{total: 41, want: 60 * time.Second},
		{total: 100, want: 60 * time.Second},
	}

	for _, tc := range cases {
		got := dynamicMaxWaitTime(tc.total)
		if got != tc.want {
			t.Fatalf("dynamicMaxWaitTime(%d) = %s, want %s", tc.total, got, tc.want)
		}
	}
}

func TestNextPerspectiveVersionsPrefersUnused(t *testing.T) {
	perspectives, versions := nextPerspectiveVersions([]string{
		domain.QuestionPerspectiveDefinition,
		domain.QuestionPerspectiveComparison,
	}, 3)

	wantPerspectives := []string{
		domain.QuestionPerspectivePractical,
		domain.QuestionPerspectiveUnderstanding,
		domain.QuestionPerspectivePitfall,
	}
	wantVersions := []int{1, 1, 1}

	for index, want := range wantPerspectives {
		if perspectives[index] != want {
			t.Fatalf("perspectives[%d] = %s, want %s", index, perspectives[index], want)
		}
	}
	for index, want := range wantVersions {
		if versions[index] != want {
			t.Fatalf("versions[%d] = %d, want %d", index, versions[index], want)
		}
	}
}

func TestShouldProcessPendingBatch(t *testing.T) {
	now := time.Date(2026, 4, 25, 6, 0, 0, 0, time.UTC)
	stat := domain.PendingHighlightUserStat{
		UserID:          uuid.New(),
		PendingCount:    3,
		TotalCount:      3,
		OldestPendingAt: now.Add(-11 * time.Second),
	}

	if !shouldProcessPendingBatch(stat, now, 5*time.Minute, defaultWorkerBatchSize) {
		t.Fatal("expected batch to be due once dynamic wait time is exceeded")
	}

	stat.PendingCount = 10
	stat.OldestPendingAt = now
	if !shouldProcessPendingBatch(stat, now, 5*time.Minute, defaultWorkerBatchSize) {
		t.Fatal("expected batch to be due immediately when pending count reaches 10")
	}

	stat.PendingCount = 5
	if !shouldProcessPendingBatch(stat, now, 5*time.Minute, 5) {
		t.Fatal("expected batch to respect configured worker batch size")
	}
}

func TestSaveGeneratedQuestionsForChunkKeepsPartialSaveFailureFailed(t *testing.T) {
	highlightID := uuid.New()
	saveCalls := 0
	uc := &QuestionWorkerUsecase{
		questionRepo: &mockQuestionWorkerRepository{
			save: func(ctx context.Context, question *domain.Question, meta *domain.QuestionMeta) error {
				saveCalls++
				if saveCalls == 2 {
					return errors.New("save failed")
				}
				return nil
			},
		},
	}

	completed := make(map[uuid.UUID]struct{})
	failed := make(map[uuid.UUID]string)
	uc.saveGeneratedQuestionsForChunk(
		context.Background(),
		uuid.NewString(),
		"generation-id",
		[]highlightGenerationPlan{{
			highlight: &domain.Highlight{
				ID: highlightID,
			},
			perspectives: []string{
				domain.QuestionPerspectiveDefinition,
				domain.QuestionPerspectiveUnderstanding,
			},
			versions: []int{1, 1},
		}},
		[]domain.GeneratedQuestion{
			{
				Content:       "question 1",
				Options:       []string{"a", "b", "c", "d"},
				CorrectAnswer: "a",
				Explanation:   "explanation 1",
			},
			{
				Content:       "question 2",
				Options:       []string{"a", "b", "c", "d"},
				CorrectAnswer: "b",
				Explanation:   "explanation 2",
			},
		},
		completed,
		failed,
	)

	if _, ok := completed[highlightID]; ok {
		t.Fatal("expected partially saved highlight not to be marked completed")
	}
	if failed[highlightID] == "" {
		t.Fatal("expected partially saved highlight to remain failed for retry")
	}
}

func TestSplitHighlightPlansByLimitRespectsQuestionBudget(t *testing.T) {
	plans := []highlightGenerationPlan{
		{highlight: &domain.Highlight{Content: strings.Repeat("a", 400)}, questionCount: 3},
		{highlight: &domain.Highlight{Content: strings.Repeat("b", 400)}, questionCount: 3},
		{highlight: &domain.Highlight{Content: strings.Repeat("c", 400)}, questionCount: 3},
	}

	chunks := splitHighlightPlansByLimit(plans, 100000, 8)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}

	firstChunkQuestions := 0
	for _, plan := range chunks[0] {
		firstChunkQuestions += plan.questionCount
	}
	if firstChunkQuestions > 8 {
		t.Fatalf("expected first chunk to have at most 8 questions, got %d", firstChunkQuestions)
	}
}

type mockWorkerHighlightLifecycle struct {
	highlights       []*domain.Highlight
	listByIDsCalled  int
	completed        []uuid.UUID
	failed           []uuid.UUID
	claimedByUserIDs []uuid.UUID
}

func (m *mockWorkerHighlightLifecycle) ListPendingUserStats(ctx context.Context) ([]domain.PendingHighlightUserStat, error) {
	return []domain.PendingHighlightUserStat{}, nil
}

func (m *mockWorkerHighlightLifecycle) ClaimPendingByUserID(ctx context.Context, userID uuid.UUID, limit int) ([]*domain.Highlight, error) {
	m.claimedByUserIDs = append(m.claimedByUserIDs, userID)
	return []*domain.Highlight{}, nil
}

func (m *mockWorkerHighlightLifecycle) ClaimPendingByIDs(ctx context.Context, userID uuid.UUID, highlightIDs []uuid.UUID) ([]*domain.Highlight, error) {
	return []*domain.Highlight{}, nil
}

func (m *mockWorkerHighlightLifecycle) ListByIDs(ctx context.Context, userID uuid.UUID, highlightIDs []uuid.UUID) ([]*domain.Highlight, error) {
	m.listByIDsCalled++
	return append([]*domain.Highlight(nil), m.highlights...), nil
}

func (m *mockWorkerHighlightLifecycle) RequeueStaleProcessing(ctx context.Context, cutoff time.Time) (int, error) {
	return 0, nil
}

func (m *mockWorkerHighlightLifecycle) MarkGenerationCompleted(ctx context.Context, highlightIDs []uuid.UUID) error {
	m.completed = append(m.completed, highlightIDs...)
	return nil
}

func (m *mockWorkerHighlightLifecycle) MarkGenerationFailed(ctx context.Context, highlightIDs []uuid.UUID, lastError string, maxRetry int) error {
	m.failed = append(m.failed, highlightIDs...)
	return nil
}

type mockQuestionWorkerLLMClient struct {
	generateCalled int
}

func (m *mockQuestionWorkerLLMClient) ModelForPlan(plan string) string {
	return "gemini-test"
}

func (m *mockQuestionWorkerLLMClient) GenerateQuestions(ctx context.Context, points []domain.ExtractedPoint, questionType domain.QuestionType, customInstruction string, model string) ([]domain.GeneratedQuestion, error) {
	m.generateCalled++
	questions := make([]domain.GeneratedQuestion, 0, len(points))
	for range points {
		questions = append(questions, domain.GeneratedQuestion{
			Content:       "問題",
			Options:       []string{"A", "B", "C", "D"},
			CorrectAnswer: "A",
			Explanation:   "解説",
		})
	}
	return questions, nil
}

func TestProcessQuestionGenerationJobNoOpsWhenJobAlreadyClaimed(t *testing.T) {
	jobID := uuid.New()
	userID := uuid.New()
	jobRepo := &mockQuestionGenerationJobRepository{claimOK: false}
	highlightRepo := &mockWorkerHighlightLifecycle{}
	llm := &mockQuestionWorkerLLMClient{}
	uc := NewQuestionWorkerUsecaseWithJobRepository(highlightRepo, &mockQuestionWorkerRepository{}, jobRepo, llm)

	if err := uc.ProcessQuestionGenerationJob(context.Background(), jobID, userID); err != nil {
		t.Fatalf("ProcessQuestionGenerationJob failed: %v", err)
	}

	if jobRepo.claimCalls != 1 {
		t.Fatalf("expected one claim attempt, got %d", jobRepo.claimCalls)
	}
	if highlightRepo.listByIDsCalled != 0 || llm.generateCalled != 0 {
		t.Fatalf("expected no work after unclaimed job, list=%d generate=%d", highlightRepo.listByIDsCalled, llm.generateCalled)
	}
}

func TestProcessQuestionGenerationJobReturnsQueuedWhenDailyLimitExceeded(t *testing.T) {
	jobID := uuid.New()
	userID := uuid.New()
	highlightID := uuid.New()
	jobRepo := &mockQuestionGenerationJobRepository{
		claimOK:  true,
		claimJob: &domain.QuestionGenerationJob{ID: jobID, UserID: userID, HighlightIDs: []uuid.UUID{highlightID}},
	}
	questionRepo := &mockQuestionWorkerRepository{
		reserveDaily: func(ctx context.Context, userID uuid.UUID, day time.Time, delta int, limit int) (bool, error) {
			return false, nil
		},
	}
	highlightRepo := &mockWorkerHighlightLifecycle{}
	uc := NewQuestionWorkerUsecaseWithJobRepository(highlightRepo, questionRepo, jobRepo, &mockQuestionWorkerLLMClient{})

	if err := uc.ProcessQuestionGenerationJob(context.Background(), jobID, userID); err != nil {
		t.Fatalf("ProcessQuestionGenerationJob failed: %v", err)
	}

	if len(jobRepo.markedQueued) != 1 || jobRepo.markedQueued[0] != jobID {
		t.Fatalf("expected quota-limited job returned to queued, got %#v", jobRepo.markedQueued)
	}
	if highlightRepo.listByIDsCalled != 0 {
		t.Fatalf("expected no highlight load after quota limit, got %d", highlightRepo.listByIDsCalled)
	}
}

func TestProcessQuestionGenerationJobCompletesWhenHighlightsWereDeleted(t *testing.T) {
	jobID := uuid.New()
	userID := uuid.New()
	jobRepo := &mockQuestionGenerationJobRepository{
		claimOK:  true,
		claimJob: &domain.QuestionGenerationJob{ID: jobID, UserID: userID},
	}
	uc := NewQuestionWorkerUsecaseWithJobRepository(&mockWorkerHighlightLifecycle{}, &mockQuestionWorkerRepository{}, jobRepo, &mockQuestionWorkerLLMClient{})

	if err := uc.ProcessQuestionGenerationJob(context.Background(), jobID, userID); err != nil {
		t.Fatalf("ProcessQuestionGenerationJob failed: %v", err)
	}

	if len(jobRepo.markedCompleted) != 1 || jobRepo.markedCompleted[0] != jobID {
		t.Fatalf("expected empty job completed, got %#v", jobRepo.markedCompleted)
	}
}

func TestProcessQuestionGenerationJobCreatesOneActiveQuestionPerHighlight(t *testing.T) {
	jobID := uuid.New()
	userID := uuid.New()
	highlightID := uuid.New()
	jobRepo := &mockQuestionGenerationJobRepository{
		claimOK:  true,
		claimJob: &domain.QuestionGenerationJob{ID: jobID, UserID: userID, HighlightIDs: []uuid.UUID{highlightID}},
	}
	highlightRepo := &mockWorkerHighlightLifecycle{
		highlights: []*domain.Highlight{{
			ID:      highlightID,
			UserID:  userID,
			Content: "ハイライト本文",
		}},
	}
	questionRepo := &mockQuestionWorkerRepository{}
	llm := &mockQuestionWorkerLLMClient{}
	uc := NewQuestionWorkerUsecaseWithJobRepository(highlightRepo, questionRepo, jobRepo, llm)

	if err := uc.ProcessQuestionGenerationJob(context.Background(), jobID, userID); err != nil {
		t.Fatalf("ProcessQuestionGenerationJob failed: %v", err)
	}

	if questionRepo.reservedDelta != 1 {
		t.Fatalf("expected one daily quota reservation, got %d", questionRepo.reservedDelta)
	}
	if len(questionRepo.superseded) != 1 || questionRepo.superseded[0] != highlightID {
		t.Fatalf("expected active question superseded, got %#v", questionRepo.superseded)
	}
	if len(questionRepo.savedHighlight) != 1 || questionRepo.savedHighlight[0] != highlightID {
		t.Fatalf("expected generated question saved for highlight, got %#v", questionRepo.savedHighlight)
	}
	if len(highlightRepo.completed) != 1 || highlightRepo.completed[0] != highlightID {
		t.Fatalf("expected highlight completed, got %#v", highlightRepo.completed)
	}
	if len(jobRepo.markedCompleted) != 1 || jobRepo.markedCompleted[0] != jobID {
		t.Fatalf("expected job completed, got %#v", jobRepo.markedCompleted)
	}
	if llm.generateCalled != 1 {
		t.Fatalf("expected one Gemini call, got %d", llm.generateCalled)
	}
}
