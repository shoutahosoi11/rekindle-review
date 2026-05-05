package domain

import (
	"context"

	"github.com/google/uuid"
)

type UserRepository interface {
	GetByFirebaseUID(ctx context.Context, firebaseUID string) (*User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByUsername(ctx context.Context, username string) (*User, error)
	Create(ctx context.Context, input CreateUserInput) (*User, error)
	Update(ctx context.Context, id uuid.UUID, input UpdateUserInput) (*User, error)
	UpdateQuestionSettings(ctx context.Context, id uuid.UUID, input UpdateQuestionSettingsInput) (*User, error)
}
