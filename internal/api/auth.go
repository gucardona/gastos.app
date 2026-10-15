package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"net/mail"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

const (
	sessionCookie = "gastos_session"
	sessionTTL    = 30 * 24 * time.Hour
	sessionRenew  = 15 * 24 * time.Hour
	tsLayout      = "2006-01-02T15:04:05Z"
)

type ctxKey int

const (
	ctxUserID ctxKey = iota
	ctxWalletID
	ctxRole
)

func userIDFrom(ctx context.Context) int64   { v, _ := ctx.Value(ctxUserID).(int64); return v }
func walletIDFrom(ctx context.Context) int64 { v, _ := ctx.Value(ctxWalletID).(int64); return v }
func roleFrom(ctx context.Context) role      { v, _ := ctx.Value(ctxRole).(role); return v }

type userDTO struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type registrationMode string

const (
	modeOpen   registrationMode = "open"   // anyone can sign up (only while the database has no users)
	modeCode   registrationMode = "code"   // signing up requires the invite code
	modeClosed registrationMode = "closed" // nobody can sign up
)

// modeFor decides who may sign up: anyone while the database has no users (so the first account can be
// created), afterwards only people holding the invite code, if one is configured.
func modeFor(userCount int, inviteCode string) registrationMode {
	switch {
	case userCount == 0:
		return modeOpen
	case inviteCode != "":
		return modeCode
	default:
		return modeClosed
	}
}

// errSignupClosed is the same for a missing and a wrong code, so it reveals nothing about the code.
var errSignupClosed = &apiError{Status: 403, Code: "forbidden", Message: "O cadastro está fechado. Peça um convite."}

func (s *Server) currentMode(ctx context.Context, q querier) (registrationMode, error) {
	var n int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return "", err
	}
	return modeFor(n, s.cfg.InviteCode), nil
}

// checkRegistration applies the sign-up policy. q is the transaction when called from inside one,
// so two simultaneous "first" sign-ups cannot both succeed.
func (s *Server) checkRegistration(ctx context.Context, q querier, code string) error {
	mode, err := s.currentMode(ctx, q)
	if err != nil {
		return err
	}
	switch mode {
	case modeOpen:
		return nil
	case modeCode:
		if subtle.ConstantTimeCompare([]byte(code), []byte(s.cfg.InviteCode)) == 1 {
			return nil
		}
	}
	return errSignupClosed
}

