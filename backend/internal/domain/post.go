package domain

import (
	"time"

	"github.com/google/uuid"
)

type Post struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	QuestionID    *uuid.UUID
	BookID        *uuid.UUID
	FieldID       *uuid.UUID
	Body          *string
	BookTitle     *string
	QuestionCount int
	Type          string
	RepostCount   int
	LikeCount     int
	CommentCount  int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type TimelinePost struct {
	Post
	Score       int
	Username    string
	DisplayName string
	AvatarURL   *string
	FieldName   *string
}

type PostQuestionItem struct {
	QuestionID uuid.UUID
	SortOrder  int
	Note       string
}

type PostedQuestion struct {
	Question
	Note      string
	SortOrder int
}

type CreatePostInput struct {
	UserID        uuid.UUID
	QuestionID    *uuid.UUID
	BookID        *uuid.UUID
	FieldID       *uuid.UUID
	Body          string
	BookTitle     string
	QuestionCount int
	Questions     []PostQuestionItem
	Type          string
}

type TimelineParams struct {
	UserID uuid.UUID
	Limit  int
	Offset int
}
