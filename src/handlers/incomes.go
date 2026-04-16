package handlers

import (
	"gastos/src/db"
	"gastos/src/middleware"
	"gastos/src/models"
	"net/http"
	"strings"
)

func Incomes(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	accountID := middleware.AccountIDFromContext(r.Context())

	switch r.Method {
	case http.MethodGet:
		rows, err := db.DB.Query(`
			SELECT id, user_id, account_id, amount, description, type, date
			FROM incomes
			WHERE account_id = ?
			ORDER BY date DESC, id DESC
		`, accountID)
		if err != nil {
			jsonError(w, "Erro ao buscar entradas", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		incomes := make([]models.Income, 0)
		for rows.Next() {
			var income models.Income
			if err := rows.Scan(
				&income.ID,
				&income.UserID,
				&income.AccountID,
				&income.Amount,
				&income.Description,
				&income.Type,
				&income.Date,
			); err != nil {
				jsonError(w, "Erro ao ler entradas", http.StatusInternalServerError)
				return
			}
			incomes = append(incomes, income)
		}
		if err := rows.Err(); err != nil {
			jsonError(w, "Erro ao iterar entradas", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, incomes)

	case http.MethodPost:
		if !requireAccountEdit(w, r) {
			return
		}

		var income models.Income
		if err := decodeJSON(r, &income); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		income.UserID = userID
		income.AccountID = accountID
		income.Description = strings.TrimSpace(income.Description)
		income.Type = strings.TrimSpace(income.Type)
		if err := validateIncome(income); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		res, err := db.DB.Exec(`
			INSERT INTO incomes (user_id, account_id, amount, description, type, date)
			VALUES (?, ?, ?, ?, ?, ?)
		`, income.UserID, income.AccountID, income.Amount, income.Description, income.Type, income.Date)
		if err != nil {
			jsonError(w, "Erro ao salvar entrada", http.StatusInternalServerError)
			return
		}

		id, err := res.LastInsertId()
		if err != nil {
			jsonError(w, "Erro ao obter entrada salva", http.StatusInternalServerError)
			return
		}

		income.ID = id
		writeJSON(w, http.StatusCreated, income)

	case http.MethodDelete:
		if !requireAccountEdit(w, r) {
			return
		}

		id, err := parseIntPathID(r.URL.Path, "/api/incomes/")
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		res, err := db.DB.Exec("DELETE FROM incomes WHERE id = ? AND account_id = ?", id, accountID)
		if err != nil {
			jsonError(w, "Erro ao remover entrada", http.StatusInternalServerError)
			return
		}
		rowsAffected, err := res.RowsAffected()
		if err != nil {
			jsonError(w, "Erro ao confirmar remoção", http.StatusInternalServerError)
			return
		}
		if rowsAffected == 0 {
			jsonError(w, "Entrada não encontrada", http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusNoContent)

	default:
		w.Header().Set("Allow", "GET, POST, DELETE, OPTIONS")
		jsonError(w, "Método não permitido", http.StatusMethodNotAllowed)
	}
}
