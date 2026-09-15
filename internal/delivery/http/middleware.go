package http

import (
	"context"
	"net/http"

	"chat-backend/internal/domain"
	"chat-backend/internal/usecase"
)

type contextKey string

const UserContextKey contextKey = "currentUser"

type AuthMiddleware struct {
	userUseCase usecase.UserUseCase
}

func NewAuthMiddleware(userUseCase usecase.UserUseCase) *AuthMiddleware {
	return &AuthMiddleware{userUseCase: userUseCase}
}

func (m *AuthMiddleware) HeaderAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		login := r.Header.Get("Login")
		if login == "" {
			login = r.URL.Query().Get("login")
		}

		if login == "" {
			RespondError(w, http.StatusUnauthorized, "Login header or query param is required")
			return
		}

		user, err := m.userUseCase.GetByLogin(r.Context(), login)
		if err != nil {
			RespondError(w, http.StatusUnauthorized, "Unauthorized: user not found in system")
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetUserFromContext(ctx context.Context) (*domain.User, error) {
	user, ok := ctx.Value(UserContextKey).(*domain.User)
	if !ok || user == nil {
		return nil, domain.ErrUnauthorized
	}
	return user, nil
}
