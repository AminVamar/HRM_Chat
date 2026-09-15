package http

import (
	"net/http"

	_ "chat-backend/docs"
	"chat-backend/internal/delivery/ws"
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
) *mux.Router {
	r := mux.NewRouter()

	r.PathPrefix("/swagger/").Handler(httpSwagger.WrapHandler)
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
