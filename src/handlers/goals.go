package handlers

import (
	"gastos/src/db"
	"gastos/src/middleware"
	"gastos/src/models"
	"net/http"
	"strings"
)

func Goals(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	switch r.Method {
	case http.MethodGet:
		rows, err := db.DB.Query(`
			SELECT id, user_id, category, "limit"
			FROM goals
			WHERE user_id = ?
			ORDER BY category ASC
		`, userID)
		if err != nil {
			jsonError(w, "Erro ao buscar metas", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		goals := make([]models.Goal, 0)
		for rows.Next() {
			var goal models.Goal
			if err := rows.Scan(&goal.ID, &goal.UserID, &goal.Category, &goal.Limit); err != nil {
				jsonError(w, "Erro ao ler metas", http.StatusInternalServerError)
				return
			}
			goals = append(goals, goal)
		}
		if err := rows.Err(); err != nil {
			jsonError(w, "Erro ao iterar metas", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, goals)

	case http.MethodPost:
		var goal models.Goal
		if err := decodeJSON(r, &goal); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		goal.UserID = userID
		goal.Category = strings.TrimSpace(goal.Category)
		if err := validateGoal(goal); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		_, err := db.DB.Exec(`
			INSERT INTO goals (user_id, category, "limit")
			VALUES (?, ?, ?)
			ON CONFLICT(user_id, category)
			DO UPDATE SET "limit" = excluded."limit"
		`, goal.UserID, goal.Category, goal.Limit)
		if err != nil {
			jsonError(w, "Erro ao salvar meta", http.StatusInternalServerError)
			return
		}

		if err := db.DB.QueryRow(`
			SELECT id, user_id, category, "limit"
			FROM goals
			WHERE user_id = ? AND category = ?
		`, goal.UserID, goal.Category).Scan(&goal.ID, &goal.UserID, &goal.Category, &goal.Limit); err != nil {
			jsonError(w, "Erro ao buscar meta salva", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusCreated, goal)

	case http.MethodDelete:
		category := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/goals/"))
		if category == "" {
			jsonError(w, "Categoria inválida", http.StatusBadRequest)
			return
		}

		res, err := db.DB.Exec("DELETE FROM goals WHERE user_id = ? AND category = ?", userID, category)
		if err != nil {
			jsonError(w, "Erro ao remover meta", http.StatusInternalServerError)
			return
		}
		rowsAffected, err := res.RowsAffected()
		if err != nil {
			jsonError(w, "Erro ao confirmar remoção", http.StatusInternalServerError)
			return
		}
		if rowsAffected == 0 {
			jsonError(w, "Meta não encontrada", http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusNoContent)

	default:
		w.Header().Set("Allow", "GET, POST, DELETE, OPTIONS")
		jsonError(w, "Método não permitido", http.StatusMethodNotAllowed)
	}
}
