package handlers

import (
	"gastos/src/models"
	"testing"
)

func TestValidateExpense(t *testing.T) {
	good := models.Expense{
		Amount:      10,
		Description: "Cafe",
		Category:    "food",
		Payment:     "pix",
		Date:        "2026-01-15",
	}
	if err := validateExpense(good); err != nil {
		t.Fatalf("valid expense: unexpected error: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*models.Expense)
	}{
		{"zero amount", func(e *models.Expense) { e.Amount = 0 }},
		{"negative amount", func(e *models.Expense) { e.Amount = -1 }},
		{"empty description", func(e *models.Expense) { e.Description = "" }},
		{"whitespace description", func(e *models.Expense) { e.Description = "   " }},
		{"empty category", func(e *models.Expense) { e.Category = "" }},
		{"empty payment", func(e *models.Expense) { e.Payment = "" }},
		{"empty date", func(e *models.Expense) { e.Date = "" }},
		{"invalid date format", func(e *models.Expense) { e.Date = "15-01-2026" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := good
			tc.mutate(&e)
			if err := validateExpense(e); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestValidateIncome(t *testing.T) {
	good := models.Income{
		Amount:      100,
		Description: "Salario",
		Type:        "salary",
		Date:        "2026-01-15",
	}
	if err := validateIncome(good); err != nil {
		t.Fatalf("valid income: unexpected error: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*models.Income)
	}{
		{"zero amount", func(i *models.Income) { i.Amount = 0 }},
		{"negative amount", func(i *models.Income) { i.Amount = -5 }},
		{"empty description", func(i *models.Income) { i.Description = "" }},
		{"whitespace description", func(i *models.Income) { i.Description = "   " }},
		{"empty type", func(i *models.Income) { i.Type = "" }},
		{"empty date", func(i *models.Income) { i.Date = "" }},
		{"invalid date format", func(i *models.Income) { i.Date = "2026/01/15" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inc := good
			tc.mutate(&inc)
			if err := validateIncome(inc); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestValidateGoal(t *testing.T) {
	good := models.Goal{Category: "food", Limit: 500}
	if err := validateGoal(good); err != nil {
		t.Fatalf("valid goal: unexpected error: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*models.Goal)
	}{
		{"empty category", func(g *models.Goal) { g.Category = "" }},
		{"whitespace category", func(g *models.Goal) { g.Category = "   " }},
		{"zero limit", func(g *models.Goal) { g.Limit = 0 }},
		{"negative limit", func(g *models.Goal) { g.Limit = -1 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := good
			tc.mutate(&g)
			if err := validateGoal(g); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestValidateRecurringExpense(t *testing.T) {
	good := models.RecurringExpense{
		Amount:      49.9,
		Description: "Netflix",
		Category:    "subscriptions",
		Payment:     "credit",
		Frequency:   "monthly",
		DayOfMonth:  15,
		StartDate:   "2026-01-01",
		EndDate:     "2026-12-01",
	}
	if err := validateRecurringExpense(good); err != nil {
		t.Fatalf("valid recurring expense: unexpected error: %v", err)
	}

	// EndDate may be empty (no end)
	noEnd := good
	noEnd.EndDate = ""
	if err := validateRecurringExpense(noEnd); err != nil {
		t.Fatalf("recurring expense without end date: unexpected error: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*models.RecurringExpense)
	}{
		{"zero amount", func(r *models.RecurringExpense) { r.Amount = 0 }},
		{"empty description", func(r *models.RecurringExpense) { r.Description = "" }},
		{"empty category", func(r *models.RecurringExpense) { r.Category = "" }},
		{"empty payment", func(r *models.RecurringExpense) { r.Payment = "" }},
		{"invalid frequency", func(r *models.RecurringExpense) { r.Frequency = "weekly" }},
		{"day zero", func(r *models.RecurringExpense) { r.DayOfMonth = 0 }},
		{"day 32", func(r *models.RecurringExpense) { r.DayOfMonth = 32 }},
		{"invalid start date", func(r *models.RecurringExpense) { r.StartDate = "2026/01/01" }},
		{"end before start", func(r *models.RecurringExpense) { r.EndDate = "2025-12-01" }},
		{"invalid end date", func(r *models.RecurringExpense) { r.EndDate = "not-a-date" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			re := good
			tc.mutate(&re)
			if err := validateRecurringExpense(re); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestValidateRecurringIncome(t *testing.T) {
	good := models.RecurringIncome{
		Amount:      2000,
		Description: "Salario",
		Type:        "salary",
		DayOfMonth:  5,
		StartDate:   "2026-01-05",
		EndDate:     "2026-12-05",
	}
	if err := validateRecurringIncome(good); err != nil {
		t.Fatalf("valid recurring income: unexpected error: %v", err)
	}

	noEnd := good
	noEnd.EndDate = ""
	if err := validateRecurringIncome(noEnd); err != nil {
		t.Fatalf("recurring income without end date: unexpected error: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*models.RecurringIncome)
	}{
		{"zero amount", func(r *models.RecurringIncome) { r.Amount = 0 }},
		{"empty description", func(r *models.RecurringIncome) { r.Description = "" }},
		{"empty type", func(r *models.RecurringIncome) { r.Type = "" }},
		{"day zero", func(r *models.RecurringIncome) { r.DayOfMonth = 0 }},
		{"day 32", func(r *models.RecurringIncome) { r.DayOfMonth = 32 }},
		{"invalid start date", func(r *models.RecurringIncome) { r.StartDate = "01-05-2026" }},
		{"end before start", func(r *models.RecurringIncome) { r.EndDate = "2025-01-01" }},
		{"invalid end date", func(r *models.RecurringIncome) { r.EndDate = "not-a-date" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ri := good
			tc.mutate(&ri)
			if err := validateRecurringIncome(ri); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestValidateAccountName(t *testing.T) {
	if err := validateAccountName("Casa"); err != nil {
		t.Fatalf("valid name: unexpected error: %v", err)
	}
	if err := validateAccountName(""); err == nil {
		t.Error("empty name: expected error, got nil")
	}
	if err := validateAccountName("   "); err == nil {
		t.Error("whitespace name: expected error, got nil")
	}
}

func TestValidateCategory(t *testing.T) {
	if err := validateCategory("Alimentação", "🍔", "#FF5733", "essential"); err != nil {
		t.Fatalf("valid category: unexpected error: %v", err)
	}
	for _, ess := range []string{"essential", "nonessential", "investment"} {
		if err := validateCategory("Test", "x", "#AABBCC", ess); err != nil {
			t.Errorf("essentiality %q should be valid: %v", ess, err)
		}
	}

	cases := []struct {
		name, catName, icon, color, essentiality string
	}{
		{"empty name", "", "🍔", "#FF5733", "essential"},
		{"whitespace name", "  ", "🍔", "#FF5733", "essential"},
		{"empty icon", "Food", "", "#FF5733", "essential"},
		{"no hash", "Food", "x", "FF5733", "essential"},
		{"too short hex", "Food", "x", "#FF573", "essential"},
		{"too long hex", "Food", "x", "#FF57331", "essential"},
		{"non-hex chars", "Food", "x", "#ZZZZZZ", "essential"},
		{"invalid essentiality", "Food", "x", "#FF5733", "luxury"},
		{"empty essentiality", "Food", "x", "#FF5733", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateCategory(tc.catName, tc.icon, tc.color, tc.essentiality); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestValidatePaymentMethod(t *testing.T) {
	if err := validatePaymentMethod("Pix", "💸"); err != nil {
		t.Fatalf("valid payment method: unexpected error: %v", err)
	}
	if err := validatePaymentMethod("", "💸"); err == nil {
		t.Error("empty name: expected error, got nil")
	}
	if err := validatePaymentMethod("  ", "💸"); err == nil {
		t.Error("whitespace name: expected error, got nil")
	}
	if err := validatePaymentMethod("Pix", ""); err == nil {
		t.Error("empty icon: expected error, got nil")
	}
}

func TestValidateShareRole(t *testing.T) {
	if err := validateShareRole("editor"); err != nil {
		t.Fatalf("editor: unexpected error: %v", err)
	}
	if err := validateShareRole("reader"); err != nil {
		t.Fatalf("reader: unexpected error: %v", err)
	}
	if err := validateShareRole("owner"); err == nil {
		t.Error("owner: expected error, got nil")
	}
	if err := validateShareRole(""); err == nil {
		t.Error("empty: expected error, got nil")
	}
	if err := validateShareRole("admin"); err == nil {
		t.Error("admin: expected error, got nil")
	}
}

func TestIsValidDate(t *testing.T) {
	for _, d := range []string{"2026-01-01", "2026-12-31", "2000-02-29"} {
		if !isValidDate(d) {
			t.Errorf("isValidDate(%q) = false, want true", d)
		}
	}
	for _, d := range []string{"", "   ", "01-01-2026", "2026/01/01", "not-a-date", "2026-13-01", "2026-00-01"} {
		if isValidDate(d) {
			t.Errorf("isValidDate(%q) = true, want false", d)
		}
	}
}
