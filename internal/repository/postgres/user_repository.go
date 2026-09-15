package postgres

import (
	"context"
	"errors"
	"fmt"

	"chat-backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type userRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *userRepository {
	return &userRepository{pool: pool}
}

func (r *userRepository) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	query := `SELECT id, username, email, avatar_url, created_at, updated_at FROM users WHERE id = $1`
	var u domain.User
	err := r.pool.QueryRow(ctx, query, id).Scan(&u.ID, &u.Username, &u.Email, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("user GetByID: %w", err)
	}
	return &u, nil
}

func (r *userRepository) GetByLogin(ctx context.Context, login string) (*domain.User, error) {
	query := `SELECT id, username, email, avatar_url, created_at, updated_at FROM users WHERE LOWER(username) = LOWER($1) OR LOWER(email) = LOWER($1)`
	var u domain.User
	err := r.pool.QueryRow(ctx, query, login).Scan(&u.ID, &u.Username, &u.Email, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("user GetByLogin: %w", err)
	}
	return &u, nil
}

func (r *userRepository) Search(ctx context.Context, query string) ([]domain.User, error) {
	sqlQuery := `
		SELECT id, username, email, avatar_url, created_at, updated_at 
		FROM users 
		WHERE username ILIKE $1 OR email ILIKE $1
		ORDER BY username ASC
		LIMIT 50
	`
	pattern := "%" + query + "%"
	rows, err := r.pool.Query(ctx, sqlQuery, pattern)
	if err != nil {
		return nil, fmt.Errorf("user Search: %w", err)
	}
	defer rows.Close()

	users := make([]domain.User, 0)
	for rows.Next() {
		var u domain.User
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("user Search scan: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("user Search rows: %w", err)
	}
	return users, nil
}

func (r *userRepository) GetAll(ctx context.Context) ([]domain.User, error) {
	sqlQuery := `SELECT id, username, email, avatar_url, created_at, updated_at FROM users ORDER BY username ASC`
	rows, err := r.pool.Query(ctx, sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("user GetAll: %w", err)
	}
	defer rows.Close()

	users := make([]domain.User, 0)
	for rows.Next() {
		var u domain.User
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("user GetAll scan: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("user GetAll rows: %w", err)
	}
	return users, nil
}

func (r *userRepository) UpdateAvatar(ctx context.Context, userID int64, avatarURL string) error {
	query := `UPDATE users SET avatar_url = $1, updated_at = NOW() WHERE id = $2`
	res, err := r.pool.Exec(ctx, query, avatarURL, userID)
	if err != nil {
		return fmt.Errorf("user UpdateAvatar: %w", err)
	}
	if res.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
