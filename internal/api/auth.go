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
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"gastos/internal/api/httpx"
)

const (
	sessionCookie = "gastos_session"
	sessionTTL    = 30 * 24 * time.Hour
	sessionRenew  = 15 * 24 * time.Hour
	tsLayout      = "2006-01-02T15:04:05Z"
)

// userDTO is a user as the API returns it.
type userDTO struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// registrationMode says who may sign up right now.
type registrationMode string

const (
	modeOpen   registrationMode = "open"   // anyone can sign up (only while the database has no users)
	modeCode   registrationMode = "code"   // signing up requires the invite code
	modeClosed registrationMode = "closed" // nobody can sign up
)

// modeFor decides who may sign up: anyone while the database has no users (so the first account can be
// created), afterwards only people holding an invite code, if one can be used (GASTOS_INVITE_CODE or a valid
// single-use code).
func modeFor(userCount int, codeAvailable bool) registrationMode {
	switch {
	case userCount == 0:
		return modeOpen
	case codeAvailable:
		return modeCode
	default:
		return modeClosed
	}
}

// errSignupClosed is the same for a missing and a wrong code, so it reveals nothing about the code.
var errSignupClosed = &httpx.APIError{Status: 403, Code: "forbidden", Message: "O cadastro está fechado. Peça um convite."}

// currentMode works out the registration mode from the number of users and the unused invite codes.
func (s *Server) currentMode(ctx context.Context, q httpx.Querier) (registrationMode, error) {
	var users int
	var dbCode bool
	if err := q.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM users),
		EXISTS (SELECT 1 FROM invite_codes WHERE used_at IS NULL AND expires_at > ?)`,
		s.Now().UTC().Format(tsLayout)).Scan(&users, &dbCode); err != nil {
		return "", err
	}
	return modeFor(users, s.Cfg.InviteCode != "" || dbCode), nil
}

// checkRegistration applies the sign-up policy. q is the transaction when called from inside one,
// so two simultaneous "first" sign-ups cannot both succeed. dbCode reports that the code is a valid
// single-use one, which the caller must consume (useInvite) in the transaction that creates the user.
func (s *Server) checkRegistration(ctx context.Context, q httpx.Querier, code string) (dbCode bool, err error) {
	mode, err := s.currentMode(ctx, q)
	if err != nil {
		return false, err
	}
	switch mode {
	case modeOpen:
		return false, nil
	case modeCode:
		if s.Cfg.InviteCode != "" && subtle.ConstantTimeCompare([]byte(code), []byte(s.Cfg.InviteCode)) == 1 {
			return false, nil
		}
		var ok bool
		if err := q.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM invite_codes WHERE code_hash = ? AND used_at IS NULL AND expires_at > ?)`,
			inviteHash(code), s.Now().UTC().Format(tsLayout)).Scan(&ok); err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, errSignupClosed
}

// useInvite marks a single-use code as used by userID; the conditional update lets only one sign-up have it.
func (s *Server) useInvite(ctx context.Context, tx *sql.Tx, code string, userID int64) error {
	now := s.Now().UTC().Format(tsLayout)
	res, err := tx.ExecContext(ctx, `UPDATE invite_codes SET used_at = ?, used_by = ?
		WHERE code_hash = ? AND used_at IS NULL AND expires_at > ?`, now, userID, inviteHash(code), now)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errSignupClosed
	}
	return nil
}

// registrationInfo tells the sign-in screen what to show. It is public and reveals only the mode.
func (s *Server) registrationInfo(w http.ResponseWriter, r *http.Request) error {
	mode, err := s.currentMode(r.Context(), s.DB)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]registrationMode{"mode": mode})
	return nil
}

