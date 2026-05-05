package domain

import "time"

type Follow struct {
	FollowerID string
	FolloweeID string
	CreatedAt  time.Time
}

type Like struct {
	UserID    string
	PostID    string
	CreatedAt time.Time
}

type Repost struct {
	UserID    string
	PostID    string
	CreatedAt time.Time
}

type Comment struct {
	ID          string
	PostID      string
	UserID      string
	Username    string
	DisplayName string
	AvatarURL   *string
	Content     string
	CreatedAt   time.Time
}
