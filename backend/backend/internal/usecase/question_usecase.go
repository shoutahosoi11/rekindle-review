package usecase

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shout/ai-study-tool/backend/internal/domain"
)

type QuestionUsecase struct {
	repo           domain.QuestionUsecaseRepository
	llmClient      domain.LLMClient
	sourceResolver domain.QuestionSourceResolver
}

const (
	maxExplicitQuestionCount = 10
	maxQuestionCountForAll   = 20
)

func NewQuestionUsecase(repo domain.QuestionUsecaseRepository, llmClient domain.LLMClient, sourceResolver domain.QuestionSourceResolver) *QuestionUsecase {
	return &QuestionUsecase{
		repo:           repo,
		llmClient:      llmClient,
		sourceResolver: sourceResolver,
	}
}

func (u *QuestionUsecase) ListQuestions(ctx context.Context, creatorID string, limit int) ([]*domain.Question, error) {
	questions, err := u.repo.ListByCreatorID(ctx, creatorID, limit)
	if err != nil {
		return nil, fmt.Errorf("question usecase: list questions: %w", err)
	}
	if questions == nil {
		return make([]*domain.Question, 0), nil
	}

	return questions, nil
}

func (u *QuestionUsecase) ListSavedQuestions(ctx context.Context, userID string, limit int) ([]*domain.SavedQuestion, error) {
	savedQuestions, err := u.repo.ListSavedByUserID(ctx, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("question usecase: list saved questions: %w", err)
	}
	if savedQuestions == nil {
		return make([]*domain.SavedQuestion, 0), nil
	}

	return savedQuestions, nil
}

func (u *QuestionUsecase) ListIncorrectQuestions(ctx context.Context, userID string, limit int) ([]*domain.IncorrectQuestion, error) {
	incorrectQuestions, err := u.repo.ListIncorrectByUserID(ctx, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("question usecase: list incorrect questions: %w", err)
	}
	if incorrectQuestions == nil {
		return make([]*domain.IncorrectQuestion, 0), nil
	}

	return incorrectQuestions, nil
}

func (u *QuestionUsecase) ListPreparedQuestions(ctx context.Context, input domain.GenerateQuestionsInput) ([]*domain.Question, error) {
	if !isSupportedQuestionSourceType(input.SourceType) {
		return nil, domain.ErrInvalidSourceType
	}

	sourceHighlights, err := u.sourceResolver.ResolveHighlights(
		ctx,
		input.CreatorID,
		input.SourceType,
		input.SourceID,
		input.BookTitle,
		input.BookAuthor,
	)
	if err != nil {
		return nil, fmt.Errorf("question usecase: resolve source highlights: %w", err)
	}

	candidates := filterNonEmptyHighlights(sourceHighlights)
	candidates = filterHighlightsByBookOrderIndex(candidates, input.HighlightStartIndex, input.HighlightEndIndex)
	if len(candidates) == 0 {
		return nil, domain.ErrSourceTextUnavailable
	}

	highlightIDs := make([]uuid.UUID, 0, len(candidates))
	hasPending := false
	hasFailed := false
	for _, highlight := range candidates {
		highlightIDs = append(highlightIDs, highlight.ID)
		if highlight.Status == domain.HighlightStatusPending || highlight.Status == domain.HighlightStatusProcessing {
			hasPending = true
		}
		if highlight.Status == domain.HighlightStatusFailed {
			hasFailed = true
		}
	}

	limit := resolveQuestionSelectionCount(len(candidates), input.QuestionCount)
	preparedQuestions, err := u.repo.ListPreparedByUserIDAndHighlightIDs(ctx, input.CreatorID, highlightIDs, limit)
	if err != nil {
		return nil, fmt.Errorf("question usecase: list prepared questions: %w", err)
	}
	if len(preparedQuestions) > 0 {
		return preparedQuestions, nil
	}
	if hasPending {
		return nil, domain.ErrQuestionsPreparing
	}
	if hasFailed {
		return nil, domain.ErrQuestionGenerationFailed
	}

	return nil, domain.ErrSourceTextUnavailable
}

func filterHighlightsByBookOrderIndex(highlights []*domain.Highlight, startIndex int, endIndex int) []*domain.Highlight {
	if startIndex == 0 && endIndex == 0 {
		return highlights
	}
	filtered := make([]*domain.Highlight, 0, len(highlights))
	for _, highlight := range highlights {
		if highlight == nil || highlight.BookOrderIndex == nil {
			continue
		}
		index := *highlight.BookOrderIndex
		if startIndex > 0 && index < startIndex {
			continue
		}
		if endIndex > 0 && index > endIndex {
			continue
		}
		filtered = append(filtered, highlight)
	}
	return filtered
}

