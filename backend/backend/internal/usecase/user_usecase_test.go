package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/shout/ai-study-tool/backend/internal/domain"
	"github.com/shout/ai-study-tool/backend/internal/usecase"
)

type mockUserRepository struct {
	getByFirebaseUID       func(ctx context.Context, firebaseUID string) (*domain.User, error)
	getByID                func(ctx context.Context, id uuid.UUID) (*domain.User, error)
	getByUsername          func(ctx context.Context, username string) (*domain.User, error)
	create                 func(ctx context.Context, input domain.CreateUserInput) (*domain.User, error)
	update                 func(ctx context.Context, id uuid.UUID, input domain.UpdateUserInput) (*domain.User, error)
	updateQuestionSettings func(ctx context.Context, id uuid.UUID, input domain.UpdateQuestionSettingsInput) (*domain.User, error)
}

func (m *mockUserRepository) GetByFirebaseUID(ctx context.Context, firebaseUID string) (*domain.User, error) {
	return m.getByFirebaseUID(ctx, firebaseUID)
}

func (m *mockUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	if m.getByID == nil {
		return nil, domain.ErrNotFound
	}
	return m.getByID(ctx, id)
}

func (m *mockUserRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	if m.getByUsername == nil {
		return nil, domain.ErrNotFound
	}
	return m.getByUsername(ctx, username)
}

func (m *mockUserRepository) Create(ctx context.Context, input domain.CreateUserInput) (*domain.User, error) {
	if m.create == nil {
		return nil, errors.New("unexpected create call")
	}
	return m.create(ctx, input)
}

func (m *mockUserRepository) Update(ctx context.Context, id uuid.UUID, input domain.UpdateUserInput) (*domain.User, error) {
	if m.update == nil {
		return nil, errors.New("unexpected update call")
	}
	return m.update(ctx, id, input)
}

func (m *mockUserRepository) UpdateQuestionSettings(ctx context.Context, id uuid.UUID, input domain.UpdateQuestionSettingsInput) (*domain.User, error) {
	if m.updateQuestionSettings == nil {
		return nil, errors.New("unexpected update question settings call")
	}
	return m.updateQuestionSettings(ctx, id, input)
}

func TestGetPlanByFirebaseUIDReturnsFreeWhenUserNotFound(t *testing.T) {
	uc := usecase.NewUserUsecase(&mockUserRepository{
		getByFirebaseUID: func(ctx context.Context, firebaseUID string) (*domain.User, error) {
			return nil, domain.ErrNotFound
		},
	})

	plan, err := uc.GetPlanByFirebaseUID(context.Background(), "firebase-uid-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan != "free" {
		t.Fatalf("unexpected plan: %s", plan)
	}
}

