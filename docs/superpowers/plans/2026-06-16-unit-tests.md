# Unit Tests Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add comprehensive pure unit tests and integration tests to cover all untested logic in the codebase.

**Architecture:** Bottom-up — pure unit tests first (no DB, no HTTP) for models, events, validation, and helpers; then integration tests appended to `src/main_test.go` for the five uncovered HTTP endpoint groups.

**Tech Stack:** Go standard library `testing` package only; `net/http/httptest` + real SQLite (via `modernc.org/sqlite`) for integration tests.

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `src/models/models_test.go` | Create | Unit tests for PermissionsForRole, IsValidAccountRole, IsValidShareRole |
| `src/events/bus_test.go` | Create | Unit tests for EventBus Subscribe/Unsubscribe/Notify |
| `src/handlers/helpers_test.go` | Create | Unit tests for parseIntPathID, parseSplitParticipantIds, splitAmount |
| `src/handlers/validation_test.go` | Create | Unit tests for all pure validate* functions |
| `src/main_test.go` | Extend | New Test* functions for categories, payment methods, recurring incomes, recurring payments, split payments, expense splits |

---

## Task 1: Unit tests for `models` package

**Files:**
- Create: `src/models/models_test.go`

- [ ] **Step 1: Write the test file**

```go
package models

import "testing"

func TestPermissionsForRole(t *testing.T) {
	tests := []struct {
		role              string
		wantEdit          bool
		wantManageMembers bool
		wantDelete        bool
	}{
		{AccountRoleOwner, true, true, true},
		{AccountRoleEditor, true, false, false},
		{AccountRoleReader, false, false, false},
		{"", false, false, false},
		{"admin", false, false, false},
	}
	for _, tc := range tests {
		p := PermissionsForRole(tc.role)
		if p.CanEdit != tc.wantEdit || p.CanManageMembers != tc.wantManageMembers || p.CanDelete != tc.wantDelete {
			t.Errorf("PermissionsForRole(%q) = %+v, want {CanEdit:%v CanManageMembers:%v CanDelete:%v}",
				tc.role, p, tc.wantEdit, tc.wantManageMembers, tc.wantDelete)
		}
	}
}

func TestIsValidAccountRole(t *testing.T) {
	for _, r := range []string{AccountRoleOwner, AccountRoleEditor, AccountRoleReader} {
		if !IsValidAccountRole(r) {
			t.Errorf("IsValidAccountRole(%q) = false, want true", r)
		}
	}
	for _, r := range []string{"", "admin", "superuser", "OWNER"} {
		if IsValidAccountRole(r) {
			t.Errorf("IsValidAccountRole(%q) = true, want false", r)
		}
	}
}

func TestIsValidShareRole(t *testing.T) {
	for _, r := range []string{AccountRoleEditor, AccountRoleReader} {
		if !IsValidShareRole(r) {
			t.Errorf("IsValidShareRole(%q) = false, want true", r)
		}
	}
	for _, r := range []string{AccountRoleOwner, "", "admin", "EDITOR"} {
		if IsValidShareRole(r) {
			t.Errorf("IsValidShareRole(%q) = true, want false", r)
		}
	}
}
```

- [ ] **Step 2: Run to verify passing**

```
cd /home/gustavo/gupa.dev/gastos.app && go test ./src/models/... -v
```

Expected output contains: `PASS` for all three test functions, no compile errors.

- [ ] **Step 3: Commit**

```bash
git add src/models/models_test.go
git commit -m "test: unit tests for models helpers (PermissionsForRole, role validation)"
```

---

## Task 2: Unit tests for `EventBus`

**Files:**
- Create: `src/events/bus_test.go`

- [ ] **Step 1: Write the test file**

