package handlers

import (
	"database/sql"
	"fmt"
	"gastos/src/db"
	"gastos/src/events"
	"gastos/src/middleware"
	"net/http"
	"strconv"

	"github.com/golang-jwt/jwt/v5"
)

func Events(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	tok, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		return middleware.JWTSecret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || !tok.Valid {
		jsonError(w, "invalid token", http.StatusUnauthorized)
		return
	}
	claims, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		jsonError(w, "invalid token", http.StatusUnauthorized)
		return
	}
	sub, ok := claims["sub"].(float64)
	if !ok || sub <= 0 {
		jsonError(w, "invalid token", http.StatusUnauthorized)
		return
	}
	userID := int64(sub)

	accountIDStr := r.URL.Query().Get("account_id")
	accountID, err := strconv.ParseInt(accountIDStr, 10, 64)
	if err != nil || accountID <= 0 {
		jsonError(w, "invalid account_id", http.StatusBadRequest)
		return
	}

	var role string
	err = db.DB.QueryRow(`SELECT role FROM account_members WHERE account_id = ? AND user_id = ?`, accountID, userID).Scan(&role)
	if err != nil {
		if err == sql.ErrNoRows {
			jsonError(w, "account access denied", http.StatusForbidden)
		} else {
			jsonError(w, "server error", http.StatusInternalServerError)
		}
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		jsonError(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := events.Bus.Subscribe(accountID, userID)
	defer events.Bus.Unsubscribe(accountID, ch)

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ch:
			fmt.Fprintf(w, "data: refresh\n\n")
			flusher.Flush()
		}
	}
}
