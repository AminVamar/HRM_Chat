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

type ChatHandler struct {
	chatUseCase usecase.ChatUseCase
	fileUseCase usecase.FileUseCase
	hub         *ws.Hub
	notifier    *ws.Notifier
}

func NewChatHandler(chatUseCase usecase.ChatUseCase, fileUseCase usecase.FileUseCase, hub *ws.Hub, notifier *ws.Notifier) *ChatHandler {
	return &ChatHandler{
		chatUseCase: chatUseCase,
		fileUseCase: fileUseCase,
		hub:         hub,
		notifier:    notifier,
	}
}

// CreateChat godoc
// @Summary      Создать новый чат
// @Description  Создает личный (direct) или групповой (group) чат. Личный чат требует ровно 1 собеседника (нельзя с самим собой). Групповой чат требует минимум 3 участников.
// @Tags         чаты
// @Accept       json
// @Produce      json
// @Security     LoginHeaderAuth
// @Param        body body domain.CreateChatDTO true "Параметры создания чата"
// @Success      201 {object} Response{data=domain.Chat}
// @Failure      400 {object} Response
// @Failure      401 {object} Response
// @Router       /chats [post]
func (h *ChatHandler) CreateChat(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r.Context())
	if err != nil {
		RespondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	var dto domain.CreateChatDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		RespondError(w, http.StatusBadRequest, "Неверный формат JSON")
		return
	}

	chat, err := h.chatUseCase.CreateChat(r.Context(), user.ID, dto)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidChatType) ||
			errors.Is(err, domain.ErrInvalidInput) ||
			errors.Is(err, domain.ErrDirectChatSelf) ||
			errors.Is(err, domain.ErrGroupMinMembers) {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, domain.ErrUserNotFound) {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondJSON(w, http.StatusCreated, chat)
}

// GetMyChats godoc
// @Summary      Получить чаты пользователя
// @Description  Возвращает все чаты, участником которых является текущий пользователь. Для личных (direct) чатов имя автоматически устанавливается как имя собеседника. Можно отфильтровать по типу: direct или group.
// @Tags         чаты
// @Accept       json
// @Produce      json
// @Security     LoginHeaderAuth
// @Param        type  query     string  false  "Тип чата"  Enums(direct, group)
// @Success      200 {object} Response{data=[]domain.Chat}
// @Failure      400 {object} Response
// @Failure      401 {object} Response
// @Router       /chats [get]
func (h *ChatHandler) GetMyChats(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r.Context())
	if err != nil {
		RespondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	chatType := r.URL.Query().Get("type")
	if chatType != "" && chatType != domain.ChatTypeDirect && chatType != domain.ChatTypeGroup {
		RespondError(w, http.StatusBadRequest, "type должен быть direct или group")
		return
	}

	chats, err := h.chatUseCase.GetChatsForUser(r.Context(), user.ID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Фильтр по типу чата
	if chatType != "" {
		filtered := make([]domain.Chat, 0, len(chats))
		for _, c := range chats {
			if c.Type == chatType {
				filtered = append(filtered, c)
			}
		}
		chats = filtered
	}

	for i := range chats {
		for j := range chats[i].Members {
			if chats[i].Members[j].User != nil {
				chats[i].Members[j].User.IsOnline = h.hub.IsUserOnline(chats[i].Members[j].UserID)
			}
		}
	}

	RespondJSON(w, http.StatusOK, chats)
}

// GetChat godoc
// @Summary      Получить детали чата по ID
// @Description  Возвращает информацию о чате и его участниках по идентификатору чата
// @Tags         чаты
// @Accept       json
// @Produce      json
// @Security     LoginHeaderAuth
// @Param        id path int true "ID чата"
// @Success      200 {object} Response{data=domain.Chat}
// @Failure      403 {object} Response
// @Failure      404 {object} Response
// @Router       /chats/{id} [get]
func (h *ChatHandler) GetChat(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r.Context())
	if err != nil {
		RespondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	chatID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Неверный ID чата")
		return
	}

	chat, err := h.chatUseCase.GetChatByID(r.Context(), user.ID, chatID)
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

	for i := range chat.Members {
		if chat.Members[i].User != nil {
			chat.Members[i].User.IsOnline = h.hub.IsUserOnline(chat.Members[i].UserID)
		}
	}

	RespondJSON(w, http.StatusOK, chat)
}

