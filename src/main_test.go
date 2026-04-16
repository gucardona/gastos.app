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
		"name":     name,
		"email":    email,
		"password": "secret123",
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
