package bank_test

import (
	"strings"
	"testing"

	"gastos/internal/api/apitest"
	"gastos/internal/api/bank"
)

var monthStatement = apitest.OfxRows(
	"20261001|-48.90|F1|COMPRA MERCADO DO BAIRRO",
	"20261005|8500.00|F2|CREDITO SALARIO",
	"20261006|-25.00|F3|UBER *TRIP 123",
)

type matchRef struct {
	ID          int64  `json:"id"`
	Description string `json:"description"`
	Date        string `json:"date"`
}

type inboxItem struct {
	ID                   int64     `json:"id"`
	BankAccountID        int64     `json:"bankAccountId"`
	AccountName          string    `json:"accountName"`
	Date                 string    `json:"date"`
	AmountCents          int64     `json:"amountCents"`
	Description          string    `json:"description"`
	Status               string    `json:"status"`
	SuggestedCategoryID  *int64    `json:"suggestedCategoryId"`
	SuggestedDescription *string   `json:"suggestedDescription"`
	SuggestedMatch       *matchRef `json:"suggestedMatch"`
}

func newAccount(t *testing.T, c *apitest.Client, name string) bank.BankAccountDTO {
	t.Helper()
	var a bank.BankAccountDTO
	c.Do("POST", c.WPath("/bank-accounts"), map[string]any{"name": name}, 201, &a)
	return a
}

func pending(t *testing.T, c *apitest.Client) []inboxItem {
	t.Helper()
	var list []inboxItem
	c.Do("GET", c.WPath("/inbox"), nil, 200, &list)
	return list
}

func findByDescription(t *testing.T, list []inboxItem, contains string) inboxItem {
	t.Helper()
	for _, it := range list {
		if strings.Contains(it.Description, contains) {
			return it
		}
	}
	t.Fatalf("no inbox item containing %q in %+v", contains, list)
	return inboxItem{}
}

func categoryByName(t *testing.T, c *apitest.Client, kind, name string) int64 {
	t.Helper()
	var cats []category
	c.Do("GET", c.WPath("/categories"), nil, 200, &cats)
	for _, cat := range cats {
		if cat.Kind == kind && cat.Name == name {
			return cat.ID
		}
	}
	t.Fatalf("no %s category named %q", kind, name)
	return 0
}

// category is the part of a category the tests read.
type category struct {
	ID   int64  `json:"id"`
	Kind string `json:"kind"`
	Name string `json:"name"`
}
