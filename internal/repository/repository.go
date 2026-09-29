package repository

import (
	"context"
	"time"

	"chat-backend/internal/domain"
)

type UserRepository interface {
	GetByID(ctx context.Context, id int64) (*domain.User, error)
	GetOrCreateByUsername(ctx context.Context, username string) (*domain.User, error)
	Search(ctx context.Context, query string) ([]domain.User, error)
	GetAll(ctx context.Context) ([]domain.User, error)
	UpdateAvatar(ctx context.Context, userID int64, avatarURL string) error
}

type ChatRepository interface {
	// Create создаёт чат и участников. Для личного чата directKey = "<меньшийID>_<большийID>",
	// чтобы не было дублей. Для группы — пустая строка.
	Create(ctx context.Context, chat *domain.Chat, creatorID int64, memberIDs []int64, directKey string) (*domain.Chat, error)
	GetByID(ctx context.Context, chatID int64) (*domain.Chat, error)
	GetByUserID(ctx context.Context, userID int64) ([]domain.Chat, error)
	GetDirectChat(ctx context.Context, user1ID, user2ID int64) (*domain.Chat, error)
	UpdateName(ctx context.Context, chatID int64, name string) error
	UpdateAvatar(ctx context.Context, chatID int64, avatarURL string) error
	Delete(ctx context.Context, chatID int64) error
	IsMember(ctx context.Context, chatID, userID int64) (bool, error)
	GetMember(ctx context.Context, chatID, userID int64) (*domain.ChatMember, error)
	GetMembers(ctx context.Context, chatID int64) ([]domain.ChatMember, error)
	AddMember(ctx context.Context, chatID, userID int64, role string) error
	UpdateMemberRole(ctx context.Context, chatID, userID int64, role string) error
	RemoveMember(ctx context.Context, chatID, userID int64) error
	UpdateLastRead(ctx context.Context, chatID, userID, lastReadMessageID int64) error
	GetGlobalChat(ctx context.Context) (*domain.Chat, error)
}

type MessageRepository interface {
	Create(ctx context.Context, msg *domain.Message) (*domain.Message, error)
	GetByID(ctx context.Context, msgID int64) (*domain.Message, error)
	GetByChatID(ctx context.Context, chatID int64, limit, offset int) ([]domain.Message, error)
	UpdateContent(ctx context.Context, msgID int64, content string) error
	SoftDelete(ctx context.Context, msgID int64) (string, error)
	GetLastMessage(ctx context.Context, chatID int64) (*domain.Message, error)
	GetUnreadCount(ctx context.Context, chatID, userID, lastReadMessageID int64) (int, error)
	DeleteOlderThan(ctx context.Context, before time.Time) (int64, []string, error)
}

type FileRepository interface {
	Create(ctx context.Context, file *domain.File) (*domain.File, error)
	GetByID(ctx context.Context, fileID int64) (*domain.File, error)
	GetChatID(ctx context.Context, fileID int64) (int64, error)
}

type ReactionRepository interface {
	AddOrUpdate(ctx context.Context, reaction *domain.Reaction) error
	Delete(ctx context.Context, messageID, userID int64, reaction string) error
	GetByMessageID(ctx context.Context, messageID int64) ([]domain.Reaction, error)
}
