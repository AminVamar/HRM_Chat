package http

import (
	"errors"
	"net/http"
	"strconv"

	"chat-backend/internal/delivery/ws"
	"chat-backend/internal/domain"
	"chat-backend/internal/usecase"
	"github.com/gorilla/mux"
)

type UserHandler struct {
	userUseCase usecase.UserUseCase
	hub         *ws.Hub
}

func NewUserHandler(userUseCase usecase.UserUseCase, hub *ws.Hub) *UserHandler {
	return &UserHandler{
		userUseCase: userUseCase,
		hub:         hub,
	}
}

// GetMe godoc
// @Summary      Получить профиль текущего пользователя
// @Description  Возвращает данные профиля авторизованного пользователя
// @Tags         пользователи
// @Accept       json
// @Produce      json
// @Security     LoginHeaderAuth
// @Success      200 {object} Response{data=domain.User}
// @Failure      401 {object} Response
// @Router       /users/me [get]
func (h *UserHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r.Context())
	if err != nil {
		RespondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	user.IsOnline = h.hub.IsUserOnline(user.ID)
	RespondJSON(w, http.StatusOK, user)
}

// Search godoc
// @Summary      Поиск пользователей
// @Description  Поиск пользователей по имени пользователя или email
// @Tags         пользователи
// @Accept       json
// @Produce      json
// @Security     LoginHeaderAuth
// @Param        query query string true "Поисковая строка (например: fed)"
// @Success      200 {object} Response{data=[]domain.User}
// @Failure      400 {object} Response
// @Failure      401 {object} Response
// @Router       /users/search [get]
func (h *UserHandler) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if query == "" {
		RespondError(w, http.StatusBadRequest, "Параметр query обязателен")
		return
	}

	users, err := h.userUseCase.Search(r.Context(), query)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	for i := range users {
		users[i].IsOnline = h.hub.IsUserOnline(users[i].ID)
	}

	RespondJSON(w, http.StatusOK, users)
}

// GetAll godoc
// @Summary      Получить всех пользователей
// @Description  Возвращает список всех зарегистрированных пользователей системы
// @Tags         пользователи
// @Accept       json
// @Produce      json
// @Security     LoginHeaderAuth
// @Success      200 {object} Response{data=[]domain.User}
// @Failure      401 {object} Response
// @Router       /users [get]
func (h *UserHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	users, err := h.userUseCase.GetAll(r.Context())
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	for i := range users {
		users[i].IsOnline = h.hub.IsUserOnline(users[i].ID)
	}

	RespondJSON(w, http.StatusOK, users)
}

// UploadAvatar godoc
// @Summary      Загрузить аватар пользователя
// @Description  Загружает изображение аватарки текущего пользователя. В ответе avatar_url — прямая ссылка на файл вида /uploads/avatars/<имя файла>, её можно сразу ставить в <img src> (авторизация не нужна)
// @Tags         пользователи
// @Accept       multipart/form-data
// @Produce      json
// @Security     LoginHeaderAuth
// @Param        file formData file true "Файл изображения аватарки"
// @Success      200 {object} Response{data=domain.User}
// @Failure      400 {object} Response
// @Failure      401 {object} Response
// @Router       /users/avatar [post]
func (h *UserHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r.Context())
	if err != nil {
		RespondError(w, http.StatusUnauthorized, err.Error())
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

	updatedUser, err := h.userUseCase.UploadAvatar(r.Context(), user.ID, header)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	updatedUser.IsOnline = h.hub.IsUserOnline(updatedUser.ID)
	RespondJSON(w, http.StatusOK, updatedUser)
}

// GetAvatar godoc
// @Summary      Получить аватар пользователя по ID
// @Description  Возвращает адрес аватарки указанного пользователя (/uploads/avatars/<имя файла>). Саму картинку можно открыть по этому адресу без авторизации
// @Tags         пользователи
// @Produce      json
// @Security     LoginHeaderAuth
// @Param        id path int true "ID пользователя"
// @Success      200 {object} Response{data=AvatarResponse}
// @Failure      404 {object} Response
// @Router       /users/{id}/avatar [get]
func (h *UserHandler) GetAvatar(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Неверный ID пользователя")
		return
	}

	avatarURL, err := h.userUseCase.GetAvatarURL(r.Context(), userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			RespondError(w, http.StatusNotFound, "Аватар не найден")
			return
		}
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, AvatarResponse{AvatarURL: avatarURL})
}
