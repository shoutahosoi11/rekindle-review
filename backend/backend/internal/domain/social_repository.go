package domain

import "context"

type ListCommentsInput struct {
	PostID string
	Limit  int
	Offset int
}

type SocialRepository interface {
	Follow(ctx context.Context, followerID, followeeID string) error
	Unfollow(ctx context.Context, followerID, followeeID string) error
	Like(ctx context.Context, userID, postID string) error
	Unlike(ctx context.Context, userID, postID string) error
	Repost(ctx context.Context, userID, postID string) error
	Unrepost(ctx context.Context, userID, postID string) error
	CreateComment(ctx context.Context, comment *Comment) (*Comment, error)
	ListComments(ctx context.Context, input ListCommentsInput) ([]*Comment, error)
}
