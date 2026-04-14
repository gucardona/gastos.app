package handlers

import (
	"errors"
	"gastos/src/models"
	"strings"
	"time"
)

func validateExpense(expense models.Expense) error {
	if expense.Amount <= 0 {
		return errors.New("Valor do gasto deve ser maior que zero")
	}
	if strings.TrimSpace(expense.Description) == "" {
		return errors.New("Descrição é obrigatória")
	}
	if strings.TrimSpace(expense.Category) == "" {
		return errors.New("Categoria é obrigatória")
	}
	if strings.TrimSpace(expense.Payment) == "" {
		return errors.New("Pagamento é obrigatório")
	}
	if !isValidDate(expense.Date) {
		return errors.New("Data inválida")
	}
	expense.Description = strings.TrimSpace(expense.Description)
	return nil
}

func validateIncome(income models.Income) error {
	if income.Amount <= 0 {
		return errors.New("Valor da entrada deve ser maior que zero")
	}
	if strings.TrimSpace(income.Description) == "" {
		return errors.New("Descrição é obrigatória")
	}
	if strings.TrimSpace(income.Type) == "" {
		return errors.New("Tipo é obrigatório")
	}
	if !isValidDate(income.Date) {
		return errors.New("Data inválida")
	}
	return nil
}

func validateGoal(goal models.Goal) error {
	if strings.TrimSpace(goal.Category) == "" {
		return errors.New("Categoria é obrigatória")
	}
	if goal.Limit <= 0 {
		return errors.New("Limite deve ser maior que zero")
	}
	return nil
}

func isValidDate(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	_, err := time.Parse("2006-01-02", value)
	return err == nil
}
