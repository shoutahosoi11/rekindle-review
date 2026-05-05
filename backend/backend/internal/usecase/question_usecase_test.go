package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/shout/ai-study-tool/backend/internal/domain"
	"github.com/shout/ai-study-tool/backend/internal/usecase"
)

type mockLLMClient struct {
	generateQuestions func(ctx context.Context, points []domain.ExtractedPoint, questionType domain.QuestionType, customInstruction string, model string) ([]domain.GeneratedQuestion, error)
}

func (m *mockLLMClient) ModelForPlan(plan string) string {
	if plan == "pro" {
		return "gemini-2.5-pro"
	}
	return "gemini-2.5-flash"
}

func (m *mockLLMClient) GenerateQuestions(ctx context.Context, points []domain.ExtractedPoint, questionType domain.QuestionType, customInstruction string, model string) ([]domain.GeneratedQuestion, error) {
	if m.generateQuestions == nil {
		return nil, nil
	}
	return m.generateQuestions(ctx, points, questionType, customInstruction, model)
}

type mockQuestionRepository struct {
	save                         func(ctx context.Context, q *domain.Question, meta *domain.QuestionMeta) error
	listByCreatorID              func(ctx context.Context, creatorID string, limit int) ([]*domain.Question, error)
	listSavedByUserID            func(ctx context.Context, userID string, limit int) ([]*domain.SavedQuestion, error)
	listIncorrectByUserID        func(ctx context.Context, userID string, limit int) ([]*domain.IncorrectQuestion, error)
	listPreparedByHighlightIDs   func(ctx context.Context, userID string, highlightIDs []uuid.UUID, limit int) ([]*domain.Question, error)
	listPerspectivesByHighlight  func(ctx context.Context, userID string, highlightID uuid.UUID) ([]string, error)
	listUsedHighlightIDsByUserID func(ctx context.Context, userID string, highlightIDs []uuid.UUID) ([]uuid.UUID, error)
	findByID                     func(ctx context.Context, id string) (*domain.Question, *domain.QuestionMeta, *domain.QuestionStats, error)
	updateStats                  func(ctx context.Context, questionID string, isCorrect bool) error
	saveGeneration               func(ctx context.Context, userID, sourceType, sourceID, promptUsed, modelUsed string) (string, error)
	saveForUser                  func(ctx context.Context, userID, questionID, note string) error
}

func (m *mockQuestionRepository) Save(ctx context.Context, q *domain.Question, meta *domain.QuestionMeta) error {
	return m.save(ctx, q, meta)
}

func (m *mockQuestionRepository) SupersedeActiveQuestionsForHighlight(ctx context.Context, userID uuid.UUID, highlightID uuid.UUID) error {
	return nil
}

func (m *mockQuestionRepository) ListByCreatorID(ctx context.Context, creatorID string, limit int) ([]*domain.Question, error) {
	if m.listByCreatorID == nil {
		return make([]*domain.Question, 0), nil
	}
	return m.listByCreatorID(ctx, creatorID, limit)
}

func (m *mockQuestionRepository) ListSavedByUserID(ctx context.Context, userID string, limit int) ([]*domain.SavedQuestion, error) {
	if m.listSavedByUserID == nil {
		return make([]*domain.SavedQuestion, 0), nil
	}
	return m.listSavedByUserID(ctx, userID, limit)
}

func (m *mockQuestionRepository) ListIncorrectByUserID(ctx context.Context, userID string, limit int) ([]*domain.IncorrectQuestion, error) {
	if m.listIncorrectByUserID == nil {
		return make([]*domain.IncorrectQuestion, 0), nil
	}
	return m.listIncorrectByUserID(ctx, userID, limit)
}

func (m *mockQuestionRepository) ListPreparedByUserIDAndHighlightIDs(ctx context.Context, userID string, highlightIDs []uuid.UUID, limit int) ([]*domain.Question, error) {
	if m.listPreparedByHighlightIDs == nil {
		return make([]*domain.Question, 0), nil
	}
	return m.listPreparedByHighlightIDs(ctx, userID, highlightIDs, limit)
}

