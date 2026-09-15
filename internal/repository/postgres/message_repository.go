package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"chat-backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type messageRepository struct {
	pool *pgxpool.Pool
}

func NewMessageRepository(pool *pgxpool.Pool) *messageRepository {
	return &messageRepository{pool: pool}
}

const messageColumns = `
	m.id, m.chat_id, m.sender_id, m.content, m.file_id, m.is_edited, m.is_deleted, m.created_at, m.updated_at,
	u.id, u.username, u.email, u.avatar_url, u.created_at, u.updated_at,
	f.id, f.filename, f.file_path, f.file_size, f.mime_type, f.uploader_id, f.created_at`

const messageFrom = `
	FROM messages m
	JOIN users u ON m.sender_id = u.id
	LEFT JOIN files f ON m.file_id = f.id`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanMessage(row rowScanner) (*domain.Message, error) {
	var m domain.Message
	var u domain.User
	var fID, fSize, fUploaderID *int64
	var fName, fPath, fMime *string
	var fCreatedAt *time.Time

	err := row.Scan(
		&m.ID, &m.ChatID, &m.SenderID, &m.Content, &m.FileID, &m.IsEdited, &m.IsDeleted, &m.CreatedAt, &m.UpdatedAt,
		&u.ID, &u.Username, &u.Email, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt,
		&fID, &fName, &fPath, &fSize, &fMime, &fUploaderID, &fCreatedAt,
	)
	if err != nil {
		return nil, err
	}

	m.Sender = &u
	if fID != nil {
		fileTime := time.Time{}
		if fCreatedAt != nil {
			fileTime = *fCreatedAt
		}
		m.File = &domain.File{
			ID:         *fID,
			Filename:   *fName,
			FilePath:   *fPath,
			FileSize:   *fSize,
			MimeType:   *fMime,
			UploaderID: *fUploaderID,
			CreatedAt:  fileTime,
		}
	}
	return &m, nil
}

func (r *messageRepository) Create(ctx context.Context, msg *domain.Message) (*domain.Message, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("message Create tx: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO messages (chat_id, sender_id, content, file_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id, is_edited, is_deleted, created_at, updated_at
	`
	err = tx.QueryRow(ctx, query, msg.ChatID, msg.SenderID, msg.Content, msg.FileID).
		Scan(&msg.ID, &msg.IsEdited, &msg.IsDeleted, &msg.CreatedAt, &msg.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("message Create insert: %w", err)
	}

	updateChatQuery := `UPDATE chats SET updated_at = NOW() WHERE id = $1`
	if _, err := tx.Exec(ctx, updateChatQuery, msg.ChatID); err != nil {
		return nil, fmt.Errorf("message Create update chat: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("message Create commit: %w", err)
	}

	return msg, nil
}

func (r *messageRepository) GetByID(ctx context.Context, msgID int64) (*domain.Message, error) {
	query := `SELECT ` + messageColumns + messageFrom + ` WHERE m.id = $1`

	m, err := scanMessage(r.pool.QueryRow(ctx, query, msgID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("message GetByID: %w", err)
	}
	return m, nil
}

func (r *messageRepository) GetByChatID(ctx context.Context, chatID int64, limit, offset int) ([]domain.Message, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	query := `SELECT ` + messageColumns + messageFrom + `
		WHERE m.chat_id = $1
		ORDER BY m.id DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, query, chatID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("message GetByChatID: %w", err)
	}
	defer rows.Close()

	messages := make([]domain.Message, 0)
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, fmt.Errorf("message GetByChatID scan: %w", err)
		}
		messages = append(messages, *m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("message GetByChatID rows: %w", err)
	}

	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

