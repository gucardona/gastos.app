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

	switch r.Method {
	case http.MethodGet:
		rows, err := db.DB.Query(`
			SELECT id, user_id, amount, description, type, date
			FROM incomes
			WHERE user_id = ?
			ORDER BY date DESC, id DESC
		`, userID)
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
		var income models.Income
		if err := decodeJSON(r, &income); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		income.UserID = userID
		income.Description = strings.TrimSpace(income.Description)
		income.Type = strings.TrimSpace(income.Type)
		if err := validateIncome(income); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		res, err := db.DB.Exec(`
			INSERT INTO incomes (user_id, amount, description, type, date)
			VALUES (?, ?, ?, ?, ?)
		`, income.UserID, income.Amount, income.Description, income.Type, income.Date)
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
		id, err := parseIntPathID(r.URL.Path, "/api/incomes/")
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		res, err := db.DB.Exec("DELETE FROM incomes WHERE id = ? AND user_id = ?", id, userID)
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
