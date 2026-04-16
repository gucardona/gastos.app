package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"gastos/src/db"
	"gastos/src/middleware"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	errInvalidJSON = errors.New("JSON inválido")
	errInvalidID   = errors.New("ID inválido")
)

func Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST, OPTIONS")
		jsonError(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &body); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	body.Name = strings.TrimSpace(body.Name)
	body.Email = strings.TrimSpace(strings.ToLower(body.Email))
	if body.Name == "" || body.Email == "" || len(body.Password) < 6 {
		jsonError(w, "Dados inválidos", http.StatusBadRequest)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), 12)
	if err != nil {
		jsonError(w, "Erro ao processar senha", http.StatusInternalServerError)
		return
	}

	tx, err := db.DB.Begin()
	if err != nil {
		jsonError(w, "Erro ao criar usuário", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		`INSERT INTO users(name,email,password) VALUES(?,?,?)`,
		body.Name, body.Email, string(hash),
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			jsonError(w, "E-mail já cadastrado", http.StatusConflict)
			return
		}
		jsonError(w, "Erro ao criar usuário", http.StatusInternalServerError)
		return
	}

	id, err := res.LastInsertId()
	if err != nil {
		jsonError(w, "Erro ao obter usuário criado", http.StatusInternalServerError)
		return
	}

	if _, err := db.CreateAccountWithOwner(tx, "Pessoal", id); err != nil {
		jsonError(w, "Erro ao criar conta padrão", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		jsonError(w, "Erro ao finalizar cadastro", http.StatusInternalServerError)
		return
	}

	resp, err := tokenResponse(id, body.Name, body.Email)
	if err != nil {
		jsonError(w, "Erro ao gerar token", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, resp)
}

func Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST, OPTIONS")
		jsonError(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &body); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	body.Email = strings.TrimSpace(strings.ToLower(body.Email))

	var id int64
	var name, hash string
	err := db.DB.QueryRow(
		`SELECT id,name,password FROM users WHERE email=?`, body.Email,
	).Scan(&id, &name, &hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			jsonError(w, "Credenciais inválidas", http.StatusUnauthorized)
			return
		}
		jsonError(w, "Erro ao buscar usuário", http.StatusInternalServerError)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Password)) != nil {
		jsonError(w, "Credenciais inválidas", http.StatusUnauthorized)
		return
	}

	resp, err := tokenResponse(id, name, body.Email)
	if err != nil {
		jsonError(w, "Erro ao gerar token", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func tokenResponse(id int64, name, email string) (map[string]any, error) {
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": id,
		"exp": time.Now().Add(30 * 24 * time.Hour).Unix(),
	})
	token, err := tok.SignedString(middleware.JWTSecret)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"token": token,
		"user":  map[string]any{"id": id, "name": name, "email": email},
	}, nil
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}
