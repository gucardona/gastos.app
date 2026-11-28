package wallet

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"gastos/internal/api/httpx"
	"gastos/internal/money"
)

// memberDTO is a wallet member as the API returns it.
type memberDTO struct {
	UserID          int64  `json:"userId"`
	Name            string `json:"name"`
	Email           string `json:"email"`
	Role            string `json:"role"`
	SplitsByDefault bool   `json:"splitsByDefault"`
	DefaultShareBP  int    `json:"defaultShareBp"` // 0 = not set
	// the method a new transaction of theirs starts with; nil = the first one
	DefaultPaymentMethodID *int64 `json:"defaultPaymentMethodId"`
}

// memberByID returns userID's membership in walletID.
func (s *Handler) memberByID(ctx context.Context, walletID, userID int64) (memberDTO, error) {
	var m memberDTO
	err := s.DB.QueryRowContext(ctx, `
		SELECT u.id, u.name, u.email, wm.role, wm.splits_by_default, COALESCE(wm.default_share_bp, 0), wm.default_payment_method_id
		FROM wallet_members wm JOIN users u ON u.id = wm.user_id
		WHERE wm.wallet_id = ? AND wm.user_id = ?`, walletID, userID).
		Scan(&m.UserID, &m.Name, &m.Email, &m.Role, &m.SplitsByDefault, &m.DefaultShareBP, &m.DefaultPaymentMethodID)
	return m, err
}

