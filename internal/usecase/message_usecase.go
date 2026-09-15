package usecase

import (
	"context"
	"fmt"
	"log"
	"os"

	"chat-backend/internal/domain"
	"chat-backend/internal/repository"
	"chat-backend/pkg/crypto"
)

const maxMessagePageSize = 100

type MessageUseCase interface {
	SendMessage(ctx context.Context, senderID int64, req domain.CreateMessageDTO) (*domain.Message, error)
	SendMessageWithFile(ctx context.Context, senderID, chatID, fileID int64, content string) (*domain.Message, error)
	GetMessages(ctx context.Context, userID, chatID int64, limit, offset int) ([]domain.Message, error)
	UpdateMessage(ctx context.Context, userID, msgID int64, req domain.UpdateMessageDTO) (*domain.Message, error)
	DeleteMessage(ctx context.Context, userID, msgID int64) (chatID int64, err error)
	AddReaction(ctx context.Context, userID, msgID int64, reactionStr string) (*domain.Reaction, int64, error)
	RemoveReaction(ctx context.Context, userID, msgID int64, reactionStr string) (chatID int64, err error)
	GetReactions(ctx context.Context, userID, msgID int64) ([]domain.ReactionGroup, error)
}

type messageUseCase struct {
	messageRepo  repository.MessageRepository
	chatRepo     repository.ChatRepository
	reactionRepo repository.ReactionRepository
}

func NewMessageUseCase(
	messageRepo repository.MessageRepository,
	chatRepo repository.ChatRepository,
	reactionRepo repository.ReactionRepository,
) MessageUseCase {
	return &messageUseCase{
		messageRepo:  messageRepo,
		chatRepo:     chatRepo,
		reactionRepo: reactionRepo,
	}
}

func (m *messageUseCase) SendMessage(ctx context.Context, senderID int64, req domain.CreateMessageDTO) (*domain.Message, error) {
	isMember, err := m.chatRepo.IsMember(ctx, req.ChatID, senderID)
	if err != nil || !isMember {
		return nil, domain.ErrUserNotInChat
	}

	encryptedContent, err := crypto.Encrypt(req.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt message: %w", err)
	}

	msg := &domain.Message{
		ChatID:   req.ChatID,
		SenderID: senderID,
		Content:  encryptedContent,
	}

	created, err := m.messageRepo.Create(ctx, msg)
	if err != nil {
		return nil, err
	}

	fullMsg, err := m.messageRepo.GetByID(ctx, created.ID)
	if err != nil {
		return created, nil
	}

	fullMsg.Content = req.Content 
	return fullMsg, nil
}

func (m *messageUseCase) SendMessageWithFile(ctx context.Context, senderID, chatID, fileID int64, content string) (*domain.Message, error) {
	isMember, err := m.chatRepo.IsMember(ctx, chatID, senderID)
	if err != nil || !isMember {
		return nil, domain.ErrUserNotInChat
	}

	encryptedContent, err := crypto.Encrypt(content)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt caption: %w", err)
	}

	msg := &domain.Message{
		ChatID:   chatID,
		SenderID: senderID,
		Content:  encryptedContent,
		FileID:   &fileID,
	}

	created, err := m.messageRepo.Create(ctx, msg)
	if err != nil {
		return nil, err
	}

	fullMsg, err := m.messageRepo.GetByID(ctx, created.ID)
	if err != nil {
		return created, nil
	}

	fullMsg.Content = content
	return fullMsg, nil
}

func (m *messageUseCase) GetMessages(ctx context.Context, userID, chatID int64, limit, offset int) ([]domain.Message, error) {
	isMember, err := m.chatRepo.IsMember(ctx, chatID, userID)
	if err != nil || !isMember {
		return nil, domain.ErrUserNotInChat
	}

	if limit > maxMessagePageSize {
		limit = maxMessagePageSize
	}

	messages, err := m.messageRepo.GetByChatID(ctx, chatID, limit, offset)
	if err != nil {
		return nil, err
	}

	for i := range messages {
		revealContent(&messages[i])

		reactions, err := m.reactionRepo.GetByMessageID(ctx, messages[i].ID)
		if err == nil {
			messages[i].Reactions = reactions
		}
	}

	return messages, nil
}

