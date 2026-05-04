package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"gastos/src/db"
	"gastos/src/models"
	"math"
)

func normalizeSplits(accountID int64, amount float64, requested []models.Split) ([]models.Split, error) {
	members, err := accountMemberSplits(accountID)
	if err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return nil, errors.New("a carteira precisa ter ao menos um participante")
	}

	byUser := make(map[int64]models.Split, len(members))
	for _, member := range members {
		byUser[member.UserID] = member
	}

	if len(requested) == 0 {
		percentage := 100 / float64(len(members))
		for i := range members {
			members[i].Percentage = percentage
			if i == len(members)-1 {
				members[i].Percentage = 100 - percentage*float64(len(members)-1)
			}
			members[i].Amount = splitAmount(amount, members[i].Percentage)
		}
		return members, nil
	}

	seen := make(map[int64]bool, len(requested))
	result := make([]models.Split, 0, len(requested))
	total := 0.0
	for _, split := range requested {
		if split.UserID <= 0 {
			return nil, errors.New("participante do racha inválido")
		}
		member, ok := byUser[split.UserID]
		if !ok {
			return nil, errors.New("participante não pertence a esta carteira")
		}
		if seen[split.UserID] {
			return nil, errors.New("participante repetido no racha")
		}
		if split.Percentage <= 0 || split.Percentage > 100 {
			return nil, errors.New("percentual do racha deve estar entre 0 e 100")
		}
		seen[split.UserID] = true
		member.Percentage = math.Round(split.Percentage*10000) / 10000
		member.Amount = splitAmount(amount, member.Percentage)
		total += member.Percentage
		result = append(result, member)
	}

	if math.Abs(total-100) > 0.01 {
		return nil, fmt.Errorf("percentuais do racha devem somar 100%%, soma atual %.2f%%", total)
	}
	return result, nil
}

func accountMemberSplits(accountID int64) ([]models.Split, error) {
	rows, err := db.DB.Query(`
		SELECT u.id, u.name, u.email
		FROM account_members am
		INNER JOIN users u ON u.id = am.user_id
		WHERE am.account_id = ?
		ORDER BY
			CASE am.role
				WHEN 'owner' THEN 0
				WHEN 'editor' THEN 1
				ELSE 2
			END,
			u.name ASC,
			u.id ASC
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := make([]models.Split, 0)
	for rows.Next() {
		var member models.Split
		if err := rows.Scan(&member.UserID, &member.Name, &member.Email); err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

func loadExpenseSplits(expenseID int64, amount float64) ([]models.Split, error) {
	return loadSplits(`
		SELECT es.user_id, u.name, u.email, es.percentage
		FROM expense_splits es
		INNER JOIN users u ON u.id = es.user_id
		WHERE es.expense_id = ?
		ORDER BY u.name ASC, u.id ASC
	`, expenseID, amount)
}

func loadRecurringExpenseSplits(recurringExpenseID int64, amount float64) ([]models.Split, error) {
	return loadSplits(`
		SELECT res.user_id, u.name, u.email, res.percentage
		FROM recurring_expense_splits res
		INNER JOIN users u ON u.id = res.user_id
		WHERE res.recurring_expense_id = ?
		ORDER BY u.name ASC, u.id ASC
	`, recurringExpenseID, amount)
}

func loadSplits(query string, parentID int64, amount float64) ([]models.Split, error) {
	rows, err := db.DB.Query(query, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	splits := make([]models.Split, 0)
	for rows.Next() {
		var split models.Split
		if err := rows.Scan(&split.UserID, &split.Name, &split.Email, &split.Percentage); err != nil {
			return nil, err
		}
		split.Amount = splitAmount(amount, split.Percentage)
		splits = append(splits, split)
	}
	return splits, rows.Err()
}

func replaceExpenseSplits(tx *sql.Tx, expenseID int64, splits []models.Split) error {
	if _, err := tx.Exec(`DELETE FROM expense_splits WHERE expense_id = ?`, expenseID); err != nil {
		return err
	}
	for _, split := range splits {
		if _, err := tx.Exec(`
			INSERT INTO expense_splits (expense_id, user_id, percentage)
			VALUES (?, ?, ?)
		`, expenseID, split.UserID, split.Percentage); err != nil {
			return err
		}
	}
	return nil
}

func replaceRecurringExpenseSplits(tx *sql.Tx, recurringExpenseID int64, splits []models.Split) error {
	if _, err := tx.Exec(`DELETE FROM recurring_expense_splits WHERE recurring_expense_id = ?`, recurringExpenseID); err != nil {
		return err
	}
	for _, split := range splits {
		if _, err := tx.Exec(`
			INSERT INTO recurring_expense_splits (recurring_expense_id, user_id, percentage)
			VALUES (?, ?, ?)
		`, recurringExpenseID, split.UserID, split.Percentage); err != nil {
			return err
		}
	}
	return nil
}

func splitAmount(amount, percentage float64) float64 {
	return math.Round(amount*percentage) / 100
}
