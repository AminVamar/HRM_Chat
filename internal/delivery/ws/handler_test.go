package ws

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"chat-backend/internal/domain"
	"chat-backend/internal/usecase"
)

type authUseCase struct {
	usecase.UserUseCase
	err error
}

func (u *authUseCase) GetOrCreateByUsername(context.Context, string) (*domain.User, error) {
	return nil, u.err
}

func TestServeWSAuthErrors(t *testing.T) {
	for _, tc := range []struct {
		name, query string
		err         error
		status      int
	}{
		{name: "missing", status: 401},
		{name: "blank", query: "?login=%20", err: domain.ErrUnauthorized, status: 401},
		{name: "invalid", query: "?login=bad", err: domain.ErrInvalidInput, status: 400},
		{name: "database", query: "?login=amin", err: errors.New("db unavailable"), status: 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := &WSHandler{userUseCase: &authUseCase{err: tc.err}}
			w := httptest.NewRecorder()
			h.ServeWS(w, httptest.NewRequest("GET", "/api/ws"+tc.query, nil))
			if w.Code != tc.status {
				t.Fatalf("status=%d, want %d", w.Code, tc.status)
			}
		})
	}
}
