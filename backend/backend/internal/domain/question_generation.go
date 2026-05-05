package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	QuestionPerspectiveDefinition    = "definition"
	QuestionPerspectiveComparison    = "comparison"
	QuestionPerspectivePractical     = "practical"
	QuestionPerspectiveUnderstanding = "understanding"
	QuestionPerspectivePitfall       = "pitfall"
	QuestionPerspectiveApplication   = "application"
)

var QuestionPerspectiveOrder = []string{
	QuestionPerspectiveDefinition,
	QuestionPerspectiveComparison,
	QuestionPerspectivePractical,
	QuestionPerspectiveUnderstanding,
	QuestionPerspectivePitfall,
	QuestionPerspectiveApplication,
}

type PendingHighlightUserStat struct {
	UserID          uuid.UUID
	PendingCount    int
	TotalCount      int
	OldestPendingAt time.Time
}

type BookStock struct {
	BookKey           string
	BookTitle         string
	BookAuthor        string
	Stock             int
	Preparing         int
	LatestHighlightAt time.Time
}

type QuestionGenerationBookCandidate struct {
	BookKey                 string
	BookTitle               string
	BookAuthor              string
	PendingHighlightCount   int
	UnansweredQuestionCount int
	LatestHighlightAt       time.Time
}

type RegenerationTask struct {
	ID                    uuid.UUID
	UserID                uuid.UUID
	HighlightID           uuid.UUID
	Highlight             *Highlight
	RetryCount            int
	RequestedAt           time.Time
	RequestedFromQuestion *uuid.UUID
	Reason                string
}
