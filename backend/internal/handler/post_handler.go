package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/shout/ai-study-tool/backend/internal/domain"
	"github.com/shout/ai-study-tool/backend/internal/usecase"
)

type PostHandler struct {
	postUsecase PostUsecase
	userUsecase usecase.UserUsecaseInterface
}

type PostUsecase interface {
	GetTimeline(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.TimelinePost, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.TimelinePost, error)
	CreatePost(ctx context.Context, input domain.CreatePostInput) (*domain.Post, error)
	ListQuestionsByPostID(ctx context.Context, postID uuid.UUID) ([]*domain.PostedQuestion, error)
	EnsureVisible(ctx context.Context, viewerID, postID uuid.UUID) error
}

func NewPostHandler(postUsecase PostUsecase, userUsecase usecase.UserUsecaseInterface) *PostHandler {
	return &PostHandler{postUsecase: postUsecase, userUsecase: userUsecase}
}

func (h *PostHandler) currentUser(c echo.Context) (*domain.User, error) {
	return resolveCurrentUser(c, h.userUsecase, "post")
}

func (h *PostHandler) GetTimeline(c echo.Context) error {
	user, err := h.currentUser(c)
	if err != nil {
		return err
	}

	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	offset, _ := strconv.Atoi(c.QueryParam("offset"))

	posts, err := h.postUsecase.GetTimeline(c.Request().Context(), user.ID, limit, offset)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}

	if posts == nil {
		posts = []*domain.TimelinePost{}
	}

	responses := make([]TimelinePostResponse, 0, len(posts))
	for _, post := range posts {
		responses = append(responses, toTimelinePostResponse(post))
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"posts":  responses,
		"limit":  limit,
		"offset": offset,
	})
}

func (h *PostHandler) GetPost(c echo.Context) error {
	user, err := h.currentUser(c)
	if err != nil {
		return err
	}

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid post id")
	}

	if err := h.postUsecase.EnsureVisible(c.Request().Context(), user.ID, id); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "post not found")
	}

	post, err := h.postUsecase.GetByID(c.Request().Context(), id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "post not found")
	}

	return c.JSON(http.StatusOK, toTimelinePostResponse(post))
}

type CreatePostRequest struct {
	QuestionID    *string                     `json:"question_id"`
	BookID        *string                     `json:"book_id"`
	FieldID       *string                     `json:"field_id"`
	Body          string                      `json:"body"`
	BookTitle     string                      `json:"book_title"`
	QuestionCount int                         `json:"question_count"`
	Questions     []CreatePostQuestionRequest `json:"questions"`
	Type          string                      `json:"type"`
}

type CreatePostQuestionRequest struct {
	QuestionID string `json:"question_id"`
	SortOrder  int    `json:"sort_order"`
	Note       string `json:"note"`
}

type PostQuestionResponse struct {
	ID            string   `json:"id"`
	QuestionType  string   `json:"question_type"`
	Content       string   `json:"content"`
	Options       []string `json:"options"`
	CorrectAnswer string   `json:"correct_answer"`
	Explanation   string   `json:"explanation"`
	Note          string   `json:"note"`
	SortOrder     int      `json:"sort_order"`
}

type PostResponse struct {
	ID            string  `json:"id"`
	UserID        string  `json:"user_id"`
	QuestionID    *string `json:"question_id,omitempty"`
	BookID        *string `json:"book_id,omitempty"`
	FieldID       *string `json:"field_id,omitempty"`
	Body          *string `json:"body,omitempty"`
	BookTitle     *string `json:"book_title,omitempty"`
	QuestionCount int     `json:"question_count"`
	Type          string  `json:"type"`
	RepostCount   int     `json:"repost_count"`
	LikeCount     int     `json:"like_count"`
	CommentCount  int     `json:"comment_count"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

type TimelinePostResponse struct {
	PostResponse
	Score       int     `json:"score"`
	Username    string  `json:"username"`
	DisplayName string  `json:"display_name"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
	FieldName   *string `json:"field_name,omitempty"`
}

