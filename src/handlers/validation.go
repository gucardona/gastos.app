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

func validateRecurringExpense(recurring models.RecurringExpense) error {
	if recurring.Amount <= 0 {
		return errors.New("Valor do gasto recorrente deve ser maior que zero")
	}
	if strings.TrimSpace(recurring.Description) == "" {
		return errors.New("Descrição é obrigatória")
	}
	if strings.TrimSpace(recurring.Category) == "" {
		return errors.New("Categoria é obrigatória")
	}
	if strings.TrimSpace(recurring.Payment) == "" {
		return errors.New("Pagamento é obrigatório")
	}
	if recurring.Frequency == "" {
		recurring.Frequency = "monthly"
	}
	if recurring.Frequency != "monthly" {
		return errors.New("Frequência inválida")
	}
	if recurring.DayOfMonth < 1 || recurring.DayOfMonth > 31 {
		return errors.New("Dia do mês deve estar entre 1 e 31")
	}
	if !isValidDate(recurring.StartDate) {
		return errors.New("Data inicial inválida")
	}
	if strings.TrimSpace(recurring.EndDate) != "" {
		if !isValidDate(recurring.EndDate) {
			return errors.New("Data limite inválida")
		}
		startDate, _ := time.Parse("2006-01-02", recurring.StartDate)
		endDate, _ := time.Parse("2006-01-02", recurring.EndDate)
		if endDate.Before(startDate) {
			return errors.New("Data limite deve ser igual ou posterior à data inicial")
		}
	}
	return nil
}

func validateAccountName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("Nome da conta é obrigatório")
	}
	return nil
}

func validateShareRole(role string) error {
	if !models.IsValidShareRole(strings.TrimSpace(role)) {
		return errors.New("Papel inválido")
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
