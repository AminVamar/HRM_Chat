package postgres

import (
	"context"
	"fmt"

	"chat-backend/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type reactionRepository struct {
	pool *pgxpool.Pool
}

func NewReactionRepository(pool *pgxpool.Pool) *reactionRepository {
	return &reactionRepository{pool: pool}
}

func (r *reactionRepository) AddOrUpdate(ctx context.Context, reaction *domain.Reaction) error {
	query := `
		INSERT INTO reactions (message_id, user_id, reaction)
		VALUES ($1, $2, $3)
		ON CONFLICT (message_id, user_id) 
		DO UPDATE SET reaction = EXCLUDED.reaction, created_at = NOW()
		RETURNING id, created_at
	`
	err := r.pool.QueryRow(ctx, query, reaction.MessageID, reaction.UserID, reaction.Reaction).
		Scan(&reaction.ID, &reaction.CreatedAt)
	if err != nil {
		return fmt.Errorf("reaction AddOrUpdate: %w", err)
	}
	return nil
}

func (r *reactionRepository) Delete(ctx context.Context, messageID, userID int64, reactionStr string) error {
	var query string
	var args []interface{}

	if reactionStr != "" {
		query = `DELETE FROM reactions WHERE message_id = $1 AND user_id = $2 AND reaction = $3`
		args = []interface{}{messageID, userID, reactionStr}
	} else {
		query = `DELETE FROM reactions WHERE message_id = $1 AND user_id = $2`
		args = []interface{}{messageID, userID}
	}

	res, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("reaction Delete: %w", err)
	}
	if res.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *reactionRepository) GetByMessageID(ctx context.Context, messageID int64) ([]domain.Reaction, error) {
	query := `
		SELECT r.id, r.message_id, r.user_id, r.reaction, r.created_at,
		       u.id, u.username, COALESCE(u.email, ''), u.avatar_url, u.created_at, u.updated_at
		FROM reactions r
		JOIN users u ON r.user_id = u.id
		WHERE r.message_id = $1
		ORDER BY r.created_at ASC
	`
	rows, err := r.pool.Query(ctx, query, messageID)
	if err != nil {
		return nil, fmt.Errorf("reaction GetByMessageID: %w", err)
	}
	defer rows.Close()

	reactions := make([]domain.Reaction, 0)
	for rows.Next() {
		var react domain.Reaction
		var u domain.User
		err := rows.Scan(
			&react.ID, &react.MessageID, &react.UserID, &react.Reaction, &react.CreatedAt,
			&u.ID, &u.Username, &u.Email, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("reaction GetByMessageID scan: %w", err)
		}
		react.User = &u
		reactions = append(reactions, react)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reaction GetByMessageID rows: %w", err)
	}
	return reactions, nil
}
