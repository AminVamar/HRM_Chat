package domain

import (
	"encoding/json"
	"strings"
	"time"
)

type User struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	FullName  string    `json:"full_name"`
	Email     string    `json:"email"`
	AvatarURL string    `json:"avatar_url"`
	IsOnline  bool      `json:"is_online"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UpdateProfileDTO struct {
	FirstName string `json:"first_name" example:"Иван"`
	LastName  string `json:"last_name" example:"Петров"`
}

func (u User) JoinedName() string {
	return strings.TrimSpace(u.FirstName + " " + u.LastName)
}

func (u User) DisplayName() string {
	if name := u.JoinedName(); name != "" {
		return name
	}
	return u.Username
}

func (u User) MarshalJSON() ([]byte, error) {
	type plain User
	p := plain(u)
	p.FullName = u.JoinedName()
	return json.Marshal(p)
}
