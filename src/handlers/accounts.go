package handlers

import (
	"database/sql"
	"errors"
	"gastos/src/db"
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
			SELECT a.id, a.name, am.role
			FROM accounts a
			INNER JOIN account_members am ON am.account_id = a.id
			WHERE am.user_id = ?
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
				accountID int64
				name      string
				role      string
			)
			if err := rows.Scan(&accountID, &name, &role); err != nil {
				jsonError(w, "Erro ao ler contas", http.StatusInternalServerError)
				return
			}
			accounts = append(accounts, accountResponse(accountID, name, role))
		}
		if err := rows.Err(); err != nil {
			jsonError(w, "Erro ao iterar contas", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, accounts)

	case http.MethodPost:
		var body struct {
			Name string `json:"name"`
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

		accountID, err := db.CreateAccountWithOwner(tx, body.Name, userID)
		if err != nil {
			jsonError(w, "Erro ao criar conta", http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			jsonError(w, "Erro ao criar conta", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusCreated, accountResponse(accountID, body.Name, models.AccountRoleOwner))

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
			Name string `json:"name"`
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

		if _, err := db.DB.Exec(`UPDATE accounts SET name = ? WHERE id = ?`, body.Name, accountID); err != nil {
			jsonError(w, "Erro ao renomear conta", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, accountResponse(account.ID, body.Name, account.Role))

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
	if account.Role != models.AccountRoleOwner {
		jsonError(w, "Apenas o owner pode gerenciar membros", http.StatusForbidden)
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
	if account.Role != models.AccountRoleOwner {
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

		w.WriteHeader(http.StatusNoContent)

	default:
		w.Header().Set("Allow", "PATCH, DELETE, OPTIONS")
		jsonError(w, "Método não permitido", http.StatusMethodNotAllowed)
	}
}

func lookupAccountForUser(userID, accountID int64) (models.Account, error) {
	var account models.Account
	err := db.DB.QueryRow(`
		SELECT a.id, a.name, am.role
		FROM accounts a
		INNER JOIN account_members am ON am.account_id = a.id
		WHERE a.id = ? AND am.user_id = ?
	`, accountID, userID).Scan(&account.ID, &account.Name, &account.Role)
	if err != nil {
		return models.Account{}, err
	}
	account.Permissions = models.PermissionsForRole(account.Role)
	return account, nil
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
