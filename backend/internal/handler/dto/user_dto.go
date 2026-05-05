package dto

import (
	"github.com/shout/ai-study-tool/backend/internal/domain"
)

type SignUpRequest struct {
	Username    string  `json:"username"`
	DisplayName string  `json:"display_name"`
	AvatarURL   *string `json:"avatar_url"`
	University  *string `json:"university"`
	Faculty     *string `json:"faculty"`
	Grade       *int16  `json:"grade"`
	Country     *string `json:"country"`
}

type UpdateProfileRequest struct {
	Username    string  `json:"username"`
	DisplayName string  `json:"display_name"`
	AvatarURL   *string `json:"avatar_url"`
	Bio         *string `json:"bio"`
	University  *string `json:"university"`
	Faculty     *string `json:"faculty"`
	Grade       *int16  `json:"grade"`
	Country     *string `json:"country"`
}

type PublicUserProfileResponse struct {
	ID          string  `json:"id"`
	Username    string  `json:"username"`
	DisplayName string  `json:"display_name"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
	Bio         *string `json:"bio,omitempty"`
	University  *string `json:"university,omitempty"`
	Faculty     *string `json:"faculty,omitempty"`
	Grade       *int16  `json:"grade,omitempty"`
	Country     *string `json:"country,omitempty"`
}

type MeResponse struct {
	PublicUserProfileResponse
	Plan                 string `json:"plan"`
	DefaultQuestionCount int16  `json:"default_question_count"`
}

type UpdateQuestionSettingsRequest struct {
	DefaultQuestionCount int16 `json:"default_question_count"`
}

func ToPublicUserProfileResponse(user *domain.User) PublicUserProfileResponse {
	return PublicUserProfileResponse{
		ID:          user.ID.String(),
		Username:    user.Username,
		DisplayName: user.DisplayName,
		AvatarURL:   user.AvatarURL,
		Bio:         user.Bio,
		University:  user.University,
		Faculty:     user.Faculty,
		Grade:       user.Grade,
		Country:     user.Country,
	}
}

func ToMeResponse(user *domain.User) MeResponse {
	return MeResponse{
		PublicUserProfileResponse: ToPublicUserProfileResponse(user),
		Plan:                      user.Plan,
		DefaultQuestionCount:      user.DefaultQuestionCount,
	}
}
