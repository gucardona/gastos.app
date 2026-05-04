package handlers

import (
	"gastos/src/db"
	"gastos/src/middleware"
	"gastos/src/models"
	"net/http"
	"strings"
)

func Expenses(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	accountID := middleware.AccountIDFromContext(r.Context())

	switch r.Method {
	case http.MethodGet:
		rows, err := db.DB.Query(`
			SELECT id, user_id, account_id, amount, description, category, payment, date
			FROM expenses
			WHERE account_id = ?
			ORDER BY date DESC, id DESC
		`, accountID)
		if err != nil {
			jsonError(w, "Erro ao buscar gastos", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		expenses := make([]models.Expense, 0)
		for rows.Next() {
			var expense models.Expense
			if err := rows.Scan(
				&expense.ID,
				&expense.UserID,
				&expense.AccountID,
				&expense.Amount,
				&expense.Description,
				&expense.Category,
				&expense.Payment,
				&expense.Date,
			); err != nil {
				jsonError(w, "Erro ao ler gastos", http.StatusInternalServerError)
				return
			}
			expense.Splits, err = loadExpenseSplits(expense.ID, expense.Amount)
			if err != nil {
				jsonError(w, "Erro ao ler racha do gasto", http.StatusInternalServerError)
				return
			}
			expenses = append(expenses, expense)
		}
		if err := rows.Err(); err != nil {
			jsonError(w, "Erro ao iterar gastos", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, expenses)

	case http.MethodPost:
		if !requireAccountEdit(w, r) {
			return
		}

		var expense models.Expense
		if err := decodeJSON(r, &expense); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		expense.UserID = userID
		expense.AccountID = accountID
		expense.Description = strings.TrimSpace(expense.Description)
		expense.Category = strings.TrimSpace(expense.Category)
		expense.Payment = strings.TrimSpace(expense.Payment)
		if err := validateExpense(expense); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		splits, err := normalizeSplits(accountID, expense.Amount, expense.Splits)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		tx, err := db.DB.Begin()
		if err != nil {
			jsonError(w, "Erro ao salvar gasto", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		res, err := tx.Exec(`
			INSERT INTO expenses (user_id, account_id, amount, description, category, payment, date)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, expense.UserID, expense.AccountID, expense.Amount, expense.Description, expense.Category, expense.Payment, expense.Date)
		if err != nil {
			jsonError(w, "Erro ao salvar gasto", http.StatusInternalServerError)
			return
		}

		id, err := res.LastInsertId()
		if err != nil {
			jsonError(w, "Erro ao obter gasto salvo", http.StatusInternalServerError)
			return
		}

		expense.ID = id
		expense.Splits = splits
		if err := replaceExpenseSplits(tx, expense.ID, expense.Splits); err != nil {
			jsonError(w, "Erro ao salvar racha do gasto", http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			jsonError(w, "Erro ao salvar gasto", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, expense)

	case http.MethodPatch:
		if !requireAccountEdit(w, r) {
			return
		}

		id, err := parseIntPathID(r.URL.Path, "/api/expenses/")
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		var expense models.Expense
		if err := decodeJSON(r, &expense); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		expense.ID = id
		expense.UserID = userID
		expense.AccountID = accountID
		expense.Description = strings.TrimSpace(expense.Description)
		expense.Category = strings.TrimSpace(expense.Category)
		expense.Payment = strings.TrimSpace(expense.Payment)
		if err := validateExpense(expense); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		splits, err := normalizeSplits(accountID, expense.Amount, expense.Splits)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		tx, err := db.DB.Begin()
		if err != nil {
			jsonError(w, "Erro ao atualizar gasto", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		res, err := tx.Exec(`
			UPDATE expenses
			SET user_id = ?, amount = ?, description = ?, category = ?, payment = ?, date = ?
			WHERE id = ? AND account_id = ?
		`, expense.UserID, expense.Amount, expense.Description, expense.Category, expense.Payment, expense.Date, expense.ID, accountID)
		if err != nil {
			jsonError(w, "Erro ao atualizar gasto", http.StatusInternalServerError)
			return
		}
		rowsAffected, err := res.RowsAffected()
		if err != nil {
			jsonError(w, "Erro ao confirmar atualização", http.StatusInternalServerError)
			return
		}
		if rowsAffected == 0 {
			jsonError(w, "Gasto não encontrado", http.StatusNotFound)
			return
		}

		expense.Splits = splits
		if err := replaceExpenseSplits(tx, expense.ID, expense.Splits); err != nil {
			jsonError(w, "Erro ao atualizar racha do gasto", http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			jsonError(w, "Erro ao atualizar gasto", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, expense)

	case http.MethodDelete:
		if !requireAccountEdit(w, r) {
			return
		}

		id, err := parseIntPathID(r.URL.Path, "/api/expenses/")
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		res, err := db.DB.Exec("DELETE FROM expenses WHERE id = ? AND account_id = ?", id, accountID)
		if err != nil {
			jsonError(w, "Erro ao remover gasto", http.StatusInternalServerError)
			return
		}
		rowsAffected, err := res.RowsAffected()
		if err != nil {
			jsonError(w, "Erro ao confirmar remoção", http.StatusInternalServerError)
			return
		}
		if rowsAffected == 0 {
			jsonError(w, "Gasto não encontrado", http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusNoContent)

	default:
		w.Header().Set("Allow", "GET, POST, PATCH, DELETE, OPTIONS")
		jsonError(w, "Método não permitido", http.StatusMethodNotAllowed)
	}
}
