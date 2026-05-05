package dto

import "time"

type CreateCommentRequest struct {
	Content string `json:"content"`
}

type CommentResponse struct {
	ID          string    `json:"id"`
	PostID      string    `json:"post_id"`
	UserID      string    `json:"user_id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	AvatarURL   *string   `json:"avatar_url,omitempty"`
	Content     string    `json:"content"`
	CreatedAt   time.Time `json:"created_at"`
}
