package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/shout/ai-study-tool/backend/internal/domain"
)

type SocialUsecase struct {
	socialRepo domain.SocialRepository
}

func NewSocialUsecase(socialRepo domain.SocialRepository) *SocialUsecase {
	return &SocialUsecase{socialRepo: socialRepo}
}

func (u *SocialUsecase) Follow(ctx context.Context, followerID, followeeID string) error {
	if followerID == followeeID {
		return fmt.Errorf("validation: cannot follow yourself")
	}
	return u.socialRepo.Follow(ctx, followerID, followeeID)
}

func (u *SocialUsecase) Unfollow(ctx context.Context, followerID, followeeID string) error {
	if followerID == followeeID {
		return fmt.Errorf("validation: cannot unfollow yourself")
	}
	return u.socialRepo.Unfollow(ctx, followerID, followeeID)
}

func (u *SocialUsecase) Like(ctx context.Context, userID, postID string) error {
	return u.socialRepo.Like(ctx, userID, postID)
}

func (u *SocialUsecase) Unlike(ctx context.Context, userID, postID string) error {
	return u.socialRepo.Unlike(ctx, userID, postID)
}

func (u *SocialUsecase) Repost(ctx context.Context, userID, postID string) error {
	return u.socialRepo.Repost(ctx, userID, postID)
}

func (u *SocialUsecase) Unrepost(ctx context.Context, userID, postID string) error {
	return u.socialRepo.Unrepost(ctx, userID, postID)
}

func (u *SocialUsecase) CreateComment(ctx context.Context, comment *domain.Comment) (*domain.Comment, error) {
	content := strings.TrimSpace(comment.Content)
	if content == "" {
		return nil, fmt.Errorf("validation: content is required")
	}
	if len([]rune(content)) > 500 {
		return nil, fmt.Errorf("validation: content must be 500 characters or less")
	}

	comment.Content = content
	return u.socialRepo.CreateComment(ctx, comment)
}

func (u *SocialUsecase) ListComments(ctx context.Context, postID string, limit, offset int) ([]*domain.Comment, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	return u.socialRepo.ListComments(ctx, domain.ListCommentsInput{
		PostID: postID,
		Limit:  limit,
		Offset: offset,
	})
}
