package handlers

import (
	"gastos/src/db"
	"gastos/src/middleware"
	"gastos/src/models"
	"net/http"
	"strings"
)

func RecurringExpenses(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	accountID := middleware.AccountIDFromContext(r.Context())

	switch r.Method {
	case http.MethodGet:
		rows, err := db.DB.Query(`
			SELECT id, user_id, account_id, amount, description, category, payment, frequency, day_of_month, start_date, COALESCE(end_date, ''), enabled
			FROM recurring_expenses
			WHERE account_id = ?
			ORDER BY enabled DESC, day_of_month ASC, id ASC
		`, accountID)
		if err != nil {
			jsonError(w, "Erro ao buscar recorrências", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		recurringExpenses := make([]models.RecurringExpense, 0)
		for rows.Next() {
			var recurring models.RecurringExpense
			if err := rows.Scan(
				&recurring.ID,
				&recurring.UserID,
				&recurring.AccountID,
				&recurring.Amount,
				&recurring.Description,
				&recurring.Category,
				&recurring.Payment,
				&recurring.Frequency,
				&recurring.DayOfMonth,
				&recurring.StartDate,
				&recurring.EndDate,
				&recurring.Enabled,
			); err != nil {
				jsonError(w, "Erro ao ler recorrências", http.StatusInternalServerError)
				return
			}
			recurringExpenses = append(recurringExpenses, recurring)
		}
		if err := rows.Err(); err != nil {
			jsonError(w, "Erro ao iterar recorrências", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, recurringExpenses)

	case http.MethodPost:
		if !requireAccountEdit(w, r) {
			return
		}

		var recurring models.RecurringExpense
		if err := decodeJSON(r, &recurring); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		recurring.UserID = userID
		recurring.AccountID = accountID
		recurring.Description = strings.TrimSpace(recurring.Description)
		recurring.Category = strings.TrimSpace(recurring.Category)
		recurring.Payment = strings.TrimSpace(recurring.Payment)
		recurring.Frequency = strings.TrimSpace(recurring.Frequency)
		recurring.EndDate = strings.TrimSpace(recurring.EndDate)
		if recurring.Frequency == "" {
			recurring.Frequency = "monthly"
		}
		if err := validateRecurringExpense(recurring); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		res, err := db.DB.Exec(`
			INSERT INTO recurring_expenses (user_id, account_id, amount, description, category, payment, frequency, day_of_month, start_date, end_date, enabled)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?)
		`, recurring.UserID, recurring.AccountID, recurring.Amount, recurring.Description, recurring.Category, recurring.Payment, recurring.Frequency, recurring.DayOfMonth, recurring.StartDate, recurring.EndDate, recurring.Enabled)
		if err != nil {
			jsonError(w, "Erro ao salvar recorrência", http.StatusInternalServerError)
			return
		}

		id, err := res.LastInsertId()
		if err != nil {
			jsonError(w, "Erro ao obter recorrência salva", http.StatusInternalServerError)
			return
		}

		recurring.ID = id
		writeJSON(w, http.StatusCreated, recurring)

	case http.MethodPatch:
		if !requireAccountEdit(w, r) {
			return
		}

		id, err := parseIntPathID(r.URL.Path, "/api/recurring-expenses/")
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		var recurring models.RecurringExpense
		if err := decodeJSON(r, &recurring); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		recurring.ID = id
		recurring.UserID = userID
		recurring.AccountID = accountID
		recurring.Description = strings.TrimSpace(recurring.Description)
		recurring.Category = strings.TrimSpace(recurring.Category)
		recurring.Payment = strings.TrimSpace(recurring.Payment)
		recurring.Frequency = strings.TrimSpace(recurring.Frequency)
		recurring.EndDate = strings.TrimSpace(recurring.EndDate)
		if recurring.Frequency == "" {
			recurring.Frequency = "monthly"
		}
		if err := validateRecurringExpense(recurring); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		res, err := db.DB.Exec(`
			UPDATE recurring_expenses
			SET user_id = ?, amount = ?, description = ?, category = ?, payment = ?, frequency = ?, day_of_month = ?, start_date = ?, end_date = NULLIF(?, ''), enabled = ?
			WHERE id = ? AND account_id = ?
		`, recurring.UserID, recurring.Amount, recurring.Description, recurring.Category, recurring.Payment, recurring.Frequency, recurring.DayOfMonth, recurring.StartDate, recurring.EndDate, recurring.Enabled, recurring.ID, accountID)
		if err != nil {
			jsonError(w, "Erro ao atualizar recorrência", http.StatusInternalServerError)
			return
		}
		rowsAffected, err := res.RowsAffected()
		if err != nil {
			jsonError(w, "Erro ao confirmar atualização", http.StatusInternalServerError)
			return
		}
		if rowsAffected == 0 {
			jsonError(w, "Recorrência não encontrada", http.StatusNotFound)
			return
		}

		writeJSON(w, http.StatusOK, recurring)

	case http.MethodDelete:
		if !requireAccountEdit(w, r) {
			return
		}

		id, err := parseIntPathID(r.URL.Path, "/api/recurring-expenses/")
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		res, err := db.DB.Exec(`
			DELETE FROM recurring_expenses
			WHERE id = ? AND account_id = ?
		`, id, accountID)
		if err != nil {
			jsonError(w, "Erro ao remover recorrência", http.StatusInternalServerError)
			return
		}
		rowsAffected, err := res.RowsAffected()
		if err != nil {
			jsonError(w, "Erro ao confirmar remoção", http.StatusInternalServerError)
			return
		}
		if rowsAffected == 0 {
			jsonError(w, "Recorrência não encontrada", http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusNoContent)

	default:
		w.Header().Set("Allow", "GET, POST, PATCH, DELETE, OPTIONS")
		jsonError(w, "Método não permitido", http.StatusMethodNotAllowed)
	}
}
