package usecase

import (
	"context"
	"errors"
	"testing"

	"chat-backend/internal/domain"
	"chat-backend/internal/repository"
)

type directChatRepo struct {
	repository.ChatRepository
	chats   map[string]*domain.Chat
	members map[int64]map[int64]string
}

func (r *directChatRepo) GetDirectChat(_ context.Context, user1ID, user2ID int64) (*domain.Chat, error) {
	for _, chat := range r.chats {
		if _, ok := r.members[chat.ID][user1ID]; !ok {
			continue
		}
		if _, ok := r.members[chat.ID][user2ID]; ok {
			return chat, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *directChatRepo) Create(_ context.Context, _ *domain.Chat, _ int64, _ []int64, directKey string) (*domain.Chat, error) {
	if _, ok := r.chats[directKey]; ok {
		return nil, domain.ErrAlreadyExists
	}
	return nil, errors.New("unexpected create")
}

func (r *directChatRepo) GetByDirectKey(_ context.Context, directKey string) (*domain.Chat, error) {
	if chat, ok := r.chats[directKey]; ok {
		return chat, nil
	}
	return nil, domain.ErrNotFound
}

func (r *directChatRepo) GetByID(_ context.Context, chatID int64) (*domain.Chat, error) {
	for _, chat := range r.chats {
		if chat.ID == chatID {
			copied := *chat
			return &copied, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *directChatRepo) AddMember(_ context.Context, chatID, userID int64, role string) error {
	if r.members[chatID] == nil {
		r.members[chatID] = map[int64]string{}
	}
	if _, ok := r.members[chatID][userID]; !ok {
		r.members[chatID][userID] = role
	}
	return nil
}

func (r *directChatRepo) IsMember(_ context.Context, chatID, userID int64) (bool, error) {
	_, ok := r.members[chatID][userID]
	return ok, nil
}

func (r *directChatRepo) GetMembers(_ context.Context, chatID int64) ([]domain.ChatMember, error) {
	members := make([]domain.ChatMember, 0, len(r.members[chatID]))
	for userID, role := range r.members[chatID] {
		members = append(members, domain.ChatMember{ChatID: chatID, UserID: userID, Role: role})
	}
	return members, nil
}

func (r *directChatRepo) GetMember(_ context.Context, chatID, userID int64) (*domain.ChatMember, error) {
	role, ok := r.members[chatID][userID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &domain.ChatMember{ChatID: chatID, UserID: userID, Role: role}, nil
}

type existingUserRepo struct {
	repository.UserRepository
}

func (existingUserRepo) GetByID(_ context.Context, id int64) (*domain.User, error) {
	return &domain.User{ID: id}, nil
}

type emptyMessageRepo struct {
	repository.MessageRepository
}

func (emptyMessageRepo) GetLastMessage(context.Context, int64) (*domain.Message, error) {
	return nil, domain.ErrNotFound
}

func (emptyMessageRepo) GetUnreadCount(context.Context, int64, int64, int64) (int, error) {
	return 0, nil
}

func TestCreateDirectChatRestoresOrphanedChat(t *testing.T) {
	repo := &directChatRepo{
		chats:   map[string]*domain.Chat{"3_7": {ID: 10, Type: domain.ChatTypeDirect}},
		members: map[int64]map[int64]string{},
	}
	uc := &chatUseCase{chatRepo: repo, userRepo: existingUserRepo{}, messageRepo: emptyMessageRepo{}}

	chat, err := uc.CreateChat(context.Background(), 7, domain.CreateChatDTO{Type: domain.ChatTypeDirect, MemberIDs: []int64{3}})
	if err != nil {
		t.Fatal(err)
	}
	if chat.ID != 10 {
		t.Fatalf("expected chat 10, got %d", chat.ID)
	}
	if repo.members[10][7] != domain.RoleOwner || repo.members[10][3] != domain.RoleMember {
		t.Fatalf("members not restored: %v", repo.members[10])
	}
}

func TestCreateDirectChatConflictWithoutChat(t *testing.T) {
	repo := &directChatRepo{chats: map[string]*domain.Chat{}, members: map[int64]map[int64]string{}}
	uc := &chatUseCase{chatRepo: repo, userRepo: existingUserRepo{}, messageRepo: emptyMessageRepo{}}

	got, err := uc.restoreDirectChat(context.Background(), 7, 3, "3_7")
	if !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("expected ErrAlreadyExists, got %v, %v", got, err)
	}
}
