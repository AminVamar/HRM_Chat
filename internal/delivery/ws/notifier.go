package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"chat-backend/internal/domain"
	"chat-backend/internal/repository"
)

type Notifier struct {
	hub      *Hub
	chatRepo repository.ChatRepository
}

func NewNotifier(hub *Hub, chatRepo repository.ChatRepository) *Notifier {
	return &Notifier{hub: hub, chatRepo: chatRepo}
}

func (n *Notifier) memberIDs(ctx context.Context, chatID int64) ([]int64, error) {
	members, err := n.chatRepo.GetMembers(ctx, chatID)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve members of chat %d: %w", chatID, err)
	}

	ids := make([]int64, len(members))
	for i, m := range members {
		ids[i] = m.UserID
	}
	return ids, nil
}

func (n *Notifier) send(ctx context.Context, chatID int64, eventType string, payload any) error {
	ids, err := n.memberIDs(ctx, chatID)
	if err != nil {
		return err
	}
	return n.sendTo(ids, eventType, payload)
}

func (n *Notifier) sendTo(userIDs []int64, eventType string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("cannot marshal %s payload: %w", eventType, err)
	}

	n.hub.BroadcastToUsers(userIDs, WSEvent{Type: eventType, Payload: data})
	return nil
}

func (n *Notifier) NewMessage(ctx context.Context, msg *domain.Message) error {
	return n.send(ctx, msg.ChatID, EventNewMessage, msg)
}

func (n *Notifier) MessageUpdated(ctx context.Context, msg *domain.Message) error {
	return n.send(ctx, msg.ChatID, EventMessageUpdated, msg)
}

func (n *Notifier) MessageDeleted(ctx context.Context, chatID, msgID int64) error {
	return n.send(ctx, chatID, EventMessageDeleted, MessageDeletedPayload{
		ChatID:    chatID,
		MessageID: msgID,
	})
}

func (n *Notifier) ReactionUpdated(ctx context.Context, chatID, msgID, userID int64, reaction, action string) error {
	return n.send(ctx, chatID, EventReactionUpdated, ReactionUpdatedPayload{
		ChatID:    chatID,
		MessageID: msgID,
		UserID:    userID,
		Reaction:  reaction,
		Action:    action,
	})
}

// Typing уведомляет всех в чате, кроме того, кто печатает.
func (n *Notifier) Typing(ctx context.Context, chatID, userID int64, isTyping bool) error {
	ids, err := n.memberIDs(ctx, chatID)
	if err != nil {
		return err
	}

	recipients := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id != userID {
			recipients = append(recipients, id)
		}
	}
	if len(recipients) == 0 {
		return nil
	}

	return n.sendTo(recipients, EventUserTyping, UserTypingPayload{
		ChatID:   chatID,
		UserID:   userID,
		IsTyping: isTyping,
	})
}

func (n *Notifier) ReadReceipt(ctx context.Context, chatID, userID, lastReadMessageID int64) error {
	return n.send(ctx, chatID, EventReadReceipt, ReadReceiptBroadcastPayload{
		ChatID:            chatID,
		UserID:            userID,
		LastReadMessageID: lastReadMessageID,
	})
}

func Notify(err error) {
	if err != nil {
		log.Printf("Warning: WS notification failed: %v", err)
	}
}
