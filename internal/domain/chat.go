package domain

import "time"

const (
	ChatTypeDirect = "direct"
	ChatTypeGroup  = "group"

	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
)

type Chat struct {
	ID          int64        `json:"id"`
	Name        string       `json:"name"`
	Type        string       `json:"type"` 
	AvatarURL   string       `json:"avatar_url"`
	IsGlobal    bool         `json:"is_global"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	Members     []ChatMember `json:"members,omitempty"`
	LastMessage *Message     `json:"last_message,omitempty"`
	UnreadCount int          `json:"unread_count,omitempty"`
}

type ChatMember struct {
	ChatID            int64     `json:"chat_id"`
	UserID            int64     `json:"user_id"`
	User              *User     `json:"user,omitempty"`
	Role              string    `json:"role"` 
	JoinedAt          time.Time `json:"joined_at"`
	LastReadMessageID int64     `json:"last_read_message_id"`
}

type CreateChatDTO struct {
	Type      string  `json:"type" example:"group"`
	Name      string  `json:"name" example:"Project Team"`
	MemberIDs []int64 `json:"member_ids"`
}

type UpdateChatDTO struct {
	Name string `json:"name" example:"New Group Name"`
}

type ReadChatDTO struct {
	LastReadMessageID int64 `json:"last_read_message_id"`
}

type AddMemberDTO struct {
	UserID int64 `json:"user_id"`
}

type UpdateMemberRoleDTO struct {
	Role string `json:"role" example:"admin"`
}
