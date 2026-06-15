package handlers

import (
	"gastos/src/db"
	"gastos/src/events"
	"gastos/src/middleware"
	"gastos/src/models"
	"net/http"
	"strings"
)

func RecurringIncomes(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	accountID := middleware.AccountIDFromContext(r.Context())

	switch r.Method {
	case http.MethodGet:
		rows, err := db.DB.Query(`
			SELECT id, user_id, account_id, amount, description, type, day_of_month, start_date, COALESCE(end_date, ''), enabled
			FROM recurring_incomes
			WHERE account_id = ?
			ORDER BY enabled DESC, day_of_month ASC, id ASC
		`, accountID)
		if err != nil {
			jsonError(w, "Erro ao buscar entradas recorrentes", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		result := make([]models.RecurringIncome, 0)
		for rows.Next() {
			var ri models.RecurringIncome
			if err := rows.Scan(&ri.ID, &ri.UserID, &ri.AccountID, &ri.Amount, &ri.Description, &ri.Type, &ri.DayOfMonth, &ri.StartDate, &ri.EndDate, &ri.Enabled); err != nil {
				jsonError(w, "Erro ao ler entradas recorrentes", http.StatusInternalServerError)
				return
			}
			result = append(result, ri)
		}
		if err := rows.Err(); err != nil {
			jsonError(w, "Erro ao iterar entradas recorrentes", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, result)

	case http.MethodPost:
		if !requireAccountEdit(w, r) {
			return
		}
		var ri models.RecurringIncome
		if err := decodeJSON(r, &ri); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		ri.UserID = userID
		ri.AccountID = accountID
		ri.Description = strings.TrimSpace(ri.Description)
		ri.Type = strings.TrimSpace(ri.Type)
		ri.EndDate = strings.TrimSpace(ri.EndDate)
		if err := validateRecurringIncome(ri); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		res, err := db.DB.Exec(`
			INSERT INTO recurring_incomes (user_id, account_id, amount, description, type, day_of_month, start_date, end_date, enabled)
			VALUES (?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?)
		`, ri.UserID, ri.AccountID, ri.Amount, ri.Description, ri.Type, ri.DayOfMonth, ri.StartDate, ri.EndDate, ri.Enabled)
		if err != nil {
			jsonError(w, "Erro ao salvar entrada recorrente", http.StatusInternalServerError)
			return
		}
		id, err := res.LastInsertId()
		if err != nil {
			jsonError(w, "Erro ao obter entrada recorrente salva", http.StatusInternalServerError)
			return
		}
		ri.ID = id
		events.Bus.Notify(accountID, userID)
		writeJSON(w, http.StatusCreated, ri)

	case http.MethodPatch:
		if !requireAccountEdit(w, r) {
			return
		}
		id, err := parseIntPathID(r.URL.Path, "/api/recurring-incomes/")
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		var ri models.RecurringIncome
		if err := decodeJSON(r, &ri); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		ri.ID = id
		ri.UserID = userID
		ri.AccountID = accountID
		ri.Description = strings.TrimSpace(ri.Description)
		ri.Type = strings.TrimSpace(ri.Type)
		ri.EndDate = strings.TrimSpace(ri.EndDate)
		if err := validateRecurringIncome(ri); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		res, err := db.DB.Exec(`
			UPDATE recurring_incomes
			SET user_id = ?, amount = ?, description = ?, type = ?, day_of_month = ?, start_date = ?, end_date = NULLIF(?, ''), enabled = ?
			WHERE id = ? AND account_id = ?
		`, ri.UserID, ri.Amount, ri.Description, ri.Type, ri.DayOfMonth, ri.StartDate, ri.EndDate, ri.Enabled, ri.ID, accountID)
		if err != nil {
			jsonError(w, "Erro ao atualizar entrada recorrente", http.StatusInternalServerError)
			return
		}
		rowsAffected, err := res.RowsAffected()
		if err != nil {
			jsonError(w, "Erro ao confirmar atualização", http.StatusInternalServerError)
			return
		}
		if rowsAffected == 0 {
			jsonError(w, "Entrada recorrente não encontrada", http.StatusNotFound)
			return
		}
		events.Bus.Notify(accountID, userID)
		writeJSON(w, http.StatusOK, ri)

	case http.MethodDelete:
		if !requireAccountEdit(w, r) {
			return
		}
		id, err := parseIntPathID(r.URL.Path, "/api/recurring-incomes/")
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		res, err := db.DB.Exec(`DELETE FROM recurring_incomes WHERE id = ? AND account_id = ?`, id, accountID)
		if err != nil {
			jsonError(w, "Erro ao remover entrada recorrente", http.StatusInternalServerError)
			return
		}
		rowsAffected, err := res.RowsAffected()
		if err != nil {
			jsonError(w, "Erro ao confirmar remoção", http.StatusInternalServerError)
			return
		}
		if rowsAffected == 0 {
			jsonError(w, "Entrada recorrente não encontrada", http.StatusNotFound)
			return
		}
		events.Bus.Notify(accountID, userID)
		w.WriteHeader(http.StatusNoContent)

	default:
		w.Header().Set("Allow", "GET, POST, PATCH, DELETE, OPTIONS")
		jsonError(w, "Método não permitido", http.StatusMethodNotAllowed)
	}
}
