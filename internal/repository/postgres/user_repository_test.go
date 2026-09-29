package postgres

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TEST_DATABASE_URL должен указывать на отдельную тестовую базу.
func TestUserRegistrationPostgres(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	// Изолируем таблицы, чтобы не менять другие данные даже в тестовой базе.
	schema := "auth_test_" + time.Now().Format("20060102150405000000000")
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, err := pool.Exec(cleanupCtx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Существующая схема до обновления: email обязателен.
	_, err = db.Exec(ctx, `CREATE TABLE users (
		id BIGSERIAL PRIMARY KEY, username VARCHAR(255) NOT NULL UNIQUE,
		email VARCHAR(255) NOT NULL UNIQUE, avatar_url VARCHAR(512) NOT NULL DEFAULT '',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
		INSERT INTO users (username, email) VALUES ('Existing', 'existing@example.com')`)
	if err != nil {
		t.Fatal(err)
	}
	pg := &Postgres{Pool: db}
	for i := 0; i < 2; i++ {
		if err := pg.AutoMigrate(ctx, "../../../migrations/000001_init.up.sql"); err != nil {
			t.Fatal(err)
		}
	}
	repo := NewUserRepository(db)
	existing, err := repo.GetOrCreateByUsername(ctx, "EXISTING")
	if err != nil {
		t.Fatal(err)
	}
	if existing.ID != 1 || existing.Email != "existing@example.com" || existing.Username != "Existing" {
		t.Fatalf("existing profile changed: %+v", existing)
	}
	const count = 20
	ids := make(chan int64, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := "NewUser"
			if i%2 == 0 {
				name = "newuser"
			}
			u, err := repo.GetOrCreateByUsername(ctx, name)
			if err != nil {
				t.Error(err)
				return
			}
			if u.Email != "" {
				t.Errorf("unexpected email: %q", u.Email)
			}
			ids <- u.ID
		}(i)
	}
	wg.Wait()
	close(ids)
	var id int64
	var received int
	for got := range ids {
		received++
		if id != 0 && got != id {
			t.Errorf("duplicate users: %d and %d", id, got)
		}
		id = got
	}
	if received != count {
		t.Fatalf("only %d requests succeeded", received)
	}
	if _, err := repo.GetByID(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Search(ctx, "new"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetAll(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetOrCreateByUsername(ctx, "AnotherUser"); err != nil {
		t.Fatal(err)
	}
	var chatID, messageID int64
	if err := db.QueryRow(ctx, "SELECT id FROM chats WHERE is_global").Scan(&chatID); err != nil {
		t.Fatal(err)
	}
	chats := NewChatRepository(db)
	if err := chats.AddMember(ctx, chatID, id, "member"); err != nil {
		t.Fatal(err)
	}
	if _, err := chats.GetMember(ctx, chatID, id); err != nil {
		t.Fatal(err)
	}
	if _, err := chats.GetMembers(ctx, chatID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, "INSERT INTO messages (chat_id, sender_id, content) VALUES ($1, $2, 'hello') RETURNING id", chatID, id).Scan(&messageID); err != nil {
		t.Fatal(err)
	}
	if _, err := NewMessageRepository(db).GetByID(ctx, messageID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, "INSERT INTO reactions (message_id, user_id, reaction) VALUES ($1, $2, 'like')", messageID, id); err != nil {
		t.Fatal(err)
	}
	if _, err := NewReactionRepository(db).GetByMessageID(ctx, messageID); err != nil {
		t.Fatal(err)
	}
	// Email больше не служит псевдонимом имени при входе.
	byEmail, err := repo.GetOrCreateByUsername(ctx, "existing@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if byEmail.ID == existing.ID {
		t.Fatal("email matched an existing user")
	}
	_, err = db.Exec(ctx, `DROP INDEX idx_users_username_lower;
		INSERT INTO users (username) VALUES ('EXISTING')`)
	if err != nil {
		t.Fatal(err)
	}
	if err := pg.AutoMigrate(ctx, "../../../migrations/000001_init.up.sql"); err == nil || !strings.Contains(err.Error(), "Duplicate usernames") {
		t.Fatalf("expected explicit duplicate error, got %v", err)
	}
}
