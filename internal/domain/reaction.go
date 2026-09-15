package domain

import "time"

type Reaction struct {
	ID        int64     `json:"id"`
	MessageID int64     `json:"message_id"`
	UserID    int64     `json:"user_id"`
	User      *User     `json:"user,omitempty"`
	Reaction  string    `json:"reaction"`
	CreatedAt time.Time `json:"created_at"`
}

type ReactionGroup struct {
	Reaction string  `json:"reaction"`
	Count    int     `json:"count"`
	UserIDs  []int64 `json:"user_ids"`
	Users    []User  `json:"users,omitempty"`
}

type AddReactionDTO struct {
	Reaction string `json:"reaction" example:"👍"`
}
