package domain

import "context"

type ExtractedPoint struct {
	Point   string
	Context string
}

type GeneratedQuestion struct {
	Content       string
	Options       []string
	CorrectAnswer string
	Explanation   string
}

type LLMClient interface {
	ModelForPlan(plan string) string
	GenerateQuestions(ctx context.Context, points []ExtractedPoint, questionType QuestionType, customInstruction string, model string) ([]GeneratedQuestion, error)
}