```go
package events

import (
	"sync"
	"testing"
	"time"
)

func newBus() *EventBus {
	return &EventBus{subs: make(map[int64][]*sub)}
}

func TestBusNotifyExcludesActor(t *testing.T) {
	b := newBus()

	actorCh := b.Subscribe(10, 1)
	otherCh := b.Subscribe(10, 2)

	b.Notify(10, 1) // actor = userID 1

	select {
	case <-actorCh:
		t.Error("actor should not receive its own notification")
	default:
	}

	select {
	case <-otherCh:
		// expected
	default:
		t.Error("non-actor subscriber should receive notification")
	}
}

func TestBusUnsubscribe(t *testing.T) {
	b := newBus()

	ch := b.Subscribe(10, 1)
	b.Unsubscribe(10, ch)

	b.Notify(10, 99) // actor=99 so ch (userID=1) would normally receive

	select {
	case <-ch:
		t.Error("unsubscribed channel should not receive notifications")
	default:
	}
}

func TestBusNotifyFullChannelDoesNotBlock(t *testing.T) {
	b := newBus()

	ch := b.Subscribe(10, 2)
	ch <- struct{}{} // fill the channel (capacity 1)

	done := make(chan struct{})
	go func() {
		b.Notify(10, 1) // actor=1, subscriber userID=2 would receive but channel is full
		close(done)
	}()

	select {
	case <-done:
		// Notify returned without blocking
	case <-time.After(time.Second):
		t.Error("Notify blocked on a full channel")
	}
}

func TestBusConcurrentRace(t *testing.T) {
	b := newBus()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ch := b.Subscribe(10, int64(i))
			b.Notify(10, int64(i))
			b.Unsubscribe(10, ch)
		}(i)
	}
	wg.Wait()
}
```

- [ ] **Step 2: Run with race detector**

```
cd /home/gustavo/gupa.dev/gastos.app && go test ./src/events/... -v -race
```

Expected: `PASS` for all four tests, no data race warnings.

- [ ] **Step 3: Commit**

```bash
git add src/events/bus_test.go
git commit -m "test: unit tests for EventBus (notify, unsubscribe, full channel, race)"
```

---

## Task 3: Unit tests for handler helpers and split math

**Files:**
- Create: `src/handlers/helpers_test.go`

These functions are in `package handlers` but are pure (no DB calls), so they can be tested without initializing the database.

- [ ] **Step 1: Write the test file**

```go
package handlers

import "testing"

func TestParseIntPathID(t *testing.T) {
	tests := []struct {
		path    string
		prefix  string
		want    int64
		wantErr bool
	}{
		{"/api/expenses/42", "/api/expenses/", 42, false},
		{"/api/expenses/1", "/api/expenses/", 1, false},
		{"/api/expenses/", "/api/expenses/", 0, true},   // empty suffix
		{"/api/expenses/0", "/api/expenses/", 0, true},   // zero is invalid
		{"/api/expenses/-1", "/api/expenses/", 0, true},  // negative
		{"/api/expenses/abc", "/api/expenses/", 0, true}, // non-numeric
	}
	for _, tc := range tests {
		got, err := parseIntPathID(tc.path, tc.prefix)
		if (err != nil) != tc.wantErr {
			t.Errorf("parseIntPathID(%q, %q) error = %v, wantErr %v", tc.path, tc.prefix, err, tc.wantErr)
			continue
		}
		if !tc.wantErr && got != tc.want {
			t.Errorf("parseIntPathID(%q, %q) = %d, want %d", tc.path, tc.prefix, got, tc.want)
		}
	}
}

func TestParseSplitParticipantIds(t *testing.T) {
	tests := []struct {
		input string
		want  []int64
	}{
		{"", []int64{}},
		{"1", []int64{1}},
		{"1,2,3", []int64{1, 2, 3}},
		{"1, 2, 3", []int64{1, 2, 3}},  // spaces trimmed
		{"-1,0,2", []int64{2}},          // negative and zero skipped
		{"abc,2", []int64{2}},           // non-numeric skipped
	}
	for _, tc := range tests {
		got := parseSplitParticipantIds(tc.input)
		if len(got) != len(tc.want) {
			t.Errorf("parseSplitParticipantIds(%q) = %v, want %v", tc.input, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("parseSplitParticipantIds(%q)[%d] = %d, want %d", tc.input, i, got[i], tc.want[i])
			}
		}
	}
}

func TestSplitAmount(t *testing.T) {
	// splitAmount(amount, percentage) = math.Round(amount*percentage) / 100
	tests := []struct {
		amount     float64
		percentage float64
		want       float64
	}{
		{100, 50, 50},
		{100, 33.333, 33.33},   // math.Round(3333.3)/100 = 3333/100 = 33.33
		{100, 66.667, 66.67},   // math.Round(6666.7)/100 = 6667/100 = 66.67
		{0, 50, 0},
		{200, 25, 50},
	}
	for _, tc := range tests {
		got := splitAmount(tc.amount, tc.percentage)
		if got != tc.want {
			t.Errorf("splitAmount(%v, %v) = %v, want %v", tc.amount, tc.percentage, got, tc.want)
		}
	}
}
```

