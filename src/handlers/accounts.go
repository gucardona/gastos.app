package handlers

import (
	"database/sql"
	"errors"
	"gastos/src/db"
	"gastos/src/events"
	"gastos/src/middleware"
	"gastos/src/models"
	"net/http"
	"strconv"
	"strings"
)

func Accounts(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/accounts"), "/")

	if path == "" {
		handleAccountsCollection(w, r, userID)
		return
	}

	segments := strings.Split(path, "/")
	accountID, err := strconv.ParseInt(strings.TrimSpace(segments[0]), 10, 64)
	if err != nil || accountID <= 0 {
		jsonError(w, "Conta inválida", http.StatusBadRequest)
		return
	}

	switch {
	case len(segments) == 1:
		handleAccountItem(w, r, userID, accountID)
	case len(segments) == 2 && segments[1] == "split-participants":
		handleSplitParticipants(w, r, userID, accountID)
	case len(segments) == 2 && segments[1] == "members":
		handleAccountMembersCollection(w, r, userID, accountID)
	case len(segments) == 3 && segments[1] == "members":
		memberUserID, err := strconv.ParseInt(strings.TrimSpace(segments[2]), 10, 64)
		if err != nil || memberUserID <= 0 {
			jsonError(w, "Membro inválido", http.StatusBadRequest)
			return
		}
		handleAccountMemberItem(w, r, userID, accountID, memberUserID)
	default:
		jsonError(w, "Rota não encontrada", http.StatusNotFound)
	}
}

