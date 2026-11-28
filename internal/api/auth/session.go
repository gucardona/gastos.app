package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"gastos/internal/api/assistant"
	"gastos/internal/api/httpx"
)

// BcryptCost is a variable so tests can use a low cost.
var BcryptCost = 12

// Session cookie name and lifetimes: a session lasts sessionTTL and is renewed once it is older than sessionRenew.
const (
	sessionCookie = "gastos_session"
	sessionTTL    = 30 * 24 * time.Hour
	sessionRenew  = 15 * 24 * time.Hour
)

// userDTO is a user as the API returns it.
type userDTO struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// login checks the e-mail and password and starts a session, rate-limited by IP and by e-mail.
func (s *Handler) login(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if !s.limiter.Allow("ip:" + clientIP(r)) {
		return httpx.ErrRateLimited
	}
	// The email key counts failures only and never refuses the right password, so strangers cannot lock a user out.
	failed := func() error {
		if !s.limiter.Allow("email:" + in.Email) {
			return httpx.ErrRateLimited
		}
		return errBadLogin
	}
	var u userDTO
	var hash string
	err := s.DB.QueryRowContext(r.Context(),
		`SELECT id, name, email, password_hash FROM users WHERE email = ?`, in.Email).
		Scan(&u.ID, &u.Name, &u.Email, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		// spend the same time so the response does not reveal whether the email exists
		_ = bcrypt.CompareHashAndPassword([]byte(_dummyHash), []byte(in.Password))
		return failed()
	}
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil {
		return failed()
	}
	if err := s.startSession(r.Context(), w, u.ID); err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, u)
	return nil
}

// errBadLogin is the one answer for an unknown e-mail and for a wrong password.
var errBadLogin = &httpx.APIError{Status: 401, Code: "unauthenticated", Message: "E-mail ou senha incorretos"}

// _dummyHash is a valid bcrypt hash of an arbitrary password, used to equalize response time.
var _dummyHash = func() string {
	h, _ := bcrypt.GenerateFromPassword([]byte("dummy"), BcryptCost)
	return string(h)
}()

// logout deletes the current session and clears its cookie.
func (s *Handler) logout(w http.ResponseWriter, r *http.Request) error {
	if c, err := r.Cookie(sessionCookie); err == nil {
		h := sha256.Sum256([]byte(c.Value))
		if _, err := s.DB.ExecContext(r.Context(), `DELETE FROM sessions WHERE token_hash = ?`, h[:]); err != nil {
			return err
		}
	}
	s.setSessionCookie(w, "", time.Unix(0, 0))
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// me returns the signed-in user.
func (s *Handler) me(w http.ResponseWriter, r *http.Request) error {
	var u userDTO
	if err := s.DB.QueryRowContext(r.Context(), `SELECT id, name, email FROM users WHERE id = ?`,
		httpx.UserIDFrom(r.Context())).Scan(&u.ID, &u.Name, &u.Email); err != nil {
		return err
	}
	wallets, err := s.Wallet.WalletsFor(r.Context(), u.ID)
	if err != nil {
		return err
	}
	_, _, aiErr := s.AI.UserAIKey(r.Context(), u.ID)
	if aiErr != nil && !errors.Is(aiErr, assistant.ErrAIDisabled) && !errors.Is(aiErr, assistant.ErrAIKeyUnreadable) {
		return aiErr
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"user": u, "wallets": wallets, "assistantEnabled": aiErr == nil})
	return nil
}

// startSession creates a random session token, stores its SHA-256 hash and sets the cookie.
func (s *Handler) startSession(ctx context.Context, w http.ResponseWriter, userID int64) error {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(token))
	exp := s.Now().UTC().Add(sessionTTL)
	if _, err := s.DB.ExecContext(ctx, `INSERT INTO sessions (token_hash, user_id, expires_at) VALUES (?, ?, ?)`,
		h[:], userID, exp.Format(httpx.TSLayout)); err != nil {
		return err
	}
	s.setSessionCookie(w, token, exp)
	return nil
}

// setSessionCookie sets the session cookie: HttpOnly, SameSite=Lax, and Secure unless
// GASTOS_COOKIE_SECURE is false.
func (s *Handler) setSessionCookie(w http.ResponseWriter, token string, exp time.Time) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // moved unchanged, see docs/security.md
		Name: sessionCookie, Value: token, Path: "/", Expires: exp,
		HttpOnly: true, Secure: s.Cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

// RequireUser rejects requests without a valid session and stores the user id in the context.
func (s *Handler) RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil {
			s.WriteError(w, r, httpx.ErrUnauthenticated)
			return
		}
		h := sha256.Sum256([]byte(c.Value))
		now := s.Now().UTC()
		var userID int64
		var expStr string
		err = s.DB.QueryRowContext(r.Context(),
			`SELECT user_id, expires_at FROM sessions WHERE token_hash = ? AND expires_at > ?`,
			h[:], now.Format(httpx.TSLayout)).Scan(&userID, &expStr)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				err = httpx.ErrUnauthenticated
			}
			s.WriteError(w, r, err)
			return
		}
		if exp, _ := time.Parse(httpx.TSLayout, expStr); exp.Sub(now) < sessionRenew {
			newExp := now.Add(sessionTTL)
			if _, err := s.DB.ExecContext(r.Context(), `UPDATE sessions SET expires_at = ? WHERE token_hash = ?`,
				newExp.Format(httpx.TSLayout), h[:]); err == nil {
				s.setSessionCookie(w, c.Value, newExp)
			}
		}
		next.ServeHTTP(w, r.WithContext(httpx.WithUser(r.Context(), userID)))
	})
}

// clientIP returns the caller's IP, trusting X-Forwarded-For only from a proxy on loopback.
func clientIP(r *http.Request) string {
	// Behind Caddy/nginx on localhost: only trust X-Forwarded-For when the peer is loopback.
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			return strings.TrimSpace(strings.Split(xff, ",")[0])
		}
	}
	return host
}
