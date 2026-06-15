package handlers

import (
	"gastos/src/db"
	"gastos/src/events"
	"gastos/src/middleware"
	"gastos/src/models"
	"net/http"
	"strings"
)

func RecurringPayments(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	accountID := middleware.AccountIDFromContext(r.Context())

	switch r.Method {
	case http.MethodGet:
		rows, err := db.DB.Query(`
			SELECT rp.id, rp.account_id, rp.payer_user_id, payer.name, rp.receiver_user_id, receiver.name,
			       rp.amount, rp.note, rp.day_of_month, rp.start_date, COALESCE(rp.end_date, ''), rp.enabled
			FROM recurring_payments rp
			INNER JOIN users payer    ON payer.id    = rp.payer_user_id
			INNER JOIN users receiver ON receiver.id = rp.receiver_user_id
			WHERE rp.account_id = ?
			ORDER BY rp.enabled DESC, rp.day_of_month ASC, rp.id ASC
		`, accountID)
		if err != nil {
			jsonError(w, "Erro ao buscar pagamentos recorrentes", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		result := make([]models.RecurringPayment, 0)
		for rows.Next() {
			var rp models.RecurringPayment
			if err := rows.Scan(&rp.ID, &rp.AccountID, &rp.PayerUserID, &rp.PayerName, &rp.ReceiverUserID, &rp.ReceiverName, &rp.Amount, &rp.Note, &rp.DayOfMonth, &rp.StartDate, &rp.EndDate, &rp.Enabled); err != nil {
				jsonError(w, "Erro ao ler pagamentos recorrentes", http.StatusInternalServerError)
				return
			}
			result = append(result, rp)
		}
		if err := rows.Err(); err != nil {
			jsonError(w, "Erro ao iterar pagamentos recorrentes", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, result)

	case http.MethodPost:
		if !requireAccountEdit(w, r) {
			return
		}
		var rp models.RecurringPayment
		if err := decodeJSON(r, &rp); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		rp.AccountID = accountID
		rp.Note = strings.TrimSpace(rp.Note)
		rp.EndDate = strings.TrimSpace(rp.EndDate)
		if err := validateRecurringPayment(accountID, rp); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		res, err := db.DB.Exec(`
			INSERT INTO recurring_payments (account_id, payer_user_id, receiver_user_id, amount, note, day_of_month, start_date, end_date, enabled)
			VALUES (?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?)
		`, rp.AccountID, rp.PayerUserID, rp.ReceiverUserID, rp.Amount, rp.Note, rp.DayOfMonth, rp.StartDate, rp.EndDate, rp.Enabled)
		if err != nil {
			jsonError(w, "Erro ao salvar pagamento recorrente", http.StatusInternalServerError)
			return
		}
		id, err := res.LastInsertId()
		if err != nil {
			jsonError(w, "Erro ao obter pagamento recorrente salvo", http.StatusInternalServerError)
			return
		}
		rp.ID = id
		rp.PayerName = accountMemberName(accountID, rp.PayerUserID)
		rp.ReceiverName = accountMemberName(accountID, rp.ReceiverUserID)
		events.Bus.Notify(accountID, userID)
		writeJSON(w, http.StatusCreated, rp)

	case http.MethodPatch:
		if !requireAccountEdit(w, r) {
			return
		}
		id, err := parseIntPathID(r.URL.Path, "/api/recurring-payments/")
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		var rp models.RecurringPayment
		if err := decodeJSON(r, &rp); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		rp.ID = id
		rp.AccountID = accountID
		rp.Note = strings.TrimSpace(rp.Note)
		rp.EndDate = strings.TrimSpace(rp.EndDate)
		if err := validateRecurringPayment(accountID, rp); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		res, err := db.DB.Exec(`
			UPDATE recurring_payments
			SET payer_user_id = ?, receiver_user_id = ?, amount = ?, note = ?, day_of_month = ?, start_date = ?, end_date = NULLIF(?, ''), enabled = ?
			WHERE id = ? AND account_id = ?
		`, rp.PayerUserID, rp.ReceiverUserID, rp.Amount, rp.Note, rp.DayOfMonth, rp.StartDate, rp.EndDate, rp.Enabled, rp.ID, accountID)
		if err != nil {
			jsonError(w, "Erro ao atualizar pagamento recorrente", http.StatusInternalServerError)
			return
		}
		rowsAffected, err := res.RowsAffected()
		if err != nil {
			jsonError(w, "Erro ao confirmar atualização", http.StatusInternalServerError)
			return
		}
		if rowsAffected == 0 {
			jsonError(w, "Pagamento recorrente não encontrado", http.StatusNotFound)
			return
		}
		rp.PayerName = accountMemberName(accountID, rp.PayerUserID)
		rp.ReceiverName = accountMemberName(accountID, rp.ReceiverUserID)
		events.Bus.Notify(accountID, userID)
		writeJSON(w, http.StatusOK, rp)

	case http.MethodDelete:
		if !requireAccountEdit(w, r) {
			return
		}
		id, err := parseIntPathID(r.URL.Path, "/api/recurring-payments/")
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		res, err := db.DB.Exec(`DELETE FROM recurring_payments WHERE id = ? AND account_id = ?`, id, accountID)
		if err != nil {
			jsonError(w, "Erro ao remover pagamento recorrente", http.StatusInternalServerError)
			return
		}
		rowsAffected, err := res.RowsAffected()
		if err != nil {
			jsonError(w, "Erro ao confirmar remoção", http.StatusInternalServerError)
			return
		}
		if rowsAffected == 0 {
			jsonError(w, "Pagamento recorrente não encontrado", http.StatusNotFound)
			return
		}
		events.Bus.Notify(accountID, userID)
		w.WriteHeader(http.StatusNoContent)

	default:
		w.Header().Set("Allow", "GET, POST, PATCH, DELETE, OPTIONS")
		jsonError(w, "Método não permitido", http.StatusMethodNotAllowed)
	}
}
