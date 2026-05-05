package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	DefaultQuestionCountAll     int16 = 0
	DefaultQuestionCountDefault int16 = 3
	DefaultQuestionCountMax     int16 = 10
)

type User struct {
	ID                   uuid.UUID
	FirebaseUID          string
	Username             string
	DisplayName          string
	AvatarURL            *string
	Bio                  *string
	University           *string
	Faculty              *string
	Grade                *int16
	Country              *string
	Plan                 string
	DefaultQuestionCount int16
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type CreateUserInput struct {
	FirebaseUID string
	Username    string
	DisplayName string
	AvatarURL   *string
	Bio         *string
	University  *string
	Faculty     *string
	Grade       *int16
	Country     *string
}

type UpdateUserInput struct {
	Username    string
	DisplayName string
	AvatarURL   *string
	Bio         *string
	University  *string
	Faculty     *string
	Grade       *int16
	Country     *string
}

type UpdateQuestionSettingsInput struct {
	DefaultQuestionCount int16
}

func IsValidDefaultQuestionCount(count int16) bool {
	if count == DefaultQuestionCountAll {
		return true
	}
	return count >= 1 && count <= DefaultQuestionCountMax
}
