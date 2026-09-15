package http

import (
	"net/http"

	"chat-backend/internal/usecase"
)

type GlobalHandler struct {
	chatUseCase    usecase.ChatUseCase
	messageUseCase usecase.MessageUseCase
}

func NewGlobalHandler(chatUseCase usecase.ChatUseCase, messageUseCase usecase.MessageUseCase) *GlobalHandler {
	return &GlobalHandler{
		chatUseCase:    chatUseCase,
		messageUseCase: messageUseCase,
	}
}

// GetGlobal godoc
// @Summary      Получить общий чат
// @Description  Присоединяет вызывающего пользователя к общему чату (если он ещё не участник) и возвращает его данные вместе с последними сообщениями.
// @Tags         общий чат
// @Produce      json
// @Security     LoginHeaderAuth
// @Success      200 {object} Response
// @Failure      401 {object} Response
// @Failure      500 {object} Response
// @Router       /global [get]
func (h *GlobalHandler) GetGlobal(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r.Context())
	if err != nil {
		RespondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	chat, err := h.chatUseCase.JoinGlobalChat(r.Context(), user.ID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	messages, err := h.messageUseCase.GetMessages(r.Context(), user.ID, chat.ID, 50, 0)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"chat":     chat,
		"messages": messages,
	})
}
