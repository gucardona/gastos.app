package api

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"gastos/internal/api/httpx"
	"gastos/internal/money"
)

// _bcryptCost is a variable so tests can use a low cost.
var _bcryptCost = 12

// requireWallet requires the user to be a member of the wallet in the path with role >= minRole.
// Non-members get 404 so the wallet's existence is not revealed.
func (s *Server) requireWallet(minRole httpx.Role, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		walletID, err := httpx.PathID(r, "walletID")
		if err != nil {
			s.WriteError(w, r, err)
			return
		}
		var roleStr string
		err = s.DB.QueryRowContext(r.Context(),
			`SELECT role FROM wallet_members WHERE wallet_id = ? AND user_id = ?`,
			walletID, httpx.UserIDFrom(r.Context())).Scan(&roleStr)
		if err != nil {
			s.WriteError(w, r, err) // ErrNoRows → 404
			return
		}
		have, _ := httpx.ParseRole(roleStr)
		if have < minRole {
			s.WriteError(w, r, httpx.ErrForbidden)
			return
		}
		ctx := httpx.WithWallet(r.Context(), walletID, have)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// walletDTO is a wallet as the API returns it, with the caller's role in it.
type walletDTO struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	Role             string `json:"role"`
	SplittingEnabled bool   `json:"splittingEnabled"`
	OwnerName        string `json:"ownerName"`
}

// walletsFor returns the wallets userID is a member of.
func (s *Server) walletsFor(ctx context.Context, userID int64) ([]walletDTO, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT w.id, w.name, me.role, w.splitting_enabled, u.name
		FROM wallets w
		JOIN wallet_members me ON me.wallet_id = w.id AND me.user_id = ?
		JOIN wallet_members ow ON ow.wallet_id = w.id AND ow.role = 'owner'
		JOIN users u ON u.id = ow.user_id
		ORDER BY w.created_at, w.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []walletDTO{}
	for rows.Next() {
		var d walletDTO
		if err := rows.Scan(&d.ID, &d.Name, &d.Role, &d.SplittingEnabled, &d.OwnerName); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// walletByID returns walletID as seen by userID.
func (s *Server) walletByID(ctx context.Context, userID, walletID int64) (walletDTO, error) {
	var d walletDTO
	err := s.DB.QueryRowContext(ctx, `
		SELECT w.id, w.name, me.role, w.splitting_enabled, u.name
		FROM wallets w
		JOIN wallet_members me ON me.wallet_id = w.id AND me.user_id = ?
		JOIN wallet_members ow ON ow.wallet_id = w.id AND ow.role = 'owner'
		JOIN users u ON u.id = ow.user_id
		WHERE w.id = ?`, userID, walletID).Scan(&d.ID, &d.Name, &d.Role, &d.SplittingEnabled, &d.OwnerName)
	return d, err
}

// createWallet inserts the wallet, the owner and the defaults (categories and payment methods).
func createWallet(ctx context.Context, q httpx.Querier, name string, ownerID int64, splitting bool) (int64, error) {
	var id int64
	if err := q.QueryRowContext(ctx, `INSERT INTO wallets (name, splitting_enabled) VALUES (?, ?) RETURNING id`,
		name, splitting).Scan(&id); err != nil {
		return 0, err
	}
	if _, err := q.ExecContext(ctx, `INSERT INTO wallet_members (wallet_id, user_id, role) VALUES (?, ?, 'owner')`,
		id, ownerID); err != nil {
		return 0, err
	}
	return id, seedWallet(ctx, q, id)
}

// seedWallet creates the default categories and payment methods of a new wallet.
func seedWallet(ctx context.Context, q httpx.Querier, walletID int64) error {
	expense := []struct{ name, icon, color, essentiality string }{
		{"Alimentação", "🍽", "#0f6579", "essential"},
		{"Mercado", "🛒", "#1f7d8f", "essential"},
		{"Transporte", "🚗", "#2b92a1", "essential"},
		{"Saúde", "💊", "#27ae60", "essential"},
		{"Moradia", "🏠", "#2c9b88", "essential"},
		{"Contas", "⚡", "#d6a23a", "essential"},
		{"Restaurante", "🍜", "#55b38c", "nonessential"},
		{"Lazer", "🎮", "#66c3a1", "nonessential"},
		{"Assinaturas", "📺", "#8e44ad", "nonessential"},
		{"Vestuário", "👕", "#88c0b3", "nonessential"},
		{"Beleza", "✨", "#79b7ae", "nonessential"},
		{"Viagem", "✈️", "#54a7b9", "nonessential"},
		{"Educação", "📚", "#2ba18a", "investment"},
		{"Investimentos", "📈", "#f39c12", "investment"},
		{"Outros", "◈", "#7f9ea5", "nonessential"},
	}
	for i, c := range expense {
		if _, err := q.ExecContext(ctx, `
			INSERT INTO categories (wallet_id, kind, name, icon, color, essentiality, sort_order)
			VALUES (?, 'expense', ?, ?, ?, ?, ?)`, walletID, c.name, c.icon, c.color, c.essentiality, i); err != nil {
			return err
		}
	}
	income := []struct{ name, icon, color string }{
		{"Salário", "💼", "#0f6579"},
		{"Freelance", "🧑‍💻", "#1f7d8f"},
		{"Investimentos", "📈", "#f39c12"},
		{"Reembolso", "↩️", "#2b92a1"},
		{"Outros", "◈", "#7f9ea5"},
	}
	for i, c := range income {
		if _, err := q.ExecContext(ctx, `
			INSERT INTO categories (wallet_id, kind, name, icon, color, sort_order)
			VALUES (?, 'income', ?, ?, ?, ?)`, walletID, c.name, c.icon, c.color, i); err != nil {
			return err
		}
	}
	methods := []struct{ name, icon string }{
		{"Pix", "⚡"}, {"Crédito", "💳"}, {"Débito", "🏦"}, {"Dinheiro", "💵"},
		{"Boleto", "📄"}, {"Vale Alim.", "🍽"}, {"Vale Ref.", "🛒"}, {"Transferência", "↔"},
	}
	for i, m := range methods {
		if _, err := q.ExecContext(ctx, `
			INSERT INTO payment_methods (wallet_id, name, icon, sort_order) VALUES (?, ?, ?, ?)`,
			walletID, m.name, m.icon, i); err != nil {
			return err
		}
	}
	return nil
}

// listWallets handles GET /api/wallets: the caller's wallets.
func (s *Server) listWallets(w http.ResponseWriter, r *http.Request) error {
	list, err := s.walletsFor(r.Context(), httpx.UserIDFrom(r.Context()))
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, list)
	return nil
}

// validWalletName requires a name of 1 to 60 characters.
func validWalletName(fe httpx.FieldErrors, name string) {
	if name == "" || len([]rune(name)) > 60 {
		fe.Add("name", "obrigatório, até 60 caracteres")
	}
}

// createWallet handles POST /api/wallets: a new wallet owned by the caller.
func (s *Server) createWallet(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Name             string `json:"name"`
		SplittingEnabled bool   `json:"splittingEnabled"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	in.Name = strings.TrimSpace(in.Name)
	fe := httpx.FieldErrors{}
	validWalletName(fe, in.Name)
	if err := fe.Err(); err != nil {
		return err
	}
	userID := httpx.UserIDFrom(r.Context())
	var id int64
	err := s.InTx(r.Context(), func(tx *sql.Tx) (err error) {
		id, err = createWallet(r.Context(), tx, in.Name, userID, in.SplittingEnabled)
		return err
	})
	if err != nil {
		return err
	}
	d, err := s.walletByID(r.Context(), userID, id)
	if err != nil {
		return err
	}
	s.NotifyUsers(userID) // the creator's other tabs list the new wallet
	httpx.WriteJSON(w, http.StatusCreated, d)
	return nil
}

// updateWallet handles PATCH /api/wallets/{walletID}: name and splitting, owner only.
func (s *Server) updateWallet(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Name             *string `json:"name"`
		SplittingEnabled *bool   `json:"splittingEnabled"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	if in.Name == nil && in.SplittingEnabled == nil {
		return httpx.ErrBadRequest("Nenhuma alteração informada")
	}
	walletID := httpx.WalletIDFrom(r.Context())
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		fe := httpx.FieldErrors{}
		validWalletName(fe, name)
		if err := fe.Err(); err != nil {
			return err
		}
		if _, err := s.DB.ExecContext(r.Context(), `UPDATE wallets SET name = ? WHERE id = ?`, name, walletID); err != nil {
			return err
		}
	}
	if in.SplittingEnabled != nil {
		if _, err := s.DB.ExecContext(r.Context(), `UPDATE wallets SET splitting_enabled = ? WHERE id = ?`,
			*in.SplittingEnabled, walletID); err != nil {
			return err
		}
	}
	d, err := s.walletByID(r.Context(), httpx.UserIDFrom(r.Context()), walletID)
	if err != nil {
		return err
	}
	s.Mutated(r)
	httpx.WriteJSON(w, http.StatusOK, d)
	return nil
}

