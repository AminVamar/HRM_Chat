package http

import (
	"net/http"
	"os"
	"strings"
)

const defaultCORSOrigins = "http://localhost:3000,http://127.0.0.1:3000"

// CORS разрешает запросы с фронтенда из CORS_ORIGINS (через запятую).
// Preflight OPTIONS отвечаем здесь, до роутера и авторизации.
func CORS(next http.Handler) http.Handler {
	raw := os.Getenv("CORS_ORIGINS")
	if raw == "" {
		raw = defaultCORSOrigins
	}
	allowed := make(map[string]bool)
	for _, o := range strings.Split(raw, ",") {
		if o = strings.TrimSpace(o); o != "" {
			allowed[o] = true
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && allowed[origin] {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Add("Vary", "Origin")
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Content-Type, Login, Authorization")
			h.Set("Access-Control-Expose-Headers", "Content-Disposition")
			h.Set("Access-Control-Max-Age", "600")
		}

		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
