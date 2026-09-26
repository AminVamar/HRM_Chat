package postgres

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const uniqueViolation = "23505"

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolation
}

type Postgres struct {
	Pool *pgxpool.Pool
}

func NewPostgres(ctx context.Context, dbURL string) (*Postgres, error) {
	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, fmt.Errorf("unable to parse db config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	log.Println("Successfully connected to PostgreSQL database")

	return &Postgres{Pool: pool}, nil
}

func (p *Postgres) Close() {
	if p.Pool != nil {
		p.Pool.Close()
	}
}

func (p *Postgres) AutoMigrate(ctx context.Context, migrationPath string) error {
	content, err := os.ReadFile(migrationPath)
	if err != nil {
		return fmt.Errorf("could not read migration file: %w", err)
	}

	_, err = p.Pool.Exec(ctx, string(content))
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Println("Database migration applied successfully")
	return nil
}

// FixLegacyAvatarURLs переписывает старые avatar_url вида /api/users/{id}/avatar и /api/chats/{id}/avatar
// на прямую ссылку на файл (/uploads/avatars/<имя файла>). Если файла на диске нет — очищает поле.
func (p *Postgres) FixLegacyAvatarURLs(ctx context.Context, avatarDir, urlPrefix string) error {
	tables := []struct{ table, filePrefix string }{
		{"users", "user_"},
		{"chats", "chat_"},
	}

	for _, t := range tables {
		rows, err := p.Pool.Query(ctx, fmt.Sprintf(`SELECT id FROM %s WHERE avatar_url LIKE '/api/%%'`, t.table))
		if err != nil {
			return fmt.Errorf("select legacy avatars from %s: %w", t.table, err)
		}
		var ids []int64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		rows.Close()

		for _, id := range ids {
			url := ""
			if name := latestFileByPrefix(avatarDir, fmt.Sprintf("%s%d_", t.filePrefix, id)); name != "" {
				url = urlPrefix + name
			}
			query := fmt.Sprintf(`UPDATE %s SET avatar_url = $1 WHERE id = $2`, t.table)
			if _, err := p.Pool.Exec(ctx, query, url, id); err != nil {
				return fmt.Errorf("update avatar in %s: %w", t.table, err)
			}
		}
		if len(ids) > 0 {
			log.Printf("Аватарки: обновлено ссылок в %s: %d", t.table, len(ids))
		}
	}
	return nil
}

func latestFileByPrefix(dir, prefix string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var latest string
	var latestTime int64
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), prefix) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if t := info.ModTime().UnixNano(); t > latestTime {
			latestTime = t
			latest = e.Name()
		}
	}
	return latest
}
