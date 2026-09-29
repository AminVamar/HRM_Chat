package postgres

import (
	"context"
	"errors"
	"fmt"

	"chat-backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type chatRepository struct {
	pool *pgxpool.Pool
}

func NewChatRepository(pool *pgxpool.Pool) *chatRepository {
	return &chatRepository{pool: pool}
}

func (r *chatRepository) Create(ctx context.Context, chat *domain.Chat, creatorID int64, memberIDs []int64, directKey string) (*domain.Chat, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("chat Create begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var key *string
	if directKey != "" {
		key = &directKey
	}

	query := `
		INSERT INTO chats (name, type, avatar_url, direct_key)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at
	`
	err = tx.QueryRow(ctx, query, chat.Name, chat.Type, chat.AvatarURL, key).Scan(&chat.ID, &chat.CreatedAt, &chat.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, domain.ErrAlreadyExists
		}
		return nil, fmt.Errorf("chat Create insert: %w", err)
	}

	membersMap := make(map[int64]string)
	membersMap[creatorID] = domain.RoleOwner

	for _, mID := range memberIDs {
		if mID != 0 && mID != creatorID {
			if _, exists := membersMap[mID]; !exists {
				membersMap[mID] = domain.RoleMember
			}
		}
	}

	for mID, role := range membersMap {
		memberQuery := `
			INSERT INTO chat_members (chat_id, user_id, role)
			VALUES ($1, $2, $3)
		`
		_, err := tx.Exec(ctx, memberQuery, chat.ID, mID, role)
		if err != nil {
			return nil, fmt.Errorf("chat Create add member %d: %w", mID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("chat Create commit: %w", err)
	}

	return chat, nil
}

const chatColumns = `c.id, c.name, c.type, c.avatar_url, c.is_global, c.created_at, c.updated_at`

func scanChat(row rowScanner) (*domain.Chat, error) {
	var c domain.Chat
	err := row.Scan(&c.ID, &c.Name, &c.Type, &c.AvatarURL, &c.IsGlobal, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *chatRepository) GetByID(ctx context.Context, chatID int64) (*domain.Chat, error) {
	query := `SELECT ` + chatColumns + ` FROM chats c WHERE c.id = $1`
	c, err := scanChat(r.pool.QueryRow(ctx, query, chatID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("chat GetByID: %w", err)
	}
	return c, nil
}

func (r *chatRepository) GetByUserID(ctx context.Context, userID int64) ([]domain.Chat, error) {
	query := `
		SELECT ` + chatColumns + `
		FROM chats c
		JOIN chat_members cm ON c.id = cm.chat_id
		WHERE cm.user_id = $1
		ORDER BY c.updated_at DESC
	`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("chat GetByUserID: %w", err)
	}
	defer rows.Close()

	chats := make([]domain.Chat, 0)
	for rows.Next() {
		c, err := scanChat(rows)
		if err != nil {
			return nil, fmt.Errorf("chat GetByUserID scan: %w", err)
		}
		chats = append(chats, *c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chat GetByUserID rows: %w", err)
	}
	return chats, nil
}

func (r *chatRepository) GetDirectChat(ctx context.Context, user1ID, user2ID int64) (*domain.Chat, error) {
	query := `
		SELECT ` + chatColumns + `
		FROM chats c
		JOIN chat_members cm1 ON c.id = cm1.chat_id AND cm1.user_id = $1
		JOIN chat_members cm2 ON c.id = cm2.chat_id AND cm2.user_id = $2
		WHERE c.type = $3
		LIMIT 1
	`
	c, err := scanChat(r.pool.QueryRow(ctx, query, user1ID, user2ID, domain.ChatTypeDirect))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("chat GetDirectChat: %w", err)
	}
	return c, nil
}

func (r *chatRepository) UpdateName(ctx context.Context, chatID int64, name string) error {
	query := `UPDATE chats SET name = $1, updated_at = NOW() WHERE id = $2`
	res, err := r.pool.Exec(ctx, query, name, chatID)
	if err != nil {
		return fmt.Errorf("chat UpdateName: %w", err)
	}
	if res.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *chatRepository) UpdateAvatar(ctx context.Context, chatID int64, avatarURL string) error {
	query := `UPDATE chats SET avatar_url = $1, updated_at = NOW() WHERE id = $2`
	res, err := r.pool.Exec(ctx, query, avatarURL, chatID)
	if err != nil {
		return fmt.Errorf("chat UpdateAvatar: %w", err)
	}
	if res.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *chatRepository) Delete(ctx context.Context, chatID int64) error {
	query := `DELETE FROM chats WHERE id = $1`
	res, err := r.pool.Exec(ctx, query, chatID)
	if err != nil {
		return fmt.Errorf("chat Delete: %w", err)
	}
	if res.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *chatRepository) IsMember(ctx context.Context, chatID, userID int64) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM chat_members WHERE chat_id = $1 AND user_id = $2)`
	var exists bool
	err := r.pool.QueryRow(ctx, query, chatID, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("chat IsMember: %w", err)
	}
	return exists, nil
}

const memberColumns = `
	cm.chat_id, cm.user_id, cm.role, cm.joined_at, cm.last_read_message_id,
	u.id, u.username, COALESCE(u.email, ''), u.avatar_url, u.created_at, u.updated_at`

func scanMember(row rowScanner) (*domain.ChatMember, error) {
	var cm domain.ChatMember
	var u domain.User
	err := row.Scan(
		&cm.ChatID, &cm.UserID, &cm.Role, &cm.JoinedAt, &cm.LastReadMessageID,
		&u.ID, &u.Username, &u.Email, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	cm.User = &u
	return &cm, nil
}

func (r *chatRepository) GetMember(ctx context.Context, chatID, userID int64) (*domain.ChatMember, error) {
	query := `
		SELECT ` + memberColumns + `
		FROM chat_members cm
		JOIN users u ON cm.user_id = u.id
		WHERE cm.chat_id = $1 AND cm.user_id = $2
	`
	cm, err := scanMember(r.pool.QueryRow(ctx, query, chatID, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("chat GetMember: %w", err)
	}
	return cm, nil
}

func (r *chatRepository) GetMembers(ctx context.Context, chatID int64) ([]domain.ChatMember, error) {
	query := `
		SELECT ` + memberColumns + `
		FROM chat_members cm
		JOIN users u ON cm.user_id = u.id
		WHERE cm.chat_id = $1
		ORDER BY cm.joined_at ASC
	`
	rows, err := r.pool.Query(ctx, query, chatID)
	if err != nil {
		return nil, fmt.Errorf("chat GetMembers: %w", err)
	}
	defer rows.Close()

	members := make([]domain.ChatMember, 0)
	for rows.Next() {
		cm, err := scanMember(rows)
		if err != nil {
			return nil, fmt.Errorf("chat GetMembers scan: %w", err)
		}
		members = append(members, *cm)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("chat GetMembers rows: %w", err)
	}
	return members, nil
}

func (r *chatRepository) AddMember(ctx context.Context, chatID, userID int64, role string) error {
	query := `
		INSERT INTO chat_members (chat_id, user_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (chat_id, user_id) DO NOTHING
	`
	_, err := r.pool.Exec(ctx, query, chatID, userID, role)
	if err != nil {
		return fmt.Errorf("chat AddMember: %w", err)
	}
	return nil
}

func (r *chatRepository) UpdateMemberRole(ctx context.Context, chatID, userID int64, role string) error {
	query := `UPDATE chat_members SET role = $1 WHERE chat_id = $2 AND user_id = $3`
	res, err := r.pool.Exec(ctx, query, role, chatID, userID)
	if err != nil {
		return fmt.Errorf("chat UpdateMemberRole: %w", err)
	}
	if res.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *chatRepository) RemoveMember(ctx context.Context, chatID, userID int64) error {
	query := `DELETE FROM chat_members WHERE chat_id = $1 AND user_id = $2`
	res, err := r.pool.Exec(ctx, query, chatID, userID)
	if err != nil {
		return fmt.Errorf("chat RemoveMember: %w", err)
	}
	if res.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *chatRepository) UpdateLastRead(ctx context.Context, chatID, userID, lastReadMessageID int64) error {
	query := `UPDATE chat_members SET last_read_message_id = $1 WHERE chat_id = $2 AND user_id = $3`
	_, err := r.pool.Exec(ctx, query, lastReadMessageID, chatID, userID)
	if err != nil {
		return fmt.Errorf("chat UpdateLastRead: %w", err)
	}
	return nil
}

func (r *chatRepository) GetGlobalChat(ctx context.Context) (*domain.Chat, error) {
	query := `SELECT ` + chatColumns + ` FROM chats c WHERE c.is_global = true LIMIT 1`
	c, err := scanChat(r.pool.QueryRow(ctx, query))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("chat GetGlobalChat: %w", err)
	}
	return c, nil
}
