package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/shout/ai-study-tool/backend/internal/domain"
	"github.com/shout/ai-study-tool/backend/internal/repository/sqlcgen"
)

type userRepository struct {
	queries *sqlcgen.Queries
}

func NewUserRepository(db *sql.DB) domain.UserRepository {
	return &userRepository{queries: sqlcgen.New(db)}
}

func (r *userRepository) GetByFirebaseUID(ctx context.Context, firebaseUID string) (*domain.User, error) {
	user, err := r.queries.GetUserByFirebaseUID(ctx, firebaseUID)
	if err != nil {
		return nil, wrapUserError("get by firebase uid", err)
	}
	return toDomainUser(user), nil
}

func (r *userRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	user, err := r.queries.GetUserByID(ctx, id)
	if err != nil {
		return nil, wrapUserError("get by id", err)
	}
	return toDomainUser(user), nil
}

func (r *userRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	user, err := r.queries.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, wrapUserError("get by username", err)
	}
	return toDomainUser(user), nil
}

func (r *userRepository) Create(ctx context.Context, input domain.CreateUserInput) (*domain.User, error) {
	user, err := r.queries.CreateUser(ctx, sqlcgen.CreateUserParams{
		FirebaseUid: input.FirebaseUID,
		Username:    input.Username,
		DisplayName: input.DisplayName,
		AvatarUrl:   toNullString(input.AvatarURL),
		Bio:         toNullString(input.Bio),
		University:  toNullString(input.University),
		Faculty:     toNullString(input.Faculty),
		Grade:       toNullInt16(input.Grade),
		Country:     toNullString(input.Country),
	})
	if err != nil {
		return nil, wrapUserError("create", err)
	}
	return toDomainUser(user), nil
}

func (r *userRepository) Update(ctx context.Context, id uuid.UUID, input domain.UpdateUserInput) (*domain.User, error) {
	user, err := r.queries.UpdateUser(ctx, sqlcgen.UpdateUserParams{
		ID:          id,
		Username:    input.Username,
		DisplayName: input.DisplayName,
		AvatarUrl:   toNullString(input.AvatarURL),
		Bio:         toNullString(input.Bio),
		University:  toNullString(input.University),
		Faculty:     toNullString(input.Faculty),
		Grade:       toNullInt16(input.Grade),
		Country:     toNullString(input.Country),
	})
	if err != nil {
		return nil, wrapUserError("update", err)
	}
	return toDomainUser(user), nil
}

func (r *userRepository) UpdateQuestionSettings(ctx context.Context, id uuid.UUID, input domain.UpdateQuestionSettingsInput) (*domain.User, error) {
	user, err := r.queries.UpdateUserQuestionSettings(ctx, sqlcgen.UpdateUserQuestionSettingsParams{
		ID:                   id,
		DefaultQuestionCount: input.DefaultQuestionCount,
	})
	if err != nil {
		return nil, wrapUserError("update question settings", err)
	}
	return toDomainUser(user), nil
}

func wrapUserError(action string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("user repo: %s: %w", action, domain.ErrNotFound)
	}

	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		return fmt.Errorf("user repo: %s: %w", action, domain.ErrAlreadyExists)
	}

	return fmt.Errorf("user repo: %s: %w", action, err)
}

func toDomainUser(user sqlcgen.User) *domain.User {
	return &domain.User{
		ID:                   user.ID,
		FirebaseUID:          user.FirebaseUid,
		Username:             user.Username,
		DisplayName:          user.DisplayName,
		AvatarURL:            fromNullString(user.AvatarUrl),
		Bio:                  fromNullString(user.Bio),
		University:           fromNullString(user.University),
		Faculty:              fromNullString(user.Faculty),
		Grade:                fromNullInt16(user.Grade),
		Country:              fromNullString(user.Country),
		Plan:                 user.Plan,
		DefaultQuestionCount: user.DefaultQuestionCount,
		CreatedAt:            user.CreatedAt,
		UpdatedAt:            user.UpdatedAt,
	}
}

func toNullString(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *value, Valid: true}
}

func fromNullString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func toNullInt16(value *int16) sql.NullInt16 {
	if value == nil {
		return sql.NullInt16{}
	}
	return sql.NullInt16{Int16: *value, Valid: true}
}

func fromNullInt16(value sql.NullInt16) *int16 {
	if !value.Valid {
		return nil
	}
	return &value.Int16
}
