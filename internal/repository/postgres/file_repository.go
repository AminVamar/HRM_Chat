package postgres

import (
	"context"
	"errors"
	"fmt"

	"chat-backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type fileRepository struct {
	pool *pgxpool.Pool
}

func NewFileRepository(pool *pgxpool.Pool) *fileRepository {
	return &fileRepository{pool: pool}
}

func (r *fileRepository) Create(ctx context.Context, file *domain.File) (*domain.File, error) {
	query := `
		INSERT INTO files (filename, file_path, file_size, mime_type, uploader_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`
	err := r.pool.QueryRow(ctx, query, file.Filename, file.FilePath, file.FileSize, file.MimeType, file.UploaderID).
		Scan(&file.ID, &file.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("file Create: %w", err)
	}
	return file, nil
}

func (r *fileRepository) GetByID(ctx context.Context, fileID int64) (*domain.File, error) {
	query := `
		SELECT id, filename, file_path, file_size, mime_type, uploader_id, created_at
		FROM files
		WHERE id = $1
	`
	var f domain.File
	err := r.pool.QueryRow(ctx, query, fileID).Scan(
		&f.ID, &f.Filename, &f.FilePath, &f.FileSize, &f.MimeType, &f.UploaderID, &f.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("file GetByID: %w", err)
	}
	return &f, nil
}

func (r *fileRepository) GetChatID(ctx context.Context, fileID int64) (int64, error) {
	query := `SELECT chat_id FROM messages WHERE file_id = $1 LIMIT 1`
	var chatID int64
	err := r.pool.QueryRow(ctx, query, fileID).Scan(&chatID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, domain.ErrNotFound
		}
		return 0, fmt.Errorf("file GetChatID: %w", err)
	}
	return chatID, nil
}
