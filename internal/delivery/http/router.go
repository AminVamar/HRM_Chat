package http

import (
	"net/http"
	"strings"

	_ "chat-backend/docs"
	"chat-backend/internal/delivery/ws"
	"chat-backend/internal/usecase"
	"github.com/gorilla/mux"
	httpSwagger "github.com/swaggo/http-swagger"
)

func NewRouter(
	userHandler *UserHandler,
	chatHandler *ChatHandler,
	messageHandler *MessageHandler,
	fileHandler *FileHandler,
	globalHandler *GlobalHandler,
	wsHandler *ws.WSHandler,
	authMiddleware *AuthMiddleware,
	avatarDir string,
) *mux.Router {
	r := mux.NewRouter()

	r.PathPrefix("/swagger/").Handler(httpSwagger.WrapHandler)
	// Аватарки отдаём напрямую без авторизации, чтобы avatar_url можно было подставить в <img src>.
	// Открыта только папка avatars, файлы из чатов по-прежнему идут через /api/files/{id}/download.
	r.PathPrefix(usecase.AvatarURLPrefix).
		Handler(http.StripPrefix(usecase.AvatarURLPrefix, avatarFileServer(avatarDir))).
		Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc("/api/ws", wsHandler.ServeWS).Methods(http.MethodGet)

	api := r.PathPrefix("/api").Subrouter()
	api.Use(authMiddleware.HeaderAuthMiddleware)

	api.HandleFunc("/users/me", userHandler.GetMe).Methods(http.MethodGet)
	api.HandleFunc("/users/search", userHandler.Search).Methods(http.MethodGet)
	api.HandleFunc("/users/avatar", userHandler.UploadAvatar).Methods(http.MethodPost)
	api.HandleFunc("/users/{id:[0-9]+}/avatar", userHandler.GetAvatar).Methods(http.MethodGet)
	api.HandleFunc("/users", userHandler.GetAll).Methods(http.MethodGet)

	api.HandleFunc("/chats", chatHandler.CreateChat).Methods(http.MethodPost)
	api.HandleFunc("/chats", chatHandler.GetMyChats).Methods(http.MethodGet)
	api.HandleFunc("/chats/{id:[0-9]+}", chatHandler.GetChat).Methods(http.MethodGet)
	api.HandleFunc("/chats/{id:[0-9]+}", chatHandler.UpdateChat).Methods(http.MethodPatch)
	api.HandleFunc("/chats/{id:[0-9]+}", chatHandler.DeleteChat).Methods(http.MethodDelete)
	api.HandleFunc("/chats/{id:[0-9]+}/out", chatHandler.LeaveChat).Methods(http.MethodDelete)
	api.HandleFunc("/chats/{id:[0-9]+}/read", chatHandler.MarkAsRead).Methods(http.MethodPost)
	api.HandleFunc("/chats/{id:[0-9]+}/members", chatHandler.GetMembers).Methods(http.MethodGet)
	api.HandleFunc("/chats/{id:[0-9]+}/members/{user_id:[0-9]+}", chatHandler.UpdateMemberRole).Methods(http.MethodPost)
	api.HandleFunc("/chats/{id:[0-9]+}/members", chatHandler.AddMember).Methods(http.MethodPost)
	api.HandleFunc("/chats/{id:[0-9]+}/members/{user_id:[0-9]+}", chatHandler.RemoveMember).Methods(http.MethodDelete)
	api.HandleFunc("/chats/{id:[0-9]+}/files", chatHandler.UploadChatFile).Methods(http.MethodPost)
	api.HandleFunc("/chats/{id:[0-9]+}/avatar", chatHandler.UploadAvatar).Methods(http.MethodPost)
	api.HandleFunc("/chats/{id:[0-9]+}/avatar", chatHandler.GetAvatar).Methods(http.MethodGet)

	api.HandleFunc("/messages", messageHandler.SendMessage).Methods(http.MethodPost)
	api.HandleFunc("/messages", messageHandler.GetMessages).Methods(http.MethodGet)
	api.HandleFunc("/messages/file", messageHandler.UploadMessageFile).Methods(http.MethodPost)
	api.HandleFunc("/messages/{id:[0-9]+}", messageHandler.UpdateMessage).Methods(http.MethodPut)
	api.HandleFunc("/messages/{id:[0-9]+}", messageHandler.DeleteMessage).Methods(http.MethodDelete)
	api.HandleFunc("/messages/{id:[0-9]+}/reactions", messageHandler.AddReaction).Methods(http.MethodPost)
	api.HandleFunc("/messages/{id:[0-9]+}/reactions", messageHandler.RemoveReaction).Methods(http.MethodDelete)
	api.HandleFunc("/messages/{id:[0-9]+}/reactions", messageHandler.GetReactions).Methods(http.MethodGet)

	api.HandleFunc("/files/{id:[0-9]+}/download", fileHandler.DownloadFile).Methods(http.MethodGet)

	api.HandleFunc("/global", globalHandler.GetGlobal).Methods(http.MethodGet)

	return r
}

// avatarFileServer отдаёт файлы из папки аватарок без листинга директории.
func avatarFileServer(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "" || strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=86400")
		fs.ServeHTTP(w, r)
	})
}
