package api

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
)

// bcryptCost é variável para os testes usarem um custo baixo.
var bcryptCost = 12

type role int

const (
	roleReader role = iota + 1
	roleEditor
	roleOwner
)

func (r role) String() string {
	switch r {
	case roleOwner:
		return "owner"
	case roleEditor:
		return "editor"
	case roleReader:
		return "reader"
	}
	return ""
}

func parseRole(s string) (role, bool) {
	switch s {
	case "owner":
		return roleOwner, true
	case "editor":
		return roleEditor, true
	case "reader":
		return roleReader, true
	}
	return 0, false
}

// requireWallet exige que o usuário seja membro da carteira do path com papel >= min.
// Quem não é membro recebe 404 para não revelar que a carteira existe.
func (s *Server) requireWallet(min role, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		walletID, err := pathID(r, "walletID")
		if err != nil {
			s.writeError(w, r, err)
			return
		}
		var roleStr string
		err = s.db.QueryRowContext(r.Context(),
			`SELECT role FROM wallet_members WHERE wallet_id = ? AND user_id = ?`,
			walletID, userIDFrom(r.Context())).Scan(&roleStr)
		if err != nil {
			s.writeError(w, r, err) // ErrNoRows → 404
			return
		}
		have, _ := parseRole(roleStr)
		if have < min {
			s.writeError(w, r, errForbidden)
			return
		}
		ctx := context.WithValue(r.Context(), ctxWalletID, walletID)
		ctx = context.WithValue(ctx, ctxRole, have)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type walletDTO struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	Role             string `json:"role"`
	SplittingEnabled bool   `json:"splittingEnabled"`
	OwnerName        string `json:"ownerName"`
}

