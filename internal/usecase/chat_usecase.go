package usecase

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"sync"
	"time"

	"chat-backend/internal/domain"
	"chat-backend/internal/repository"
)

type ChatUseCase interface {
	CreateChat(ctx context.Context, creatorID int64, req domain.CreateChatDTO) (*domain.Chat, error)
	GetChatsForUser(ctx context.Context, userID int64) ([]domain.Chat, error)
	GetChatByID(ctx context.Context, userID, chatID int64) (*domain.Chat, error)
	UpdateChatName(ctx context.Context, userID, chatID int64, name string) error
	DeleteChat(ctx context.Context, userID, chatID int64) error
	LeaveChat(ctx context.Context, userID, chatID int64) error
	MarkAsRead(ctx context.Context, userID, chatID, lastReadMsgID int64) error
	GetMembers(ctx context.Context, userID, chatID int64) ([]domain.ChatMember, error)
	AddMember(ctx context.Context, currentUserID, chatID, targetUserID int64) error
	UpdateMemberRole(ctx context.Context, currentUserID, chatID, targetUserID int64, newRole string) error
	RemoveMember(ctx context.Context, currentUserID, chatID, targetUserID int64) error
	UploadAvatar(ctx context.Context, userID, chatID int64, fileHeader *multipart.FileHeader) (*domain.Chat, error)
	GetAvatarURL(ctx context.Context, userID, chatID int64) (string, error)
	JoinGlobalChat(ctx context.Context, userID int64) (*domain.Chat, error)
}

type chatUseCase struct {
	chatRepo    repository.ChatRepository
	userRepo    repository.UserRepository
	messageRepo repository.MessageRepository
	uploadDir   string

	globalMu sync.Mutex
	globalID int64
}

func NewChatUseCase(
	chatRepo repository.ChatRepository,
	userRepo repository.UserRepository,
	messageRepo repository.MessageRepository,
	uploadDir string,
) ChatUseCase {
	avatarDir := filepath.Join(uploadDir, "avatars")
	if err := os.MkdirAll(avatarDir, os.ModePerm); err != nil {
		fmt.Printf("Warning: failed to create avatars dir: %v\n", err)
	}

	return &chatUseCase{
		chatRepo:    chatRepo,
		userRepo:    userRepo,
		messageRepo: messageRepo,
		uploadDir:   uploadDir,
	}
}

func (c *chatUseCase) globalChatID(ctx context.Context) (int64, error) {
	c.globalMu.Lock()
	defer c.globalMu.Unlock()

	if c.globalID != 0 {
		return c.globalID, nil
	}

	chat, err := c.chatRepo.GetGlobalChat(ctx)
	if err != nil {
		return 0, err
	}
	c.globalID = chat.ID
	return c.globalID, nil
}

func directChatKey(a, b int64) string {
	if a > b {
		a, b = b, a
	}
	return fmt.Sprintf("%d_%d", a, b)
}

func (c *chatUseCase) JoinGlobalChat(ctx context.Context, userID int64) (*domain.Chat, error) {
	chatID, err := c.globalChatID(ctx)
	if err != nil {
		return nil, err
	}
	if err := c.chatRepo.AddMember(ctx, chatID, userID, domain.RoleMember); err != nil {
		return nil, err
	}
	return c.GetChatByID(ctx, userID, chatID)
}

