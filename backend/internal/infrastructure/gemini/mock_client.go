package gemini

import (
	"context"
	"fmt"
	"strings"

	"github.com/shout/ai-study-tool/backend/internal/domain"
)

type MockClient struct {
}

func NewMockClient() *MockClient {
	return &MockClient{}
}

func (c *MockClient) Close() {
}

func (c *MockClient) ModelForPlan(plan string) string {
	return "mock-model"
}

func (c *MockClient) GenerateQuestions(ctx context.Context, points []domain.ExtractedPoint, questionType domain.QuestionType, customInstruction string, model string) ([]domain.GeneratedQuestion, error) {
	questions := make([]domain.GeneratedQuestion, 0, len(points))
	for index, point := range points {
		text := strings.TrimSpace(point.Point)
		if len([]rune(text)) > 80 {
			text = string([]rune(text)[:80]) + "..."
		}
		if text == "" {
			text = "this highlight"
		}

		questions = append(questions, domain.GeneratedQuestion{
			Content:       fmt.Sprintf("[MOCK] Which option best captures the key idea from %s?", text),
			Options:       []string{"The option that matches the key idea", "An unrelated option", "The opposite meaning", "An overly broad option"},
			CorrectAnswer: "The option that matches the key idea",
			Explanation:   fmt.Sprintf("[MOCK] Generated from mock material %d.", index+1),
		})
	}

	return questions, nil
}
