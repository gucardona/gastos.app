package handlers

import (
	"errors"
	"gastos/src/models"
	"regexp"
	"strings"
	"time"
)

var (
	errAmountPositive             = errors.New("Valor deve ser maior que zero")
	errInvalidDate                = errors.New("Data inválida")
	errInvalidSplitPaymentMembers = errors.New("Pagador e recebedor precisam ser participantes diferentes desta carteira")
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

func validateRecurringIncome(ri models.RecurringIncome) error {
	if ri.Amount <= 0 {
		return errors.New("Valor da entrada recorrente deve ser maior que zero")
	}
	if strings.TrimSpace(ri.Description) == "" {
		return errors.New("Descrição é obrigatória")
	}
	if strings.TrimSpace(ri.Type) == "" {
		return errors.New("Tipo é obrigatório")
	}
	if ri.DayOfMonth < 1 || ri.DayOfMonth > 31 {
		return errors.New("Dia do mês deve estar entre 1 e 31")
	}
	if !isValidDate(ri.StartDate) {
		return errors.New("Data inicial inválida")
	}
	if strings.TrimSpace(ri.EndDate) != "" {
		if !isValidDate(ri.EndDate) {
			return errors.New("Data limite inválida")
		}
		startDate, _ := time.Parse("2006-01-02", ri.StartDate)
		endDate, _ := time.Parse("2006-01-02", ri.EndDate)
		if endDate.Before(startDate) {
			return errors.New("Data limite deve ser igual ou posterior à data inicial")
		}
	}
	return nil
}

func validateRecurringPayment(accountID int64, rp models.RecurringPayment) error {
	if rp.Amount <= 0 {
		return errAmountPositive
	}
	if rp.PayerUserID <= 0 || rp.ReceiverUserID <= 0 || rp.PayerUserID == rp.ReceiverUserID {
		return errInvalidSplitPaymentMembers
	}
	if rp.DayOfMonth < 1 || rp.DayOfMonth > 31 {
		return errors.New("Dia do mês deve estar entre 1 e 31")
	}
	if !isValidDate(rp.StartDate) {
		return errors.New("Data inicial inválida")
	}
	if strings.TrimSpace(rp.EndDate) != "" {
		if !isValidDate(rp.EndDate) {
			return errors.New("Data limite inválida")
		}
		startDate, _ := time.Parse("2006-01-02", rp.StartDate)
		endDate, _ := time.Parse("2006-01-02", rp.EndDate)
		if endDate.Before(startDate) {
			return errors.New("Data limite deve ser igual ou posterior à data inicial")
		}
	}
	members, err := accountMemberSplits(accountID)
	if err != nil {
		return err
	}
	foundPayer, foundReceiver := false, false
	for _, m := range members {
		if m.UserID == rp.PayerUserID {
			foundPayer = true
		}
		if m.UserID == rp.ReceiverUserID {
			foundReceiver = true
		}
	}
	if !foundPayer || !foundReceiver {
		return errInvalidSplitPaymentMembers
	}
	return nil
}

func validateAccountName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("Nome da conta é obrigatório")
	}
	return nil
}

var colorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func validateCategory(name, icon, color, essentiality string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("Nome é obrigatório")
	}
	if strings.TrimSpace(icon) == "" {
		return errors.New("Ícone é obrigatório")
	}
	if !colorRe.MatchString(color) {
		return errors.New("Cor inválida (use #RRGGBB)")
	}
	switch essentiality {
	case "essential", "nonessential", "investment":
	default:
		return errors.New("Essencialidade inválida")
	}
	return nil
}

func validatePaymentMethod(name, icon string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("Nome é obrigatório")
	}
	if strings.TrimSpace(icon) == "" {
		return errors.New("Ícone é obrigatório")
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
