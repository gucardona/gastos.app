package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"gastos/src/db"
)

type authResponse struct {
	Token string `json:"token"`
	User  struct {
		ID    int64  `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"user"`
}

type accountResponse struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Role        string `json:"role"`
	OwnerName   string `json:"ownerName"`
	Permissions struct {
		CanEdit          bool `json:"canEdit"`
		CanManageMembers bool `json:"canManageMembers"`
		CanDelete        bool `json:"canDelete"`
	} `json:"permissions"`
}

type accountMemberResponse struct {
	UserID int64  `json:"userId"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Role   string `json:"role"`
}

type expenseResponse struct {
	ID          int64   `json:"id"`
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	Payment     string  `json:"payment"`
	Date        string  `json:"date"`
}

type incomeResponse struct {
	ID          int64   `json:"id"`
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
	Type        string  `json:"type"`
	Date        string  `json:"date"`
}

type goalResponse struct {
	ID       int64   `json:"id"`
	Category string  `json:"category"`
	Limit    float64 `json:"limit"`
}

type recurringExpenseResponse struct {
	ID          int64   `json:"id"`
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	Payment     string  `json:"payment"`
	Frequency   string  `json:"frequency"`
	DayOfMonth  int     `json:"dayOfMonth"`
	StartDate   string  `json:"startDate"`
	EndDate     string  `json:"endDate"`
	Enabled     bool    `json:"enabled"`
}

type categoryResponse struct {
	ID           int64  `json:"id"`
	Key          string `json:"key"`
	Name         string `json:"name"`
	Icon         string `json:"icon"`
	Color        string `json:"color"`
	Essentiality string `json:"essentiality"`
	SortOrder    int    `json:"sortOrder"`
}

type paymentMethodResponse struct {
	ID        int64  `json:"id"`
	Key       string `json:"key"`
	Icon      string `json:"icon"`
	Name      string `json:"name"`
	SortOrder int    `json:"sortOrder"`
}

type recurringIncomeResponse struct {
	ID          int64   `json:"id"`
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
	Type        string  `json:"type"`
	DayOfMonth  int     `json:"dayOfMonth"`
	StartDate   string  `json:"startDate"`
	EndDate     string  `json:"endDate"`
	Enabled     bool    `json:"enabled"`
}

type recurringPaymentResponse struct {
	ID             int64   `json:"id"`
	PayerUserID    int64   `json:"payerUserId"`
	PayerName      string  `json:"payerName"`
	ReceiverUserID int64   `json:"receiverUserId"`
	ReceiverName   string  `json:"receiverName"`
	Amount         float64 `json:"amount"`
	Note           string  `json:"note"`
	DayOfMonth     int     `json:"dayOfMonth"`
	StartDate      string  `json:"startDate"`
	EndDate        string  `json:"endDate"`
	Enabled        bool    `json:"enabled"`
}

type splitPaymentResponse struct {
	ID             int64   `json:"id"`
	PayerUserID    int64   `json:"payerUserId"`
	PayerName      string  `json:"payerName"`
	ReceiverUserID int64   `json:"receiverUserId"`
	ReceiverName   string  `json:"receiverName"`
	Amount         float64 `json:"amount"`
	Date           string  `json:"date"`
	Note           string  `json:"note"`
}

type apiClient struct {
	t         *testing.T
	serverURL string
	token     string
	accountID string
	userID    int64
	userName  string
	userEmail string
}

func TestSharedAccountPermissionsAndIsolation(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	alice := registerUser(t, server.URL, "Alice", "alice@example.com")
	aliceAccounts := fetchAccounts(t, alice)
	if len(aliceAccounts) != 1 {
		t.Fatalf("alice should start with one personal account, got %d", len(aliceAccounts))
	}
	if aliceAccounts[0].Name != "Pessoal" || aliceAccounts[0].Role != "owner" {
		t.Fatalf("unexpected default account: %+v", aliceAccounts[0])
	}
	if aliceAccounts[0].OwnerName != "Alice" {
		t.Fatalf("default account owner name = %q, want Alice", aliceAccounts[0].OwnerName)
	}
	if !aliceAccounts[0].Permissions.CanEdit || !aliceAccounts[0].Permissions.CanManageMembers || !aliceAccounts[0].Permissions.CanDelete {
		t.Fatalf("owner permissions were not returned correctly: %+v", aliceAccounts[0].Permissions)
	}
	personalID := aliceAccounts[0].ID

	var sharedHouse accountResponse
	alice.request(http.MethodPost, "/api/accounts", map[string]any{"name": "Casa"}, http.StatusCreated, &sharedHouse)
	if sharedHouse.Role != "owner" {
		t.Fatalf("new account role = %q, want owner", sharedHouse.Role)
	}

	alice.accountID = personalIDString(personalID)
	var personalGoal goalResponse
	alice.request(http.MethodPost, "/api/goals", map[string]any{"category": "food", "limit": 400}, http.StatusCreated, &personalGoal)
	if personalGoal.ID == 0 {
		t.Fatal("personal goal should be created with an id")
	}

	alice.accountID = personalIDString(sharedHouse.ID)
	var houseGoal goalResponse
	alice.request(http.MethodPost, "/api/goals", map[string]any{"category": "food", "limit": 900}, http.StatusCreated, &houseGoal)
	if houseGoal.ID == 0 {
		t.Fatal("shared account goal should be created with an id")
	}

	var houseExpense expenseResponse
	alice.request(http.MethodPost, "/api/expenses", map[string]any{
		"amount":      120.50,
		"description": "Mercado da casa",
		"category":    "market",
		"payment":     "pix",
		"date":        "2026-04-10",
	}, http.StatusCreated, &houseExpense)
	if houseExpense.ID == 0 {
		t.Fatal("shared account expense should be created with an id")
	}

	alice.accountID = personalIDString(personalID)
	var personalExpenses []expenseResponse
	alice.request(http.MethodGet, "/api/expenses", nil, http.StatusOK, &personalExpenses)
	if len(personalExpenses) != 0 {
		t.Fatalf("personal account should not see house expenses, got %d rows", len(personalExpenses))
	}

	alice.accountID = personalIDString(sharedHouse.ID)
	var sharedExpenses []expenseResponse
	alice.request(http.MethodGet, "/api/expenses", nil, http.StatusOK, &sharedExpenses)
	if len(sharedExpenses) != 1 || sharedExpenses[0].Description != "Mercado da casa" {
		t.Fatalf("shared account expenses not scoped correctly: %+v", sharedExpenses)
	}

	bob := registerUser(t, server.URL, "Bob", "bob@example.com")
	alice.accountID = ""
	var bobMembership accountMemberResponse
	alice.request(http.MethodPost, "/api/accounts/"+personalIDString(sharedHouse.ID)+"/members", map[string]any{
		"email": "bob@example.com",
		"role":  "reader",
	}, http.StatusCreated, &bobMembership)
	if bobMembership.Role != "reader" {
		t.Fatalf("bob membership role = %q, want reader", bobMembership.Role)
	}

	alice.request(http.MethodPost, "/api/accounts/"+personalIDString(sharedHouse.ID)+"/members", map[string]any{
		"email": "bob@example.com",
		"role":  "reader",
	}, http.StatusConflict, nil)

	alice.request(http.MethodPost, "/api/accounts/"+personalIDString(sharedHouse.ID)+"/members", map[string]any{
		"email": "missing@example.com",
		"role":  "reader",
	}, http.StatusNotFound, nil)

	bobAccounts := fetchAccounts(t, bob)
	if len(bobAccounts) != 2 {
		t.Fatalf("bob should see his personal account and the shared one, got %d", len(bobAccounts))
	}
	sharedForBob := findAccountByName(t, bobAccounts, "Casa")
	if sharedForBob.Role != "reader" || sharedForBob.Permissions.CanEdit || sharedForBob.Permissions.CanManageMembers || sharedForBob.Permissions.CanDelete {
		t.Fatalf("reader permissions incorrect: %+v", sharedForBob)
	}

	bob.accountID = personalIDString(sharedHouse.ID)
	var bobSharedExpenses []expenseResponse
	bob.request(http.MethodGet, "/api/expenses", nil, http.StatusOK, &bobSharedExpenses)
	if len(bobSharedExpenses) != 1 {
		t.Fatalf("reader should see shared expenses, got %d", len(bobSharedExpenses))
	}

	bob.request(http.MethodPost, "/api/expenses", map[string]any{
		"amount":      20,
		"description": "Tentativa reader",
		"category":    "food",
		"payment":     "pix",
		"date":        "2026-04-11",
	}, http.StatusForbidden, nil)

	bob.accountID = personalIDString(personalID)
	bob.request(http.MethodGet, "/api/expenses", nil, http.StatusForbidden, nil)

	alice.request(http.MethodGet, "/api/accounts/"+personalIDString(sharedHouse.ID)+"/members", nil, http.StatusOK, &[]accountMemberResponse{})
	alice.request(http.MethodPatch, "/api/accounts/"+personalIDString(sharedHouse.ID)+"/members/"+personalIDString(bob.userID), map[string]any{
		"role": "editor",
	}, http.StatusOK, &bobMembership)
	if bobMembership.Role != "editor" {
		t.Fatalf("bob membership role after update = %q, want editor", bobMembership.Role)
	}

	bob.accountID = personalIDString(sharedHouse.ID)
	var editorIncome incomeResponse
	bob.request(http.MethodPost, "/api/incomes", map[string]any{
		"amount":      450,
		"description": "Contribuição",
		"type":        "gift",
		"date":        "2026-04-12",
	}, http.StatusCreated, &editorIncome)
	if editorIncome.Description != "Contribuição" {
		t.Fatalf("editor income not created correctly: %+v", editorIncome)
	}

	bob.request(http.MethodPost, "/api/accounts/"+personalIDString(sharedHouse.ID)+"/members", map[string]any{
		"email": "alice@example.com",
		"role":  "reader",
	}, http.StatusForbidden, nil)

	alice.request(http.MethodDelete, "/api/accounts/"+personalIDString(sharedHouse.ID)+"/members/"+personalIDString(bob.userID), nil, http.StatusNoContent, nil)
	bob.accountID = personalIDString(sharedHouse.ID)
	bob.request(http.MethodGet, "/api/expenses", nil, http.StatusForbidden, nil)
}

func TestEditResourcesRecurringExpensesAndLeaveSharedAccount(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	alice := registerUser(t, server.URL, "Alice", "alice@example.com")
	bob := registerUser(t, server.URL, "Bob", "bob@example.com")

	var shared accountResponse
	alice.request(http.MethodPost, "/api/accounts", map[string]any{"name": "Casa"}, http.StatusCreated, &shared)
	alice.request(http.MethodPost, "/api/accounts/"+personalIDString(shared.ID)+"/members", map[string]any{
		"email": "bob@example.com",
		"role":  "editor",
	}, http.StatusCreated, &accountMemberResponse{})

	alice.accountID = personalIDString(shared.ID)
	bob.accountID = personalIDString(shared.ID)

	var expense expenseResponse
	alice.request(http.MethodPost, "/api/expenses", map[string]any{
		"amount":      90,
		"description": "Mercado",
		"category":    "market",
		"payment":     "pix",
		"date":        "2026-04-10",
	}, http.StatusCreated, &expense)
	bob.request(http.MethodPatch, "/api/expenses/"+personalIDString(expense.ID), map[string]any{
		"amount":      110,
		"description": "Mercado mensal",
		"category":    "market",
		"payment":     "credit",
		"date":        "2026-04-11",
	}, http.StatusOK, &expense)
	if expense.Description != "Mercado mensal" || expense.Payment != "credit" || expense.Amount != 110 {
		t.Fatalf("expense not updated correctly: %+v", expense)
	}

	var income incomeResponse
	alice.request(http.MethodPost, "/api/incomes", map[string]any{
		"amount":      500,
		"description": "Extra",
		"type":        "freelance",
		"date":        "2026-04-12",
	}, http.StatusCreated, &income)
	bob.request(http.MethodPatch, "/api/incomes/"+personalIDString(income.ID), map[string]any{
		"amount":      700,
		"description": "Extra revisado",
		"type":        "salary",
		"date":        "2026-04-13",
	}, http.StatusOK, &income)
	if income.Description != "Extra revisado" || income.Type != "salary" || income.Amount != 700 {
		t.Fatalf("income not updated correctly: %+v", income)
	}

	var goal goalResponse
	alice.request(http.MethodPost, "/api/goals", map[string]any{
		"category": "food",
		"limit":    300,
	}, http.StatusCreated, &goal)
	bob.request(http.MethodPatch, "/api/goals/food", map[string]any{
		"category": "market",
		"limit":    650,
	}, http.StatusOK, &goal)
	if goal.Category != "market" || goal.Limit != 650 {
		t.Fatalf("goal not updated correctly: %+v", goal)
	}

	var recurring recurringExpenseResponse
	bob.request(http.MethodPost, "/api/recurring-expenses", map[string]any{
		"amount":      49.9,
		"description": "Streaming",
		"category":    "subscriptions",
		"payment":     "credit",
		"frequency":   "monthly",
		"dayOfMonth":  15,
		"startDate":   "2026-04-15",
		"endDate":     "2026-12-15",
		"enabled":     true,
	}, http.StatusCreated, &recurring)
	if recurring.ID == 0 {
		t.Fatal("recurring expense should be created with an id")
	}

	bob.request(http.MethodPatch, "/api/recurring-expenses/"+personalIDString(recurring.ID), map[string]any{
		"amount":      59.9,
		"description": "Streaming família",
		"category":    "subscriptions",
		"payment":     "credit",
		"frequency":   "monthly",
		"dayOfMonth":  20,
		"startDate":   "2026-04-15",
		"endDate":     "2027-01-20",
		"enabled":     false,
	}, http.StatusOK, &recurring)
	if recurring.Description != "Streaming família" || recurring.DayOfMonth != 20 || recurring.Enabled {
		t.Fatalf("recurring expense not updated correctly: %+v", recurring)
	}

	var recurringList []recurringExpenseResponse
	bob.request(http.MethodGet, "/api/recurring-expenses", nil, http.StatusOK, &recurringList)
	if len(recurringList) != 1 || recurringList[0].Description != "Streaming família" {
		t.Fatalf("unexpected recurring expenses list: %+v", recurringList)
	}

	bob.request(http.MethodDelete, "/api/accounts/"+personalIDString(shared.ID)+"/members/"+personalIDString(bob.userID), nil, http.StatusNoContent, nil)
	bob.request(http.MethodGet, "/api/expenses", nil, http.StatusForbidden, nil)
}

func TestRegisterRequiresPasswordConfirmation(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	client := &apiClient{t: t, serverURL: server.URL}
	client.request(http.MethodPost, "/api/auth/register", map[string]any{
		"name":                 "Alice",
		"email":                "alice@example.com",
		"password":             "secret123",
		"passwordConfirmation": "secret321",
	}, http.StatusBadRequest, nil)
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db.Init(path)
	t.Cleanup(func() {
		if db.DB != nil {
			_ = db.DB.Close()
			db.DB = nil
		}
	})
	return httptest.NewServer(newMux())
}

func registerUser(t *testing.T, serverURL, name, email string) *apiClient {
	t.Helper()
	client := &apiClient{t: t, serverURL: serverURL}
	var auth authResponse
	client.request(http.MethodPost, "/api/auth/register", map[string]any{
		"name":                 name,
		"email":                email,
		"password":             "secret123",
		"passwordConfirmation": "secret123",
	}, http.StatusCreated, &auth)
	client.token = auth.Token
	client.userID = auth.User.ID
	client.userName = auth.User.Name
	client.userEmail = auth.User.Email
	return client
}

func fetchAccounts(t *testing.T, client *apiClient) []accountResponse {
	t.Helper()
	var accounts []accountResponse
	client.request(http.MethodGet, "/api/accounts", nil, http.StatusOK, &accounts)
	return accounts
}

func findAccountByName(t *testing.T, accounts []accountResponse, name string) accountResponse {
	t.Helper()
	for _, account := range accounts {
		if account.Name == name {
			return account
		}
	}
	t.Fatalf("account %q not found in %+v", name, accounts)
	return accountResponse{}
}

func personalIDString(id int64) string {
	return strconv.FormatInt(id, 10)
}

func (c *apiClient) request(method, path string, body any, expectedStatus int, out any) {
	c.t.Helper()

	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			c.t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequest(method, c.serverURL+path, reader)
	if err != nil {
		c.t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if c.accountID != "" {
		req.Header.Set("X-Account-ID", c.accountID)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatalf("perform request: %v", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		c.t.Fatalf("read response body: %v", err)
	}
	if resp.StatusCode != expectedStatus {
		c.t.Fatalf("%s %s status = %d, want %d, body = %s", method, path, resp.StatusCode, expectedStatus, string(data))
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			c.t.Fatalf("unmarshal response: %v, body=%s", err, string(data))
		}
	}
}

func TestCategoriesAndPaymentMethods(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	alice := registerUser(t, server.URL, "Alice", "alice@example.com")
	aliceAccounts := fetchAccounts(t, alice)
	alice.accountID = personalIDString(aliceAccounts[0].ID)

	// Invite Bob as reader on a shared account to test permission enforcement
	bob := registerUser(t, server.URL, "Bob", "bob@example.com")
	var shared accountResponse
	alice.request(http.MethodPost, "/api/accounts", map[string]any{"name": "Casa"}, http.StatusCreated, &shared)
	alice.request(http.MethodPost, "/api/accounts/"+personalIDString(shared.ID)+"/members", map[string]any{
		"email": "bob@example.com",
		"role":  "reader",
	}, http.StatusCreated, nil)
	bob.accountID = personalIDString(shared.ID)

	// --- Categories (on Alice's personal account) ---

	var initCats []categoryResponse
	alice.request(http.MethodGet, "/api/categories", nil, http.StatusOK, &initCats)
	if len(initCats) == 0 {
		t.Fatal("expected default categories to be seeded on account creation")
	}
	replacementKey := initCats[0].Key // use first seeded category as replacement target

	// Create
	var created categoryResponse
	alice.request(http.MethodPost, "/api/categories", map[string]any{
		"name":         "Lazer Extra",
		"icon":         "🎮",
		"color":        "#00AAFF",
		"essentiality": "nonessential",
	}, http.StatusCreated, &created)
	if created.ID == 0 || created.Key == "" || created.Name != "Lazer Extra" || created.Color != "#00AAFF" || created.Essentiality != "nonessential" {
		t.Fatalf("unexpected created category: %+v", created)
	}

	// Reader on shared account cannot create
	bob.request(http.MethodPost, "/api/categories", map[string]any{
		"name": "Tentativa", "icon": "x", "color": "#000000", "essentiality": "essential",
	}, http.StatusForbidden, nil)

	// Patch
	var patched categoryResponse
	alice.request(http.MethodPatch, "/api/categories/"+created.Key, map[string]any{
		"name":         "Lazer Atualizado",
		"icon":         "🎯",
		"color":        "#112233",
		"essentiality": "investment",
	}, http.StatusOK, &patched)
	if patched.Name != "Lazer Atualizado" || patched.Color != "#112233" || patched.Essentiality != "investment" {
		t.Fatalf("unexpected patched category: %+v", patched)
	}

	// Patch unknown key → 404
	alice.request(http.MethodPatch, "/api/categories/nonexistent", map[string]any{
		"name": "X", "icon": "x", "color": "#000000", "essentiality": "essential",
	}, http.StatusNotFound, nil)

	// Delete requires a replacement key (passed as query param)
	alice.request(http.MethodDelete, "/api/categories/"+created.Key+"?replacement="+replacementKey, nil, http.StatusNoContent, nil)

	// Deleted key no longer in list
	var afterDelete []categoryResponse
	alice.request(http.MethodGet, "/api/categories", nil, http.StatusOK, &afterDelete)
	for _, c := range afterDelete {
		if c.Key == created.Key {
			t.Fatalf("deleted category %q still in list", created.Key)
		}
	}

	// Delete without replacement → 400
	var extra categoryResponse
	alice.request(http.MethodPost, "/api/categories", map[string]any{
		"name": "Temp", "icon": "t", "color": "#FFFFFF", "essentiality": "essential",
	}, http.StatusCreated, &extra)
	alice.request(http.MethodDelete, "/api/categories/"+extra.Key, nil, http.StatusBadRequest, nil)

	// --- Payment Methods (on Alice's personal account) ---

	var initPMs []paymentMethodResponse
	alice.request(http.MethodGet, "/api/payment-methods", nil, http.StatusOK, &initPMs)
	if len(initPMs) == 0 {
		t.Fatal("expected default payment methods to be seeded on account creation")
	}
	replacementPMKey := initPMs[0].Key

	// Create
	var createdPM paymentMethodResponse
	alice.request(http.MethodPost, "/api/payment-methods", map[string]any{
		"name": "Dinheiro Extra",
		"icon": "💵",
	}, http.StatusCreated, &createdPM)
	if createdPM.ID == 0 || createdPM.Key == "" || createdPM.Name != "Dinheiro Extra" {
		t.Fatalf("unexpected created payment method: %+v", createdPM)
	}

	// Reader cannot create payment method
	bob.request(http.MethodPost, "/api/payment-methods", map[string]any{
		"name": "Tentativa", "icon": "x",
	}, http.StatusForbidden, nil)

	// Patch
	var patchedPM paymentMethodResponse
	alice.request(http.MethodPatch, "/api/payment-methods/"+createdPM.Key, map[string]any{
		"name": "Dinheiro Vivo",
		"icon": "💰",
	}, http.StatusOK, &patchedPM)
	if patchedPM.Name != "Dinheiro Vivo" || patchedPM.Icon != "💰" {
		t.Fatalf("unexpected patched payment method: %+v", patchedPM)
	}

	// Delete with replacement
	alice.request(http.MethodDelete, "/api/payment-methods/"+createdPM.Key+"?replacement="+replacementPMKey, nil, http.StatusNoContent, nil)

	// Deleted key no longer in list
	var afterDeletePMs []paymentMethodResponse
	alice.request(http.MethodGet, "/api/payment-methods", nil, http.StatusOK, &afterDeletePMs)
	for _, p := range afterDeletePMs {
		if p.Key == createdPM.Key {
			t.Fatalf("deleted payment method %q still in list", createdPM.Key)
		}
	}
}

func TestRecurringIncomesLifecycle(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	alice := registerUser(t, server.URL, "Alice", "alice@example.com")
	bob := registerUser(t, server.URL, "Bob", "bob@example.com")

	var shared accountResponse
	alice.request(http.MethodPost, "/api/accounts", map[string]any{"name": "Casa"}, http.StatusCreated, &shared)
	alice.request(http.MethodPost, "/api/accounts/"+personalIDString(shared.ID)+"/members", map[string]any{
		"email": "bob@example.com",
		"role":  "reader",
	}, http.StatusCreated, nil)
	alice.accountID = personalIDString(shared.ID)
	bob.accountID = personalIDString(shared.ID)

	// Create
	var ri recurringIncomeResponse
	alice.request(http.MethodPost, "/api/recurring-incomes", map[string]any{
		"amount":      3000,
		"description": "Salário",
		"type":        "salary",
		"dayOfMonth":  5,
		"startDate":   "2026-01-05",
		"endDate":     "2026-12-05",
		"enabled":     true,
	}, http.StatusCreated, &ri)
	if ri.ID == 0 || ri.Description != "Salário" || ri.Amount != 3000 || ri.DayOfMonth != 5 || !ri.Enabled {
		t.Fatalf("unexpected created recurring income: %+v", ri)
	}

	// Reader cannot create
	bob.request(http.MethodPost, "/api/recurring-incomes", map[string]any{
		"amount": 100, "description": "X", "type": "gift", "dayOfMonth": 1, "startDate": "2026-01-01", "enabled": true,
	}, http.StatusForbidden, nil)

	// List
	var list []recurringIncomeResponse
	alice.request(http.MethodGet, "/api/recurring-incomes", nil, http.StatusOK, &list)
	if len(list) != 1 || list[0].Description != "Salário" {
		t.Fatalf("unexpected recurring incomes list: %+v", list)
	}

	// Patch
	var updated recurringIncomeResponse
	alice.request(http.MethodPatch, "/api/recurring-incomes/"+personalIDString(ri.ID), map[string]any{
		"amount":      3500,
		"description": "Salário Atualizado",
		"type":        "salary",
		"dayOfMonth":  10,
		"startDate":   "2026-02-10",
		"endDate":     "",
		"enabled":     false,
	}, http.StatusOK, &updated)
	if updated.Amount != 3500 || updated.Description != "Salário Atualizado" || updated.DayOfMonth != 10 || updated.Enabled {
		t.Fatalf("unexpected updated recurring income: %+v", updated)
	}

	// Reader cannot patch
	bob.request(http.MethodPatch, "/api/recurring-incomes/"+personalIDString(ri.ID), map[string]any{
		"amount": 1, "description": "X", "type": "gift", "dayOfMonth": 1, "startDate": "2026-01-01", "enabled": true,
	}, http.StatusForbidden, nil)

	// Delete
	alice.request(http.MethodDelete, "/api/recurring-incomes/"+personalIDString(ri.ID), nil, http.StatusNoContent, nil)

	// List is now empty
	var afterDelete []recurringIncomeResponse
	alice.request(http.MethodGet, "/api/recurring-incomes", nil, http.StatusOK, &afterDelete)
	if len(afterDelete) != 0 {
		t.Fatalf("expected empty list after delete, got %d items", len(afterDelete))
	}

	// Delete nonexistent → 404
	alice.request(http.MethodDelete, "/api/recurring-incomes/"+personalIDString(ri.ID), nil, http.StatusNotFound, nil)
}

func TestRecurringPaymentsLifecycle(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	alice := registerUser(t, server.URL, "Alice", "alice@example.com")
	bob := registerUser(t, server.URL, "Bob", "bob@example.com")

	// Create shared account and add Bob as editor
	var shared accountResponse
	alice.request(http.MethodPost, "/api/accounts", map[string]any{"name": "Casa"}, http.StatusCreated, &shared)
	alice.request(http.MethodPost, "/api/accounts/"+personalIDString(shared.ID)+"/members", map[string]any{
		"email": "bob@example.com",
		"role":  "editor",
	}, http.StatusCreated, nil)
	alice.accountID = personalIDString(shared.ID)
	bob.accountID = personalIDString(shared.ID)

	// Create recurring payment (payer=alice, receiver=bob)
	var rp recurringPaymentResponse
	alice.request(http.MethodPost, "/api/recurring-payments", map[string]any{
		"payerUserId":    alice.userID,
		"receiverUserId": bob.userID,
		"amount":         200,
		"note":           "Aluguel mensal",
		"dayOfMonth":     1,
		"startDate":      "2026-01-01",
		"endDate":        "2026-12-01",
		"enabled":        true,
	}, http.StatusCreated, &rp)
	if rp.ID == 0 || rp.PayerUserID != alice.userID || rp.ReceiverUserID != bob.userID || rp.Amount != 200 {
		t.Fatalf("unexpected created recurring payment: %+v", rp)
	}

	// List
	var list []recurringPaymentResponse
	alice.request(http.MethodGet, "/api/recurring-payments", nil, http.StatusOK, &list)
	if len(list) != 1 || list[0].Note != "Aluguel mensal" {
		t.Fatalf("unexpected recurring payments list: %+v", list)
	}

	// Patch
	var updated recurringPaymentResponse
	alice.request(http.MethodPatch, "/api/recurring-payments/"+personalIDString(rp.ID), map[string]any{
		"payerUserId":    alice.userID,
		"receiverUserId": bob.userID,
		"amount":         250,
		"note":           "Aluguel revisado",
		"dayOfMonth":     5,
		"startDate":      "2026-02-05",
		"endDate":        "",
		"enabled":        false,
	}, http.StatusOK, &updated)
	if updated.Amount != 250 || updated.Note != "Aluguel revisado" || updated.DayOfMonth != 5 || updated.Enabled {
		t.Fatalf("unexpected updated recurring payment: %+v", updated)
	}

	// Same payer and receiver → 400
	alice.request(http.MethodPost, "/api/recurring-payments", map[string]any{
		"payerUserId":    alice.userID,
		"receiverUserId": alice.userID,
		"amount":         50,
		"dayOfMonth":     1,
		"startDate":      "2026-01-01",
		"enabled":        true,
	}, http.StatusBadRequest, nil)

	// User not in account → 400 (use a fake user id)
	alice.request(http.MethodPost, "/api/recurring-payments", map[string]any{
		"payerUserId":    alice.userID,
		"receiverUserId": int64(9999),
		"amount":         50,
		"dayOfMonth":     1,
		"startDate":      "2026-01-01",
		"enabled":        true,
	}, http.StatusBadRequest, nil)

	// Delete
	alice.request(http.MethodDelete, "/api/recurring-payments/"+personalIDString(rp.ID), nil, http.StatusNoContent, nil)

	var afterDelete []recurringPaymentResponse
	alice.request(http.MethodGet, "/api/recurring-payments", nil, http.StatusOK, &afterDelete)
	if len(afterDelete) != 0 {
		t.Fatalf("expected empty list after delete, got %d", len(afterDelete))
	}
}

func TestSplitPaymentsLifecycle(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	alice := registerUser(t, server.URL, "Alice", "alice@example.com")
	bob := registerUser(t, server.URL, "Bob", "bob@example.com")
	carol := registerUser(t, server.URL, "Carol", "carol@example.com")

	var shared accountResponse
	alice.request(http.MethodPost, "/api/accounts", map[string]any{"name": "Casa"}, http.StatusCreated, &shared)
	alice.request(http.MethodPost, "/api/accounts/"+personalIDString(shared.ID)+"/members", map[string]any{
		"email": "bob@example.com",
		"role":  "editor",
	}, http.StatusCreated, nil)
	alice.request(http.MethodPost, "/api/accounts/"+personalIDString(shared.ID)+"/members", map[string]any{
		"email": "carol@example.com",
		"role":  "reader",
	}, http.StatusCreated, nil)

	alice.accountID = personalIDString(shared.ID)
	bob.accountID = personalIDString(shared.ID)
	carol.accountID = personalIDString(shared.ID)

	// Create split payment (alice pays bob)
	var sp splitPaymentResponse
	alice.request(http.MethodPost, "/api/split-payments", map[string]any{
		"payerUserId":    alice.userID,
		"receiverUserId": bob.userID,
		"amount":         150.50,
		"date":           "2026-04-10",
		"note":           "Conta do mercado",
	}, http.StatusCreated, &sp)
	if sp.ID == 0 || sp.PayerUserID != alice.userID || sp.ReceiverUserID != bob.userID || sp.Amount != 150.50 {
		t.Fatalf("unexpected created split payment: %+v", sp)
	}
	if sp.PayerName != "Alice" || sp.ReceiverName != "Bob" {
		t.Fatalf("split payment names incorrect: payer=%q receiver=%q", sp.PayerName, sp.ReceiverName)
	}

	// Reader cannot create
	carol.request(http.MethodPost, "/api/split-payments", map[string]any{
		"payerUserId": carol.userID, "receiverUserId": alice.userID,
		"amount": 50, "date": "2026-04-10",
	}, http.StatusForbidden, nil)

	// List
	var list []splitPaymentResponse
	alice.request(http.MethodGet, "/api/split-payments", nil, http.StatusOK, &list)
	if len(list) != 1 || list[0].Note != "Conta do mercado" {
		t.Fatalf("unexpected split payments list: %+v", list)
	}

	// Patch
	var updated splitPaymentResponse
	alice.request(http.MethodPatch, "/api/split-payments/"+personalIDString(sp.ID), map[string]any{
		"payerUserId":    alice.userID,
		"receiverUserId": bob.userID,
		"amount":         200,
		"date":           "2026-04-15",
		"note":           "Conta revisada",
	}, http.StatusOK, &updated)
	if updated.Amount != 200 || updated.Note != "Conta revisada" || updated.Date != "2026-04-15" {
		t.Fatalf("unexpected updated split payment: %+v", updated)
	}

	// Delete
	alice.request(http.MethodDelete, "/api/split-payments/"+personalIDString(sp.ID), nil, http.StatusNoContent, nil)

	var afterDelete []splitPaymentResponse
	alice.request(http.MethodGet, "/api/split-payments", nil, http.StatusOK, &afterDelete)
	if len(afterDelete) != 0 {
		t.Fatalf("expected empty list after delete, got %d", len(afterDelete))
	}
}
