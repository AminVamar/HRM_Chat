package ws

import "encoding/json"

const (
	EventPing            = "ping"
	EventPong            = "pong"
	EventSendMessage     = "send_message"
	EventNewMessage      = "new_message"
	EventEditMessage     = "edit_message"
	EventMessageUpdated  = "message_updated"
	EventDeleteMessage   = "delete_message"
	EventMessageDeleted  = "message_deleted"
	EventAddReaction     = "add_reaction"
	EventRemoveReaction  = "remove_reaction"
	EventReactionUpdated = "reaction_updated"
	EventTyping          = "typing"
	EventUserTyping      = "user_typing"
	EventUserPresence    = "user_presence"
	EventReadReceipt     = "read_receipt"
	EventError           = "error"
	EventOnlineUsers     = "online_users"
)

// Значения поля "action" в событии reaction_updated.
const (
	ReactionAdded   = "added"
	ReactionRemoved = "removed"
)

type WSEvent struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type SendMessagePayload struct {
	ChatID  int64  `json:"chat_id"`
	Content string `json:"content"`
}

type EditMessagePayload struct {
	MessageID int64  `json:"message_id"`
	Content   string `json:"content"`
}

type DeleteMessagePayload struct {
	MessageID int64 `json:"message_id"`
}

type AddReactionPayload struct {
	MessageID int64  `json:"message_id"`
	Reaction  string `json:"reaction"`
}

type RemoveReactionPayload struct {
	MessageID int64  `json:"message_id"`
	Reaction  string `json:"reaction"`
}

type TypingPayload struct {
	ChatID   int64 `json:"chat_id"`
	IsTyping bool  `json:"is_typing"`
}

type ReadReceiptPayload struct {
	ChatID            int64 `json:"chat_id"`
	LastReadMessageID int64 `json:"last_read_message_id"`
}

// Исходящие события. В каждом есть chat_id, чтобы клиент сразу знал, какой чат обновить.

type MessageDeletedPayload struct {
	ChatID    int64 `json:"chat_id"`
	MessageID int64 `json:"message_id"`
}

type ReactionUpdatedPayload struct {
	ChatID    int64  `json:"chat_id"`
	MessageID int64  `json:"message_id"`
	UserID    int64  `json:"user_id"`
	Reaction  string `json:"reaction"`
	Action    string `json:"action"` // "added" или "removed"
}

// ReadReceiptBroadcastPayload — то же, что ReadReceiptPayload, плюс id того, кто прочитал.
type ReadReceiptBroadcastPayload struct {
	ChatID            int64 `json:"chat_id"`
	UserID            int64 `json:"user_id"`
	LastReadMessageID int64 `json:"last_read_message_id"`
}

type UserPresencePayload struct {
	UserID   int64 `json:"user_id"`
	IsOnline bool  `json:"is_online"`
}

type UserTypingPayload struct {
	ChatID   int64 `json:"chat_id"`
	UserID   int64 `json:"user_id"`
	IsTyping bool  `json:"is_typing"`
}

type ErrorPayload struct {
	Message string `json:"message"`
	Code    int    `json:"code,omitempty"`
}

type OnlineUserDTO struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
}

type OnlineUsersPayload struct {
	Count int             `json:"count"`
	Users []OnlineUserDTO `json:"users"`
}
