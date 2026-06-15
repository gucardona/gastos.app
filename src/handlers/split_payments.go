package handlers

import (
	"gastos/src/db"
	"gastos/src/events"
	"gastos/src/middleware"
	"gastos/src/models"
	"net/http"
	"strings"
)

func SplitPayments(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	accountID := middleware.AccountIDFromContext(r.Context())

	switch r.Method {
	case http.MethodGet:
		rows, err := db.DB.Query(`
			SELECT sp.id, sp.account_id, sp.payer_user_id, payer.name, sp.receiver_user_id, receiver.name, sp.amount, sp.date, sp.note
			FROM split_payments sp
			INNER JOIN users payer ON payer.id = sp.payer_user_id
			INNER JOIN users receiver ON receiver.id = sp.receiver_user_id
			WHERE sp.account_id = ?
			ORDER BY sp.date DESC, sp.id DESC
		`, accountID)
		if err != nil {
			jsonError(w, "Erro ao buscar pagamentos", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		payments := make([]models.SplitPayment, 0)
		for rows.Next() {
			var payment models.SplitPayment
			if err := rows.Scan(
				&payment.ID,
				&payment.AccountID,
				&payment.PayerUserID,
				&payment.PayerName,
				&payment.ReceiverUserID,
				&payment.ReceiverName,
				&payment.Amount,
				&payment.Date,
				&payment.Note,
			); err != nil {
				jsonError(w, "Erro ao ler pagamentos", http.StatusInternalServerError)
				return
			}
			payments = append(payments, payment)
		}
		if err := rows.Err(); err != nil {
			jsonError(w, "Erro ao iterar pagamentos", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, payments)

	case http.MethodPost:
		if !requireAccountEdit(w, r) {
			return
		}

		var payment models.SplitPayment
		if err := decodeJSON(r, &payment); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		payment.AccountID = accountID
		payment.Note = strings.TrimSpace(payment.Note)
		if err := validateSplitPayment(accountID, payment); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		res, err := db.DB.Exec(`
			INSERT INTO split_payments (account_id, payer_user_id, receiver_user_id, amount, date, note)
			VALUES (?, ?, ?, ?, ?, ?)
		`, payment.AccountID, payment.PayerUserID, payment.ReceiverUserID, payment.Amount, payment.Date, payment.Note)
		if err != nil {
			jsonError(w, "Erro ao salvar pagamento", http.StatusInternalServerError)
			return
		}
		id, err := res.LastInsertId()
		if err != nil {
			jsonError(w, "Erro ao obter pagamento salvo", http.StatusInternalServerError)
			return
		}

		payment.ID = id
		payment.PayerName = accountMemberName(accountID, payment.PayerUserID)
		payment.ReceiverName = accountMemberName(accountID, payment.ReceiverUserID)
		events.Bus.Notify(accountID, userID)
		writeJSON(w, http.StatusCreated, payment)

	case http.MethodPatch:
		if !requireAccountEdit(w, r) {
			return
		}

		id, err := parseIntPathID(r.URL.Path, "/api/split-payments/")
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		var payment models.SplitPayment
		if err := decodeJSON(r, &payment); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		payment.ID = id
		payment.AccountID = accountID
		payment.Note = strings.TrimSpace(payment.Note)
		if err := validateSplitPayment(accountID, payment); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		res, err := db.DB.Exec(`
			UPDATE split_payments
			SET payer_user_id = ?, receiver_user_id = ?, amount = ?, date = ?, note = ?
			WHERE id = ? AND account_id = ?
		`, payment.PayerUserID, payment.ReceiverUserID, payment.Amount, payment.Date, payment.Note, payment.ID, accountID)
		if err != nil {
			jsonError(w, "Erro ao atualizar pagamento", http.StatusInternalServerError)
			return
		}
		rowsAffected, err := res.RowsAffected()
		if err != nil {
			jsonError(w, "Erro ao confirmar atualização", http.StatusInternalServerError)
			return
		}
		if rowsAffected == 0 {
			jsonError(w, "Pagamento não encontrado", http.StatusNotFound)
			return
		}

		payment.PayerName = accountMemberName(accountID, payment.PayerUserID)
		payment.ReceiverName = accountMemberName(accountID, payment.ReceiverUserID)
		events.Bus.Notify(accountID, userID)
		writeJSON(w, http.StatusOK, payment)

	case http.MethodDelete:
		if !requireAccountEdit(w, r) {
			return
		}

		id, err := parseIntPathID(r.URL.Path, "/api/split-payments/")
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		res, err := db.DB.Exec(`DELETE FROM split_payments WHERE id = ? AND account_id = ?`, id, accountID)
		if err != nil {
			jsonError(w, "Erro ao remover pagamento", http.StatusInternalServerError)
			return
		}
		rowsAffected, err := res.RowsAffected()
		if err != nil {
			jsonError(w, "Erro ao confirmar remoção", http.StatusInternalServerError)
			return
		}
		if rowsAffected == 0 {
			jsonError(w, "Pagamento não encontrado", http.StatusNotFound)
			return
		}

		events.Bus.Notify(accountID, userID)
		w.WriteHeader(http.StatusNoContent)

	default:
		w.Header().Set("Allow", "GET, POST, PATCH, DELETE, OPTIONS")
		jsonError(w, "Método não permitido", http.StatusMethodNotAllowed)
	}
}

func validateSplitPayment(accountID int64, payment models.SplitPayment) error {
	if payment.Amount <= 0 {
		return errAmountPositive
	}
	if !isValidDate(payment.Date) {
		return errInvalidDate
	}
	if payment.PayerUserID <= 0 || payment.ReceiverUserID <= 0 || payment.PayerUserID == payment.ReceiverUserID {
		return errInvalidSplitPaymentMembers
	}
	members, err := accountMemberSplits(accountID)
	if err != nil {
		return err
	}
	foundPayer := false
	foundReceiver := false
	for _, member := range members {
		if member.UserID == payment.PayerUserID {
			foundPayer = true
		}
		if member.UserID == payment.ReceiverUserID {
			foundReceiver = true
		}
	}
	if !foundPayer || !foundReceiver {
		return errInvalidSplitPaymentMembers
	}
	return nil
}

func accountMemberName(accountID, userID int64) string {
	members, err := accountMemberSplits(accountID)
	if err != nil {
		return ""
	}
	for _, member := range members {
		if member.UserID == userID {
			return member.Name
		}
	}
	return ""
}
