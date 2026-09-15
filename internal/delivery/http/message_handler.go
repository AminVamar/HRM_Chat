package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"chat-backend/internal/delivery/ws"
	"chat-backend/internal/domain"
	"chat-backend/internal/usecase"
	"github.com/gorilla/mux"
)

const (
	maxUploadBytes = 32 << 20 
	maxAvatarBytes = 10 << 20 
)

type MessageHandler struct {
	messageUseCase usecase.MessageUseCase
	fileUseCase    usecase.FileUseCase
	notifier       *ws.Notifier
}

func NewMessageHandler(messageUseCase usecase.MessageUseCase, fileUseCase usecase.FileUseCase, notifier *ws.Notifier) *MessageHandler {
	return &MessageHandler{
		messageUseCase: messageUseCase,
		fileUseCase:    fileUseCase,
		notifier:       notifier,
	}
}

// SendMessage godoc
// @Summary      Отправить текстовое сообщение
// @Description  Отправляет текстовое сообщение в чат (содержимое зашифровано алгоритмом AES-256-GCM в базе данных)
// @Tags         сообщения
// @Accept       json
// @Produce      json
// @Security     LoginHeaderAuth
// @Param        body body domain.CreateMessageDTO true "Данные отправляемого сообщения"
// @Success      201 {object} Response{data=domain.Message}
// @Failure      400 {object} Response
// @Failure      403 {object} Response
// @Router       /messages [post]
func (h *MessageHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r.Context())
	if err != nil {
		RespondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	var dto domain.CreateMessageDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		RespondError(w, http.StatusBadRequest, "Неверный формат JSON")
		return
	}

	if dto.ChatID <= 0 || dto.Content == "" {
		RespondError(w, http.StatusBadRequest, "Поля chat_id и content обязательны")
		return
	}

	msg, err := h.messageUseCase.SendMessage(r.Context(), user.ID, dto)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotInChat) {
			RespondError(w, http.StatusForbidden, err.Error())
			return
		}
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	ws.Notify(h.notifier.NewMessage(r.Context(), msg))

	RespondJSON(w, http.StatusCreated, msg)
}

// GetMessages godoc
// @Summary      Получить историю сообщений
// @Description  Возвращает историю сообщений указанного чата (требуется параметр query chat_id)
// @Tags         сообщения
// @Accept       json
// @Produce      json
// @Security     LoginHeaderAuth
// @Param        chat_id query int true "ID чата"
// @Param        limit query int false "Количество сообщений (по умолчанию 50)"
// @Param        offset query int false "Смещение для пагинации"
// @Success      200 {object} Response{data=[]domain.Message}
// @Failure      400 {object} Response
// @Failure      403 {object} Response
// @Router       /messages [get]
func (h *MessageHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r.Context())
	if err != nil {
		RespondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	chatIDStr := r.URL.Query().Get("chat_id")
	if chatIDStr == "" {
		RespondError(w, http.StatusBadRequest, "Параметр chat_id обязателен")
		return
	}

	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Неверный параметр chat_id")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit < 0 {
		limit = 0 
	}
	if offset < 0 {
		offset = 0
	}

	messages, err := h.messageUseCase.GetMessages(r.Context(), user.ID, chatID, limit, offset)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotInChat) {
			RespondError(w, http.StatusForbidden, err.Error())
			return
		}
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, messages)
}

// UploadMessageFile godoc
// @Summary      Загрузить файл в чат (альтернативный эндпоинт)
// @Description  Загружает файл в чат, указанный в поле chat_id формы
// @Tags         сообщения
// @Accept       multipart/form-data
// @Produce      json
// @Security     LoginHeaderAuth
// @Param        chat_id formData int true "ID чата"
// @Param        file formData file true "Файл для загрузки"
// @Param        caption formData string false "Необязательная подпись"
// @Success      201 {object} Response{data=domain.Message}
// @Failure      400 {object} Response
// @Failure      403 {object} Response
// @Router       /messages/file [post]
func (h *MessageHandler) UploadMessageFile(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r.Context())
	if err != nil {
		RespondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		RespondError(w, http.StatusBadRequest, "Ошибка разбора мультипарт-формы или превышен размер файла")
		return
	}

	chatIDStr := r.FormValue("chat_id")
	if chatIDStr == "" {
		RespondError(w, http.StatusBadRequest, "Поле формы chat_id обязательно")
		return
	}

	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Неверный ID чата")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Поле файла 'file' обязательно")
		return
	}
	defer file.Close()

	caption := r.FormValue("caption")

	msg, err := h.fileUseCase.UploadFile(r.Context(), user.ID, chatID, header, caption)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotInChat) {
			RespondError(w, http.StatusForbidden, err.Error())
			return
		}
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	ws.Notify(h.notifier.NewMessage(r.Context(), msg))

	RespondJSON(w, http.StatusCreated, msg)
}