// UpdateChat godoc
// @Summary      Изменить имя группового чата
// @Description  Обновляет название группового чата. Для личных (direct) чатов операция запрещена.
// @Tags         чаты
// @Accept       json
// @Produce      json
// @Security     LoginHeaderAuth
// @Param        id path int true "ID чата"
// @Param        body body domain.UpdateChatDTO true "Новое имя чата"
// @Success      200 {object} Response{data=string}
// @Failure      400 {object} Response
// @Failure      403 {object} Response
// @Failure      404 {object} Response
// @Router       /chats/{id} [patch]
func (h *ChatHandler) UpdateChat(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r.Context())
	if err != nil {
		RespondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	chatID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Неверный ID чата")
		return
	}

	var dto domain.UpdateChatDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		RespondError(w, http.StatusBadRequest, "Неверный формат JSON")
		return
	}

	if err := h.chatUseCase.UpdateChatName(r.Context(), user.ID, chatID, dto.Name); err != nil {
		if errors.Is(err, domain.ErrDirectChatRename) || errors.Is(err, domain.ErrGlobalChatOp) {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, domain.ErrForbidden) {
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

	RespondJSON(w, http.StatusOK, map[string]string{"message": "Имя чата успешно обновлено"})
}

// DeleteChat godoc
// @Summary      Удалить чат
// @Description  Полностью удаляет чат. Разрешено владельцу группового чата или участнику личного чата.
// @Tags         чаты
// @Accept       json
// @Produce      json
// @Security     LoginHeaderAuth
// @Param        id path int true "ID чата"
// @Success      200 {object} Response{data=string}
// @Failure      403 {object} Response
// @Failure      404 {object} Response
// @Router       /chats/{id} [delete]
func (h *ChatHandler) DeleteChat(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r.Context())
	if err != nil {
		RespondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	chatID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Неверный ID чата")
		return
	}

	if err := h.chatUseCase.DeleteChat(r.Context(), user.ID, chatID); err != nil {
		if errors.Is(err, domain.ErrGlobalChatOp) {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, domain.ErrForbidden) {
			RespondError(w, http.StatusForbidden, "Только владелец может удалить групповой чат")
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			RespondError(w, http.StatusNotFound, err.Error())
			return
		}
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, map[string]string{"message": "Чат успешно удален"})
}

// LeaveChat godoc
// @Summary      Выйти из группового чата
// @Description  Позволяет пользователю выйти из группового чата. Для личных (direct) чатов операция запрещена.
// @Tags         чаты
// @Accept       json
// @Produce      json
// @Security     LoginHeaderAuth
// @Param        id path int true "ID чата"
// @Success      200 {object} Response{data=string}
// @Failure      400 {object} Response
// @Failure      403 {object} Response
// @Router       /chats/{id}/out [delete]
func (h *ChatHandler) LeaveChat(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r.Context())
	if err != nil {
		RespondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	chatID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Неверный ID чата")
		return
	}

	if err := h.chatUseCase.LeaveChat(r.Context(), user.ID, chatID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			RespondError(w, http.StatusNotFound, err.Error())
			return
		}
		RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, map[string]string{"message": "Вы успешно вышли из чата"})
}

// MarkAsRead godoc
// @Summary      Отметить сообщения чата как прочитанные
// @Description  Обновляет отметку о прочтении до указанного last_read_message_id
// @Tags         чаты
// @Accept       json
// @Produce      json
// @Security     LoginHeaderAuth
// @Param        id path int true "ID чата"
// @Param        body body domain.ReadChatDTO true "ID последнего прочитанного сообщения"
// @Success      200 {object} Response{data=string}
// @Failure      400 {object} Response
// @Router       /chats/{id}/read [post]
func (h *ChatHandler) MarkAsRead(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r.Context())
	if err != nil {
		RespondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	chatID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Неверный ID чата")
		return
	}

	var dto domain.ReadChatDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		RespondError(w, http.StatusBadRequest, "Неверный формат JSON")
		return
	}

	if err := h.chatUseCase.MarkAsRead(r.Context(), user.ID, chatID, dto.LastReadMessageID); err != nil {
		RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, map[string]string{"message": "Сообщения отмечены как прочитанные"})
}