func toPostResponse(post *domain.Post) PostResponse {
	if post == nil {
		return PostResponse{}
	}
	return PostResponse{
		ID:            post.ID.String(),
		UserID:        post.UserID.String(),
		QuestionID:    uuidStringPtr(post.QuestionID),
		BookID:        uuidStringPtr(post.BookID),
		FieldID:       uuidStringPtr(post.FieldID),
		Body:          post.Body,
		BookTitle:     post.BookTitle,
		QuestionCount: post.QuestionCount,
		Type:          post.Type,
		RepostCount:   post.RepostCount,
		LikeCount:     post.LikeCount,
		CommentCount:  post.CommentCount,
		CreatedAt:     post.CreatedAt.Format(time.RFC3339Nano),
		UpdatedAt:     post.UpdatedAt.Format(time.RFC3339Nano),
	}
}

func toTimelinePostResponse(post *domain.TimelinePost) TimelinePostResponse {
	if post == nil {
		return TimelinePostResponse{}
	}
	return TimelinePostResponse{
		PostResponse: toPostResponse(&post.Post),
		Score:        post.Score,
		Username:     post.Username,
		DisplayName:  post.DisplayName,
		AvatarURL:    post.AvatarURL,
		FieldName:    post.FieldName,
	}
}

func toPostQuestionResponse(question *domain.PostedQuestion) PostQuestionResponse {
	return PostQuestionResponse{
		ID:            question.ID,
		QuestionType:  string(question.QuestionType),
		Content:       question.Content,
		Options:       question.Options,
		CorrectAnswer: question.CorrectAnswer,
		Explanation:   question.Explanation,
		Note:          question.Note,
		SortOrder:     question.SortOrder,
	}
}

func (h *PostHandler) CreatePost(c echo.Context) error {
	user, err := h.currentUser(c)
	if err != nil {
		return err
	}

	req := new(CreatePostRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	input := domain.CreatePostInput{
		UserID:        user.ID,
		Type:          strings.TrimSpace(req.Type),
		Body:          req.Body,
		BookTitle:     req.BookTitle,
		QuestionCount: req.QuestionCount,
	}

	if req.QuestionID != nil {
		id, err := parseOptionalPostUUID(*req.QuestionID, "question id")
		if err != nil {
			return err
		}
		input.QuestionID = id
	}
	if req.BookID != nil {
		id, err := parseOptionalPostUUID(*req.BookID, "book id")
		if err != nil {
			return err
		}
		input.BookID = id
	}
	if req.FieldID != nil {
		id, err := parseOptionalPostUUID(*req.FieldID, "field id")
		if err != nil {
			return err
		}
		input.FieldID = id
	}
	for index, question := range req.Questions {
		id, err := uuid.Parse(question.QuestionID)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid question id")
		}
		sortOrder := question.SortOrder
		if sortOrder < 0 {
			sortOrder = index
		}
		input.Questions = append(input.Questions, domain.PostQuestionItem{
			QuestionID: id,
			SortOrder:  sortOrder,
			Note:       question.Note,
		})
	}

	post, err := h.postUsecase.CreatePost(c.Request().Context(), input)
	if err != nil {
		if strings.HasPrefix(err.Error(), "validation:") {
			return echo.NewHTTPError(http.StatusBadRequest, strings.TrimPrefix(err.Error(), "validation: "))
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}

	return c.JSON(http.StatusCreated, toPostResponse(post))
}

func parseOptionalPostUUID(value string, fieldName string) (*uuid.UUID, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	id, err := uuid.Parse(trimmed)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "invalid "+fieldName)
	}
	return &id, nil
}

func (h *PostHandler) ListQuestions(c echo.Context) error {
	user, err := h.currentUser(c)
	if err != nil {
		return err
	}

	postID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid post id")
	}

	if err := h.postUsecase.EnsureVisible(c.Request().Context(), user.ID, postID); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "post not found")
	}

	questions, err := h.postUsecase.ListQuestionsByPostID(c.Request().Context(), postID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "post not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}

	responses := make([]PostQuestionResponse, 0, len(questions))
	for _, question := range questions {
		responses = append(responses, toPostQuestionResponse(question))
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"questions": responses,
	})
}

func uuidStringPtr(value *uuid.UUID) *string {
	if value == nil {
		return nil
	}
	formatted := value.String()
	return &formatted
}