func (c *chatUseCase) CreateChat(ctx context.Context, creatorID int64, req domain.CreateChatDTO) (*domain.Chat, error) {
	if req.Type != domain.ChatTypeDirect && req.Type != domain.ChatTypeGroup {
		return nil, domain.ErrInvalidChatType
	}

	uniqueMemberMap := make(map[int64]bool)
	for _, id := range req.MemberIDs {
		if id != 0 && id != creatorID {
			uniqueMemberMap[id] = true
		}
	}

	uniqueMemberIDs := make([]int64, 0, len(uniqueMemberMap))
	for id := range uniqueMemberMap {
		uniqueMemberIDs = append(uniqueMemberIDs, id)
	}

	for _, targetID := range uniqueMemberIDs {
		if _, err := c.userRepo.GetByID(ctx, targetID); err != nil {
			return nil, fmt.Errorf("%w: user id %d", domain.ErrUserNotFound, targetID)
		}
	}

	var directKey string
	var targetUserID int64

	if req.Type == domain.ChatTypeDirect {
		if len(uniqueMemberIDs) == 0 {
			return nil, fmt.Errorf("%w: direct chat requires target user_id", domain.ErrInvalidInput)
		}
		targetUserID = uniqueMemberIDs[0]
		if targetUserID == creatorID {
			return nil, domain.ErrDirectChatSelf
		}

		existing, err := c.chatRepo.GetDirectChat(ctx, creatorID, targetUserID)
		if err == nil && existing != nil {
			return c.GetChatByID(ctx, creatorID, existing.ID)
		}

		targetUser, err := c.userRepo.GetByID(ctx, targetUserID)
		if err != nil {
			return nil, domain.ErrUserNotFound
		}
		req.Name = targetUser.Username
		directKey = directChatKey(creatorID, targetUserID)
	} else {
		if len(uniqueMemberIDs)+1 < 3 {
			return nil, domain.ErrGroupMinMembers
		}
		if req.Name == "" {
			return nil, fmt.Errorf("%w: group chat requires a name", domain.ErrInvalidInput)
		}
	}

	chat := &domain.Chat{
		Name: req.Name,
		Type: req.Type,
	}

	created, err := c.chatRepo.Create(ctx, chat, creatorID, uniqueMemberIDs, directKey)
	if err != nil {
		// Чат для этой пары уже создан параллельным запросом — возвращаем его.
		if errors.Is(err, domain.ErrAlreadyExists) {
			existing, lookupErr := c.chatRepo.GetDirectChat(ctx, creatorID, targetUserID)
			if lookupErr != nil {
				return nil, err
			}
			return c.GetChatByID(ctx, creatorID, existing.ID)
		}
		return nil, err
	}

	return c.GetChatByID(ctx, creatorID, created.ID)
}

// enrichChat дополняет чат: участники, имя личного чата, последнее сообщение и непрочитанные.
func (c *chatUseCase) enrichChat(ctx context.Context, chat *domain.Chat, userID int64) {
	members, err := c.chatRepo.GetMembers(ctx, chat.ID)
	if err == nil {
		chat.Members = members

		// Личный чат показываем с именем и аватаром собеседника.
		if chat.Type == domain.ChatTypeDirect {
			for _, m := range members {
				if m.UserID != userID && m.User != nil {
					chat.Name = m.User.Username
					chat.AvatarURL = m.User.AvatarURL
					break
				}
			}
		}
	}

	lastMsg, err := c.messageRepo.GetLastMessage(ctx, chat.ID)
	if err == nil && lastMsg != nil {
		revealContent(lastMsg)
		chat.LastMessage = lastMsg
	}

	member, err := c.chatRepo.GetMember(ctx, chat.ID, userID)
	if err == nil {
		unread, err := c.messageRepo.GetUnreadCount(ctx, chat.ID, userID, member.LastReadMessageID)
		if err == nil {
			chat.UnreadCount = unread
		}
	}
}

func (c *chatUseCase) GetChatsForUser(ctx context.Context, userID int64) ([]domain.Chat, error) {
	chats, err := c.chatRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	for i := range chats {
		c.enrichChat(ctx, &chats[i], userID)
	}

	return chats, nil
}

func (c *chatUseCase) GetChatByID(ctx context.Context, userID, chatID int64) (*domain.Chat, error) {
	// Сначала проверяем, что чат существует: иначе вернётся 403 вместо 404.
	chat, err := c.chatRepo.GetByID(ctx, chatID)
	if err != nil {
		return nil, err
	}

	isMember, err := c.chatRepo.IsMember(ctx, chatID, userID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, domain.ErrUserNotInChat
	}

	c.enrichChat(ctx, chat, userID)
	return chat, nil
}

func (c *chatUseCase) UpdateChatName(ctx context.Context, userID, chatID int64, name string) error {
	chat, err := c.chatRepo.GetByID(ctx, chatID)
	if err != nil {
		return err
	}

	if chat.IsGlobal {
		return domain.ErrGlobalChatOp
	}

	if chat.Type == domain.ChatTypeDirect {
		return domain.ErrDirectChatRename
	}

	member, err := c.chatRepo.GetMember(ctx, chatID, userID)
	if err != nil {
		return domain.ErrUserNotInChat
	}

	if member.Role != domain.RoleOwner && member.Role != domain.RoleAdmin {
		return domain.ErrForbidden
	}

	return c.chatRepo.UpdateName(ctx, chatID, name)
}

