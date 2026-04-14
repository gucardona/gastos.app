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
	uid := r.Context().Value(middleware.UserIDKey).(int64)
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		rows, err := db.DB.Query(
			`SELECT id,amount,description,category,payment,date FROM expenses
             WHERE user_id=? ORDER BY date DESC, id DESC`, uid)
		if err != nil {
			jsonError(w, "db error", 500)
			return
		}
		defer rows.Close()
		items := []models.Expense{}
		for rows.Next() {
			var e models.Expense
			rows.Scan(&e.ID, &e.Amount, &e.Description, &e.Category, &e.Payment, &e.Date)
			items = append(items, e)
		}
		json.NewEncoder(w).Encode(items)

	case http.MethodPost:
		var e models.Expense
		json.NewDecoder(r.Body).Decode(&e)
		if e.Amount <= 0 {
			jsonError(w, "valor inválido", 400)
			return
		}
		res, err := db.DB.Exec(
			`INSERT INTO expenses(user_id,amount,description,category,payment,date)
             VALUES(?,?,?,?,?,?)`,
			uid, e.Amount, e.Description, e.Category, e.Payment, e.Date)
		if err != nil {
			jsonError(w, "db error", 500)
			return
		}
		e.ID, _ = res.LastInsertId()
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(e)

	case http.MethodDelete:
		// DELETE /api/expenses/123
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		id, _ := strconv.ParseInt(parts[len(parts)-1], 10, 64)
		db.DB.Exec(`DELETE FROM expenses WHERE id=? AND user_id=?`, id, uid)
		w.WriteHeader(http.StatusNoContent)
	}
}