func (r *messageRepository) UpdateContent(ctx context.Context, msgID int64, content string) error {
	query := `UPDATE messages SET content = $1, is_edited = TRUE, updated_at = NOW() WHERE id = $2`
	res, err := r.pool.Exec(ctx, query, content, msgID)
	if err != nil {
		return fmt.Errorf("message UpdateContent: %w", err)
	}
	if res.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *messageRepository) SoftDelete(ctx context.Context, msgID int64) (string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("message SoftDelete tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var fileID *int64
	err = tx.QueryRow(ctx, `SELECT file_id FROM messages WHERE id = $1 FOR UPDATE`, msgID).Scan(&fileID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", domain.ErrNotFound
		}
		return "", fmt.Errorf("message SoftDelete lookup: %w", err)
	}

	query := `UPDATE messages SET content = '', is_deleted = TRUE, file_id = NULL, updated_at = NOW() WHERE id = $1`
	if _, err := tx.Exec(ctx, query, msgID); err != nil {
		return "", fmt.Errorf("message SoftDelete: %w", err)
	}

	var orphanPath string
	if fileID != nil {
		orphan := `
			DELETE FROM files
			WHERE id = $1 AND NOT EXISTS (SELECT 1 FROM messages WHERE file_id = files.id)
			RETURNING file_path
		`
		err := tx.QueryRow(ctx, orphan, *fileID).Scan(&orphanPath)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("message SoftDelete orphan file: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("message SoftDelete commit: %w", err)
	}
	return orphanPath, nil
}

func (r *messageRepository) GetLastMessage(ctx context.Context, chatID int64) (*domain.Message, error) {
	query := `SELECT ` + messageColumns + messageFrom + `
		WHERE m.chat_id = $1
		ORDER BY m.id DESC
		LIMIT 1`

	m, err := scanMessage(r.pool.QueryRow(ctx, query, chatID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // сообщений ещё нет
		}
		return nil, fmt.Errorf("message GetLastMessage: %w", err)
	}
	return m, nil
}

func (r *messageRepository) GetUnreadCount(ctx context.Context, chatID, userID, lastReadMessageID int64) (int, error) {
	query := `
		SELECT COUNT(*) FROM messages
		WHERE chat_id = $1 AND id > $2 AND sender_id <> $3 AND is_deleted = FALSE
	`
	var count int
	err := r.pool.QueryRow(ctx, query, chatID, lastReadMessageID, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("message GetUnreadCount: %w", err)
	}
	return count, nil
}

func (r *messageRepository) DeleteOlderThan(ctx context.Context, before time.Time) (int64, []string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, nil, fmt.Errorf("message DeleteOlderThan tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Запоминаем файлы старых сообщений до их удаления.
	fileIDs := make([]int64, 0)
	rows, err := tx.Query(ctx, `SELECT DISTINCT file_id FROM messages WHERE created_at < $1 AND file_id IS NOT NULL`, before)
	if err != nil {
		return 0, nil, fmt.Errorf("message DeleteOlderThan collect files: %w", err)
	}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, nil, fmt.Errorf("message DeleteOlderThan scan file id: %w", err)
		}
		fileIDs = append(fileIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, nil, fmt.Errorf("message DeleteOlderThan collect files rows: %w", err)
	}

	res, err := tx.Exec(ctx, `DELETE FROM messages WHERE created_at < $1`, before)
	if err != nil {
		return 0, nil, fmt.Errorf("message DeleteOlderThan: %w", err)
	}
	deleted := res.RowsAffected()

	// Удаляем только те файлы, на которые больше не ссылается ни одно сообщение.
	paths := make([]string, 0, len(fileIDs))
	if len(fileIDs) > 0 {
		orphans := `
			DELETE FROM files
			WHERE id = ANY($1) AND NOT EXISTS (SELECT 1 FROM messages WHERE file_id = files.id)
			RETURNING file_path
		`
		pathRows, err := tx.Query(ctx, orphans, fileIDs)
		if err != nil {
			return 0, nil, fmt.Errorf("message DeleteOlderThan orphan files: %w", err)
		}
		for pathRows.Next() {
			var p string
			if err := pathRows.Scan(&p); err != nil {
				pathRows.Close()
				return 0, nil, fmt.Errorf("message DeleteOlderThan scan path: %w", err)
			}
			paths = append(paths, p)
		}
		pathRows.Close()
		if err := pathRows.Err(); err != nil {
			return 0, nil, fmt.Errorf("message DeleteOlderThan orphan rows: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, nil, fmt.Errorf("message DeleteOlderThan commit: %w", err)
	}
	return deleted, paths, nil
}