func (u *QuestionUsecase) GenerateQuestions(ctx context.Context, input domain.GenerateQuestionsInput) ([]*domain.Question, error) {
	model := u.llmClient.ModelForPlan(input.UserPlan)

	if !isSupportedQuestionSourceType(input.SourceType) {
		return nil, domain.ErrInvalidSourceType
	}

	sourceHighlights, err := u.sourceResolver.ResolveHighlights(
		ctx,
		input.CreatorID,
		input.SourceType,
		input.SourceID,
		input.BookTitle,
		input.BookAuthor,
	)
	if err != nil {
		return nil, fmt.Errorf("question usecase: resolve source highlights: %w", err)
	}

	selectedHighlights, err := u.selectHighlightsForGeneration(ctx, input.CreatorID, sourceHighlights, input.QuestionCount)
	if err != nil {
		return nil, fmt.Errorf("question usecase: select source highlights: %w", err)
	}

	genID, err := u.repo.SaveGeneration(ctx,
		input.CreatorID,
		string(input.SourceType),
		input.SourceID,
		"2-step prompt",
		model,
	)
	if err != nil {
		return nil, fmt.Errorf("question usecase: save generation: %w", err)
	}

	points := buildGenerationMaterials(selectedHighlights)
	generatedQuestions, err := u.llmClient.GenerateQuestions(ctx, points, input.QuestionType, input.CustomInstruction, model)
	if err != nil {
		return nil, fmt.Errorf("question usecase: generate questions: %w", err)
	}

	pairCount := len(generatedQuestions)
	if len(selectedHighlights) < pairCount {
		pairCount = len(selectedHighlights)
	}

	questions := make([]*domain.Question, 0, pairCount)
	for index := 0; index < pairCount; index++ {
		generatedQuestion := generatedQuestions[index]
		sourceHighlight := selectedHighlights[index]

		q := &domain.Question{
			ID:            uuid.New().String(),
			QuestionType:  input.QuestionType,
			Content:       generatedQuestion.Content,
			Options:       generatedQuestion.Options,
			CorrectAnswer: generatedQuestion.CorrectAnswer,
			Explanation:   generatedQuestion.Explanation,
		}
		meta := &domain.QuestionMeta{
			QuestionID:    q.ID,
			CreatorID:     input.CreatorID,
			SourceType:    input.SourceType,
			SourceID:      input.SourceID,
			HighlightID:   sourceHighlight.ID.String(),
			GenerationID:  genID,
			IsAIGenerated: true,
		}

		if err := u.repo.Save(ctx, q, meta); err != nil {
			log.Printf("question usecase: save question error: %v", err)
			continue
		}
		questions = append(questions, q)
	}

	if len(questions) == 0 {
		return nil, fmt.Errorf("question usecase: all question generation failed")
	}

	return questions, nil
}

func (u *QuestionUsecase) selectHighlightsForGeneration(ctx context.Context, userID string, highlights []*domain.Highlight, questionCount int) ([]*domain.Highlight, error) {
	candidates := filterNonEmptyHighlights(highlights)
	if len(candidates) == 0 {
		return nil, domain.ErrSourceTextUnavailable
	}

	highlightIDs := make([]uuid.UUID, 0, len(candidates))
	for _, highlight := range candidates {
		highlightIDs = append(highlightIDs, highlight.ID)
	}

	usedHighlightIDs, err := u.repo.ListUsedHighlightIDsByUserID(ctx, userID, highlightIDs)
	if err != nil {
		return nil, fmt.Errorf("question usecase: list used highlight ids: %w", err)
	}

	usedSet := make(map[uuid.UUID]struct{}, len(usedHighlightIDs))
	for _, highlightID := range usedHighlightIDs {
		usedSet[highlightID] = struct{}{}
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	prioritized := prioritizeHighlightsForGeneration(candidates, usedSet, rng)
	selectedCount := resolveQuestionSelectionCount(len(prioritized), questionCount)
	if selectedCount == 0 {
		return nil, domain.ErrSourceTextUnavailable
	}

	return prioritized[:selectedCount], nil
}

func (u *QuestionUsecase) SaveQuestion(ctx context.Context, userID string, questionID string, note string) error {
	if _, err := u.repo.GetByID(ctx, questionID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ErrNotFound
		}
		return fmt.Errorf("question usecase: get question for save: %w", err)
	}

	if err := u.repo.SaveForUser(ctx, userID, questionID, strings.TrimSpace(note)); err != nil {
		return fmt.Errorf("question usecase: save question: %w", err)
	}

	return nil
}