func (m *mockQuestionRepository) ListPerspectivesByHighlightID(ctx context.Context, userID string, highlightID uuid.UUID) ([]string, error) {
	if m.listPerspectivesByHighlight == nil {
		return make([]string, 0), nil
	}
	return m.listPerspectivesByHighlight(ctx, userID, highlightID)
}

func (m *mockQuestionRepository) ListUsedHighlightIDsByUserID(ctx context.Context, userID string, highlightIDs []uuid.UUID) ([]uuid.UUID, error) {
	if m.listUsedHighlightIDsByUserID == nil {
		return make([]uuid.UUID, 0), nil
	}
	return m.listUsedHighlightIDsByUserID(ctx, userID, highlightIDs)
}

func (m *mockQuestionRepository) FindByID(ctx context.Context, id string) (*domain.Question, *domain.QuestionMeta, *domain.QuestionStats, error) {
	return m.findByID(ctx, id)
}

func (m *mockQuestionRepository) GetByID(ctx context.Context, id string) (*domain.Question, error) {
	q, _, _, err := m.findByID(ctx, id)
	return q, err
}

func (m *mockQuestionRepository) UpdateStats(ctx context.Context, questionID string, isCorrect bool) error {
	return m.updateStats(ctx, questionID, isCorrect)
}

func (m *mockQuestionRepository) SaveGeneration(ctx context.Context, userID, sourceType, sourceID, promptUsed, modelUsed string) (string, error) {
	return m.saveGeneration(ctx, userID, sourceType, sourceID, promptUsed, modelUsed)
}

func (m *mockQuestionRepository) SaveForUser(ctx context.Context, userID, questionID, note string) error {
	if m.saveForUser == nil {
		return nil
	}
	return m.saveForUser(ctx, userID, questionID, note)
}

type mockQuestionSourceResolver struct {
	resolveHighlights func(ctx context.Context, userID string, sourceType domain.SourceType, sourceID string, bookTitle string, bookAuthor string) ([]*domain.Highlight, error)
}

func (m *mockQuestionSourceResolver) ResolveHighlights(ctx context.Context, userID string, sourceType domain.SourceType, sourceID string, bookTitle string, bookAuthor string) ([]*domain.Highlight, error) {
	if m.resolveHighlights == nil {
		return make([]*domain.Highlight, 0), nil
	}
	return m.resolveHighlights(ctx, userID, sourceType, sourceID, bookTitle, bookAuthor)
}

