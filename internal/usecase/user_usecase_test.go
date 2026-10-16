package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"chat-backend/internal/domain"
	"chat-backend/internal/repository"
)

type authUserRepo struct {
	repository.UserRepository
	username string
	user     *domain.User
	err      error
}

func (r *authUserRepo) GetOrCreateByUsername(_ context.Context, username string) (*domain.User, error) {
	r.username = username
	return r.user, r.err
}

func TestGetOrCreateByUsername(t *testing.T) {
	dbErr := errors.New("database unavailable")
	for _, tc := range []struct {
		name     string
		input    string
		repoErr  error
		wantErr  error
		wantName string
	}{
		{name: "trim", input: "  Amin  ", wantName: "Amin"},
		{name: "empty", input: " \t", wantErr: domain.ErrUnauthorized},
		{name: "too long", input: strings.Repeat("я", 256), wantErr: domain.ErrInvalidInput},
		{name: "unicode limit", input: strings.Repeat("я", 255), wantName: strings.Repeat("я", 255)},
		{name: "null byte", input: "a\x00b", wantErr: domain.ErrInvalidInput},
		{name: "invalid utf8", input: "\xff", wantErr: domain.ErrInvalidInput},
		{name: "database error", input: "amin", repoErr: dbErr, wantErr: dbErr, wantName: "amin"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &authUserRepo{user: &domain.User{ID: 42}, err: tc.repoErr}
			uc := &userUseCase{userRepo: repo}
			user, err := uc.GetOrCreateByUsername(context.Background(), tc.input)
			if !errors.Is(err, tc.wantErr) || repo.username != tc.wantName {
				t.Fatalf("user=%v err=%v lookup=%q", user, err, repo.username)
			}
			if err == nil && user.ID != 42 {
				t.Fatalf("unexpected user: %v", user)
			}
		})
	}
}

type profileUserRepo struct {
	repository.UserRepository
	firstName, lastName string
	calls               int
}

func (r *profileUserRepo) UpdateProfile(_ context.Context, userID int64, firstName, lastName string) (*domain.User, error) {
	r.calls++
	r.firstName, r.lastName = firstName, lastName
	return &domain.User{ID: userID, FirstName: firstName, LastName: lastName}, nil
}

func TestUpdateProfile(t *testing.T) {
	for _, tc := range []struct {
		name      string
		dto       domain.UpdateProfileDTO
		wantErr   error
		wantFirst string
		wantLast  string
	}{
		{name: "trim", dto: domain.UpdateProfileDTO{FirstName: "  Иван ", LastName: " Петров  "}, wantFirst: "Иван", wantLast: "Петров"},
		{name: "empty first", dto: domain.UpdateProfileDTO{FirstName: " ", LastName: "Петров"}, wantErr: domain.ErrInvalidInput},
		{name: "empty last", dto: domain.UpdateProfileDTO{FirstName: "Иван"}, wantErr: domain.ErrInvalidInput},
		{name: "too long", dto: domain.UpdateProfileDTO{FirstName: strings.Repeat("я", 256), LastName: "Петров"}, wantErr: domain.ErrInvalidInput},
		{name: "null byte", dto: domain.UpdateProfileDTO{FirstName: "Иван", LastName: "a\x00b"}, wantErr: domain.ErrInvalidInput},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &profileUserRepo{}
			uc := &userUseCase{userRepo: repo}
			user, err := uc.UpdateProfile(context.Background(), 5, tc.dto)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("user=%v err=%v", user, err)
			}
			if tc.wantErr != nil {
				if repo.calls != 0 {
					t.Fatal("repository must not be called for invalid input")
				}
				return
			}
			if repo.firstName != tc.wantFirst || repo.lastName != tc.wantLast || user.ID != 5 {
				t.Fatalf("saved %q %q, user %v", repo.firstName, repo.lastName, user)
			}
		})
	}
}
