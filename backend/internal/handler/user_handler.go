package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/shout/ai-study-tool/backend/internal/domain"
	"github.com/shout/ai-study-tool/backend/internal/handler/dto"
	"github.com/shout/ai-study-tool/backend/internal/middleware"
	"github.com/shout/ai-study-tool/backend/internal/usecase"
)

// ユーザーデータに関する処理をするための部品などをまとめたファイル
type UserHandler struct {
	userUsecase usecase.UserUsecaseInterface
}

func NewUserHandler(userUsecase usecase.UserUsecaseInterface) *UserHandler {
	return &UserHandler{userUsecase: userUsecase}
}

func (h *UserHandler) SignUp(c echo.Context) error {
	firebaseUID, ok := middleware.GetFirebaseUID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}
	// usernameやdisplay_nameなどをリクエストボディから受け取る
	//　DBからログイン中のユーザーのusernameを取得しているわけではない
	req := new(dto.SignUpRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	// usecaseに渡す形に整形
	input, err := buildCreateUserInput(firebaseUID, req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	// 既存ユーザー確認や username 重複確認
	user, err := h.userUsecase.SignUp(c.Request().Context(), input)
	if err != nil {
		if errors.Is(err, domain.ErrAlreadyExists) {
			return echo.NewHTTPError(http.StatusConflict, "user already exists")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}

	return c.JSON(http.StatusOK, dto.ToMeResponse(user))
}

func (h *UserHandler) GetMe(c echo.Context) error {
	firebaseUID, ok := middleware.GetFirebaseUID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	user, err := h.userUsecase.GetByFirebaseUID(c.Request().Context(), firebaseUID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "user not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}

	return c.JSON(http.StatusOK, dto.ToMeResponse(user))
}

func (h *UserHandler) GetUser(c echo.Context) error {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}

	user, err := h.userUsecase.GetByID(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "user not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}

	return c.JSON(http.StatusOK, dto.ToPublicUserProfileResponse(user))
}

func (h *UserHandler) UpdateProfile(c echo.Context) error {
	firebaseUID, ok := middleware.GetFirebaseUID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}
	// firebaseUID から me を取得しているのは、「誰のプロフィールを更新するか」を安全に確定
	me, err := h.userUsecase.GetByFirebaseUID(c.Request().Context(), firebaseUID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "user not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}
	// プロフィール更新リクエストを受け取る箱を作る
	req := new(dto.UpdateProfileRequest)
	// リクエストボディのJSONを req に詰める
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	input, err := buildUpdateUserInput(req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	// me.ID を使っているので、リクエスト側が勝手に他人のIDを指定して更新する形にはなっていません。
	updated, err := h.userUsecase.UpdateProfile(c.Request().Context(), me.ID, input)
	if err != nil {
		if errors.Is(err, domain.ErrAlreadyExists) {
			return echo.NewHTTPError(http.StatusConflict, "user already exists")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}

	return c.JSON(http.StatusOK, dto.ToMeResponse(updated))
}

func (h *UserHandler) UpdateQuestionSettings(c echo.Context) error {
	firebaseUID, ok := middleware.GetFirebaseUID(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	me, err := h.userUsecase.GetByFirebaseUID(c.Request().Context(), firebaseUID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "user not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}

	req := new(dto.UpdateQuestionSettingsRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if !domain.IsValidDefaultQuestionCount(req.DefaultQuestionCount) {
		return echo.NewHTTPError(http.StatusBadRequest, "default_question_count must be 0 or between 1 and 10")
	}

	updated, err := h.userUsecase.UpdateQuestionSettings(c.Request().Context(), me.ID, domain.UpdateQuestionSettingsInput{
		DefaultQuestionCount: req.DefaultQuestionCount,
	})
	if err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid question settings")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}

	return c.JSON(http.StatusOK, dto.ToMeResponse(updated))
}

func buildCreateUserInput(firebaseUID string, req *dto.SignUpRequest) (domain.CreateUserInput, error) {
	username := strings.TrimSpace(req.Username)
	if err := validateRequiredText(username, "username", 3, 50); err != nil {
		return domain.CreateUserInput{}, err
	}

	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		displayName = username
	}
	if err := validateRequiredText(displayName, "display_name", 1, 100); err != nil {
		return domain.CreateUserInput{}, err
	}
	university := normalizeOptionalText(req.University)
	if err := validateOptionalText(university, "university", 100); err != nil {
		return domain.CreateUserInput{}, err
	}
	faculty := normalizeOptionalText(req.Faculty)
	if err := validateOptionalText(faculty, "faculty", 100); err != nil {
		return domain.CreateUserInput{}, err
	}
	country := normalizeOptionalText(req.Country)
	if err := validateOptionalText(country, "country", 10); err != nil {
		return domain.CreateUserInput{}, err
	}

	return domain.CreateUserInput{
		FirebaseUID: firebaseUID,
		Username:    username,
		DisplayName: displayName,
		AvatarURL:   req.AvatarURL,
		University:  university,
		Faculty:     faculty,
		Grade:       req.Grade,
		Country:     country,
	}, nil
}

func buildUpdateUserInput(req *dto.UpdateProfileRequest) (domain.UpdateUserInput, error) {
	username := strings.TrimSpace(req.Username)
	if err := validateRequiredText(username, "username", 3, 50); err != nil {
		return domain.UpdateUserInput{}, err
	}

	displayName := strings.TrimSpace(req.DisplayName)
	if err := validateRequiredText(displayName, "display_name", 1, 100); err != nil {
		return domain.UpdateUserInput{}, err
	}
	avatarURL := normalizeOptionalText(req.AvatarURL)
	bio := normalizeOptionalText(req.Bio)
	university := normalizeOptionalText(req.University)
	if err := validateOptionalText(university, "university", 100); err != nil {
		return domain.UpdateUserInput{}, err
	}
	faculty := normalizeOptionalText(req.Faculty)
	if err := validateOptionalText(faculty, "faculty", 100); err != nil {
		return domain.UpdateUserInput{}, err
	}
	country := normalizeOptionalText(req.Country)
	if err := validateOptionalText(country, "country", 10); err != nil {
		return domain.UpdateUserInput{}, err
	}

	return domain.UpdateUserInput{
		Username:    username,
		DisplayName: displayName,
		AvatarURL:   avatarURL,
		Bio:         bio,
		University:  university,
		Faculty:     faculty,
		Grade:       req.Grade,
		Country:     country,
	}, nil
}

func validateRequiredText(value, field string, minLen, maxLen int) error {
	if value == "" {
		return errors.New(field + " is required")
	}

	length := utf8.RuneCountInString(value)
	if length < minLen {
		return errors.New(field + " must be at least " + strconv.Itoa(minLen) + " characters")
	}
	if length > maxLen {
		return errors.New(field + " must be at most " + strconv.Itoa(maxLen) + " characters")
	}

	return nil
}

func validateOptionalText(value *string, field string, maxLen int) error {
	if value == nil {
		return nil
	}

	if utf8.RuneCountInString(strings.TrimSpace(*value)) > maxLen {
		return errors.New(field + " must be at most " + strconv.Itoa(maxLen) + " characters")
	}

	return nil
}

func normalizeOptionalText(value *string) *string {
	if value == nil {
		return nil
	}

	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}
