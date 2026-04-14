package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

var JWTSecret = []byte("troque-por-segredo-forte")

type ctxKey string

const UserIDKey ctxKey = "userID"

func Auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bearer := r.Header.Get("Authorization")
		if !strings.HasPrefix(bearer, "Bearer ") {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		tok, err := jwt.Parse(strings.TrimPrefix(bearer, "Bearer "),
			func(t *jwt.Token) (any, error) { return JWTSecret, nil })
		if err != nil || !tok.Valid {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}
		claims := tok.Claims.(jwt.MapClaims)
		uid := int64(claims["sub"].(float64))
		ctx := context.WithValue(r.Context(), UserIDKey, uid)
		next(w, r.WithContext(ctx))
	}
}