// GetMembers godoc
// @Summary      Получить участников чата
// @Description  Возвращает список всех участников чата и их ролей
// @Tags         чаты
// @Accept       json
// @Produce      json
// @Security     LoginHeaderAuth
// @Param        id path int true "ID чата"
// @Success      200 {object} Response{data=[]domain.ChatMember}
// @Failure      403 {object} Response
// @Router       /chats/{id}/members [get]
func (h *ChatHandler) GetMembers(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r.Context())
	if err != nil {
		RespondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	chatID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Неверный ID чата")
		return
	}

	members, err := h.chatUseCase.GetMembers(r.Context(), user.ID, chatID)
	if err != nil {
		RespondError(w, http.StatusForbidden, err.Error())
		return
	}

	for i := range members {
		if members[i].User != nil {
			members[i].User.IsOnline = h.hub.IsUserOnline(members[i].UserID)
		}
	}

	RespondJSON(w, http.StatusOK, members)
}

// AddMember godoc
// @Summary      Добавить участника в групповой чат
// @Description  Добавляет пользователя в групповой чат. Запрещено для личных (direct) чатов. Разрешено владельцу или админу.
// @Tags         чаты
// @Accept       json
// @Produce      json
// @Security     LoginHeaderAuth
// @Param        id path int true "ID чата"
// @Param        body body domain.AddMemberDTO true "ID добавляемого пользователя"
// @Success      200 {object} Response{data=string}
// @Failure      400 {object} Response
// @Failure      403 {object} Response
// @Router       /chats/{id}/members [post]
func (h *ChatHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r.Context())
	if err != nil {
		RespondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	chatID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Неверный ID чата")
		return
	}

	var dto domain.AddMemberDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		RespondError(w, http.StatusBadRequest, "Неверный формат JSON")
		return
	}

	if err := h.chatUseCase.AddMember(r.Context(), user.ID, chatID, dto.UserID); err != nil {
		if errors.Is(err, domain.ErrDirectChatMemberOp) {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, domain.ErrForbidden) {
			RespondError(w, http.StatusForbidden, err.Error())
			return
		}
		if errors.Is(err, domain.ErrUserNotFound) {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}
		RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, map[string]string{"message": "Участник успешно добавлен"})
}

// UpdateMemberRole godoc
// @Summary      Изменить роль участника группового чата
// @Description  Изменяет роль участника на 'admin' или 'member'. Запрещено для личных (direct) чатов. Нельзя назначить роль 'owner'.
// @Tags         чаты
// @Accept       json
// @Produce      json
// @Security     LoginHeaderAuth
// @Param        id path int true "ID чата"
// @Param        user_id path int true "ID целевого пользователя"
// @Param        body body domain.UpdateMemberRoleDTO true "Новая роль ('admin' или 'member')"
// @Success      200 {object} Response{data=string}
// @Failure      400 {object} Response
// @Failure      403 {object} Response
// @Router       /chats/{id}/members/{user_id} [post]
func (h *ChatHandler) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r.Context())
	if err != nil {
		RespondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	vars := mux.Vars(r)
	chatID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Неверный ID чата")
		return
	}

	targetUserID, err := strconv.ParseInt(vars["user_id"], 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Неверный ID целевого пользователя")
		return
	}

	var dto domain.UpdateMemberRoleDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		RespondError(w, http.StatusBadRequest, "Неверный формат JSON")
		return
	}

	if err := h.chatUseCase.UpdateMemberRole(r.Context(), user.ID, chatID, targetUserID, dto.Role); err != nil {
		if errors.Is(err, domain.ErrDirectChatMemberOp) || errors.Is(err, domain.ErrCannotSetOwner) ||
			errors.Is(err, domain.ErrInvalidInput) || errors.Is(err, domain.ErrGlobalChatOp) {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, domain.ErrForbidden) {
			RespondError(w, http.StatusForbidden, err.Error())
			return
		}
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, map[string]string{"message": "Роль участника успешно обновлена"})
}

