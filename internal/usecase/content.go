package usecase

import (
	"log"

	"chat-backend/internal/domain"
	"chat-backend/pkg/crypto"
)

const (
	deletedMessageText       = "Сообщение удалено"
	undecryptableMessageText = "[не удалось расшифровать]"
)

func revealContent(msg *domain.Message) {
	if msg.IsDeleted {
		msg.Content = deletedMessageText
		return
	}

	plain, err := crypto.Decrypt(msg.Content)
	if err != nil {
		log.Printf("Warning: cannot decrypt message %d: %v", msg.ID, err)
		msg.Content = undecryptableMessageText
		return
	}
	msg.Content = plain
}
