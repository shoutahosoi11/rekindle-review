package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/shout/ai-study-tool/backend/internal/domain"
)

type PostUsecase struct {
	postRepo domain.PostRepository
}

func NewPostUsecase(postRepo domain.PostRepository) *PostUsecase {
	return &PostUsecase{postRepo: postRepo}
}

func (u *PostUsecase) GetTimeline(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.TimelinePost, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	return u.postRepo.GetTimeline(ctx, domain.TimelineParams{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	})
}

func (u *PostUsecase) GetByID(ctx context.Context, id uuid.UUID) (*domain.TimelinePost, error) {
	return u.postRepo.GetByID(ctx, id)
}

func (u *PostUsecase) CreatePost(ctx context.Context, input domain.CreatePostInput) (*domain.Post, error) {
	input.Body = strings.TrimSpace(input.Body)
	input.BookTitle = strings.TrimSpace(input.BookTitle)
	if input.Type == "question" && len(input.Questions) == 0 {
		return nil, fmt.Errorf("validation: questions are required")
	}
	if input.Type == "question" && input.BookTitle == "" {
		return nil, fmt.Errorf("validation: book title is required")
	}
	if input.Type == "question" && input.QuestionCount <= 0 {
		input.QuestionCount = len(input.Questions)
	}
	if len([]rune(input.Body)) > 280 {
		return nil, fmt.Errorf("validation: body must be 280 characters or less")
	}
	return u.postRepo.Create(ctx, input)
}

func (u *PostUsecase) ListQuestionsByPostID(ctx context.Context, postID uuid.UUID) ([]*domain.PostedQuestion, error) {
	return u.postRepo.ListQuestionsByPostID(ctx, postID)
}

func (u *PostUsecase) EnsureVisible(ctx context.Context, viewerID, postID uuid.UUID) error {
	canView, err := u.postRepo.CanView(ctx, viewerID, postID)
	if err != nil {
		return err
	}
	if !canView {
		return domain.ErrNotFound
	}
	return nil
}
