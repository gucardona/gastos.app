package handlers

import (
	"encoding/json"
	"gastos/src/db"
	"gastos/src/middleware"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func Register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.Name == "" || body.Email == "" || len(body.Password) < 6 {
		jsonError(w, "Dados inválidos", 400)
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(body.Password), 12)
	res, err := db.DB.Exec(
		`INSERT INTO users(name,email,password) VALUES(?,?,?)`,
		body.Name, body.Email, string(hash))
	if err != nil {
		jsonError(w, "E-mail já cadastrado", 409)
		return
	}
	id, _ := res.LastInsertId()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tokenResponse(id, body.Name, body.Email))
}

func Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	var id int64
	var name, hash string
	err := db.DB.QueryRow(
		`SELECT id,name,password FROM users WHERE email=?`, body.Email).
		Scan(&id, &name, &hash)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Password)) != nil {
		jsonError(w, "Credenciais inválidas", 401)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tokenResponse(id, name, body.Email))
}

func tokenResponse(id int64, name, email string) map[string]any {
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": id,
		"exp": time.Now().Add(30 * 24 * time.Hour).Unix(),
	})
	token, _ := tok.SignedString(middleware.JWTSecret)
	return map[string]any{
		"token": token,
		"user":  map[string]any{"id": id, "name": name, "email": email},
	}
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
