package wallet

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"gastos/internal/api/httpx"
)

// WalletDTO is a wallet as the API returns it, with the caller's role in it.
type WalletDTO struct { //nolint:revive // moved unchanged, see docs/security.md
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	Role             string `json:"role"`
	SplittingEnabled bool   `json:"splittingEnabled"`
	OwnerName        string `json:"ownerName"`
}

// WalletsFor returns the wallets userID is a member of.
func (s *Handler) WalletsFor(ctx context.Context, userID int64) ([]WalletDTO, error) {
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
	defer rows.Close() //nolint:errcheck,gosec // moved unchanged, see docs/security.md
	out := []WalletDTO{}
	for rows.Next() {
		var d WalletDTO
		if err := rows.Scan(&d.ID, &d.Name, &d.Role, &d.SplittingEnabled, &d.OwnerName); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// walletByID returns walletID as seen by userID.
func (s *Handler) walletByID(ctx context.Context, userID, walletID int64) (WalletDTO, error) {
	var d WalletDTO
	err := s.DB.QueryRowContext(ctx, `
		SELECT w.id, w.name, me.role, w.splitting_enabled, u.name
		FROM wallets w
		JOIN wallet_members me ON me.wallet_id = w.id AND me.user_id = ?
		JOIN wallet_members ow ON ow.wallet_id = w.id AND ow.role = 'owner'
		JOIN users u ON u.id = ow.user_id
		WHERE w.id = ?`, userID, walletID).Scan(&d.ID, &d.Name, &d.Role, &d.SplittingEnabled, &d.OwnerName)
	return d, err
}

// CreateWallet inserts the wallet, the owner and the defaults (categories and payment methods).
func CreateWallet(ctx context.Context, q httpx.Querier, name string, ownerID int64, splitting bool) (int64, error) {
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

// listWallets handles GET /api/wallets: the caller's wallets.
func (s *Handler) listWallets(w http.ResponseWriter, r *http.Request) error {
	list, err := s.WalletsFor(r.Context(), httpx.UserIDFrom(r.Context()))
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
func (s *Handler) createWallet(w http.ResponseWriter, r *http.Request) error {
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
		id, err = CreateWallet(r.Context(), tx, in.Name, userID, in.SplittingEnabled)
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
func (s *Handler) updateWallet(w http.ResponseWriter, r *http.Request) error {
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
func (s *Handler) deleteWallet(w http.ResponseWriter, r *http.Request) error {
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