// listMembers handles GET /members.
func (s *Handler) listMembers(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.DB.QueryContext(r.Context(), `
		SELECT u.id, u.name, u.email, wm.role, wm.splits_by_default, COALESCE(wm.default_share_bp, 0), wm.default_payment_method_id
		FROM wallet_members wm JOIN users u ON u.id = wm.user_id
		WHERE wm.wallet_id = ?
		ORDER BY CASE wm.role WHEN 'owner' THEN 0 WHEN 'editor' THEN 1 ELSE 2 END, u.name, u.id`,
		httpx.WalletIDFrom(r.Context()))
	if err != nil {
		return err
	}
	defer rows.Close() //nolint:errcheck,gosec // moved unchanged, see docs/security.md
	out := []memberDTO{}
	for rows.Next() {
		var m memberDTO
		if err := rows.Scan(&m.UserID, &m.Name, &m.Email, &m.Role, &m.SplitsByDefault, &m.DefaultShareBP, &m.DefaultPaymentMethodID); err != nil {
			return err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

// addMember handles POST /members: adds an existing user, by e-mail, as editor or reader.
func (s *Handler) addMember(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if rl, ok := httpx.ParseRole(in.Role); !ok || rl == httpx.RoleOwner {
		return httpx.ValidationError(map[string]string{"role": "deve ser editor ou reader"})
	}
	var userID int64
	if err := s.DB.QueryRowContext(r.Context(), `SELECT id FROM users WHERE email = ?`, in.Email).Scan(&userID); err != nil {
		if err == sql.ErrNoRows {
			return httpx.ErrNotFoundMsg("Usuário não encontrado")
		}
		return err
	}
	walletID := httpx.WalletIDFrom(r.Context())
	if _, err := s.DB.ExecContext(r.Context(),
		`INSERT INTO wallet_members (wallet_id, user_id, role) VALUES (?, ?, ?)`, walletID, userID, in.Role); err != nil {
		if httpx.IsUnique(err) {
			return httpx.ErrConflict("Usuário já faz parte da carteira")
		}
		return err
	}
	m, err := s.memberByID(r.Context(), walletID, userID)
	if err != nil {
		return err
	}
	s.Mutated(r)
	httpx.WriteJSON(w, http.StatusCreated, m)
	return nil
}

// updateMember handles PATCH /members/{userID}: role, splits by default and default share.
// The owner's role cannot change.
func (s *Handler) updateMember(w http.ResponseWriter, r *http.Request) error {
	target, err := httpx.PathID(r, "userID")
	if err != nil {
		return err
	}
	var in struct {
		Role            *string `json:"role"`
		SplitsByDefault *bool   `json:"splitsByDefault"`
		DefaultShareBP  *int    `json:"defaultShareBp"` // 0 clears it
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	if in.DefaultShareBP != nil && (*in.DefaultShareBP < 0 || *in.DefaultShareBP > money.FullShare) {
		return httpx.ValidationError(map[string]string{"defaultShareBp": "use entre 0 e 100%"})
	}
	walletID := httpx.WalletIDFrom(r.Context())
	cur, err := s.memberByID(r.Context(), walletID, target)
	if err != nil {
		return err
	}
	if in.Role != nil {
		if cur.Role == "owner" {
			return httpx.ErrBadRequest("O papel do owner não pode ser alterado")
		}
		if rl, ok := httpx.ParseRole(*in.Role); !ok || rl == httpx.RoleOwner {
			return httpx.ValidationError(map[string]string{"role": "deve ser editor ou reader"})
		}
		if _, err := s.DB.ExecContext(r.Context(), `UPDATE wallet_members SET role = ? WHERE wallet_id = ? AND user_id = ?`,
			*in.Role, walletID, target); err != nil {
			return err
		}
	}
	if in.SplitsByDefault != nil {
		if _, err := s.DB.ExecContext(r.Context(), `UPDATE wallet_members SET splits_by_default = ? WHERE wallet_id = ? AND user_id = ?`,
			*in.SplitsByDefault, walletID, target); err != nil {
			return err
		}
	}
	if in.DefaultShareBP != nil {
		if _, err := s.DB.ExecContext(r.Context(), `UPDATE wallet_members SET default_share_bp = NULLIF(?, 0) WHERE wallet_id = ? AND user_id = ?`,
			*in.DefaultShareBP, walletID, target); err != nil {
			return err
		}
	}
	m, err := s.memberByID(r.Context(), walletID, target)
	if err != nil {
		return err
	}
	s.Mutated(r)
	httpx.WriteJSON(w, http.StatusOK, m)
	return nil
}

// removeMember handles DELETE /members/{userID}: the owner removes a member, or a member leaves.
// The owner cannot leave, and a member still in a recurrence or a bank account must be edited out first.
func (s *Handler) removeMember(w http.ResponseWriter, r *http.Request) error {
	target, err := httpx.PathID(r, "userID")
	if err != nil {
		return err
	}
	me := httpx.UserIDFrom(r.Context())
	if httpx.RoleFrom(r.Context()) != httpx.RoleOwner && target != me {
		return httpx.ErrForbidden
	}
	walletID := httpx.WalletIDFrom(r.Context())
	cur, err := s.memberByID(r.Context(), walletID, target)
	if err != nil {
		return err
	}
	if cur.Role == "owner" {
		return httpx.ErrBadRequest("O owner não pode ser removido nem sair da carteira")
	}
	// The materializer and imports would keep creating entries in the name of someone no longer in the wallet.
	var inRecurrence, inBankAccount bool
	if err := s.DB.QueryRowContext(r.Context(), `
		SELECT EXISTS(SELECT 1 FROM recurrences rc
		              WHERE rc.wallet_id = ?1 AND rc.enabled = 1 AND (rc.end_date IS NULL OR rc.end_date >= ?3)
		                AND (rc.paid_by_user_id = ?2 OR rc.to_user_id = ?2
		                     OR EXISTS(SELECT 1 FROM recurrence_splits rs WHERE rs.recurrence_id = rc.id AND rs.user_id = ?2))),
		       EXISTS(SELECT 1 FROM bank_accounts WHERE wallet_id = ?1 AND paid_by_user_id = ?2)`,
		walletID, target, s.Now().In(s.Cfg.Location).Format(httpx.DateLayout)).Scan(&inRecurrence, &inBankAccount); err != nil {
		return err
	}
	if inRecurrence {
		return httpx.ErrBadRequest("Antes, edite ou exclua as recorrências que envolvem essa pessoa")
	}
	if inBankAccount {
		return httpx.ErrBadRequest("Antes, edite ou exclua as contas bancárias que usam essa pessoa como pagadora")
	}
	if _, err := s.DB.ExecContext(r.Context(), `DELETE FROM wallet_members WHERE wallet_id = ? AND user_id = ?`,
		walletID, target); err != nil {
		return err
	}
	s.Mutated(r)
	s.NotifyUsers(target) // no longer a member, so mutated does not reach them
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// setDefaultPaymentMethod sets the method the current member's new transactions start with (null: the first one).
func (s *Handler) setDefaultPaymentMethod(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		PaymentMethodID *int64 `json:"paymentMethodId"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	ctx, walletID, userID := r.Context(), httpx.WalletIDFrom(r.Context()), httpx.UserIDFrom(r.Context())
	if in.PaymentMethodID != nil {
		var ok bool
		if err := s.DB.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM payment_methods WHERE id = ? AND wallet_id = ?)`,
			*in.PaymentMethodID, walletID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return httpx.ValidationError(map[string]string{"paymentMethodId": "forma de pagamento inexistente"})
		}
	}
	if _, err := s.DB.ExecContext(ctx, `UPDATE wallet_members SET default_payment_method_id = ? WHERE wallet_id = ? AND user_id = ?`,
		in.PaymentMethodID, walletID, userID); err != nil {
		return err
	}
	m, err := s.memberByID(ctx, walletID, userID)
	if err != nil {
		return err
	}
	s.Mutated(r)
	httpx.WriteJSON(w, http.StatusOK, m)
	return nil
}
