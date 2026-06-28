package app

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := ""
		if s != nil && s.cfg != nil {
			token = s.cfg.APIToken
		}
		if token == "" {
			next(w, r)
			return
		}
		if !validBearerToken(r, token) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func validBearerToken(r *http.Request, expected string) bool {
	provided := strings.TrimSpace(r.Header.Get("X-7review-Token"))
	if provided == "" {
		auth := strings.TrimSpace(r.Header.Get("Authorization"))
		if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			provided = strings.TrimSpace(auth[len("Bearer "):])
		}
	}
	if provided == "" || expected == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}