// deleteWallet handles DELETE /api/wallets/{walletID}, owner only, and notifies every member.
func (s *Server) deleteWallet(w http.ResponseWriter, r *http.Request) error {
	walletID := httpx.WalletIDFrom(r.Context())
	members, err := s.WalletMembers(r.Context(), walletID) // read before the delete takes them away
	if err != nil {
		return err
	}
	if _, err := s.DB.ExecContext(r.Context(), `DELETE FROM wallets WHERE id = ?`, walletID); err != nil {
		return err
	}
	s.NotifyUsers(members...)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

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
func (s *Server) memberByID(ctx context.Context, walletID, userID int64) (memberDTO, error) {
	var m memberDTO
	err := s.DB.QueryRowContext(ctx, `
		SELECT u.id, u.name, u.email, wm.role, wm.splits_by_default, COALESCE(wm.default_share_bp, 0), wm.default_payment_method_id
		FROM wallet_members wm JOIN users u ON u.id = wm.user_id
		WHERE wm.wallet_id = ? AND wm.user_id = ?`, walletID, userID).
		Scan(&m.UserID, &m.Name, &m.Email, &m.Role, &m.SplitsByDefault, &m.DefaultShareBP, &m.DefaultPaymentMethodID)
	return m, err
}

// listMembers handles GET /members.
func (s *Server) listMembers(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.DB.QueryContext(r.Context(), `
		SELECT u.id, u.name, u.email, wm.role, wm.splits_by_default, COALESCE(wm.default_share_bp, 0), wm.default_payment_method_id
		FROM wallet_members wm JOIN users u ON u.id = wm.user_id
		WHERE wm.wallet_id = ?
		ORDER BY CASE wm.role WHEN 'owner' THEN 0 WHEN 'editor' THEN 1 ELSE 2 END, u.name, u.id`,
		httpx.WalletIDFrom(r.Context()))
	if err != nil {
		return err
	}
	defer rows.Close()
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
func (s *Server) addMember(w http.ResponseWriter, r *http.Request) error {
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
func (s *Server) updateMember(w http.ResponseWriter, r *http.Request) error {
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
func (s *Server) removeMember(w http.ResponseWriter, r *http.Request) error {
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
func (s *Server) setDefaultPaymentMethod(w http.ResponseWriter, r *http.Request) error {
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