// registrationInfo tells the sign-in screen what to show. It is public and reveals only the mode.
func (s *Server) registrationInfo(w http.ResponseWriter, r *http.Request) error {
	mode, err := s.currentMode(r.Context(), s.db)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]registrationMode{"mode": mode})
	return nil
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) error {
	if !s.limiter.allow("register:ip:" + clientIP(r)) {
		return errRateLimited
	}
	var in struct {
		Name       string `json:"name"`
		Email      string `json:"email"`
		Password   string `json:"password"`
		InviteCode string `json:"inviteCode"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	// refuse early, before spending a bcrypt hash; the check is repeated inside the transaction
	if err := s.checkRegistration(r.Context(), s.db, in.InviteCode); err != nil {
		return err
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	fe := fieldErrors{}
	if in.Name == "" {
		fe.add("name", "obrigatório")
	}
	if _, err := mail.ParseAddress(in.Email); err != nil {
		fe.add("email", "e-mail inválido")
	}
	if len(in.Password) < 8 {
		fe.add("password", "mínimo de 8 caracteres")
	}
	if err := fe.err(); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcryptCost)
	if err != nil {
		return err
	}

	var userID int64
	err = s.inTx(r.Context(), func(tx *sql.Tx) error {
		if err := s.checkRegistration(r.Context(), tx, in.InviteCode); err != nil {
			return err
		}
		err := tx.QueryRowContext(r.Context(),
			`INSERT INTO users (name, email, password_hash) VALUES (?, ?, ?) RETURNING id`,
			in.Name, in.Email, string(hash)).Scan(&userID)
		if err != nil {
			if isUnique(err) {
				return errConflict("E-mail já cadastrado")
			}
			return err
		}
		_, err = createWallet(r.Context(), tx, "Pessoal", userID, false)
		return err
	})
	if err != nil {
		return err
	}
	if err := s.startSession(r.Context(), w, userID); err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, userDTO{userID, in.Name, in.Email})
	return nil
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if !s.limiter.allow("ip:"+clientIP(r)) || !s.limiter.allow("email:"+in.Email) {
		return errRateLimited
	}
	var u userDTO
	var hash string
	err := s.db.QueryRowContext(r.Context(),
		`SELECT id, name, email, password_hash FROM users WHERE email = ?`, in.Email).
		Scan(&u.ID, &u.Name, &u.Email, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		// spend the same time so the response does not reveal whether the email exists
		bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(in.Password))
		return errBadLogin
	}
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)) != nil {
		return errBadLogin
	}
	if err := s.startSession(r.Context(), w, u.ID); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, u)
	return nil
}

var errBadLogin = &apiError{Status: 401, Code: "unauthenticated", Message: "E-mail ou senha incorretos"}

// dummyHash is a valid bcrypt hash of an arbitrary password, used to equalize response time.
var dummyHash = func() string {
	h, _ := bcrypt.GenerateFromPassword([]byte("dummy"), bcryptCost)
	return string(h)
}()

func (s *Server) logout(w http.ResponseWriter, r *http.Request) error {
	if c, err := r.Cookie(sessionCookie); err == nil {
		h := sha256.Sum256([]byte(c.Value))
		if _, err := s.db.ExecContext(r.Context(), `DELETE FROM sessions WHERE token_hash = ?`, h[:]); err != nil {
			return err
		}
	}
	s.setSessionCookie(w, "", time.Unix(0, 0))
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) error {
	var u userDTO
	if err := s.db.QueryRowContext(r.Context(), `SELECT id, name, email FROM users WHERE id = ?`,
		userIDFrom(r.Context())).Scan(&u.ID, &u.Name, &u.Email); err != nil {
		return err
	}
	wallets, err := s.walletsFor(r.Context(), u.ID)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": u, "wallets": wallets})
	return nil
}

const maxNameLen = 60

// rename changes the caller's own name; the other members of their wallets are told to reload.
func (s *Server) rename(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Name string `json:"name"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	in.Name = strings.TrimSpace(in.Name)
	fe := fieldErrors{}
	if in.Name == "" {
		fe.add("name", "obrigatório")
	} else if utf8.RuneCountInString(in.Name) > maxNameLen {
		fe.add("name", "use até 60 caracteres")
	}
	if err := fe.err(); err != nil {
		return err
	}
	uid := userIDFrom(r.Context())
	var u userDTO
	if err := s.db.QueryRowContext(r.Context(), `UPDATE users SET name = ? WHERE id = ? RETURNING id, name, email`,
		in.Name, uid).Scan(&u.ID, &u.Name, &u.Email); err != nil {
		return err
	}
	wallets, err := s.walletsFor(r.Context(), uid)
	if err != nil {
		return err
	}
	for _, wl := range wallets {
		s.bus.Notify(wl.ID, uid)
	}
	writeJSON(w, http.StatusOK, u)
	return nil
}

func (s *Server) startSession(ctx context.Context, w http.ResponseWriter, userID int64) error {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(token))
	exp := s.now().UTC().Add(sessionTTL)
	if _, err := s.db.ExecContext(ctx, `INSERT INTO sessions (token_hash, user_id, expires_at) VALUES (?, ?, ?)`,
		h[:], userID, exp.Format(tsLayout)); err != nil {
		return err
	}
	s.setSessionCookie(w, token, exp)
	return nil
}

func (s *Server) setSessionCookie(w http.ResponseWriter, token string, exp time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/", Expires: exp,
		HttpOnly: true, Secure: s.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) requireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil {
			s.writeError(w, r, errUnauthenticated)
			return
		}
		h := sha256.Sum256([]byte(c.Value))
		now := s.now().UTC()
		var userID int64
		var expStr string
		err = s.db.QueryRowContext(r.Context(),
			`SELECT user_id, expires_at FROM sessions WHERE token_hash = ? AND expires_at > ?`,
			h[:], now.Format(tsLayout)).Scan(&userID, &expStr)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				err = errUnauthenticated
			}
			s.writeError(w, r, err)
			return
		}
		if exp, _ := time.Parse(tsLayout, expStr); exp.Sub(now) < sessionRenew {
			newExp := now.Add(sessionTTL)
			if _, err := s.db.ExecContext(r.Context(), `UPDATE sessions SET expires_at = ? WHERE token_hash = ?`,
				newExp.Format(tsLayout), h[:]); err == nil {
				s.setSessionCookie(w, c.Value, newExp)
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxUserID, userID)))
	})
}

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

type rateLimiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	hits   map[string][]time.Time
}

func newRateLimiter(max int, window time.Duration) *rateLimiter {
	return &rateLimiter{max: max, window: window, hits: map[string][]time.Time{}}
}

// allow records an attempt and reports whether it is still within the limit.
func (l *rateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	kept := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if now.Sub(t) < l.window {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.max {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	return true
}