func TestGenerateQuestions_GeneratesFromResolvedKindleHighlights(t *testing.T) {
	ctx := context.Background()
	highlightIDOne := uuid.New()
	highlightIDTwo := uuid.New()
	explanation := "解説1"

	llm := &mockLLMClient{
		generateQuestions: func(ctx context.Context, points []domain.ExtractedPoint, questionType domain.QuestionType, customInstruction string, model string) ([]domain.GeneratedQuestion, error) {
			if len(points) != 2 {
				t.Fatalf("expected 2 generation materials, got %d", len(points))
			}
			if points[0].Point != "ハイライト1" || points[0].Context != explanation {
				t.Fatalf("unexpected first material: %+v", points[0])
			}
			if points[1].Point != "ハイライト2" || points[1].Context != "" {
				t.Fatalf("unexpected second material: %+v", points[1])
			}
			return []domain.GeneratedQuestion{
				{
					Content:       "問題1",
					Options:       []string{"A", "B", "C", "D"},
					CorrectAnswer: "A",
					Explanation:   "解説1",
				},
				{
					Content:       "問題2",
					Options:       []string{"A", "B", "C", "D"},
					CorrectAnswer: "B",
					Explanation:   "解説2",
				},
			}, nil
		},
	}

	savedHighlightIDs := make([]string, 0, 2)
	repo := &mockQuestionRepository{
		saveGeneration: func(ctx context.Context, userID, sourceType, sourceID, promptUsed, modelUsed string) (string, error) {
			return "gen-id-123", nil
		},
		save: func(ctx context.Context, q *domain.Question, meta *domain.QuestionMeta) error {
			savedHighlightIDs = append(savedHighlightIDs, meta.HighlightID)
			return nil
		},
	}

	resolver := &mockQuestionSourceResolver{
		resolveHighlights: func(ctx context.Context, userID string, sourceType domain.SourceType, sourceID string, bookTitle string, bookAuthor string) ([]*domain.Highlight, error) {
			if userID != "user-123" {
				t.Fatalf("unexpected user id: %s", userID)
			}
			if sourceType != domain.SourceTypeKindleBook {
				t.Fatalf("unexpected source type: %s", sourceType)
			}
			if sourceID != "B00BOOK" {
				t.Fatalf("unexpected source id: %s", sourceID)
			}
			return []*domain.Highlight{
				{ID: highlightIDOne, Content: "ハイライト1", Explanation: &explanation},
				{ID: highlightIDTwo, Content: "ハイライト2"},
			}, nil
		},
	}

	uc := usecase.NewQuestionUsecase(repo, llm, resolver)

	questions, err := uc.GenerateQuestions(ctx, domain.GenerateQuestionsInput{
		CreatorID:    "user-123",
		SourceType:   domain.SourceTypeKindleBook,
		SourceID:     "B00BOOK",
		QuestionType: domain.QuestionTypeMultipleChoice,
		UserPlan:     "free",
	})
	if err != nil {
		t.Fatalf("GenerateQuestions failed: %v", err)
	}

	if len(questions) != 2 {
		t.Fatalf("expected 2 questions, got %d", len(questions))
	}
	if len(savedHighlightIDs) != 2 || savedHighlightIDs[0] != highlightIDOne.String() || savedHighlightIDs[1] != highlightIDTwo.String() {
		t.Fatalf("unexpected saved highlight ids: %#v", savedHighlightIDs)
	}
}