- [ ] **Step 2: Run to verify passing**

```
cd /home/gustavo/gupa.dev/gastos.app && go test ./src/handlers/... -v -run "TestParseIntPathID|TestParseSplitParticipantIds|TestSplitAmount"
```

Expected: `PASS` for all three tests.

- [ ] **Step 3: Commit**

```bash
git add src/handlers/helpers_test.go
git commit -m "test: unit tests for parseIntPathID, parseSplitParticipantIds, splitAmount"
```

---

## Task 4: Unit tests for validation functions

**Files:**
- Create: `src/handlers/validation_test.go`

All validate* functions tested here are pure (no DB calls). `validateRecurringPayment` and `validateSplitPayment` call `accountMemberSplits` (which uses `db.DB`) — those are covered by integration tests instead.

- [ ] **Step 1: Write the test file**

```go
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
```

- [ ] **Step 2: Run to verify passing**

```
cd /home/gustavo/gupa.dev/gastos.app && go test ./src/handlers/... -v -run "TestValidate|TestIsValidDate"
```

Expected: all subtests pass with `PASS`. Specifically verify that `TestValidateRecurringExpense/end_before_start` and `TestValidateCategory/non-hex_chars` pass.

- [ ] **Step 3: Commit**

```bash
git add src/handlers/validation_test.go
git commit -m "test: unit tests for all pure validation functions"
```

---

## Task 5: Integration test — Categories and Payment Methods

**Files:**
- Modify: `src/main_test.go`

Add new response types and `TestCategoriesAndPaymentMethods` at the bottom of the file.

Every fresh account is seeded with 15 default categories (keys: food, market, transport, …, other) and 8 payment methods (keys: pix, credit, debit, …, transfer) by `db.seedAccountDefaults`. The test can rely on those being present.

- [ ] **Step 1: Append new response types to `src/main_test.go`**

Add after the `recurringExpenseResponse` type block (around line 78):

```go
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
```

- [ ] **Step 2: Append `TestCategoriesAndPaymentMethods` to `src/main_test.go`**

```go
func TestCategoriesAndPaymentMethods(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	alice := registerUser(t, server.URL, "Alice", "alice@example.com")
	aliceAccounts := fetchAccounts(t, alice)
	alice.accountID = personalIDString(aliceAccounts[0].ID)

	// Invite Bob as reader to alice's personal account won't work for a "reader" shared account.
	// Instead create a shared account and invite Bob as reader to test permission enforcement.
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
	replacementKey := initCats[0].Key // use the first seeded category as replacement

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
```

- [ ] **Step 3: Run to verify passing**

```
cd /home/gustavo/gupa.dev/gastos.app && go test ./src/... -v -run TestCategoriesAndPaymentMethods
```

Expected: `--- PASS: TestCategoriesAndPaymentMethods`

- [ ] **Step 4: Commit**

```bash
git add src/main_test.go
git commit -m "test: integration tests for categories and payment methods CRUD"
```

---

## Task 6: Integration test — Recurring Incomes

**Files:**
- Modify: `src/main_test.go`

- [ ] **Step 1: Append new response type to `src/main_test.go`**

Add after the `paymentMethodResponse` type:

```go
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
```

- [ ] **Step 2: Append `TestRecurringIncomesLifecycle` to `src/main_test.go`**

```go
func TestRecurringIncomesLifecycle(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	alice := registerUser(t, server.URL, "Alice", "alice@example.com")
	aliceAccounts := fetchAccounts(t, alice)
	alice.accountID = personalIDString(aliceAccounts[0].ID)

	bob := registerUser(t, server.URL, "Bob", "bob@example.com")
	var shared accountResponse
	alice.request(http.MethodPost, "/api/accounts", map[string]any{"name": "Casa"}, http.StatusCreated, &shared)
	alice.request(http.MethodPost, "/api/accounts/"+personalIDString(shared.ID)+"/members", map[string]any{
		"email": "bob@example.com",
		"role":  "reader",
	}, http.StatusCreated, nil)
	bob.accountID = personalIDString(shared.ID)
	alice.accountID = personalIDString(shared.ID)

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
```