func handleAccountsCollection(w http.ResponseWriter, r *http.Request, userID int64) {
	switch r.Method {
	case http.MethodGet:
		rows, err := db.DB.Query(`
			SELECT a.id, a.name, am.role, owner.name, a.splitting_enabled,
			       COALESCE(GROUP_CONCAT(asp.user_id), '') AS split_participant_ids
			FROM accounts a
			INNER JOIN account_members am ON am.account_id = a.id AND am.user_id = ?
			INNER JOIN account_members owner_am ON owner_am.account_id = a.id AND owner_am.role = 'owner'
			INNER JOIN users owner ON owner.id = owner_am.user_id
			LEFT JOIN account_split_participants asp ON asp.account_id = a.id
			GROUP BY a.id, a.name, am.role, owner.name, a.splitting_enabled
			ORDER BY a.created_at ASC, a.id ASC
		`, userID)
		if err != nil {
			jsonError(w, "Erro ao buscar contas", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		accounts := make([]models.Account, 0)
		for rows.Next() {
			var (
				accountID           int64
				name                string
				role                string
				ownerName           string
				splittingEnabled    int
				splitParticipantStr string
			)
			if err := rows.Scan(&accountID, &name, &role, &ownerName, &splittingEnabled, &splitParticipantStr); err != nil {
				jsonError(w, "Erro ao ler contas", http.StatusInternalServerError)
				return
			}
			accounts = append(accounts, accountResponse(accountID, name, role, ownerName, splittingEnabled == 1, parseSplitParticipantIds(splitParticipantStr)))
		}
		if err := rows.Err(); err != nil {
			jsonError(w, "Erro ao iterar contas", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, accounts)

	case http.MethodPost:
		var body struct {
			Name             string `json:"name"`
			SplittingEnabled bool   `json:"splittingEnabled"`
		}
		if err := decodeJSON(r, &body); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		body.Name = strings.TrimSpace(body.Name)
		if err := validateAccountName(body.Name); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		tx, err := db.DB.Begin()
		if err != nil {
			jsonError(w, "Erro ao criar conta", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		accountID, err := db.CreateAccountWithOwner(tx, body.Name, userID, body.SplittingEnabled)
		if err != nil {
			jsonError(w, "Erro ao criar conta", http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			jsonError(w, "Erro ao criar conta", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusCreated, accountResponse(accountID, body.Name, models.AccountRoleOwner, currentUserName(userID), body.SplittingEnabled, []int64{}))

	default:
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		jsonError(w, "Método não permitido", http.StatusMethodNotAllowed)
	}
}

func handleAccountItem(w http.ResponseWriter, r *http.Request, userID, accountID int64) {
	account, err := lookupAccountForUser(userID, accountID)
	if err != nil {
		writeAccountLookupError(w, err)
		return
	}

	switch r.Method {
	case http.MethodPatch:
		if account.Role != models.AccountRoleOwner {
			jsonError(w, "Apenas o owner pode alterar esta conta", http.StatusForbidden)
			return
		}

		var body struct {
			Name             *string `json:"name"`
			SplittingEnabled *bool   `json:"splittingEnabled"`
		}
		if err := decodeJSON(r, &body); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if body.Name == nil && body.SplittingEnabled == nil {
			jsonError(w, "Nenhuma alteração informada", http.StatusBadRequest)
			return
		}

		setClauses := []string{}
		args := []any{}

		newName := account.Name
		if body.Name != nil {
			newName = strings.TrimSpace(*body.Name)
			if err := validateAccountName(newName); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			setClauses = append(setClauses, "name = ?")
			args = append(args, newName)
		}

		newSplitting := account.SplittingEnabled
		if body.SplittingEnabled != nil {
			newSplitting = *body.SplittingEnabled
			splitting := 0
			if newSplitting {
				splitting = 1
			}
			setClauses = append(setClauses, "splitting_enabled = ?")
			args = append(args, splitting)
		}

		args = append(args, accountID)
		if _, err := db.DB.Exec(`UPDATE accounts SET `+strings.Join(setClauses, ", ")+` WHERE id = ?`, args...); err != nil {
			jsonError(w, "Erro ao atualizar conta", http.StatusInternalServerError)
			return
		}

		events.Bus.Notify(accountID, userID)
		writeJSON(w, http.StatusOK, accountResponse(account.ID, newName, account.Role, account.OwnerName, newSplitting, loadSplitParticipantIds(accountID)))

	case http.MethodDelete:
		if account.Role != models.AccountRoleOwner {
			jsonError(w, "Apenas o owner pode remover esta conta", http.StatusForbidden)
			return
		}

		res, err := db.DB.Exec(`DELETE FROM accounts WHERE id = ?`, accountID)
		if err != nil {
			jsonError(w, "Erro ao remover conta", http.StatusInternalServerError)
			return
		}
		rowsAffected, err := res.RowsAffected()
		if err != nil {
			jsonError(w, "Erro ao confirmar remoção", http.StatusInternalServerError)
			return
		}
		if rowsAffected == 0 {
			jsonError(w, "Conta não encontrada", http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusNoContent)

	default:
		w.Header().Set("Allow", "PATCH, DELETE, OPTIONS")
		jsonError(w, "Método não permitido", http.StatusMethodNotAllowed)
	}
}

func handleAccountMembersCollection(w http.ResponseWriter, r *http.Request, userID, accountID int64) {
	account, err := lookupAccountForUser(userID, accountID)
	if err != nil {
		writeAccountLookupError(w, err)
		return
	}

	switch r.Method {
	case http.MethodGet:
		rows, err := db.DB.Query(`
			SELECT u.id, u.name, u.email, am.role
			FROM account_members am
			INNER JOIN users u ON u.id = am.user_id
			WHERE am.account_id = ?
			ORDER BY
				CASE am.role
					WHEN 'owner' THEN 0
					WHEN 'editor' THEN 1
					ELSE 2
				END,
				u.name ASC,
				u.id ASC
		`, accountID)
		if err != nil {
			jsonError(w, "Erro ao buscar membros", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		members := make([]models.AccountMember, 0)
		for rows.Next() {
			var member models.AccountMember
			if err := rows.Scan(&member.UserID, &member.Name, &member.Email, &member.Role); err != nil {
				jsonError(w, "Erro ao ler membros", http.StatusInternalServerError)
				return
			}
			members = append(members, member)
		}
		if err := rows.Err(); err != nil {
			jsonError(w, "Erro ao iterar membros", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, members)

	case http.MethodPost:
		if account.Role != models.AccountRoleOwner {
			jsonError(w, "Apenas o owner pode gerenciar membros", http.StatusForbidden)
			return
		}

		var body struct {
			Email string `json:"email"`
			Role  string `json:"role"`
		}
		if err := decodeJSON(r, &body); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		body.Email = strings.TrimSpace(strings.ToLower(body.Email))
		body.Role = strings.TrimSpace(strings.ToLower(body.Role))
		if body.Email == "" {
			jsonError(w, "E-mail é obrigatório", http.StatusBadRequest)
			return
		}
		if err := validateShareRole(body.Role); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		var member models.AccountMember
		err := db.DB.QueryRow(`
			SELECT id, name, email
			FROM users
			WHERE email = ?
		`, body.Email).Scan(&member.UserID, &member.Name, &member.Email)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				jsonError(w, "Usuário não encontrado", http.StatusNotFound)
				return
			}
			jsonError(w, "Erro ao buscar usuário", http.StatusInternalServerError)
			return
		}

		if _, err := db.DB.Exec(`
			INSERT INTO account_members (account_id, user_id, role)
			VALUES (?, ?, ?)
		`, accountID, member.UserID, body.Role); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "unique") {
				jsonError(w, "Usuário já faz parte da conta", http.StatusConflict)
				return
			}
			jsonError(w, "Erro ao compartilhar conta", http.StatusInternalServerError)
			return
		}

		member.Role = body.Role
		events.Bus.Notify(accountID, userID)
		writeJSON(w, http.StatusCreated, member)

	default:
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		jsonError(w, "Método não permitido", http.StatusMethodNotAllowed)
	}
}

func handleAccountMemberItem(w http.ResponseWriter, r *http.Request, userID, accountID, memberUserID int64) {
	account, err := lookupAccountForUser(userID, accountID)
	if err != nil {
		writeAccountLookupError(w, err)
		return
	}
	selfManagedLeave := r.Method == http.MethodDelete && userID == memberUserID
	if account.Role != models.AccountRoleOwner && !selfManagedLeave {
		jsonError(w, "Apenas o owner pode gerenciar membros", http.StatusForbidden)
		return
	}

	switch r.Method {
	case http.MethodPatch:
		var body struct {
			Role string `json:"role"`
		}
		if err := decodeJSON(r, &body); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		body.Role = strings.TrimSpace(strings.ToLower(body.Role))
		if err := validateShareRole(body.Role); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		member, err := lookupAccountMember(accountID, memberUserID)
		if err != nil {
			writeMemberLookupError(w, err)
			return
		}
		if member.Role == models.AccountRoleOwner {
			jsonError(w, "O owner não pode ser alterado", http.StatusBadRequest)
			return
		}

		if _, err := db.DB.Exec(`
			UPDATE account_members
			SET role = ?
			WHERE account_id = ? AND user_id = ?
		`, body.Role, accountID, memberUserID); err != nil {
			jsonError(w, "Erro ao atualizar membro", http.StatusInternalServerError)
			return
		}

		member.Role = body.Role
		events.Bus.Notify(accountID, userID)
		writeJSON(w, http.StatusOK, member)

	case http.MethodDelete:
		member, err := lookupAccountMember(accountID, memberUserID)
		if err != nil {
			writeMemberLookupError(w, err)
			return
		}
		if member.Role == models.AccountRoleOwner {
			jsonError(w, "O owner não pode ser removido", http.StatusBadRequest)
			return
		}
		if selfManagedLeave && member.UserID != userID {
			jsonError(w, "Você só pode sair da sua própria conta compartilhada", http.StatusForbidden)
			return
		}

		res, err := db.DB.Exec(`
			DELETE FROM account_members
			WHERE account_id = ? AND user_id = ?
		`, accountID, memberUserID)
		if err != nil {
			jsonError(w, "Erro ao remover membro", http.StatusInternalServerError)
			return
		}
		rowsAffected, err := res.RowsAffected()
		if err != nil {
			jsonError(w, "Erro ao confirmar remoção", http.StatusInternalServerError)
			return
		}
		if rowsAffected == 0 {
			jsonError(w, "Membro não encontrado", http.StatusNotFound)
			return
		}

		events.Bus.Notify(accountID, userID)
		w.WriteHeader(http.StatusNoContent)

	default:
		w.Header().Set("Allow", "PATCH, DELETE, OPTIONS")
		jsonError(w, "Método não permitido", http.StatusMethodNotAllowed)
	}
}

func handleSplitParticipants(w http.ResponseWriter, r *http.Request, userID, accountID int64) {
	account, err := lookupAccountForUser(userID, accountID)
	if err != nil {
		writeAccountLookupError(w, err)
		return
	}
	if account.Role != models.AccountRoleOwner {
		jsonError(w, "Apenas o owner pode gerenciar participantes", http.StatusForbidden)
		return
	}

	switch r.Method {
	case http.MethodPut:
		var body struct {
			UserIds []int64 `json:"userIds"`
		}
		if err := decodeJSON(r, &body); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		tx, err := db.DB.Begin()
		if err != nil {
			jsonError(w, "Erro ao atualizar participantes", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		if _, err := tx.Exec(`DELETE FROM account_split_participants WHERE account_id = ?`, accountID); err != nil {
			jsonError(w, "Erro ao atualizar participantes", http.StatusInternalServerError)
			return
		}
		for _, uid := range body.UserIds {
			if uid <= 0 {
				continue
			}
			if _, err := tx.Exec(`INSERT OR IGNORE INTO account_split_participants (account_id, user_id) VALUES (?, ?)`, accountID, uid); err != nil {
				jsonError(w, "Erro ao atualizar participantes", http.StatusInternalServerError)
				return
			}
		}
		if err := tx.Commit(); err != nil {
			jsonError(w, "Erro ao atualizar participantes", http.StatusInternalServerError)
			return
		}

		events.Bus.Notify(accountID, userID)
		writeJSON(w, http.StatusOK, map[string][]int64{"splitParticipantIds": loadSplitParticipantIds(accountID)})

	default:
		w.Header().Set("Allow", "PUT, OPTIONS")
		jsonError(w, "Método não permitido", http.StatusMethodNotAllowed)
	}
}

func lookupAccountForUser(userID, accountID int64) (models.Account, error) {
	var (
		account          models.Account
		splittingEnabled int
	)
	err := db.DB.QueryRow(`
		SELECT a.id, a.name, am.role, owner.name, a.splitting_enabled
		FROM accounts a
		INNER JOIN account_members am ON am.account_id = a.id
		INNER JOIN account_members owner_am ON owner_am.account_id = a.id AND owner_am.role = 'owner'
		INNER JOIN users owner ON owner.id = owner_am.user_id
		WHERE a.id = ? AND am.user_id = ?
	`, accountID, userID).Scan(&account.ID, &account.Name, &account.Role, &account.OwnerName, &splittingEnabled)
	if err != nil {
		return models.Account{}, err
	}
	account.Permissions = models.PermissionsForRole(account.Role)
	account.SplittingEnabled = splittingEnabled == 1
	return account, nil
}

func currentUserName(userID int64) string {
	var name string
	if err := db.DB.QueryRow(`SELECT name FROM users WHERE id = ?`, userID).Scan(&name); err != nil {
		return ""
	}
	return name
}

func lookupAccountMember(accountID, memberUserID int64) (models.AccountMember, error) {
	var member models.AccountMember
	err := db.DB.QueryRow(`
		SELECT u.id, u.name, u.email, am.role
		FROM account_members am
		INNER JOIN users u ON u.id = am.user_id
		WHERE am.account_id = ? AND am.user_id = ?
	`, accountID, memberUserID).Scan(&member.UserID, &member.Name, &member.Email, &member.Role)
	return member, err
}

func writeAccountLookupError(w http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		jsonError(w, "Conta não encontrada", http.StatusNotFound)
		return
	}
	jsonError(w, "Erro ao buscar conta", http.StatusInternalServerError)
}

func writeMemberLookupError(w http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		jsonError(w, "Membro não encontrado", http.StatusNotFound)
		return
	}
	jsonError(w, "Erro ao buscar membro", http.StatusInternalServerError)
}
