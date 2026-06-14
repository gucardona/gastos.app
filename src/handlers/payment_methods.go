package handlers

import (
	"gastos/src/db"
	"gastos/src/middleware"
	"gastos/src/models"
	"net/http"
	"strconv"
	"strings"
)

func uniquePaymentKey(base string, accountID int64) (string, error) {
	key := base
	for i := 2; ; i++ {
		var count int
		if err := db.DB.QueryRow(
			`SELECT COUNT(*) FROM account_payment_methods WHERE account_id = ? AND key = ?`,
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

func PaymentMethods(w http.ResponseWriter, r *http.Request) {
	accountID := middleware.AccountIDFromContext(r.Context())

	key := strings.TrimPrefix(r.URL.Path, "/api/payment-methods/")
	key = strings.TrimSpace(key)
	isItem := key != "" && r.URL.Path != "/api/payment-methods"

	switch r.Method {
	case http.MethodGet:
		rows, err := db.DB.Query(`
			SELECT id, account_id, key, icon, name, sort_order
			FROM account_payment_methods
			WHERE account_id = ?
			ORDER BY sort_order ASC, id ASC
		`, accountID)
		if err != nil {
			jsonError(w, "Erro ao buscar métodos de pagamento", http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		pms := make([]models.PaymentMethod, 0)
		for rows.Next() {
			var p models.PaymentMethod
			if err := rows.Scan(&p.ID, &p.AccountID, &p.Key, &p.Icon, &p.Name, &p.SortOrder); err != nil {
				jsonError(w, "Erro ao ler métodos de pagamento", http.StatusInternalServerError)
				return
			}
			pms = append(pms, p)
		}
		if err := rows.Err(); err != nil {
			jsonError(w, "Erro ao iterar métodos de pagamento", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, pms)

	case http.MethodPost:
		if !requireAccountEdit(w, r) {
			return
		}
		if isItem {
			jsonError(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Name string `json:"name"`
			Icon string `json:"icon"`
		}
		if err := decodeJSON(r, &body); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		body.Name = strings.TrimSpace(body.Name)
		body.Icon = strings.TrimSpace(body.Icon)
		if err := validatePaymentMethod(body.Name, body.Icon); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		base := slugify(body.Name)
		if base == "" {
			base = "pagamento"
		}
		pmKey, err := uniquePaymentKey(base, accountID)
		if err != nil {
			jsonError(w, "Erro ao gerar chave", http.StatusInternalServerError)
			return
		}
		var maxOrder int
		_ = db.DB.QueryRow(`SELECT COALESCE(MAX(sort_order),0) FROM account_payment_methods WHERE account_id = ?`, accountID).Scan(&maxOrder)

		var p models.PaymentMethod
		err = db.DB.QueryRow(`
			INSERT INTO account_payment_methods (account_id, key, icon, name, sort_order)
			VALUES (?, ?, ?, ?, ?)
			RETURNING id, account_id, key, icon, name, sort_order
		`, accountID, pmKey, body.Icon, body.Name, maxOrder+1).Scan(
			&p.ID, &p.AccountID, &p.Key, &p.Icon, &p.Name, &p.SortOrder,
		)
		if err != nil {
			jsonError(w, "Erro ao criar método de pagamento", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, p)

	case http.MethodPatch:
		if !requireAccountEdit(w, r) {
			return
		}
		if !isItem {
			jsonError(w, "Chave de método obrigatória", http.StatusBadRequest)
			return
		}
		var body struct {
			Name      string `json:"name"`
			Icon      string `json:"icon"`
			SortOrder *int   `json:"sortOrder"`
		}
		if err := decodeJSON(r, &body); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		body.Name = strings.TrimSpace(body.Name)
		body.Icon = strings.TrimSpace(body.Icon)
		if err := validatePaymentMethod(body.Name, body.Icon); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		var sortOrder int
		if body.SortOrder != nil {
			sortOrder = *body.SortOrder
		} else {
			_ = db.DB.QueryRow(`SELECT sort_order FROM account_payment_methods WHERE account_id = ? AND key = ?`, accountID, key).Scan(&sortOrder)
		}
		res, err := db.DB.Exec(`
			UPDATE account_payment_methods
			SET name = ?, icon = ?, sort_order = ?
			WHERE account_id = ? AND key = ?
		`, body.Name, body.Icon, sortOrder, accountID, key)
		if err != nil {
			jsonError(w, "Erro ao atualizar método de pagamento", http.StatusInternalServerError)
			return
		}
		n, err := res.RowsAffected()
		if err != nil {
			jsonError(w, "Erro ao confirmar atualização", http.StatusInternalServerError)
			return
		}
		if n == 0 {
			jsonError(w, "Método de pagamento não encontrado", http.StatusNotFound)
			return
		}
		var p models.PaymentMethod
		if err := db.DB.QueryRow(`
			SELECT id, account_id, key, icon, name, sort_order
			FROM account_payment_methods WHERE account_id = ? AND key = ?
		`, accountID, key).Scan(&p.ID, &p.AccountID, &p.Key, &p.Icon, &p.Name, &p.SortOrder); err != nil {
			jsonError(w, "Erro ao ler método atualizado", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, p)

	case http.MethodDelete:
		if !requireAccountEdit(w, r) {
			return
		}
		if !isItem {
			jsonError(w, "Chave de método obrigatória", http.StatusBadRequest)
			return
		}
		replacement := strings.TrimSpace(r.URL.Query().Get("replacement"))
		if replacement == "" {
			jsonError(w, "Chave de substituição obrigatória", http.StatusBadRequest)
			return
		}
		if replacement == key {
			jsonError(w, "Substituição deve ser diferente do método removido", http.StatusBadRequest)
			return
		}

		tx, err := db.DB.Begin()
		if err != nil {
			jsonError(w, "Erro ao remover método de pagamento", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		var repCount int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM account_payment_methods WHERE account_id = ? AND key = ?`, accountID, replacement).Scan(&repCount); err != nil {
			jsonError(w, "Erro ao verificar substituição", http.StatusInternalServerError)
			return
		}
		if repCount == 0 {
			jsonError(w, "Método de substituição não encontrado", http.StatusBadRequest)
			return
		}
		var total int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM account_payment_methods WHERE account_id = ?`, accountID).Scan(&total); err != nil {
			jsonError(w, "Erro ao verificar métodos", http.StatusInternalServerError)
			return
		}
		if total <= 1 {
			jsonError(w, "Mantenha ao menos um método de pagamento", http.StatusBadRequest)
			return
		}

		if _, err := tx.Exec(`UPDATE expenses SET payment = ? WHERE account_id = ? AND payment = ?`, replacement, accountID, key); err != nil {
			jsonError(w, "Erro ao reatribuir gastos", http.StatusInternalServerError)
			return
		}
		if _, err := tx.Exec(`UPDATE recurring_expenses SET payment = ? WHERE account_id = ? AND payment = ?`, replacement, accountID, key); err != nil {
			jsonError(w, "Erro ao reatribuir gastos recorrentes", http.StatusInternalServerError)
			return
		}
		res, err := tx.Exec(`DELETE FROM account_payment_methods WHERE account_id = ? AND key = ?`, accountID, key)
		if err != nil {
			jsonError(w, "Erro ao remover método de pagamento", http.StatusInternalServerError)
			return
		}
		n, err := res.RowsAffected()
		if err != nil {
			jsonError(w, "Erro ao confirmar remoção", http.StatusInternalServerError)
			return
		}
		if n == 0 {
			jsonError(w, "Método de pagamento não encontrado", http.StatusNotFound)
			return
		}
		if err := tx.Commit(); err != nil {
			jsonError(w, "Erro ao remover método de pagamento", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		w.Header().Set("Allow", "GET, POST, PATCH, DELETE, OPTIONS")
		jsonError(w, "Método não permitido", http.StatusMethodNotAllowed)
	}
}