- [ ] **Step 3: Run to verify passing**

```
cd /home/gustavo/gupa.dev/gastos.app && go test ./src/... -v -run TestRecurringIncomesLifecycle
```

Expected: `--- PASS: TestRecurringIncomesLifecycle`

- [ ] **Step 4: Commit**

```bash
git add src/main_test.go
git commit -m "test: integration tests for recurring incomes lifecycle"
```

---

## Task 7: Integration test — Recurring Payments

**Files:**
- Modify: `src/main_test.go`

Recurring payments require two distinct account members as payer and receiver, and membership is validated server-side.

- [ ] **Step 1: Append new response type to `src/main_test.go`**

Add after `recurringIncomeResponse`:

```go
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
```

- [ ] **Step 2: Append `TestRecurringPaymentsLifecycle` to `src/main_test.go`**

```go
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
```

- [ ] **Step 3: Run to verify passing**

```
cd /home/gustavo/gupa.dev/gastos.app && go test ./src/... -v -run TestRecurringPaymentsLifecycle
```

Expected: `--- PASS: TestRecurringPaymentsLifecycle`

- [ ] **Step 4: Commit**

```bash
git add src/main_test.go
git commit -m "test: integration tests for recurring payments lifecycle"
```

---

## Task 8: Integration test — Split Payments

**Files:**
- Modify: `src/main_test.go`

- [ ] **Step 1: Append new response type to `src/main_test.go`**

Add after `recurringPaymentResponse`:

```go
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
```

- [ ] **Step 2: Append `TestSplitPaymentsLifecycle` to `src/main_test.go`**

```go
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
```

- [ ] **Step 3: Run to verify passing**

```
cd /home/gustavo/gupa.dev/gastos.app && go test ./src/... -v -run TestSplitPaymentsLifecycle
```

Expected: `--- PASS: TestSplitPaymentsLifecycle`

- [ ] **Step 4: Commit**

```bash
git add src/main_test.go
git commit -m "test: integration tests for split payments lifecycle"
```

---

## Task 9: Integration test — Expense Splits (covers `normalizeSplits`)

**Files:**
- Modify: `src/main_test.go`

`normalizeSplits` is DB-coupled (queries `account_members`), so it is exercised here via the HTTP layer. The response includes a `splits` array with per-member `userId`, `percentage`, and `amount`.

- [ ] **Step 1: Append new response types to `src/main_test.go`**

Add after `splitPaymentResponse`:

```go
type splitEntryResponse struct {
	UserID     int64   `json:"userId"`
	Name       string  `json:"name"`
	Percentage float64 `json:"percentage"`
	Amount     float64 `json:"amount"`
}

type expenseWithSplitsResponse struct {
	ID           int64                `json:"id"`
	Amount       float64              `json:"amount"`
	Description  string               `json:"description"`
	Category     string               `json:"category"`
	Payment      string               `json:"payment"`
	Date         string               `json:"date"`
	PaidByUserID int64                `json:"paidByUserId"`
	Splits       []splitEntryResponse `json:"splits"`
}
```

- [ ] **Step 2: Append `TestExpenseSplitsRounding` to `src/main_test.go`**