func TestGenerateQuestions_UsesFlashForFreePlan(t *testing.T) {
	ctx := context.Background()
	var usedModel string

	llm := &mockLLMClient{
		generateQuestions: func(ctx context.Context, points []domain.ExtractedPoint, questionType domain.QuestionType, customInstruction string, model string) ([]domain.GeneratedQuestion, error) {
			usedModel = model
			return []domain.GeneratedQuestion{{
				Content:       "問題",
				Options:       []string{"A", "B", "C", "D"},
				CorrectAnswer: "A",
				Explanation:   "解説",
			}}, nil
		},
	}

	repo := &mockQuestionRepository{
		saveGeneration: func(_ context.Context, _, _, _, _, _ string) (string, error) { return "gen-id", nil },
		save:           func(_ context.Context, _ *domain.Question, _ *domain.QuestionMeta) error { return nil },
	}

	resolver := &mockQuestionSourceResolver{
		resolveHighlights: func(ctx context.Context, userID string, sourceType domain.SourceType, sourceID string, bookTitle string, bookAuthor string) ([]*domain.Highlight, error) {
			return []*domain.Highlight{{ID: uuid.New(), Content: "ハイライト"}}, nil
		},
	}

	uc := usecase.NewQuestionUsecase(repo, llm, resolver)

	_, err := uc.GenerateQuestions(ctx, domain.GenerateQuestionsInput{
		CreatorID:    "user-123",
		SourceType:   domain.SourceTypeKindleBook,
		SourceID:     "B00BOOK",
		QuestionType: domain.QuestionTypeMultipleChoice,
		UserPlan:     "free",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if usedModel != "gemini-2.5-flash" {
		t.Errorf("expected gemini-2.5-flash, got %s", usedModel)
	}
}

func TestGenerateQuestions_SupportsKindleBookSource(t *testing.T) {
	ctx := context.Background()

	llm := &mockLLMClient{
		generateQuestions: func(ctx context.Context, points []domain.ExtractedPoint, questionType domain.QuestionType, customInstruction string, model string) ([]domain.GeneratedQuestion, error) {
			return []domain.GeneratedQuestion{{
				Content:       "問題",
				Options:       []string{"A", "B", "C", "D"},
				CorrectAnswer: "A",
				Explanation:   "解説",
			}}, nil
		},
	}

	repo := &mockQuestionRepository{
		saveGeneration: func(_ context.Context, _, _, _, _, _ string) (string, error) { return "gen-id", nil },
		save:           func(_ context.Context, _ *domain.Question, _ *domain.QuestionMeta) error { return nil },
	}

	resolver := &mockQuestionSourceResolver{
		resolveHighlights: func(ctx context.Context, userID string, sourceType domain.SourceType, sourceID string, bookTitle string, bookAuthor string) ([]*domain.Highlight, error) {
			if sourceType != domain.SourceTypeKindleBook {
				t.Fatalf("unexpected source type: %s", sourceType)
			}
			return []*domain.Highlight{{ID: uuid.New(), Content: "book highlight"}}, nil
		},
	}

	uc := usecase.NewQuestionUsecase(repo, llm, resolver)

	questions, err := uc.GenerateQuestions(ctx, domain.GenerateQuestionsInput{
		CreatorID:    "user-123",
		SourceType:   domain.SourceTypeKindleBook,
		SourceID:     "B00BOOK",
		QuestionType: domain.QuestionTypeMultipleChoice,
		UserPlan:     "free",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(questions) != 1 {
		t.Fatalf("expected 1 question, got %d", len(questions))
	}
}

func TestListPreparedQuestions_ReturnsPreparingWhenHighlightsPending(t *testing.T) {
	ctx := context.Background()
	highlightID := uuid.New()

	repo := &mockQuestionRepository{
		listPreparedByHighlightIDs: func(ctx context.Context, userID string, highlightIDs []uuid.UUID, limit int) ([]*domain.Question, error) {
			return make([]*domain.Question, 0), nil
		},
	}
	resolver := &mockQuestionSourceResolver{
		resolveHighlights: func(ctx context.Context, userID string, sourceType domain.SourceType, sourceID string, bookTitle string, bookAuthor string) ([]*domain.Highlight, error) {
			return []*domain.Highlight{
				{
					ID:      highlightID,
					Content: "共有ハイライト",
					Status:  domain.HighlightStatusPending,
				},
			}, nil
		},
	}

	uc := usecase.NewQuestionUsecase(repo, &mockLLMClient{}, resolver)

	_, err := uc.ListPreparedQuestions(ctx, domain.GenerateQuestionsInput{
		CreatorID:     "user-123",
		SourceType:    domain.SourceTypeKindleBook,
		SourceID:      "metadata:book",
		QuestionCount: 3,
	})
	if !errors.Is(err, domain.ErrQuestionsPreparing) {
		t.Fatalf("expected ErrQuestionsPreparing, got %v", err)
	}
}

func TestGenerateQuestions_AllModeIsCapped(t *testing.T) {
	ctx := context.Background()

	highlights := make([]*domain.Highlight, 0, 25)
	for i := 1; i <= 25; i++ {
		highlights = append(highlights, &domain.Highlight{
			ID:      uuid.New(),
			Content: fmt.Sprintf("ポイント%d", i),
		})
	}

	llm := &mockLLMClient{
		generateQuestions: func(ctx context.Context, points []domain.ExtractedPoint, questionType domain.QuestionType, customInstruction string, model string) ([]domain.GeneratedQuestion, error) {
			questions := make([]domain.GeneratedQuestion, 0, len(points))
			for _, point := range points {
				questions = append(questions, domain.GeneratedQuestion{
					Content:       "問題 " + point.Point,
					Options:       []string{"A", "B", "C", "D"},
					CorrectAnswer: "A",
					Explanation:   "解説",
				})
			}
			return questions, nil
		},
	}

	savedCount := 0
	repo := &mockQuestionRepository{
		saveGeneration: func(_ context.Context, _, _, _, _, _ string) (string, error) { return "gen-id", nil },
		save: func(_ context.Context, _ *domain.Question, _ *domain.QuestionMeta) error {
			savedCount++
			return nil
		},
	}

	resolver := &mockQuestionSourceResolver{
		resolveHighlights: func(ctx context.Context, userID string, sourceType domain.SourceType, sourceID string, bookTitle string, bookAuthor string) ([]*domain.Highlight, error) {
			return highlights, nil
		},
	}

	uc := usecase.NewQuestionUsecase(repo, llm, resolver)

	questions, err := uc.GenerateQuestions(ctx, domain.GenerateQuestionsInput{
		CreatorID:     "user-123",
		SourceType:    domain.SourceTypeKindleBook,
		SourceID:      "B00BOOK",
		QuestionCount: 0,
		QuestionType:  domain.QuestionTypeMultipleChoice,
		UserPlan:      "free",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(questions) != 20 {
		t.Fatalf("expected 20 questions in capped all mode, got %d", len(questions))
	}
	if savedCount != 20 {
		t.Fatalf("expected 20 saved questions, got %d", savedCount)
	}
}

func TestGenerateQuestions_PrefersUnusedHighlightWithExplanation(t *testing.T) {
	ctx := context.Background()
	unusedWithExplanationID := uuid.New()
	usedWithoutExplanationID := uuid.New()
	explanation := "大事な補足"

	llm := &mockLLMClient{
		generateQuestions: func(ctx context.Context, points []domain.ExtractedPoint, questionType domain.QuestionType, customInstruction string, model string) ([]domain.GeneratedQuestion, error) {
			if len(points) != 1 {
				t.Fatalf("expected 1 generation material, got %d", len(points))
			}
			if points[0].Point != "未出題ハイライト" || points[0].Context != explanation {
				t.Fatalf("unexpected prioritized material: %+v", points[0])
			}
			return []domain.GeneratedQuestion{{
				Content:       "問題",
				Options:       []string{"A", "B", "C", "D"},
				CorrectAnswer: "A",
				Explanation:   "解説",
			}}, nil
		},
	}

	repo := &mockQuestionRepository{
		listUsedHighlightIDsByUserID: func(ctx context.Context, userID string, highlightIDs []uuid.UUID) ([]uuid.UUID, error) {
			return []uuid.UUID{usedWithoutExplanationID}, nil
		},
		saveGeneration: func(_ context.Context, _, _, _, _, _ string) (string, error) { return "gen-id", nil },
		save:           func(_ context.Context, _ *domain.Question, _ *domain.QuestionMeta) error { return nil },
	}

	resolver := &mockQuestionSourceResolver{
		resolveHighlights: func(ctx context.Context, userID string, sourceType domain.SourceType, sourceID string, bookTitle string, bookAuthor string) ([]*domain.Highlight, error) {
			return []*domain.Highlight{
				{ID: usedWithoutExplanationID, Content: "出題済みハイライト"},
				{ID: unusedWithExplanationID, Content: "未出題ハイライト", Explanation: &explanation},
			}, nil
		},
	}

	uc := usecase.NewQuestionUsecase(repo, llm, resolver)

	questions, err := uc.GenerateQuestions(ctx, domain.GenerateQuestionsInput{
		CreatorID:     "user-123",
		SourceType:    domain.SourceTypeKindleBook,
		SourceID:      "B00BOOK",
		QuestionCount: 1,
		QuestionType:  domain.QuestionTypeMultipleChoice,
		UserPlan:      "free",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(questions) != 1 {
		t.Fatalf("expected 1 question, got %d", len(questions))
	}
}
