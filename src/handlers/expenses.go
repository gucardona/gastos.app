package handlers

import (
	"encoding/json"
	"gastos/src/db"
	"gastos/src/middleware"
	"gastos/src/models"
	"net/http"
	"strconv"
	"strings"
)

func Expenses(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	switch r.Method {
	case http.MethodGet:
		rows, err := db.DB.Query(`
			SELECT id, user_id, amount, description, category, payment, date
			FROM expenses
			WHERE user_id = ?
			ORDER BY date DESC, id DESC
		`, userID)
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
				&expense.Amount,
				&expense.Description,
				&expense.Category,
				&expense.Payment,
				&expense.Date,
			); err != nil {
				jsonError(w, "Erro ao ler gastos", http.StatusInternalServerError)
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
		var expense models.Expense
		if err := decodeJSON(r, &expense); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		expense.UserID = userID
		expense.Description = strings.TrimSpace(expense.Description)
		expense.Category = strings.TrimSpace(expense.Category)
		expense.Payment = strings.TrimSpace(expense.Payment)
		if err := validateExpense(expense); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		res, err := db.DB.Exec(`
			INSERT INTO expenses (user_id, amount, description, category, payment, date)
			VALUES (?, ?, ?, ?, ?, ?)
		`, expense.UserID, expense.Amount, expense.Description, expense.Category, expense.Payment, expense.Date)
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
		writeJSON(w, http.StatusCreated, expense)

	case http.MethodDelete:
		id, err := parseIntPathID(r.URL.Path, "/api/expenses/")
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		res, err := db.DB.Exec("DELETE FROM expenses WHERE id = ? AND user_id = ?", id, userID)
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
		w.Header().Set("Allow", "GET, POST, DELETE, OPTIONS")
		jsonError(w, "Método não permitido", http.StatusMethodNotAllowed)
	}
}

func parseIntPathID(path, prefix string) (int64, error) {
	idStr := strings.TrimPrefix(path, prefix)
	idStr = strings.TrimSpace(idStr)
	if idStr == "" {
		return 0, errInvalidID
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		return 0, errInvalidID
	}
	return id, nil
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return errInvalidJSON
	}
	return nil
}