func (s *Server) walletsFor(ctx context.Context, userID int64) ([]walletDTO, error) {
	rows, err := s.db.QueryContext(ctx, `
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

func (s *Server) walletByID(ctx context.Context, userID, walletID int64) (walletDTO, error) {
	var d walletDTO
	err := s.db.QueryRowContext(ctx, `
		SELECT w.id, w.name, me.role, w.splitting_enabled, u.name
		FROM wallets w
		JOIN wallet_members me ON me.wallet_id = w.id AND me.user_id = ?
		JOIN wallet_members ow ON ow.wallet_id = w.id AND ow.role = 'owner'
		JOIN users u ON u.id = ow.user_id
		WHERE w.id = ?`, userID, walletID).Scan(&d.ID, &d.Name, &d.Role, &d.SplittingEnabled, &d.OwnerName)
	return d, err
}

// createWallet insere a carteira, o owner e os padrões (categorias e formas de pagamento).
func createWallet(ctx context.Context, q querier, name string, ownerID int64, splitting bool) (int64, error) {
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

func seedWallet(ctx context.Context, q querier, walletID int64) error {
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

func (s *Server) listWallets(w http.ResponseWriter, r *http.Request) error {
	list, err := s.walletsFor(r.Context(), userIDFrom(r.Context()))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, list)
	return nil
}

func validWalletName(fe fieldErrors, name string) {
	if name == "" || len([]rune(name)) > 60 {
		fe.add("name", "obrigatório, até 60 caracteres")
	}
}

func (s *Server) createWallet(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Name             string `json:"name"`
		SplittingEnabled bool   `json:"splittingEnabled"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	in.Name = strings.TrimSpace(in.Name)
	fe := fieldErrors{}
	validWalletName(fe, in.Name)
	if err := fe.err(); err != nil {
		return err
	}
	userID := userIDFrom(r.Context())
	var id int64
	err := s.inTx(r.Context(), func(tx *sql.Tx) (err error) {
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
	writeJSON(w, http.StatusCreated, d)
	return nil
}

func (s *Server) updateWallet(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Name             *string `json:"name"`
		SplittingEnabled *bool   `json:"splittingEnabled"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	if in.Name == nil && in.SplittingEnabled == nil {
		return errBadRequest("Nenhuma alteração informada")
	}
	walletID := walletIDFrom(r.Context())
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		fe := fieldErrors{}
		validWalletName(fe, name)
		if err := fe.err(); err != nil {
			return err
		}
		if _, err := s.db.ExecContext(r.Context(), `UPDATE wallets SET name = ? WHERE id = ?`, name, walletID); err != nil {
			return err
		}
	}
	if in.SplittingEnabled != nil {
		if _, err := s.db.ExecContext(r.Context(), `UPDATE wallets SET splitting_enabled = ? WHERE id = ?`,
			*in.SplittingEnabled, walletID); err != nil {
			return err
		}
	}
	d, err := s.walletByID(r.Context(), userIDFrom(r.Context()), walletID)
	if err != nil {
		return err
	}
	s.mutated(r)
	writeJSON(w, http.StatusOK, d)
	return nil
}

func (s *Server) deleteWallet(w http.ResponseWriter, r *http.Request) error {
	s.mutated(r) // avisa os outros membros antes de a carteira sumir
	if _, err := s.db.ExecContext(r.Context(), `DELETE FROM wallets WHERE id = ?`, walletIDFrom(r.Context())); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

type memberDTO struct {
	UserID          int64  `json:"userId"`
	Name            string `json:"name"`
	Email           string `json:"email"`
	Role            string `json:"role"`
	SplitsByDefault bool   `json:"splitsByDefault"`
}

func (s *Server) memberByID(ctx context.Context, walletID, userID int64) (memberDTO, error) {
	var m memberDTO
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.name, u.email, wm.role, wm.splits_by_default
		FROM wallet_members wm JOIN users u ON u.id = wm.user_id
		WHERE wm.wallet_id = ? AND wm.user_id = ?`, walletID, userID).
		Scan(&m.UserID, &m.Name, &m.Email, &m.Role, &m.SplitsByDefault)
	return m, err
}

func (s *Server) listMembers(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT u.id, u.name, u.email, wm.role, wm.splits_by_default
		FROM wallet_members wm JOIN users u ON u.id = wm.user_id
		WHERE wm.wallet_id = ?
		ORDER BY CASE wm.role WHEN 'owner' THEN 0 WHEN 'editor' THEN 1 ELSE 2 END, u.name, u.id`,
		walletIDFrom(r.Context()))
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []memberDTO{}
	for rows.Next() {
		var m memberDTO
		if err := rows.Scan(&m.UserID, &m.Name, &m.Email, &m.Role, &m.SplitsByDefault); err != nil {
			return err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

func (s *Server) addMember(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if rl, ok := parseRole(in.Role); !ok || rl == roleOwner {
		return errValidation(map[string]string{"role": "deve ser editor ou reader"})
	}
	var userID int64
	if err := s.db.QueryRowContext(r.Context(), `SELECT id FROM users WHERE email = ?`, in.Email).Scan(&userID); err != nil {
		if err == sql.ErrNoRows {
			return errNotFoundMsg("Usuário não encontrado")
		}
		return err
	}
	walletID := walletIDFrom(r.Context())
	if _, err := s.db.ExecContext(r.Context(),
		`INSERT INTO wallet_members (wallet_id, user_id, role) VALUES (?, ?, ?)`, walletID, userID, in.Role); err != nil {
		if isUnique(err) {
			return errConflict("Usuário já faz parte da carteira")
		}
		return err
	}
	m, err := s.memberByID(r.Context(), walletID, userID)
	if err != nil {
		return err
	}
	s.mutated(r)
	writeJSON(w, http.StatusCreated, m)
	return nil
}

func (s *Server) updateMember(w http.ResponseWriter, r *http.Request) error {
	target, err := pathID(r, "userID")
	if err != nil {
		return err
	}
	var in struct {
		Role            *string `json:"role"`
		SplitsByDefault *bool   `json:"splitsByDefault"`
	}
	if err := decode(w, r, &in); err != nil {
		return err
	}
	walletID := walletIDFrom(r.Context())
	cur, err := s.memberByID(r.Context(), walletID, target)
	if err != nil {
		return err
	}
	if in.Role != nil {
		if cur.Role == "owner" {
			return errBadRequest("O papel do owner não pode ser alterado")
		}
		if rl, ok := parseRole(*in.Role); !ok || rl == roleOwner {
			return errValidation(map[string]string{"role": "deve ser editor ou reader"})
		}
		if _, err := s.db.ExecContext(r.Context(), `UPDATE wallet_members SET role = ? WHERE wallet_id = ? AND user_id = ?`,
			*in.Role, walletID, target); err != nil {
			return err
		}
	}
	if in.SplitsByDefault != nil {
		if _, err := s.db.ExecContext(r.Context(), `UPDATE wallet_members SET splits_by_default = ? WHERE wallet_id = ? AND user_id = ?`,
			*in.SplitsByDefault, walletID, target); err != nil {
			return err
		}
	}
	m, err := s.memberByID(r.Context(), walletID, target)
	if err != nil {
		return err
	}
	s.mutated(r)
	writeJSON(w, http.StatusOK, m)
	return nil
}

func (s *Server) removeMember(w http.ResponseWriter, r *http.Request) error {
	target, err := pathID(r, "userID")
	if err != nil {
		return err
	}
	me := userIDFrom(r.Context())
	if roleFrom(r.Context()) != roleOwner && target != me {
		return errForbidden
	}
	walletID := walletIDFrom(r.Context())
	cur, err := s.memberByID(r.Context(), walletID, target)
	if err != nil {
		return err
	}
	if cur.Role == "owner" {
		return errBadRequest("O owner não pode ser removido nem sair da carteira")
	}
	if _, err := s.db.ExecContext(r.Context(), `DELETE FROM wallet_members WHERE wallet_id = ? AND user_id = ?`,
		walletID, target); err != nil {
		return err
	}
	s.mutated(r)
	w.WriteHeader(http.StatusNoContent)
	return nil
}
