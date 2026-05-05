package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/shout/ai-study-tool/backend/internal/domain"
)

type UserUsecaseInterface interface {
	SignUp(ctx context.Context, input domain.CreateUserInput) (*domain.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	GetByFirebaseUID(ctx context.Context, firebaseUID string) (*domain.User, error)
	UpdateProfile(ctx context.Context, id uuid.UUID, input domain.UpdateUserInput) (*domain.User, error)
	UpdateQuestionSettings(ctx context.Context, id uuid.UUID, input domain.UpdateQuestionSettingsInput) (*domain.User, error)
	GetPlanByFirebaseUID(ctx context.Context, firebaseUID string) (string, error)
}

type UserUsecase struct {
	userRepo domain.UserRepository
}

func NewUserUsecase(userRepo domain.UserRepository) *UserUsecase {
	return &UserUsecase{userRepo: userRepo}
}

func (u *UserUsecase) SignUp(ctx context.Context, input domain.CreateUserInput) (*domain.User, error) {
	// 登録済みかどうか
	existing, err := u.userRepo.GetByFirebaseUID(ctx, input.FirebaseUID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	// 他のユーザーが使っていないか
	existingByUsername, err := u.userRepo.GetByUsername(ctx, input.Username)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	if existingByUsername != nil {
		return nil, domain.ErrAlreadyExists
	}

	// DBの一意制約が最終防衛ラインなので、ここは事前チェックとの race を許容する。
	return u.userRepo.Create(ctx, input)
}

func (u *UserUsecase) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return u.userRepo.GetByID(ctx, id)
}

func (u *UserUsecase) GetByFirebaseUID(ctx context.Context, firebaseUID string) (*domain.User, error) {
	return u.userRepo.GetByFirebaseUID(ctx, firebaseUID)
}

func (u *UserUsecase) UpdateProfile(ctx context.Context, id uuid.UUID, input domain.UpdateUserInput) (*domain.User, error) {
	//　同じ username を他のユーザーが使っていないか
	existingByUsername, err := u.userRepo.GetByUsername(ctx, input.Username)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	if existingByUsername != nil && existingByUsername.ID != id {
		return nil, domain.ErrAlreadyExists
	}

	// DBの一意制約が最終防衛ラインなので、ここは事前チェックとの race を許容する。
	return u.userRepo.Update(ctx, id, input)
}

func (u *UserUsecase) UpdateQuestionSettings(ctx context.Context, id uuid.UUID, input domain.UpdateQuestionSettingsInput) (*domain.User, error) {
	if !domain.IsValidDefaultQuestionCount(input.DefaultQuestionCount) {
		return nil, domain.ErrInvalidInput
	}

	return u.userRepo.UpdateQuestionSettings(ctx, id, input)
}

func (u *UserUsecase) GetPlanByFirebaseUID(ctx context.Context, firebaseUID string) (string, error) {
	user, err := u.userRepo.GetByFirebaseUID(ctx, firebaseUID)
	if err != nil {
		// FirebaseUIDに対応するユーザーが未登録の場合はfreeプランとして扱う。
		// これは初回ログイン直後など、ユーザーレコード作成前にプラン判定が走るケースへの対応。
		if errors.Is(err, domain.ErrNotFound) {
			return "free", nil
		}
		return "", err
	}
	return user.Plan, nil
}
