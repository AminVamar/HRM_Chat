package usecase

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chat-backend/internal/domain"
	"chat-backend/internal/repository"
)

type UserUseCase interface {
	GetByLogin(ctx context.Context, login string) (*domain.User, error)
	GetByID(ctx context.Context, id int64) (*domain.User, error)
	Search(ctx context.Context, query string) ([]domain.User, error)
	GetAll(ctx context.Context) ([]domain.User, error)
	UploadAvatar(ctx context.Context, userID int64, fileHeader *multipart.FileHeader) (*domain.User, error)
	GetAvatarURL(ctx context.Context, userID int64) (string, error)
}

type userUseCase struct {
	userRepo  repository.UserRepository
	uploadDir string
}

func NewUserUseCase(userRepo repository.UserRepository, uploadDir string) UserUseCase {
	avatarDir := filepath.Join(uploadDir, "avatars")
	if err := os.MkdirAll(avatarDir, os.ModePerm); err != nil {
		fmt.Printf("Warning: failed to create avatars dir: %v\n", err)
	}

	return &userUseCase{
		userRepo:  userRepo,
		uploadDir: uploadDir,
	}
}

func (u *userUseCase) GetByLogin(ctx context.Context, login string) (*domain.User, error) {
	login = strings.TrimSpace(login)
	if login == "" {
		return nil, domain.ErrUnauthorized
	}

	user, err := u.userRepo.GetByLogin(ctx, login)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrUnauthorized
		}
		return nil, err
	}
	if user == nil {
		return nil, domain.ErrUnauthorized
	}

	return user, nil
}

func (u *userUseCase) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	return u.userRepo.GetByID(ctx, id)
}

func (u *userUseCase) Search(ctx context.Context, query string) ([]domain.User, error) {
	return u.userRepo.Search(ctx, query)
}

func (u *userUseCase) GetAll(ctx context.Context) ([]domain.User, error) {
	return u.userRepo.GetAll(ctx)
}

func (u *userUseCase) UploadAvatar(ctx context.Context, userID int64, fileHeader *multipart.FileHeader) (*domain.User, error) {
	user, err := u.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	src, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open avatar file: %w", err)
	}
	defer src.Close()

	avatarDir := filepath.Join(u.uploadDir, "avatars")
	if err := os.MkdirAll(avatarDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create avatars directory: %w", err)
	}

	filename := fmt.Sprintf("user_%d_%d_%s", userID, time.Now().UnixNano(), safeFileName(fileHeader.Filename))
	savePath := filepath.Join(avatarDir, filename)

	dst, err := os.Create(savePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return nil, fmt.Errorf("failed to save avatar file: %w", err)
	}

	url := avatarURL(filename)
	if err := u.userRepo.UpdateAvatar(ctx, userID, url); err != nil {
		return nil, fmt.Errorf("failed to update user avatar in DB: %w", err)
	}

	user.AvatarURL = url
	return user, nil
}

// GetAvatarURL возвращает ссылку на аватарку пользователя.
func (u *userUseCase) GetAvatarURL(ctx context.Context, userID int64) (string, error) {
	user, err := u.userRepo.GetByID(ctx, userID)
	if err != nil {
		return "", err
	}
	if user.AvatarURL == "" {
		return "", domain.ErrNotFound
	}
	return user.AvatarURL, nil
}
