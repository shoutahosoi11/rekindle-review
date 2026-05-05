package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	HighlightSourceExtension = "extension"
	HighlightSourceShare     = "share"
	HighlightSourcePaste     = "paste"
)

type HighlightStatus string

const (
	HighlightStatusPending    HighlightStatus = "pending"
	HighlightStatusProcessing HighlightStatus = "processing"
	HighlightStatusCompleted  HighlightStatus = "completed"
	HighlightStatusFailed     HighlightStatus = "failed"
)

type Highlight struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	BookID         *uuid.UUID
	BookTitle      *string
	BookAuthor     *string
	ASIN           *string
	Content        string
	Explanation    *string
	ContentHash    *string
	Location       *string
	HighlightedAt  *time.Time
	Source         string
	SourceApp      *string
	SourceURL      *string
	Status         HighlightStatus
	BookOrderIndex *int
	RetryCount     int
	LastError      *string
	RequestedAt    time.Time
	ProcessingAt   *time.Time
	CompletedAt    *time.Time
	FailedAt       *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
