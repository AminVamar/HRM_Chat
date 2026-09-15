package usecase

import (
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"

	"chat-backend/internal/domain"
	"chat-backend/internal/repository"
)

type FileUseCase interface {
	UploadFile(ctx context.Context, uploaderID, chatID int64, fileHeader *multipart.FileHeader, caption string) (*domain.Message, error)
	GetFileByID(ctx context.Context, userID, fileID int64) (*domain.File, error)
}

type fileUseCase struct {
	fileRepo       repository.FileRepository
	chatRepo       repository.ChatRepository
	messageUseCase MessageUseCase
	uploadDir      string
}

func NewFileUseCase(
	fileRepo repository.FileRepository,
	chatRepo repository.ChatRepository,
	messageUseCase MessageUseCase,
	uploadDir string,
) FileUseCase {
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		fmt.Printf("Warning: failed to create upload dir: %v\n", err)
	}

	return &fileUseCase{
		fileRepo:       fileRepo,
		chatRepo:       chatRepo,
		messageUseCase: messageUseCase,
		uploadDir:      uploadDir,
	}
}

func (f *fileUseCase) UploadFile(ctx context.Context, uploaderID, chatID int64, fileHeader *multipart.FileHeader, caption string) (*domain.Message, error) {
	isMember, err := f.chatRepo.IsMember(ctx, chatID, uploaderID)
	if err != nil || !isMember {
		return nil, domain.ErrUserNotInChat
	}

	src, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open file header: %w", err)
	}
	defer src.Close()

	uniqueFilename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), filepath.Base(fileHeader.Filename))
	savePath := filepath.Join(f.uploadDir, uniqueFilename)

	dst, err := os.Create(savePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	cleanup := func() {
		if err := os.Remove(savePath); err != nil && !os.IsNotExist(err) {
			log.Printf("Warning: failed to remove orphaned upload %s: %v", savePath, err)
		}
	}

	if _, err := io.Copy(dst, src); err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	fileRecord := &domain.File{
		Filename:   fileHeader.Filename,
		FilePath:   savePath,
		FileSize:   fileHeader.Size,
		MimeType:   fileHeader.Header.Get("Content-Type"),
		UploaderID: uploaderID,
	}

	savedFile, err := f.fileRepo.Create(ctx, fileRecord)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to save file record: %w", err)
	}

	msg, err := f.messageUseCase.SendMessageWithFile(ctx, uploaderID, chatID, savedFile.ID, caption)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("failed to send message with file: %w", err)
	}

	return msg, nil
}

func (f *fileUseCase) GetFileByID(ctx context.Context, userID, fileID int64) (*domain.File, error) {
	fileRecord, err := f.fileRepo.GetByID(ctx, fileID)
	if err != nil {
		return nil, err
	}

	chatID, err := f.fileRepo.GetChatID(ctx, fileID)
	if err != nil {
		return nil, err
	}

	isMember, err := f.chatRepo.IsMember(ctx, chatID, userID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, domain.ErrUserNotInChat
	}

	return fileRecord, nil
}