// register creates an account, signs it in and creates its first wallet.
func (s *Server) register(w http.ResponseWriter, r *http.Request) error {
	if !s.limiter.Allow("register:ip:" + clientIP(r)) {
		return httpx.ErrRateLimited
	}
	var in struct {
		Name       string `json:"name"`
		Email      string `json:"email"`
		Password   string `json:"password"`
		InviteCode string `json:"inviteCode"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	// refuse early, before spending a bcrypt hash; the check is repeated inside the transaction
	if _, err := s.checkRegistration(r.Context(), s.DB, in.InviteCode); err != nil {
		return err
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	fe := httpx.FieldErrors{}
	if in.Name == "" {
		fe.Add("name", "obrigatório")
	}
	if _, err := mail.ParseAddress(in.Email); err != nil {
		fe.Add("email", "e-mail inválido")
	}
	switch {
	case len(in.Password) < 8:
		fe.Add("password", "mínimo de 8 caracteres")
	case len(in.Password) > 72: // bcrypt refuses longer passwords
		fe.Add("password", "máximo de 72 caracteres")
	}
	if err := fe.Err(); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), _bcryptCost)
	if err != nil {
		return err
	}

	var userID int64
	err = s.InTx(r.Context(), func(tx *sql.Tx) error {
		dbCode, err := s.checkRegistration(r.Context(), tx, in.InviteCode)
		if err != nil {
			return err
		}
		err = tx.QueryRowContext(r.Context(),
			`INSERT INTO users (name, email, password_hash) VALUES (?, ?, ?) RETURNING id`,
			in.Name, in.Email, string(hash)).Scan(&userID)
		if err != nil {
			if httpx.IsUnique(err) {
				return httpx.ErrConflict("E-mail já cadastrado")
			}
			return err
		}
		if dbCode {
			if err := s.useInvite(r.Context(), tx, in.InviteCode, userID); err != nil {
				return err
			}
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
	httpx.WriteJSON(w, http.StatusCreated, userDTO{userID, in.Name, in.Email})
	return nil
}

// login checks the e-mail and password and starts a session, rate-limited by IP and by e-mail.
func (s *Server) login(w http.ResponseWriter, r *http.Request) error {
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

var errBadLogin = &httpx.APIError{Status: 401, Code: "unauthenticated", Message: "E-mail ou senha incorretos"}

// _dummyHash is a valid bcrypt hash of an arbitrary password, used to equalize response time.
var _dummyHash = func() string {
	h, _ := bcrypt.GenerateFromPassword([]byte("dummy"), _bcryptCost)
	return string(h)
}()

// logout deletes the current session and clears its cookie.
func (s *Server) logout(w http.ResponseWriter, r *http.Request) error {
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
func (s *Server) me(w http.ResponseWriter, r *http.Request) error {
	var u userDTO
	if err := s.DB.QueryRowContext(r.Context(), `SELECT id, name, email FROM users WHERE id = ?`,
		httpx.UserIDFrom(r.Context())).Scan(&u.ID, &u.Name, &u.Email); err != nil {
		return err
	}
	wallets, err := s.walletsFor(r.Context(), u.ID)
	if err != nil {
		return err
	}
	_, _, aiErr := s.userAIKey(r.Context(), u.ID)
	if aiErr != nil && !errors.Is(aiErr, errAIDisabled) && !errors.Is(aiErr, errAIKeyUnreadable) {
		return aiErr
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"user": u, "wallets": wallets, "assistantEnabled": aiErr == nil})
	return nil
}

const maxNameLen = 60

// rename changes the caller's own name; the other members of their wallets are told to reload.
func (s *Server) rename(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Name string `json:"name"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	in.Name = strings.TrimSpace(in.Name)
	fe := httpx.FieldErrors{}
	if in.Name == "" {
		fe.Add("name", "obrigatório")
	} else if utf8.RuneCountInString(in.Name) > maxNameLen {
		fe.Add("name", "use até 60 caracteres")
	}
	if err := fe.Err(); err != nil {
		return err
	}
	uid := httpx.UserIDFrom(r.Context())
	var u userDTO
	if err := s.DB.QueryRowContext(r.Context(), `UPDATE users SET name = ? WHERE id = ? RETURNING id, name, email`,
		in.Name, uid).Scan(&u.ID, &u.Name, &u.Email); err != nil {
		return err
	}
	s.NotifyUsers(uid) // their other tabs, even with no wallet left
	ctx, cancel := httpx.NotifyCtx(r.Context())
	defer cancel()
	wallets, err := s.walletsFor(ctx, uid)
	if err != nil {
		s.Log.ErrorContext(ctx, "notify co-members", "user", uid, "err", err) // the new name is saved: only the notice is lost
	}
	for _, wl := range wallets {
		s.NotifyWallet(ctx, wl.ID)
	}
	httpx.WriteJSON(w, http.StatusOK, u)
	return nil
}

// changePassword replaces the caller's password after checking the current one, and ends their other sessions
// (the one that made the request stays signed in), so a leaked provisional password stops working.
func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	uid := httpx.UserIDFrom(r.Context())
	if !s.limiter.Allow("password:user:" + strconv.FormatInt(uid, 10)) { // stops guessing the current password
		return httpx.ErrRateLimited
	}
	fe := httpx.FieldErrors{}
	switch {
	case len(in.NewPassword) < 8:
		fe.Add("newPassword", "mínimo de 8 caracteres")
	case len(in.NewPassword) > 72: // bcrypt ignores everything after 72 bytes
		fe.Add("newPassword", "máximo de 72 caracteres")
	case in.NewPassword == in.CurrentPassword:
		fe.Add("newPassword", "use uma senha diferente da atual")
	}
	var hash string
	if err := s.DB.QueryRowContext(r.Context(), `SELECT password_hash FROM users WHERE id = ?`, uid).Scan(&hash); err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.CurrentPassword)) != nil {
		fe.Add("currentPassword", "senha atual incorreta")
	}
	if err := fe.Err(); err != nil {
		return err
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), _bcryptCost)
	if err != nil {
		return err
	}
	keep := sha256.Sum256(nil)
	if c, err := r.Cookie(sessionCookie); err == nil {
		keep = sha256.Sum256([]byte(c.Value))
	}
	if err := s.InTx(r.Context(), func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(r.Context(), `UPDATE users SET password_hash = ? WHERE id = ?`, string(newHash), uid); err != nil {
			return err
		}
		_, err := tx.ExecContext(r.Context(), `DELETE FROM sessions WHERE user_id = ? AND token_hash != ?`, uid, keep[:])
		return err
	}); err != nil {
		return err
	}
	s.NotifyUsers(uid) // the sessions just ended refetch, get a 401 and go to the login right away
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// startSession creates a random session token, stores its SHA-256 hash and sets the cookie.
func (s *Server) startSession(ctx context.Context, w http.ResponseWriter, userID int64) error {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(token))
	exp := s.Now().UTC().Add(sessionTTL)
	if _, err := s.DB.ExecContext(ctx, `INSERT INTO sessions (token_hash, user_id, expires_at) VALUES (?, ?, ?)`,
		h[:], userID, exp.Format(tsLayout)); err != nil {
		return err
	}
	s.setSessionCookie(w, token, exp)
	return nil
}

// setSessionCookie sets the session cookie: HttpOnly, SameSite=Lax, and Secure unless
// GASTOS_COOKIE_SECURE is false.
func (s *Server) setSessionCookie(w http.ResponseWriter, token string, exp time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/", Expires: exp,
		HttpOnly: true, Secure: s.Cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

// requireUser rejects requests without a valid session and stores the user id in the context.
func (s *Server) requireUser(next http.Handler) http.Handler {
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
			h[:], now.Format(tsLayout)).Scan(&userID, &expStr)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				err = httpx.ErrUnauthenticated
			}
			s.WriteError(w, r, err)
			return
		}
		if exp, _ := time.Parse(tsLayout, expStr); exp.Sub(now) < sessionRenew {
			newExp := now.Add(sessionTTL)
			if _, err := s.DB.ExecContext(r.Context(), `UPDATE sessions SET expires_at = ? WHERE token_hash = ?`,
				newExp.Format(tsLayout), h[:]); err == nil {
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