func (c *chatUseCase) DeleteChat(ctx context.Context, userID, chatID int64) error {
	chat, err := c.chatRepo.GetByID(ctx, chatID)
	if err != nil {
		return err
	}

	if chat.IsGlobal {
		return domain.ErrGlobalChatOp
	}

	member, err := c.chatRepo.GetMember(ctx, chatID, userID)
	if err != nil {
		return domain.ErrUserNotInChat
	}

	if chat.Type == domain.ChatTypeGroup && member.Role != domain.RoleOwner {
		return domain.ErrForbidden
	}

	return c.chatRepo.Delete(ctx, chatID)
}

func (c *chatUseCase) LeaveChat(ctx context.Context, userID, chatID int64) error {
	chat, err := c.chatRepo.GetByID(ctx, chatID)
	if err != nil {
		return err
	}

	if chat.IsGlobal {
		return domain.ErrGlobalChatOp
	}

	if chat.Type == domain.ChatTypeDirect {
		return domain.ErrDirectChatLeave
	}

	isMember, err := c.chatRepo.IsMember(ctx, chatID, userID)
	if err != nil || !isMember {
		return domain.ErrUserNotInChat
	}

	return c.chatRepo.RemoveMember(ctx, chatID, userID)
}

func (c *chatUseCase) MarkAsRead(ctx context.Context, userID, chatID, lastReadMsgID int64) error {
	isMember, err := c.chatRepo.IsMember(ctx, chatID, userID)
	if err != nil || !isMember {
		return domain.ErrUserNotInChat
	}

	return c.chatRepo.UpdateLastRead(ctx, chatID, userID, lastReadMsgID)
}

func (c *chatUseCase) GetMembers(ctx context.Context, userID, chatID int64) ([]domain.ChatMember, error) {
	isMember, err := c.chatRepo.IsMember(ctx, chatID, userID)
	if err != nil || !isMember {
		return nil, domain.ErrUserNotInChat
	}

	return c.chatRepo.GetMembers(ctx, chatID)
}

func (c *chatUseCase) AddMember(ctx context.Context, currentUserID, chatID, targetUserID int64) error {
	chat, err := c.chatRepo.GetByID(ctx, chatID)
	if err != nil {
		return err
	}

	if chat.Type == domain.ChatTypeDirect {
		return domain.ErrDirectChatMemberOp
	}

	currentMember, err := c.chatRepo.GetMember(ctx, chatID, currentUserID)
	if err != nil {
		return domain.ErrUserNotInChat
	}

	if currentMember.Role != domain.RoleOwner && currentMember.Role != domain.RoleAdmin {
		return domain.ErrForbidden
	}

	if _, err := c.userRepo.GetByID(ctx, targetUserID); err != nil {
		return domain.ErrUserNotFound
	}

	return c.chatRepo.AddMember(ctx, chatID, targetUserID, domain.RoleMember)
}

func (c *chatUseCase) UpdateMemberRole(ctx context.Context, currentUserID, chatID, targetUserID int64, newRole string) error {
	chat, err := c.chatRepo.GetByID(ctx, chatID)
	if err != nil {
		return err
	}

	if chat.IsGlobal {
		return domain.ErrGlobalChatOp
	}

	if chat.Type == domain.ChatTypeDirect {
		return domain.ErrDirectChatMemberOp
	}

	if newRole == domain.RoleOwner {
		return domain.ErrCannotSetOwner
	}

	if newRole != domain.RoleAdmin && newRole != domain.RoleMember {
		return fmt.Errorf("%w: role must be 'admin' or 'member'", domain.ErrInvalidInput)
	}

	currentMember, err := c.chatRepo.GetMember(ctx, chatID, currentUserID)
	if err != nil {
		return domain.ErrUserNotInChat
	}

	if currentMember.Role != domain.RoleOwner && currentMember.Role != domain.RoleAdmin {
		return domain.ErrForbidden
	}

	targetMember, err := c.chatRepo.GetMember(ctx, chatID, targetUserID)
	if err != nil {
		return domain.ErrNotFound
	}

	// Админ не может менять роль другого админа — это может только владелец.
	if targetMember.Role == domain.RoleOwner ||
		(currentMember.Role == domain.RoleAdmin && targetMember.Role == domain.RoleAdmin) {
		return domain.ErrForbidden
	}

	return c.chatRepo.UpdateMemberRole(ctx, chatID, targetUserID, newRole)
}

