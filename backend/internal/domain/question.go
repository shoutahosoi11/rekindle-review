package domain

import "time"

type QuestionType string
type SourceType string

const (
	QuestionTypeMultipleChoice QuestionType = "multiple_choice"

	SourceTypeKindleBook SourceType = "kindle_book"
)

type Question struct {
	ID            string
	QuestionType  QuestionType
	Content       string
	Options       []string
	CorrectAnswer string
	Explanation   string
}

type SavedQuestion struct {
	Question
	Note    string
	SavedAt time.Time
}

type IncorrectQuestion struct {
	Question
	Note       string
	AnsweredAt time.Time
}

func (q *Question) IsCorrect(answer string) bool {
	return q.CorrectAnswer == answer
}