func TestGetPlanByFirebaseUIDReturnsErrorOnRepositoryFailure(t *testing.T) {
	expectedErr := errors.New("db down")
	uc := usecase.NewUserUsecase(&mockUserRepository{
		getByFirebaseUID: func(ctx context.Context, firebaseUID string) (*domain.User, error) {
			return nil, expectedErr
		},
	})

	_, err := uc.GetPlanByFirebaseUID(context.Background(), "firebase-uid-1")
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

func TestSignUpReturnsExistingUserForExistingFirebaseUID(t *testing.T) {
	existingUser := &domain.User{
		ID:          uuid.New(),
		FirebaseUID: "firebase-uid-1",
		Username:    "alice",
		DisplayName: "Alice",
		Plan:        "free",
	}
	uc := usecase.NewUserUsecase(&mockUserRepository{
		getByFirebaseUID: func(ctx context.Context, firebaseUID string) (*domain.User, error) {
			return existingUser, nil
		},
	})

	user, err := uc.SignUp(context.Background(), domain.CreateUserInput{
		FirebaseUID: "firebase-uid-1",
		Username:    "alice",
		DisplayName: "Alice",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user != existingUser {
		t.Fatal("expected existing user to be returned")
	}
}

func TestSignUpReturnsAlreadyExistsWhenUsernameConflicts(t *testing.T) {
	uc := usecase.NewUserUsecase(&mockUserRepository{
		getByFirebaseUID: func(ctx context.Context, firebaseUID string) (*domain.User, error) {
			return nil, domain.ErrNotFound
		},
		getByUsername: func(ctx context.Context, username string) (*domain.User, error) {
			return &domain.User{ID: uuid.New(), Username: username}, nil
		},
	})

	_, err := uc.SignUp(context.Background(), domain.CreateUserInput{
		FirebaseUID: "firebase-uid-1",
		Username:    "alice",
		DisplayName: "Alice",
	})
	if !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestSignUpCreatesUserWhenNoConflictExists(t *testing.T) {
	calledCreate := false
	createdUser := &domain.User{ID: uuid.New(), Username: "alice"}
	uc := usecase.NewUserUsecase(&mockUserRepository{
		getByFirebaseUID: func(ctx context.Context, firebaseUID string) (*domain.User, error) {
			return nil, domain.ErrNotFound
		},
		getByUsername: func(ctx context.Context, username string) (*domain.User, error) {
			return nil, domain.ErrNotFound
		},
		create: func(ctx context.Context, input domain.CreateUserInput) (*domain.User, error) {
			calledCreate = true
			if input.FirebaseUID != "firebase-uid-1" {
				t.Fatalf("unexpected firebase uid: %s", input.FirebaseUID)
			}
			if input.Username != "alice" {
				t.Fatalf("unexpected username: %s", input.Username)
			}
			return createdUser, nil
		},
	})

	user, err := uc.SignUp(context.Background(), domain.CreateUserInput{
		FirebaseUID: "firebase-uid-1",
		Username:    "alice",
		DisplayName: "Alice",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !calledCreate {
		t.Fatal("expected create to be called")
	}
	if user != createdUser {
		t.Fatal("expected created user to be returned")
	}
}

func TestUpdateProfileAllowsKeepingOwnUsername(t *testing.T) {
	currentUserID := uuid.New()
	updatedUser := &domain.User{ID: currentUserID, Username: "alice"}
	uc := usecase.NewUserUsecase(&mockUserRepository{
		getByUsername: func(ctx context.Context, username string) (*domain.User, error) {
			return &domain.User{ID: currentUserID, Username: username}, nil
		},
		update: func(ctx context.Context, id uuid.UUID, input domain.UpdateUserInput) (*domain.User, error) {
			if id != currentUserID {
				t.Fatalf("unexpected id: %s", id)
			}
			if input.Username != "alice" {
				t.Fatalf("unexpected username: %s", input.Username)
			}
			return updatedUser, nil
		},
	})

	user, err := uc.UpdateProfile(context.Background(), currentUserID, domain.UpdateUserInput{
		Username:    "alice",
		DisplayName: "Alice",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user != updatedUser {
		t.Fatal("expected updated user to be returned")
	}
}

func TestUpdateProfileRejectsOtherUsersUsername(t *testing.T) {
	currentUserID := uuid.New()
	otherUserID := uuid.New()
	uc := usecase.NewUserUsecase(&mockUserRepository{
		getByUsername: func(ctx context.Context, username string) (*domain.User, error) {
			return &domain.User{ID: otherUserID, Username: username}, nil
		},
	})

	_, err := uc.UpdateProfile(context.Background(), currentUserID, domain.UpdateUserInput{
		Username:    "alice",
		DisplayName: "Alice",
	})
	if !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestUpdateQuestionSettingsStoresCount(t *testing.T) {
	currentUserID := uuid.New()
	updatedUser := &domain.User{ID: currentUserID, DefaultQuestionCount: 0}
	uc := usecase.NewUserUsecase(&mockUserRepository{
		updateQuestionSettings: func(ctx context.Context, id uuid.UUID, input domain.UpdateQuestionSettingsInput) (*domain.User, error) {
			if id != currentUserID {
				t.Fatalf("unexpected id: %s", id)
			}
			if input.DefaultQuestionCount != 0 {
				t.Fatalf("unexpected default question count: %d", input.DefaultQuestionCount)
			}
			return updatedUser, nil
		},
	})

	user, err := uc.UpdateQuestionSettings(context.Background(), currentUserID, domain.UpdateQuestionSettingsInput{
		DefaultQuestionCount: 0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user != updatedUser {
		t.Fatal("expected updated user to be returned")
	}
}
