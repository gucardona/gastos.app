package ledger_test

import (
	"context"
	"testing"

	"gastos/internal/api/apitest"
	"gastos/internal/api/httpx"
	"gastos/internal/api/ledger"
)

// The owner's report: a recurring expense's occurrence deleted this month came back after logging in again,
// because the materializer recreated every missing occurrence. A deleted occurrence must stay deleted.
func TestDeletedOccurrenceStaysDeleted(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	start := apitest.MonthsAgoFirst(t, app, 2)
	var rec ledger.RecurrenceDTO
	ana.Do("POST", ana.WPath("/recurrences"), map[string]any{
		"kind": "expense", "amountCents": 9990, "description": "Streaming", "categoryId": apitest.FirstCategory(t, ana, "expense"),
		"dayOfMonth": 1, "startDate": start,
	}, 201, &rec)
	list := apitest.ListRange(ana, start)
	if len(list) != 3 {
		t.Fatalf("%d occurrences, want 3", len(list))
	}
	deleted := list[0]
	ana.Do("DELETE", ana.WPath("/transactions/%d", deleted.ID), nil, 204, nil)

	// what logging in again does: the screens load, which materializes; also the background loop
	ana.Do("GET", ana.WPath("/recurrences"), nil, 200, nil)
	if err := app.Server().Mat.Run(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	after := apitest.ListRange(ana, start)
	if len(after) != 2 {
		t.Fatalf("%d occurrences after deleting one and materializing again, want 2: %+v", len(after), after)
	}
	for _, tx := range after {
		if tx.Date == deleted.Date {
			t.Fatalf("the deleted occurrence of %s came back", deleted.Date)
		}
	}
}

func TestDeletedSettlementOccurrenceStaysDeleted(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	apitest.ShareWallet(ana, bia, "editor")
	ana.Do("PATCH", ana.WPath(""), map[string]any{"splittingEnabled": true}, 200, nil)
	start := apitest.MonthsAgoFirst(t, app, 1)
	ana.Do("POST", ana.WPath("/recurrences"), map[string]any{"kind": "settlement", "amountCents": 30000,
		"paidByUserId": bia.User.ID, "toUserId": ana.User.ID, "dayOfMonth": 1, "startDate": start}, 201, nil)
	to := ledger.MustDate(start).AddDate(0, 0, 365).Format(httpx.DateLayout)
	var list []ledger.SettlementDTO
	ana.Do("GET", ana.WPath("/settlements?from=%s&to=%s", start, to), nil, 200, &list)
	if len(list) != 2 {
		t.Fatalf("%d settlements, want 2", len(list))
	}
	ana.Do("DELETE", ana.WPath("/settlements/%d", list[0].ID), nil, 204, nil)
	if err := app.Server().Mat.Run(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	ana.Do("GET", ana.WPath("/settlements?from=%s&to=%s", start, to), nil, 200, &list)
	if len(list) != 1 {
		t.Fatalf("%d settlements after deleting one, want 1: %+v", len(list), list)
	}
}

// The other occurrences keep coming: a deleted month does not stop the recurrence.
func TestDeletingAnOccurrenceKeepsTheRecurrenceGoing(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	start := apitest.MonthsAgoFirst(t, app, 2)
	ana.Do("POST", ana.WPath("/recurrences"), map[string]any{
		"kind": "expense", "amountCents": 100, "categoryId": apitest.FirstCategory(t, ana, "expense"), "dayOfMonth": 1, "startDate": start,
	}, 201, nil)
	list := apitest.ListRange(ana, start)
	ana.Do("DELETE", ana.WPath("/transactions/%d", list[1].ID), nil, 204, nil) // a month in the middle
	// a manual delete of the newest occurrence must not block it either
	if _, err := app.Server().DB.Exec(`DELETE FROM transactions WHERE id = ?`, list[0].ID); err != nil {
		t.Fatal(err)
	}
	var skips int
	//nolint:errcheck,gosec // moved unchanged, see docs/security.md
	app.Server().DB.QueryRow(`SELECT COUNT(*) FROM recurrence_skips`).Scan(&skips)
	if skips != 2 {
		t.Fatalf("%d skipped occurrences recorded, want 2", skips)
	}
	if n := len(apitest.ListRange(ana, start)); n != 1 {
		t.Fatalf("%d occurrences left, want 1", n)
	}
}

// Deleting a wallet cascades through recurrences and their occurrences; the skip trigger must not get in the way.
func TestDeletingAWalletWithRecurringOccurrences(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	var w struct {
		ID int64 `json:"id"`
	}
	ana.Do("POST", "/api/wallets", map[string]any{"name": "Casa"}, 201, &w)
	var cats []struct {
		ID   int64  `json:"id"`
		Kind string `json:"kind"`
	}
	ana.Do("GET", "/api/wallets/"+apitest.Itoa(w.ID)+"/categories", nil, 200, &cats)
	var cat int64
	for _, c := range cats {
		if c.Kind == "expense" {
			cat = c.ID
			break
		}
	}
	ana.Do("POST", "/api/wallets/"+apitest.Itoa(w.ID)+"/recurrences", map[string]any{
		"kind": "expense", "amountCents": 100, "categoryId": cat, "dayOfMonth": 1, "startDate": apitest.MonthsAgoFirst(t, app, 2),
	}, 201, nil)
	ana.Do("DELETE", "/api/wallets/"+apitest.Itoa(w.ID), nil, 204, nil)
	var skips, txs int
	//nolint:errcheck,gosec // moved unchanged, see docs/security.md
	app.Server().DB.QueryRow(`SELECT COUNT(*) FROM recurrence_skips`).Scan(&skips)
	//nolint:errcheck,gosec // moved unchanged, see docs/security.md
	app.Server().DB.QueryRow(`SELECT COUNT(*) FROM transactions WHERE wallet_id = ?`, w.ID).Scan(&txs)
	if skips != 0 || txs != 0 {
		t.Fatalf("after deleting the wallet: %d skips, %d transactions, want 0 and 0", skips, txs)
	}
}
