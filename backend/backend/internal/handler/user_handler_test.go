package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/shout/ai-study-tool/backend/internal/domain"
	"github.com/shout/ai-study-tool/backend/internal/handler/dto"
	"github.com/shout/ai-study-tool/backend/internal/middleware"
)

func TestBuildCreateUserInputDefaultsDisplayNameToUsername(t *testing.T) {
	req := &dto.SignUpRequest{
		Username: "  alice  ",
	}

	input, err := buildCreateUserInput("firebase-uid-1", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if input.FirebaseUID != "firebase-uid-1" {
		t.Fatalf("unexpected firebase uid: %s", input.FirebaseUID)
	}
	if input.Username != "alice" {
		t.Fatalf("unexpected username: %s", input.Username)
	}
	if input.DisplayName != "alice" {
		t.Fatalf("unexpected display name: %s", input.DisplayName)
	}
}

func TestBuildCreateUserInputNormalizesOptionalText(t *testing.T) {
	university := "  Example University  "
	faculty := "   "
	country := "  JP  "
	req := &dto.SignUpRequest{
		Username:   "alice",
		University: &university,
		Faculty:    &faculty,
		Country:    &country,
	}

	input, err := buildCreateUserInput("firebase-uid-1", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if input.University == nil || *input.University != "Example University" {
		t.Fatalf("unexpected university: %#v", input.University)
	}
	if input.Faculty != nil {
		t.Fatalf("expected nil faculty, got %#v", *input.Faculty)
	}
	if input.Country == nil || *input.Country != "JP" {
		t.Fatalf("unexpected country: %#v", input.Country)
	}
}

func TestBuildCreateUserInputRejectsShortUsername(t *testing.T) {
	req := &dto.SignUpRequest{
		Username: "ab",
	}

	_, err := buildCreateUserInput("firebase-uid-1", req)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildUpdateUserInputUsesDTOBoundary(t *testing.T) {
	req := &dto.UpdateProfileRequest{
		Username:    "  alice  ",
		DisplayName: "  Alice  ",
	}

	input, err := buildUpdateUserInput(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if input.Username != "alice" {
		t.Fatalf("unexpected username: %s", input.Username)
	}
	if input.DisplayName != "Alice" {
		t.Fatalf("unexpected display name: %s", input.DisplayName)
	}
}

func TestBuildUpdateUserInputNormalizesOptionalText(t *testing.T) {
	avatarURL := "  https://example.com/avatar.png  "
	bio := "   "
	university := "  Example University  "
	req := &dto.UpdateProfileRequest{
		Username:    "alice",
		DisplayName: "Alice",
		AvatarURL:   &avatarURL,
		Bio:         &bio,
		University:  &university,
	}

	input, err := buildUpdateUserInput(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if input.AvatarURL == nil || *input.AvatarURL != "https://example.com/avatar.png" {
		t.Fatalf("unexpected avatar url: %#v", input.AvatarURL)
	}
	if input.Bio != nil {
		t.Fatalf("expected nil bio, got %#v", *input.Bio)
	}
	if input.University == nil || *input.University != "Example University" {
		t.Fatalf("unexpected university: %#v", input.University)
	}
}

type stubUserUsecase struct {
	signUp                 func(ctx context.Context, input domain.CreateUserInput) (*domain.User, error)
	getByFirebaseUID       func(ctx context.Context, firebaseUID string) (*domain.User, error)
	getByID                func(ctx context.Context, id uuid.UUID) (*domain.User, error)
	updateProfile          func(ctx context.Context, id uuid.UUID, input domain.UpdateUserInput) (*domain.User, error)
	updateQuestionSettings func(ctx context.Context, id uuid.UUID, input domain.UpdateQuestionSettingsInput) (*domain.User, error)
	getPlanByFirebaseUID   func(ctx context.Context, firebaseUID string) (string, error)
}

func (s *stubUserUsecase) SignUp(ctx context.Context, input domain.CreateUserInput) (*domain.User, error) {
	if s.signUp == nil {
		return nil, nil
	}
	return s.signUp(ctx, input)
}

func (s *stubUserUsecase) GetByFirebaseUID(ctx context.Context, firebaseUID string) (*domain.User, error) {
	if s.getByFirebaseUID == nil {
		return nil, nil
	}
	return s.getByFirebaseUID(ctx, firebaseUID)
}

func (s *stubUserUsecase) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	if s.getByID == nil {
		return nil, nil
	}
	return s.getByID(ctx, id)
}

func (s *stubUserUsecase) UpdateProfile(ctx context.Context, id uuid.UUID, input domain.UpdateUserInput) (*domain.User, error) {
	if s.updateProfile == nil {
		return nil, nil
	}
	return s.updateProfile(ctx, id, input)
}

func (s *stubUserUsecase) UpdateQuestionSettings(ctx context.Context, id uuid.UUID, input domain.UpdateQuestionSettingsInput) (*domain.User, error) {
	if s.updateQuestionSettings == nil {
		return nil, nil
	}
	return s.updateQuestionSettings(ctx, id, input)
}

func (s *stubUserUsecase) GetPlanByFirebaseUID(ctx context.Context, firebaseUID string) (string, error) {
	if s.getPlanByFirebaseUID == nil {
		return "", nil
	}
	return s.getPlanByFirebaseUID(ctx, firebaseUID)
}

func TestGetMeReturnsUnauthorizedWithoutFirebaseUID(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := NewUserHandler(&stubUserUsecase{})

	err := handler.GetMe(c)
	if err == nil {
		t.Fatal("expected error")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected *echo.HTTPError, got %T", err)
	}
	if httpErr.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: %d", httpErr.Code)
	}
}

func TestGetUserReturnsBadRequestForInvalidUUID(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/users/not-a-uuid", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("not-a-uuid")

	handler := NewUserHandler(&stubUserUsecase{})

	err := handler.GetUser(c)
	if err == nil {
		t.Fatal("expected error")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected *echo.HTTPError, got %T", err)
	}
	if httpErr.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", httpErr.Code)
	}
}

func TestGetUserReturnsNotFoundWhenRepositoryMisses(t *testing.T) {
	e := echo.New()
	userID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/users/"+userID.String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(userID.String())

	userUsecase := &stubUserUsecase{
		getByID: func(ctx context.Context, id uuid.UUID) (*domain.User, error) {
			return nil, domain.ErrNotFound
		},
	}
	handler := NewUserHandler(userUsecase)

	err := handler.GetUser(c)
	if err == nil {
		t.Fatal("expected error")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected *echo.HTTPError, got %T", err)
	}
	if httpErr.Code != http.StatusNotFound {
		t.Fatalf("unexpected status: %d", httpErr.Code)
	}
}

func TestUpdateProfileReturnsConflictWhenUsernameAlreadyExists(t *testing.T) {
	e := echo.New()
	currentUserID := uuid.New()
	body := `{"username":"alice","display_name":"Alice"}`
	req := httptest.NewRequest(http.MethodPut, "/users/me", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(middleware.ContextFirebaseUIDKey, "firebase-uid-1")

	now := time.Now()
	userUsecase := &stubUserUsecase{
		getByFirebaseUID: func(ctx context.Context, firebaseUID string) (*domain.User, error) {
			return &domain.User{
				ID:          currentUserID,
				FirebaseUID: firebaseUID,
				Username:    "me",
				DisplayName: "Me",
				Plan:        "free",
				CreatedAt:   now,
				UpdatedAt:   now,
			}, nil
		},
		updateProfile: func(ctx context.Context, id uuid.UUID, input domain.UpdateUserInput) (*domain.User, error) {
			if id != currentUserID {
				t.Fatalf("unexpected update id: %s", id)
			}
			if input.Username != "alice" {
				t.Fatalf("unexpected username: %s", input.Username)
			}
			return nil, domain.ErrAlreadyExists
		},
	}
	handler := NewUserHandler(userUsecase)

	err := handler.UpdateProfile(c)
	if err == nil {
		t.Fatal("expected error")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected *echo.HTTPError, got %T", err)
	}
	if httpErr.Code != http.StatusConflict {
		t.Fatalf("unexpected status: %d", httpErr.Code)
	}
}

func TestGetUserWritesPublicProfileResponse(t *testing.T) {
	e := echo.New()
	userID := uuid.New()
	now := time.Now()
	req := httptest.NewRequest(http.MethodGet, "/users/"+userID.String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(userID.String())

	userUsecase := &stubUserUsecase{
		getByID: func(ctx context.Context, id uuid.UUID) (*domain.User, error) {
			return &domain.User{
				ID:          id,
				FirebaseUID: "firebase-uid-1",
				Username:    "alice",
				DisplayName: "Alice",
				Plan:        "pro",
				CreatedAt:   now,
				UpdatedAt:   now,
			}, nil
		},
	}
	handler := NewUserHandler(userUsecase)

	if err := handler.GetUser(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	var response dto.PublicUserProfileResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Username != "alice" {
		t.Fatalf("unexpected username: %s", response.Username)
	}
}