func (m *messageUseCase) UpdateMessage(ctx context.Context, userID, msgID int64, req domain.UpdateMessageDTO) (*domain.Message, error) {
	msg, err := m.messageRepo.GetByID(ctx, msgID)
	if err != nil {
		return nil, err
	}

	if msg.SenderID != userID {
		return nil, domain.ErrForbidden
	}

	if msg.IsDeleted {
		return nil, domain.ErrMessageDeleted
	}

	encryptedContent, err := crypto.Encrypt(req.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt message: %w", err)
	}

	if err := m.messageRepo.UpdateContent(ctx, msgID, encryptedContent); err != nil {
		return nil, err
	}

	updated, err := m.messageRepo.GetByID(ctx, msgID)
	if err != nil {
		return nil, err
	}

	updated.Content = req.Content
	return updated, nil
}

func (m *messageUseCase) DeleteMessage(ctx context.Context, userID, msgID int64) (int64, error) {
	msg, err := m.messageRepo.GetByID(ctx, msgID)
	if err != nil {
		return 0, err
	}

	if msg.SenderID != userID {
		return 0, domain.ErrForbidden
	}

	orphanPath, err := m.messageRepo.SoftDelete(ctx, msgID)
	if err != nil {
		return 0, err
	}

	if orphanPath != "" {
		if err := os.Remove(orphanPath); err != nil && !os.IsNotExist(err) {
			log.Printf("Warning: failed to remove file of deleted message %d (%s): %v", msgID, orphanPath, err)
		}
	}

	return msg.ChatID, nil
}

func (m *messageUseCase) AddReaction(ctx context.Context, userID, msgID int64, reactionStr string) (*domain.Reaction, int64, error) {
	msg, err := m.messageRepo.GetByID(ctx, msgID)
	if err != nil {
		return nil, 0, err
	}

	isMember, err := m.chatRepo.IsMember(ctx, msg.ChatID, userID)
	if err != nil || !isMember {
		return nil, 0, domain.ErrUserNotInChat
	}

	reaction := &domain.Reaction{
		MessageID: msgID,
		UserID:    userID,
		Reaction:  reactionStr,
	}

	if err := m.reactionRepo.AddOrUpdate(ctx, reaction); err != nil {
		return nil, 0, err
	}

	return reaction, msg.ChatID, nil
}

func (m *messageUseCase) RemoveReaction(ctx context.Context, userID, msgID int64, reactionStr string) (int64, error) {
	msg, err := m.messageRepo.GetByID(ctx, msgID)
	if err != nil {
		return 0, err
	}

	isMember, err := m.chatRepo.IsMember(ctx, msg.ChatID, userID)
	if err != nil || !isMember {
		return 0, domain.ErrUserNotInChat
	}

	if err := m.reactionRepo.Delete(ctx, msgID, userID, reactionStr); err != nil {
		return 0, err
	}

	return msg.ChatID, nil
}

func (m *messageUseCase) GetReactions(ctx context.Context, userID, msgID int64) ([]domain.ReactionGroup, error) {
	msg, err := m.messageRepo.GetByID(ctx, msgID)
	if err != nil {
		return nil, err
	}

	isMember, err := m.chatRepo.IsMember(ctx, msg.ChatID, userID)
	if err != nil || !isMember {
		return nil, domain.ErrUserNotInChat
	}

	reactions, err := m.reactionRepo.GetByMessageID(ctx, msgID)
	if err != nil {
		return nil, err
	}

	groupsMap := make(map[string]*domain.ReactionGroup)
	for _, r := range reactions {
		if g, ok := groupsMap[r.Reaction]; ok {
			g.Count++
			g.UserIDs = append(g.UserIDs, r.UserID)
			if r.User != nil {
				g.Users = append(g.Users, *r.User)
			}
		} else {
			usersList := make([]domain.User, 0)
			if r.User != nil {
				usersList = append(usersList, *r.User)
			}
			groupsMap[r.Reaction] = &domain.ReactionGroup{
				Reaction: r.Reaction,
				Count:    1,
				UserIDs:  []int64{r.UserID},
				Users:    usersList,
			}
		}
	}

	result := make([]domain.ReactionGroup, 0, len(groupsMap))
	for _, g := range groupsMap {
		result = append(result, *g)
	}

	return result, nil
}
