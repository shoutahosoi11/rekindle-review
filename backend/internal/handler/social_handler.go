package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/shout/ai-study-tool/backend/internal/domain"
	"github.com/shout/ai-study-tool/backend/internal/handler/dto"
	"github.com/shout/ai-study-tool/backend/internal/usecase"
)

type SocialHandler struct {
	socialUsecase SocialUsecase
	postUsecase   SocialPostUsecase
	userUsecase   usecase.UserUsecaseInterface
}

type SocialUsecase interface {
	Follow(ctx context.Context, followerID, followingID string) error
	Unfollow(ctx context.Context, followerID, followingID string) error
	Like(ctx context.Context, userID, postID string) error
	Unlike(ctx context.Context, userID, postID string) error
	Repost(ctx context.Context, userID, postID string) error
	Unrepost(ctx context.Context, userID, postID string) error
	CreateComment(ctx context.Context, comment *domain.Comment) (*domain.Comment, error)
	ListComments(ctx context.Context, postID string, limit, offset int) ([]*domain.Comment, error)
}

type SocialPostUsecase interface {
	EnsureVisible(ctx context.Context, viewerID, postID uuid.UUID) error
}

func NewSocialHandler(socialUsecase SocialUsecase, postUsecase SocialPostUsecase, userUsecase usecase.UserUsecaseInterface) *SocialHandler {
	return &SocialHandler{socialUsecase: socialUsecase, postUsecase: postUsecase, userUsecase: userUsecase}
}

func (h *SocialHandler) currentUserID(c echo.Context) (string, error) {
	user, err := resolveCurrentUser(c, h.userUsecase, "social")
	if err != nil {
		return "", err
	}
	return user.ID.String(), nil
}

func (h *SocialHandler) ensureVisiblePost(c echo.Context, postID string) (string, error) {
	userID, err := h.currentUserID(c)
	if err != nil {
		return "", err
	}

	viewerID, err := uuid.Parse(userID)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}

	postUUID, err := uuid.Parse(postID)
	if err != nil {
		return "", echo.NewHTTPError(http.StatusBadRequest, "post id is required")
	}

	if err := h.postUsecase.EnsureVisible(c.Request().Context(), viewerID, postUUID); err != nil {
		return "", echo.NewHTTPError(http.StatusNotFound, "post not found")
	}

	return userID, nil
}

func (h *SocialHandler) Follow(c echo.Context) error {
	userID, err := h.currentUserID(c)
	if err != nil {
		return err
	}
	targetID := c.Param("id")
	if targetID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "user id is required")
	}
	if _, err := uuid.Parse(targetID); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}
	if err := h.socialUsecase.Follow(c.Request().Context(), userID, targetID); err != nil {
		return h.socialError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "followed"})
}

func (h *SocialHandler) Unfollow(c echo.Context) error {
	userID, err := h.currentUserID(c)
	if err != nil {
		return err
	}
	targetID := c.Param("id")
	if targetID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "user id is required")
	}
	if _, err := uuid.Parse(targetID); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}
	if err := h.socialUsecase.Unfollow(c.Request().Context(), userID, targetID); err != nil {
		return h.socialError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "unfollowed"})
}

func (h *SocialHandler) Like(c echo.Context) error {
	userID, err := h.currentUserID(c)
	if err != nil {
		return err
	}
	postID := c.Param("id")
	if postID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "post id is required")
	}
	if _, err := uuid.Parse(postID); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid post id")
	}
	if err := h.socialUsecase.Like(c.Request().Context(), userID, postID); err != nil {
		return h.socialError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "liked"})
}

func (h *SocialHandler) Unlike(c echo.Context) error {
	userID, err := h.currentUserID(c)
	if err != nil {
		return err
	}
	postID := c.Param("id")
	if postID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "post id is required")
	}
	if _, err := uuid.Parse(postID); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid post id")
	}
	if err := h.socialUsecase.Unlike(c.Request().Context(), userID, postID); err != nil {
		return h.socialError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "unliked"})
}

func (h *SocialHandler) Repost(c echo.Context) error {
	userID, err := h.currentUserID(c)
	if err != nil {
		return err
	}
	postID := c.Param("id")
	if postID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "post id is required")
	}
	if _, err := uuid.Parse(postID); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid post id")
	}
	if err := h.socialUsecase.Repost(c.Request().Context(), userID, postID); err != nil {
		return h.socialError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "reposted"})
}

func (h *SocialHandler) Unrepost(c echo.Context) error {
	userID, err := h.currentUserID(c)
	if err != nil {
		return err
	}
	postID := c.Param("id")
	if postID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "post id is required")
	}
	if _, err := uuid.Parse(postID); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid post id")
	}
	if err := h.socialUsecase.Unrepost(c.Request().Context(), userID, postID); err != nil {
		return h.socialError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "unreposted"})
}

func (h *SocialHandler) CreateComment(c echo.Context) error {
	postID := c.Param("id")
	if postID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "post id is required")
	}
	userID, err := h.ensureVisiblePost(c, postID)
	if err != nil {
		return err
	}

	req := new(dto.CreateCommentRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	comment, err := h.socialUsecase.CreateComment(c.Request().Context(), &domain.Comment{
		PostID:  postID,
		UserID:  userID,
		Content: req.Content,
	})
	if err != nil {
		return h.socialError(err)
	}

	return c.JSON(http.StatusCreated, toCommentResponse(comment))
}

func (h *SocialHandler) ListComments(c echo.Context) error {
	postID := c.Param("id")
	if postID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "post id is required")
	}
	if _, err := h.ensureVisiblePost(c, postID); err != nil {
		return err
	}

	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	offset, _ := strconv.Atoi(c.QueryParam("offset"))

	comments, err := h.socialUsecase.ListComments(c.Request().Context(), postID, limit, offset)
	if err != nil {
		return h.socialError(err)
	}

	responses := make([]dto.CommentResponse, 0, len(comments))
	for _, comment := range comments {
		responses = append(responses, toCommentResponse(comment))
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"comments": responses,
		"limit":    limit,
		"offset":   offset,
	})
}

func (h *SocialHandler) socialError(err error) error {
	if errors.Is(err, domain.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}
	if strings.HasPrefix(err.Error(), "validation:") {
		return echo.NewHTTPError(http.StatusBadRequest, strings.TrimPrefix(err.Error(), "validation: "))
	}
	return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
}

func toCommentResponse(comment *domain.Comment) dto.CommentResponse {
	return dto.CommentResponse{
		ID:          comment.ID,
		PostID:      comment.PostID,
		UserID:      comment.UserID,
		Username:    comment.Username,
		DisplayName: comment.DisplayName,
		AvatarURL:   comment.AvatarURL,
		Content:     comment.Content,
		CreatedAt:   comment.CreatedAt,
	}
}