// UpdateMessage godoc
// @Summary      Редактировать текст сообщения
// @Description  Изменяет текст отправленного сообщения. Разрешено только автору сообщения.
// @Tags         сообщения
// @Accept       json
// @Produce      json
// @Security     LoginHeaderAuth
// @Param        id path int true "ID сообщения"
// @Param        body body domain.UpdateMessageDTO true "Новый текст сообщения"
// @Success      200 {object} Response{data=domain.Message}
// @Failure      403 {object} Response
// @Failure      404 {object} Response
// @Router       /messages/{id} [put]
func (h *MessageHandler) UpdateMessage(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r.Context())
	if err != nil {
		RespondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	msgID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Неверный ID сообщения")
		return
	}

	var dto domain.UpdateMessageDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		RespondError(w, http.StatusBadRequest, "Неверный формат JSON")
		return
	}

	updated, err := h.messageUseCase.UpdateMessage(r.Context(), user.ID, msgID, dto)
	if err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			RespondError(w, http.StatusForbidden, "Только автор может редактировать сообщение")
			return
		}
		if errors.Is(err, domain.ErrMessageDeleted) {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			RespondError(w, http.StatusNotFound, err.Error())
			return
		}
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	ws.Notify(h.notifier.MessageUpdated(r.Context(), updated))

	RespondJSON(w, http.StatusOK, updated)
}

// DeleteMessage godoc
// @Summary      Удалить сообщение (софт-удаление)
// @Description  Мягко удаляет сообщение (текст заменяется на "Сообщение удалено"). Разрешено только автору.
// @Tags         сообщения
// @Accept       json
// @Produce      json
// @Security     LoginHeaderAuth
// @Param        id path int true "ID сообщения"
// @Success      200 {object} Response{data=string}
// @Failure      403 {object} Response
// @Failure      404 {object} Response
// @Router       /messages/{id} [delete]
func (h *MessageHandler) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r.Context())
	if err != nil {
		RespondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	msgID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Неверный ID сообщения")
		return
	}

	chatID, err := h.messageUseCase.DeleteMessage(r.Context(), user.ID, msgID)
	if err != nil {
		if errors.Is(err, domain.ErrForbidden) {
			RespondError(w, http.StatusForbidden, "Только автор может удалить сообщение")
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			RespondError(w, http.StatusNotFound, err.Error())
			return
		}
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	ws.Notify(h.notifier.MessageDeleted(r.Context(), chatID, msgID))

	RespondJSON(w, http.StatusOK, map[string]string{"message": "Сообщение успешно удалено"})
}

// AddReaction godoc
// @Summary      Добавить или обновить реакцию на сообщение
// @Description  Ставит emoji-реакцию на сообщение. Один пользователь может поставить не более одной реакции на сообщение.
// @Tags         сообщения
// @Accept       json
// @Produce      json
// @Security     LoginHeaderAuth
// @Param        id path int true "ID сообщения"
// @Param        body body domain.AddReactionDTO true "Emoji реакции"
// @Success      200 {object} Response{data=domain.Reaction}
// @Failure      400 {object} Response
// @Failure      403 {object} Response
// @Router       /messages/{id}/reactions [post]
func (h *MessageHandler) AddReaction(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r.Context())
	if err != nil {
		RespondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	msgID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Неверный ID сообщения")
		return
	}

	var dto domain.AddReactionDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		RespondError(w, http.StatusBadRequest, "Неверный формат JSON")
		return
	}

	if dto.Reaction == "" {
		RespondError(w, http.StatusBadRequest, "Поле reaction обязательно")
		return
	}

	reaction, chatID, err := h.messageUseCase.AddReaction(r.Context(), user.ID, msgID, dto.Reaction)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotInChat) {
			RespondError(w, http.StatusForbidden, err.Error())
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			RespondError(w, http.StatusNotFound, err.Error())
			return
		}
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	ws.Notify(h.notifier.ReactionUpdated(r.Context(), chatID, msgID, user.ID, dto.Reaction, ws.ReactionAdded))

	RespondJSON(w, http.StatusOK, reaction)
}

// RemoveReaction godoc
// @Summary      Удалить реакцию с сообщения
// @Description  Удаляет реакцию пользователя с указанного сообщения
// @Tags         сообщения
// @Accept       json
// @Produce      json
// @Security     LoginHeaderAuth
// @Param        id path int true "ID сообщения"
// @Param        reaction query string false "Emoji реакции для удаления"
// @Success      200 {object} Response{data=string}
// @Failure      400 {object} Response
// @Failure      403 {object} Response
// @Router       /messages/{id}/reactions [delete]
func (h *MessageHandler) RemoveReaction(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r.Context())
	if err != nil {
		RespondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	msgID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Неверный ID сообщения")
		return
	}

	reactionStr := r.URL.Query().Get("reaction")

	chatID, err := h.messageUseCase.RemoveReaction(r.Context(), user.ID, msgID, reactionStr)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotInChat) {
			RespondError(w, http.StatusForbidden, err.Error())
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			RespondError(w, http.StatusNotFound, err.Error())
			return
		}
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	ws.Notify(h.notifier.ReactionUpdated(r.Context(), chatID, msgID, user.ID, reactionStr, ws.ReactionRemoved))

	RespondJSON(w, http.StatusOK, map[string]string{"message": "Реакция успешно удалена"})
}

// GetReactions godoc
// @Summary      Получить список реакций сообщения
// @Description  Возвращает список реакций, сгруппированных по emoji для указанного сообщения
// @Tags         сообщения
// @Accept       json
// @Produce      json
// @Security     LoginHeaderAuth
// @Param        id path int true "ID сообщения"
// @Success      200 {object} Response{data=[]domain.ReactionGroup}
// @Failure      403 {object} Response
// @Router       /messages/{id}/reactions [get]
func (h *MessageHandler) GetReactions(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r.Context())
	if err != nil {
		RespondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	msgID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Неверный ID сообщения")
		return
	}

	reactions, err := h.messageUseCase.GetReactions(r.Context(), user.ID, msgID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotInChat) {
			RespondError(w, http.StatusForbidden, err.Error())
			return
		}
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, reactions)
}