// RemoveMember godoc
// @Summary      Удалить участника из группового чата
// @Description  Удаляет участника из группового чата. Запрещено для личных (direct) чатов. Разрешено владельцу или админу.
// @Tags         чаты
// @Accept       json
// @Produce      json
// @Security     LoginHeaderAuth
// @Param        id path int true "ID чата"
// @Param        user_id path int true "ID исключаемого пользователя"
// @Success      200 {object} Response{data=string}
// @Failure      400 {object} Response
// @Failure      403 {object} Response
// @Router       /chats/{id}/members/{user_id} [delete]
func (h *ChatHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r.Context())
	if err != nil {
		RespondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	vars := mux.Vars(r)
	chatID, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Неверный ID чата")
		return
	}

	targetUserID, err := strconv.ParseInt(vars["user_id"], 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Неверный ID целевого пользователя")
		return
	}

	if err := h.chatUseCase.RemoveMember(r.Context(), user.ID, chatID, targetUserID); err != nil {
		if errors.Is(err, domain.ErrDirectChatMemberOp) || errors.Is(err, domain.ErrGlobalChatOp) {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, domain.ErrForbidden) {
			RespondError(w, http.StatusForbidden, err.Error())
			return
		}
		RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, map[string]string{"message": "Участник успешно удален из чата"})
}

// UploadChatFile godoc
// @Summary      Загрузить файл в чат
// @Description  Загружает файл и отправляет сообщение с прикрепленным файлом в указанный чат
// @Tags         чаты
// @Accept       multipart/form-data
// @Produce      json
// @Security     LoginHeaderAuth
// @Param        id path int true "ID чата"
// @Param        file formData file true "Файл для загрузки"
// @Param        caption formData string false "Необязательная подпись к файлу"
// @Success      201 {object} Response{data=domain.Message}
// @Failure      400 {object} Response
// @Failure      403 {object} Response
// @Router       /chats/{id}/files [post]
func (h *ChatHandler) UploadChatFile(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r.Context())
	if err != nil {
		RespondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	chatID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Неверный ID чата")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		RespondError(w, http.StatusBadRequest, "Ошибка разбора мультипарт-формы или превышен размер файла")
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

// UploadAvatar godoc
// @Summary      Загрузить аватар чата
// @Description  Загружает изображение аватарки группового чата. В ответе avatar_url — прямая ссылка на файл вида /uploads/avatars/<имя файла>
// @Tags         чаты
// @Accept       multipart/form-data
// @Produce      json
// @Security     LoginHeaderAuth
// @Param        id path int true "ID чата"
// @Param        file formData file true "Файл изображения аватарки"
// @Success      200 {object} Response{data=domain.Chat}
// @Failure      400 {object} Response
// @Failure      403 {object} Response
// @Router       /chats/{id}/avatar [post]
func (h *ChatHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r.Context())
	if err != nil {
		RespondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	chatID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Неверный ID чата")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarBytes)
	if err := r.ParseMultipartForm(maxAvatarBytes); err != nil {
		RespondError(w, http.StatusBadRequest, "Ошибка разбора формы или превышен размер файла")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Поле файла 'file' обязательно")
		return
	}
	defer file.Close()

	chat, err := h.chatUseCase.UploadAvatar(r.Context(), user.ID, chatID, header)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotInChat) || errors.Is(err, domain.ErrForbidden) {
			RespondError(w, http.StatusForbidden, err.Error())
			return
		}
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, chat)
}

// GetAvatar godoc
// @Summary      Получить аватар чата по ID
// @Description  Возвращает адрес аватарки чата (/uploads/avatars/<имя файла>). Для личного чата — адрес аватарки собеседника
// @Tags         чаты
// @Produce      json
// @Security     LoginHeaderAuth
// @Param        id path int true "ID чата"
// @Success      200 {object} Response{data=AvatarResponse}
// @Failure      403 {object} Response
// @Failure      404 {object} Response
// @Router       /chats/{id}/avatar [get]
func (h *ChatHandler) GetAvatar(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r.Context())
	if err != nil {
		RespondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	chatID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Неверный ID чата")
		return
	}

	avatarURL, err := h.chatUseCase.GetAvatarURL(r.Context(), user.ID, chatID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotInChat) {
			RespondError(w, http.StatusForbidden, err.Error())
			return
		}
		if errors.Is(err, domain.ErrNotFound) {
			RespondError(w, http.StatusNotFound, "Аватар не найден")
			return
		}
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, AvatarResponse{AvatarURL: avatarURL})
}