func isSupportedQuestionSourceType(sourceType domain.SourceType) bool {
	switch sourceType {
	case domain.SourceTypeKindleBook:
		return true
	default:
		return false
	}
}

func sanitizeQuestionCount(questionCount int) int {
	if questionCount <= 0 {
		return 0
	}
	if questionCount > maxExplicitQuestionCount {
		return maxExplicitQuestionCount
	}
	return questionCount
}

func resolveQuestionSelectionCount(candidateCount int, questionCount int) int {
	if candidateCount <= 0 {
		return 0
	}

	if questionCount == 0 {
		if candidateCount > maxQuestionCountForAll {
			return maxQuestionCountForAll
		}
		return candidateCount
	}

	maxQuestions := sanitizeQuestionCount(questionCount)
	if maxQuestions > candidateCount {
		return candidateCount
	}

	return maxQuestions
}

func filterNonEmptyHighlights(highlights []*domain.Highlight) []*domain.Highlight {
	filtered := make([]*domain.Highlight, 0, len(highlights))
	for _, highlight := range highlights {
		if highlight == nil {
			continue
		}
		if strings.TrimSpace(highlight.Content) == "" {
			continue
		}
		filtered = append(filtered, highlight)
	}

	return filtered
}

func prioritizeHighlightsForGeneration(highlights []*domain.Highlight, usedSet map[uuid.UUID]struct{}, rng *rand.Rand) []*domain.Highlight {
	unusedWithExplanation := make([]*domain.Highlight, 0)
	unusedWithoutExplanation := make([]*domain.Highlight, 0)
	usedWithExplanation := make([]*domain.Highlight, 0)
	usedWithoutExplanation := make([]*domain.Highlight, 0)

	for _, highlight := range highlights {
		_, alreadyUsed := usedSet[highlight.ID]
		hasExplanation := highlightHasExplanation(highlight)

		switch {
		case !alreadyUsed && hasExplanation:
			unusedWithExplanation = append(unusedWithExplanation, highlight)
		case !alreadyUsed && !hasExplanation:
			unusedWithoutExplanation = append(unusedWithoutExplanation, highlight)
		case alreadyUsed && hasExplanation:
			usedWithExplanation = append(usedWithExplanation, highlight)
		default:
			usedWithoutExplanation = append(usedWithoutExplanation, highlight)
		}
	}

	shuffleHighlights(unusedWithExplanation, rng)
	shuffleHighlights(unusedWithoutExplanation, rng)
	shuffleHighlights(usedWithExplanation, rng)
	shuffleHighlights(usedWithoutExplanation, rng)

	ordered := make([]*domain.Highlight, 0, len(highlights))
	ordered = append(ordered, unusedWithExplanation...)
	ordered = append(ordered, unusedWithoutExplanation...)
	ordered = append(ordered, usedWithExplanation...)
	ordered = append(ordered, usedWithoutExplanation...)
	return ordered
}

func highlightHasExplanation(highlight *domain.Highlight) bool {
	return highlight != nil && highlight.Explanation != nil && strings.TrimSpace(*highlight.Explanation) != ""
}

func shuffleHighlights(highlights []*domain.Highlight, rng *rand.Rand) {
	if len(highlights) <= 1 || rng == nil {
		return
	}

	rng.Shuffle(len(highlights), func(i, j int) {
		highlights[i], highlights[j] = highlights[j], highlights[i]
	})
}

func buildGenerationMaterials(highlights []*domain.Highlight) []domain.ExtractedPoint {
	materials := make([]domain.ExtractedPoint, 0, len(highlights))
	for _, highlight := range highlights {
		if highlight == nil {
			continue
		}

		content := strings.TrimSpace(highlight.Content)
		if content == "" {
			continue
		}

		context := ""
		if highlight.Explanation != nil {
			context = strings.TrimSpace(*highlight.Explanation)
		}

		materials = append(materials, domain.ExtractedPoint{
			Point:   content,
			Context: context,
		})
	}

	return materials
}
