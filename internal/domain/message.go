package domain

import "time"

type Message struct {
	ID        int64      `json:"id"`
	ChatID    int64      `json:"chat_id"`
	SenderID  int64      `json:"sender_id"`
	Sender    *User      `json:"sender,omitempty"`
	Content   string     `json:"content"`
	FileID    *int64     `json:"file_id,omitempty"`
	File      *File      `json:"file,omitempty"`
	IsEdited  bool       `json:"is_edited"`
	IsDeleted bool       `json:"is_deleted"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	Reactions []Reaction `json:"reactions,omitempty"`
}

type CreateMessageDTO struct {
	ChatID  int64  `json:"chat_id" example:"1"`
	Content string `json:"content" example:"Hello world!"`
}

type UpdateMessageDTO struct {
	Content string `json:"content" example:"Updated text"`
}