func (c *chatUseCase) RemoveMember(ctx context.Context, currentUserID, chatID, targetUserID int64) error {
	chat, err := c.chatRepo.GetByID(ctx, chatID)
	if err != nil {
		return err
	}

	if chat.IsGlobal {
		return domain.ErrGlobalChatOp
	}

	if chat.Type == domain.ChatTypeDirect {
		return domain.ErrDirectChatMemberOp
	}

	currentMember, err := c.chatRepo.GetMember(ctx, chatID, currentUserID)
	if err != nil {
		return domain.ErrUserNotInChat
	}

	if currentMember.Role != domain.RoleOwner && currentMember.Role != domain.RoleAdmin {
		return domain.ErrForbidden
	}

	targetMember, err := c.chatRepo.GetMember(ctx, chatID, targetUserID)
	if err != nil {
		return domain.ErrNotFound
	}

	if targetMember.Role == domain.RoleOwner || (currentMember.Role == domain.RoleAdmin && targetMember.Role == domain.RoleAdmin) {
		return domain.ErrForbidden
	}

	return c.chatRepo.RemoveMember(ctx, chatID, targetUserID)
}

func (c *chatUseCase) UploadAvatar(ctx context.Context, userID, chatID int64, fileHeader *multipart.FileHeader) (*domain.Chat, error) {
	chat, err := c.chatRepo.GetByID(ctx, chatID)
	if err != nil {
		return nil, err
	}

	if chat.Type == domain.ChatTypeDirect {
		return nil, fmt.Errorf("%w: личный чат автоматически использует аватар собеседника", domain.ErrInvalidInput)
	}

	member, err := c.chatRepo.GetMember(ctx, chatID, userID)
	if err != nil {
		return nil, domain.ErrUserNotInChat
	}

	if member.Role != domain.RoleOwner && member.Role != domain.RoleAdmin {
		return nil, domain.ErrForbidden
	}

	src, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open avatar file: %w", err)
	}
	defer src.Close()

	avatarDir := filepath.Join(c.uploadDir, "avatars")
	if err := os.MkdirAll(avatarDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create avatars directory: %w", err)
	}

	filename := fmt.Sprintf("chat_%d_%d_%s", chatID, time.Now().UnixNano(), safeFileName(fileHeader.Filename))
	savePath := filepath.Join(avatarDir, filename)

	dst, err := os.Create(savePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return nil, fmt.Errorf("failed to save avatar file: %w", err)
	}

	if err := c.chatRepo.UpdateAvatar(ctx, chatID, avatarURL(filename)); err != nil {
		return nil, fmt.Errorf("failed to update chat avatar in DB: %w", err)
	}

	return c.GetChatByID(ctx, userID, chatID)
}

// GetAvatarURL возвращает ссылку на аватарку чата. Для личного чата — аватарку собеседника.
func (c *chatUseCase) GetAvatarURL(ctx context.Context, userID, chatID int64) (string, error) {
	isMember, err := c.chatRepo.IsMember(ctx, chatID, userID)
	if err != nil || !isMember {
		return "", domain.ErrUserNotInChat
	}

	chat, err := c.chatRepo.GetByID(ctx, chatID)
	if err != nil {
		return "", err
	}

	url := chat.AvatarURL
	if chat.Type == domain.ChatTypeDirect {
		members, err := c.chatRepo.GetMembers(ctx, chatID)
		if err != nil {
			return "", err
		}

		url = ""
		for _, m := range members {
			if m.UserID != userID && m.User != nil {
				url = m.User.AvatarURL
				break
			}
		}
	}

	if url == "" {
		return "", domain.ErrNotFound
	}
	return url, nil
}
