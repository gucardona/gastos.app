package auth

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"net/http"
	"net/mail"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"gastos/internal/api/httpx"
	"gastos/internal/api/wallet"
)

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
func (s *Handler) currentMode(ctx context.Context, q httpx.Querier) (registrationMode, error) {
	var users int
	var dbCode bool
	if err := q.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM users),
		EXISTS (SELECT 1 FROM invite_codes WHERE used_at IS NULL AND expires_at > ?)`,
		s.Now().UTC().Format(httpx.TSLayout)).Scan(&users, &dbCode); err != nil {
		return "", err
	}
	return modeFor(users, s.Cfg.InviteCode != "" || dbCode), nil
}

// checkRegistration applies the sign-up policy. q is the transaction when called from inside one,
// so two simultaneous "first" sign-ups cannot both succeed. dbCode reports that the code is a valid
// single-use one, which the caller must consume (useInvite) in the transaction that creates the user.
func (s *Handler) checkRegistration(ctx context.Context, q httpx.Querier, code string) (dbCode bool, err error) {
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
			inviteHash(code), s.Now().UTC().Format(httpx.TSLayout)).Scan(&ok); err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, errSignupClosed
}

// useInvite marks a single-use code as used by userID; the conditional update lets only one sign-up have it.
func (s *Handler) useInvite(ctx context.Context, tx *sql.Tx, code string, userID int64) error {
	now := s.Now().UTC().Format(httpx.TSLayout)
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
func (s *Handler) registrationInfo(w http.ResponseWriter, r *http.Request) error {
	mode, err := s.currentMode(r.Context(), s.DB)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]registrationMode{"mode": mode})
	return nil
}

// register creates an account, signs it in and creates its first wallet.
func (s *Handler) register(w http.ResponseWriter, r *http.Request) error {
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
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), BcryptCost)
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
		_, err = wallet.CreateWallet(r.Context(), tx, "Pessoal", userID, false)
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
