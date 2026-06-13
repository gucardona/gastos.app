package handlers

import (
	"gastos/src/db"
	"gastos/src/middleware"
	"gastos/src/models"
	"net/http"
	"strconv"
	"strings"
	"unicode"
)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else if unicode.IsSpace(r) {
			b.WriteRune('_')
		}
	}
	return b.String()
}

func uniqueCategoryKey(base string, accountID int64) (string, error) {
	key := base
	for i := 2; ; i++ {
		var count int
		if err := db.DB.QueryRow(
			`SELECT COUNT(*) FROM account_categories WHERE account_id = ? AND key = ?`,
			accountID, key,
		).Scan(&count); err != nil {
			return "", err
		}
		if count == 0 {
			return key, nil
		}
		key = base + "_" + strconv.Itoa(i)
	}
}

func Categories(w http.ResponseWriter, r *http.Request) {
	accountID := middleware.AccountIDFromContext(r.Context())

	key := strings.TrimPrefix(r.URL.Path, "/api/categories/")
	key = strings.TrimSpace(key)
	isItem := key != "" && r.URL.Path != "/api/categories"

	switch r.Method {
	case http.MethodGet:
		rows, err := db.DB.Query(`
			SELECT id, account_id, key, name, icon, color, essentiality, sort_order
			FROM account_categories
			WHERE account_id = ?
			ORDER BY sort_order ASC, id ASC
		`, accountID)
		if err != nil {
			jsonError(w, "Erro ao buscar categorias", http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		cats := make([]models.Category, 0)
		for rows.Next() {
			var c models.Category
			if err := rows.Scan(&c.ID, &c.AccountID, &c.Key, &c.Name, &c.Icon, &c.Color, &c.Essentiality, &c.SortOrder); err != nil {
				jsonError(w, "Erro ao ler categorias", http.StatusInternalServerError)
				return
			}
			cats = append(cats, c)
		}
		if err := rows.Err(); err != nil {
			jsonError(w, "Erro ao iterar categorias", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, cats)

	case http.MethodPost:
		if !requireAccountEdit(w, r) {
			return
		}
		if isItem {
			jsonError(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Name         string `json:"name"`
			Icon         string `json:"icon"`
			Color        string `json:"color"`
			Essentiality string `json:"essentiality"`
		}
		if err := decodeJSON(r, &body); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		body.Name = strings.TrimSpace(body.Name)
		body.Icon = strings.TrimSpace(body.Icon)
		body.Color = strings.TrimSpace(body.Color)
		if err := validateCategory(body.Name, body.Icon, body.Color, body.Essentiality); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		base := slugify(body.Name)
		if base == "" {
			base = "categoria"
		}
		catKey, err := uniqueCategoryKey(base, accountID)
		if err != nil {
			jsonError(w, "Erro ao gerar chave", http.StatusInternalServerError)
			return
		}
		var maxOrder int
		_ = db.DB.QueryRow(`SELECT COALESCE(MAX(sort_order),0) FROM account_categories WHERE account_id = ?`, accountID).Scan(&maxOrder)

		var c models.Category
		err = db.DB.QueryRow(`
			INSERT INTO account_categories (account_id, key, name, icon, color, essentiality, sort_order)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			RETURNING id, account_id, key, name, icon, color, essentiality, sort_order
		`, accountID, catKey, body.Name, body.Icon, body.Color, body.Essentiality, maxOrder+1).Scan(
			&c.ID, &c.AccountID, &c.Key, &c.Name, &c.Icon, &c.Color, &c.Essentiality, &c.SortOrder,
		)
		if err != nil {
			jsonError(w, "Erro ao criar categoria", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, c)

	case http.MethodPatch:
		if !requireAccountEdit(w, r) {
			return
		}
		if !isItem {
			jsonError(w, "Chave de categoria obrigatória", http.StatusBadRequest)
			return
		}
		var body struct {
			Name         string `json:"name"`
			Icon         string `json:"icon"`
			Color        string `json:"color"`
			Essentiality string `json:"essentiality"`
			SortOrder    *int   `json:"sortOrder"`
		}
		if err := decodeJSON(r, &body); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		body.Name = strings.TrimSpace(body.Name)
		body.Icon = strings.TrimSpace(body.Icon)
		body.Color = strings.TrimSpace(body.Color)
		if err := validateCategory(body.Name, body.Icon, body.Color, body.Essentiality); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		sortOrder := 0
		if body.SortOrder != nil {
			sortOrder = *body.SortOrder
		}
		res, err := db.DB.Exec(`
			UPDATE account_categories
			SET name = ?, icon = ?, color = ?, essentiality = ?, sort_order = ?
			WHERE account_id = ? AND key = ?
		`, body.Name, body.Icon, body.Color, body.Essentiality, sortOrder, accountID, key)
		if err != nil {
			jsonError(w, "Erro ao atualizar categoria", http.StatusInternalServerError)
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			jsonError(w, "Categoria não encontrada", http.StatusNotFound)
			return
		}
		var c models.Category
		_ = db.DB.QueryRow(`
			SELECT id, account_id, key, name, icon, color, essentiality, sort_order
			FROM account_categories WHERE account_id = ? AND key = ?
		`, accountID, key).Scan(&c.ID, &c.AccountID, &c.Key, &c.Name, &c.Icon, &c.Color, &c.Essentiality, &c.SortOrder)
		writeJSON(w, http.StatusOK, c)

	case http.MethodDelete:
		if !requireAccountEdit(w, r) {
			return
		}
		if !isItem {
			jsonError(w, "Chave de categoria obrigatória", http.StatusBadRequest)
			return
		}
		replacement := strings.TrimSpace(r.URL.Query().Get("replacement"))
		if replacement == "" {
			jsonError(w, "Chave de substituição obrigatória", http.StatusBadRequest)
			return
		}
		if replacement == key {
			jsonError(w, "Substituição deve ser diferente da categoria removida", http.StatusBadRequest)
			return
		}
		var repCount int
		if err := db.DB.QueryRow(`SELECT COUNT(*) FROM account_categories WHERE account_id = ? AND key = ?`, accountID, replacement).Scan(&repCount); err != nil || repCount == 0 {
			jsonError(w, "Categoria de substituição não encontrada", http.StatusBadRequest)
			return
		}
		var total int
		_ = db.DB.QueryRow(`SELECT COUNT(*) FROM account_categories WHERE account_id = ?`, accountID).Scan(&total)
		if total <= 1 {
			jsonError(w, "Mantenha ao menos uma categoria", http.StatusBadRequest)
			return
		}

		tx, err := db.DB.Begin()
		if err != nil {
			jsonError(w, "Erro ao remover categoria", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		if _, err := tx.Exec(`UPDATE expenses SET category = ? WHERE account_id = ? AND category = ?`, replacement, accountID, key); err != nil {
			jsonError(w, "Erro ao reatribuir gastos", http.StatusInternalServerError)
			return
		}
		if _, err := tx.Exec(`UPDATE recurring_expenses SET category = ? WHERE account_id = ? AND category = ?`, replacement, accountID, key); err != nil {
			jsonError(w, "Erro ao reatribuir gastos recorrentes", http.StatusInternalServerError)
			return
		}
		res, err := tx.Exec(`DELETE FROM account_categories WHERE account_id = ? AND key = ?`, accountID, key)
		if err != nil {
			jsonError(w, "Erro ao remover categoria", http.StatusInternalServerError)
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			jsonError(w, "Categoria não encontrada", http.StatusNotFound)
			return
		}
		if err := tx.Commit(); err != nil {
			jsonError(w, "Erro ao remover categoria", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		w.Header().Set("Allow", "GET, POST, PATCH, DELETE, OPTIONS")
		jsonError(w, "Método não permitido", http.StatusMethodNotAllowed)
	}
}
