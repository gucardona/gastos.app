package auth

import (
	"crypto/sha256"
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"gastos/internal/api/httpx"
)

const maxNameLen = 60

// rename changes the caller's own name; the other members of their wallets are told to reload.
func (s *Handler) rename(w http.ResponseWriter, r *http.Request) error {
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
	wallets, err := s.Wallet.WalletsFor(ctx, uid)
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
func (s *Handler) changePassword(w http.ResponseWriter, r *http.Request) error {
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
	newHash, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), BcryptCost)
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
