package ws

import (
	"log"
	"net/http"

	"chat-backend/internal/usecase"
)

type WSHandler struct {
	hub            *Hub
	userUseCase    usecase.UserUseCase
	messageUseCase usecase.MessageUseCase
	chatUseCase    usecase.ChatUseCase
	notifier       *Notifier
}

func NewWSHandler(
	hub *Hub,
	userUseCase usecase.UserUseCase,
	messageUseCase usecase.MessageUseCase,
	chatUseCase usecase.ChatUseCase,
	notifier *Notifier,
) *WSHandler {
	return &WSHandler{
		hub:            hub,
		userUseCase:    userUseCase,
		messageUseCase: messageUseCase,
		chatUseCase:    chatUseCase,
		notifier:       notifier,
	}
}

// ServeWS обрабатывает WebSocket подключение: /api/ws?login=...
func (h *WSHandler) ServeWS(w http.ResponseWriter, r *http.Request) {
	login := r.URL.Query().Get("login")
	if login == "" {
		http.Error(w, "Query parameter 'login' is required", http.StatusUnauthorized)
		return
	}

	user, err := h.userUseCase.GetByLogin(r.Context(), login)
	if err != nil {
		http.Error(w, "Unauthorized: user not found", http.StatusUnauthorized)
		return
	}

	if _, err := h.chatUseCase.JoinGlobalChat(r.Context(), user.ID); err != nil {
		log.Printf("Failed to auto-join global chat for user %d: %v", user.ID, err)
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade WebSocket connection: %v", err)
		return
	}

	client := NewClient(h.hub, conn, user, h.messageUseCase, h.chatUseCase, h.notifier)

	// Сначала запускаем запись, потом регистрируем, чтобы клиент не пропустил события.
	go client.WritePump()
	h.hub.register <- client
	go client.ReadPump()

	client.SendOnlineSnapshot()
}