```go
func TestExpenseSplitsRounding(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	alice := registerUser(t, server.URL, "Alice", "alice@example.com")
	bob := registerUser(t, server.URL, "Bob", "bob@example.com")

	var shared accountResponse
	alice.request(http.MethodPost, "/api/accounts", map[string]any{"name": "Casa"}, http.StatusCreated, &shared)
	alice.request(http.MethodPost, "/api/accounts/"+personalIDString(shared.ID)+"/members", map[string]any{
		"email": "bob@example.com",
		"role":  "editor",
	}, http.StatusCreated, nil)
	alice.accountID = personalIDString(shared.ID)
	bob.accountID = personalIDString(shared.ID)

	// Explicit splits: alice 60%, bob 40% of 100
	// splitAmount(100, 60) = math.Round(100*60)/100 = 60.00
	// splitAmount(100, 40) = math.Round(100*40)/100 = 40.00
	var expense expenseWithSplitsResponse
	alice.request(http.MethodPost, "/api/expenses", map[string]any{
		"amount":      100,
		"description": "Mercado",
		"category":    "market",
		"payment":     "pix",
		"date":        "2026-04-10",
		"splits": []map[string]any{
			{"userId": alice.userID, "percentage": 60},
			{"userId": bob.userID, "percentage": 40},
		},
	}, http.StatusCreated, &expense)
	if expense.ID == 0 {
		t.Fatal("expense should be created with an id")
	}
	if len(expense.Splits) != 2 {
		t.Fatalf("expected 2 splits, got %d: %+v", len(expense.Splits), expense.Splits)
	}

	findSplit := func(userID int64) splitEntryResponse {
		for _, s := range expense.Splits {
			if s.UserID == userID {
				return s
			}
		}
		t.Fatalf("split for userID %d not found in %+v", userID, expense.Splits)
		return splitEntryResponse{}
	}
	aliceSplit := findSplit(alice.userID)
	bobSplit := findSplit(bob.userID)
	if aliceSplit.Percentage != 60 || aliceSplit.Amount != 60 {
		t.Errorf("alice split: want percentage=60 amount=60, got %+v", aliceSplit)
	}
	if bobSplit.Percentage != 40 || bobSplit.Amount != 40 {
		t.Errorf("bob split: want percentage=40 amount=40, got %+v", bobSplit)
	}

	// Default split (no splits provided) → equal 50/50
	var expense2 expenseWithSplitsResponse
	alice.request(http.MethodPost, "/api/expenses", map[string]any{
		"amount":      200,
		"description": "Aluguel",
		"category":    "home",
		"payment":     "pix",
		"date":        "2026-04-11",
	}, http.StatusCreated, &expense2)
	if len(expense2.Splits) != 2 {
		t.Fatalf("expected 2 default splits, got %d: %+v", len(expense2.Splits), expense2.Splits)
	}
	for _, s := range expense2.Splits {
		if s.Amount != 100 {
			t.Errorf("default equal split: expected amount=100, got %+v", s)
		}
	}

	// Splits that don't sum to 100 → 400
	alice.request(http.MethodPost, "/api/expenses", map[string]any{
		"amount":      100,
		"description": "Erro",
		"category":    "food",
		"payment":     "pix",
		"date":        "2026-04-12",
		"splits": []map[string]any{
			{"userId": alice.userID, "percentage": 60},
			{"userId": bob.userID, "percentage": 30}, // sums to 90, not 100
		},
	}, http.StatusBadRequest, nil)

	// Split referencing a user not in the account → 400
	alice.request(http.MethodPost, "/api/expenses", map[string]any{
		"amount":      100,
		"description": "Erro",
		"category":    "food",
		"payment":     "pix",
		"date":        "2026-04-12",
		"splits": []map[string]any{
			{"userId": alice.userID, "percentage": 60},
			{"userId": int64(9999), "percentage": 40}, // non-member
		},
	}, http.StatusBadRequest, nil)
}
```

- [ ] **Step 3: Run to verify passing**

```
cd /home/gustavo/gupa.dev/gastos.app && go test ./src/... -v -run TestExpenseSplitsRounding
```

Expected: `--- PASS: TestExpenseSplitsRounding`

- [ ] **Step 4: Run the full suite to confirm nothing regressed**

```
cd /home/gustavo/gupa.dev/gastos.app && go test ./src/... -v -race
```

Expected: all tests pass, no data races.

- [ ] **Step 5: Commit**

```bash
git add src/main_test.go
git commit -m "test: integration tests for expense splits and normalizeSplits rounding"
```

---

## Self-Review Notes

- `validateRecurringPayment` and `validateSplitPayment` are DB-coupled (call `accountMemberSplits`) — covered by Tasks 7 and 8 respectively, not by unit tests.
- `normalizeSplits` is DB-coupled — covered by Task 9 via HTTP.
- SSE (`handlers/events.go`) excluded: not testable with `httptest`.
- All pure functions have unit tests. All uncovered HTTP endpoints have integration tests.
- Response types added to `main_test.go` in Tasks 5–9 should be added in order (each task appends to the file).
