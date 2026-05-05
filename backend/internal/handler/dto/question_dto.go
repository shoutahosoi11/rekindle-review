package dto

import (
	"time"

	"github.com/shout/ai-study-tool/backend/internal/domain"
)

type SyncQuestionStockBookResponse struct {
	BookKey    string `json:"book_key"`
	BookTitle  string `json:"book_title"`
	BookAuthor string `json:"book_author"`
	Stock      int    `json:"stock"`
	Target     int    `json:"target"`
	Preparing  int    `json:"preparing"`
}

type SyncQuestionStockResponse struct {
	Books                  []SyncQuestionStockBookResponse `json:"books"`
	QueuedCount            int                             `json:"queued_count"`
	SkippedDueToDailyLimit bool                            `json:"skipped_due_to_daily_limit"`
}

type SaveQuestionRequest struct {
	Note string `json:"note"`
}

type SaveQuestionResponse struct {
	QuestionID string `json:"question_id"`
	Note       string `json:"note"`
	Saved      bool   `json:"saved"`
}

type ManualGenerateQuestionRequest struct {
	BookKey      string   `json:"book_key"`
	HighlightIDs []string `json:"highlight_ids"`
}

type ManualGenerateQuestionResponse struct {
	JobID string `json:"job_id"`
}

type SavedQuestionResponse struct {
	ID            string    `json:"id"`
	QuestionType  string    `json:"question_type"`
	Content       string    `json:"content"`
	Options       []string  `json:"options"`
	CorrectAnswer string    `json:"correct_answer"`
	Explanation   string    `json:"explanation"`
	Note          string    `json:"note"`
	SavedAt       time.Time `json:"saved_at"`
}

type IncorrectQuestionResponse struct {
	ID            string    `json:"id"`
	QuestionType  string    `json:"question_type"`
	Content       string    `json:"content"`
	Options       []string  `json:"options"`
	CorrectAnswer string    `json:"correct_answer"`
	Explanation   string    `json:"explanation"`
	Note          string    `json:"note"`
	AnsweredAt    time.Time `json:"answered_at"`
}

type QuestionResponse struct {
	ID            string   `json:"id"`
	QuestionType  string   `json:"question_type"`
	Content       string   `json:"content"`
	Options       []string `json:"options"`
	CorrectAnswer string   `json:"correct_answer"`
	Explanation   string   `json:"explanation"`
}

func ToQuestionResponse(q *domain.Question) QuestionResponse {
	return QuestionResponse{
		ID:            q.ID,
		QuestionType:  string(q.QuestionType),
		Content:       q.Content,
		Options:       q.Options,
		CorrectAnswer: q.CorrectAnswer,
		Explanation:   q.Explanation,
	}
}

func ToSavedQuestionResponse(q *domain.SavedQuestion) SavedQuestionResponse {
	return SavedQuestionResponse{
		ID:            q.ID,
		QuestionType:  string(q.QuestionType),
		Content:       q.Content,
		Options:       q.Options,
		CorrectAnswer: q.CorrectAnswer,
		Explanation:   q.Explanation,
		Note:          q.Note,
		SavedAt:       q.SavedAt,
	}
}

func ToIncorrectQuestionResponse(q *domain.IncorrectQuestion) IncorrectQuestionResponse {
	return IncorrectQuestionResponse{
		ID:            q.ID,
		QuestionType:  string(q.QuestionType),
		Content:       q.Content,
		Options:       q.Options,
		CorrectAnswer: q.CorrectAnswer,
		Explanation:   q.Explanation,
		Note:          q.Note,
		AnsweredAt:    q.AnsweredAt,
	}
}
