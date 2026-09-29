package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"chat-backend/internal/domain"
	"chat-backend/internal/usecase"
)

type authUseCase struct {
	usecase.UserUseCase
	err   error
	login string
}

func (u *authUseCase) GetOrCreateByUsername(_ context.Context, name string) (*domain.User, error) {
	u.login = name
	return &domain.User{ID: 42, Username: name}, u.err
}

func TestHeaderAuthMiddleware(t *testing.T) {
	for _, tc := range []struct {
		name, header, query, wantLogin string
		err                            error
		status                         int
	}{
		{name: "header priority", header: "amin", query: "other", wantLogin: "amin", status: 204},
		{name: "query fallback", query: "amin", wantLogin: "amin", status: 204},
		{name: "missing", status: 401},
		{name: "blank", header: " ", wantLogin: " ", err: domain.ErrUnauthorized, status: 401},
		{name: "invalid", header: "bad", wantLogin: "bad", err: domain.ErrInvalidInput, status: 400},
		{name: "database error", header: "amin", wantLogin: "amin", err: errors.New("db failure"), status: 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			uc := &authUseCase{err: tc.err}
			h := NewAuthMiddleware(uc).HeaderAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				user, err := GetUserFromContext(r.Context())
				if err != nil || user.ID != 42 {
					t.Fatalf("missing user: %v", err)
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			r := httptest.NewRequest(http.MethodGet, "/api/users/me?login="+tc.query, nil)
			r.Header.Set("Login", tc.header)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status || uc.login != tc.wantLogin {
				t.Fatalf("status=%d login=%q", w.Code, uc.login)
			}
		})
	}
}
