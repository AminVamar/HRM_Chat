package http

import (
	"errors"
	"net/http"
	"strconv"

	"chat-backend/internal/domain"
	"chat-backend/internal/usecase"
	"github.com/gorilla/mux"
)

type FileHandler struct {
	fileUseCase usecase.FileUseCase
}

func NewFileHandler(fileUseCase usecase.FileUseCase) *FileHandler {
	return &FileHandler{fileUseCase: fileUseCase}
}

// DownloadFile godoc
// @Summary      Скачать прикрепленный файл
// @Description  Скачивает прикрепленный к сообщению файл по его идентификатору (fileID)
// @Tags         файлы
// @Security     LoginHeaderAuth
// @Param        id path int true "ID файла"
// @Success      200 {file} binary
// @Failure      403 {object} Response
// @Failure      404 {object} Response
// @Router       /files/{id}/download [get]
func (h *FileHandler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r.Context())
	if err != nil {
		RespondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	fileID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Неверный ID файла")
		return
	}

	fileRecord, err := h.fileUseCase.GetFileByID(r.Context(), user.ID, fileID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			RespondError(w, http.StatusNotFound, "Файл не найден")
			return
		}
		if errors.Is(err, domain.ErrUserNotInChat) {
			RespondError(w, http.StatusForbidden, "Доступ запрещен")
			return
		}
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename=\""+fileRecord.Filename+"\"")
	w.Header().Set("Content-Type", fileRecord.MimeType)

	http.ServeFile(w, r, fileRecord.FilePath)
}
