package api

import (
	"context"
	"testing"
)

// The owner's report: a recurring expense's occurrence deleted this month came back after logging in again,
// because the materializer recreated every missing occurrence. A deleted occurrence must stay deleted.
func TestDeletedOccurrenceStaysDeleted(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	start := monthsAgoFirst(t, app, 2)
	var rec recurrenceDTO
	ana.do("POST", ana.wpath("/recurrences"), map[string]any{
		"kind": "expense", "amountCents": 9990, "description": "Streaming", "categoryId": firstCategory(t, ana, "expense"),
		"dayOfMonth": 1, "startDate": start,
	}, 201, &rec)
	list := listRange(ana, start)
	if len(list) != 3 {
		t.Fatalf("%d occurrences, want 3", len(list))
	}
	deleted := list[0]
	ana.do("DELETE", ana.wpath("/transactions/%d", deleted.ID), nil, 204, nil)

	// what logging in again does: the screens load, which materializes; also the background loop
	ana.do("GET", ana.wpath("/recurrences"), nil, 200, nil)
	if err := app.s.mat.Run(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	after := listRange(ana, start)
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
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	shareWallet(ana, bia, "editor")
	ana.do("PATCH", ana.wpath(""), map[string]any{"splittingEnabled": true}, 200, nil)
	start := monthsAgoFirst(t, app, 1)
	ana.do("POST", ana.wpath("/recurrences"), map[string]any{"kind": "settlement", "amountCents": 30000,
		"paidByUserId": bia.user.ID, "toUserId": ana.user.ID, "dayOfMonth": 1, "startDate": start}, 201, nil)
	to := mustDate(start).AddDate(0, 0, 365).Format(dateLayout)
	var list []settlementDTO
	ana.do("GET", ana.wpath("/settlements?from=%s&to=%s", start, to), nil, 200, &list)
	if len(list) != 2 {
		t.Fatalf("%d settlements, want 2", len(list))
	}
	ana.do("DELETE", ana.wpath("/settlements/%d", list[0].ID), nil, 204, nil)
	if err := app.s.mat.Run(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	ana.do("GET", ana.wpath("/settlements?from=%s&to=%s", start, to), nil, 200, &list)
	if len(list) != 1 {
		t.Fatalf("%d settlements after deleting one, want 1: %+v", len(list), list)
	}
}

// The other occurrences keep coming: a deleted month does not stop the recurrence.
func TestDeletingAnOccurrenceKeepsTheRecurrenceGoing(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	start := monthsAgoFirst(t, app, 2)
	ana.do("POST", ana.wpath("/recurrences"), map[string]any{
		"kind": "expense", "amountCents": 100, "categoryId": firstCategory(t, ana, "expense"), "dayOfMonth": 1, "startDate": start,
	}, 201, nil)
	list := listRange(ana, start)
	ana.do("DELETE", ana.wpath("/transactions/%d", list[1].ID), nil, 204, nil) // a month in the middle
	// a manual delete of the newest occurrence must not block it either
	if _, err := app.s.db.Exec(`DELETE FROM transactions WHERE id = ?`, list[0].ID); err != nil {
		t.Fatal(err)
	}
	var skips int
	app.s.db.QueryRow(`SELECT COUNT(*) FROM recurrence_skips`).Scan(&skips)
	if skips != 2 {
		t.Fatalf("%d skipped occurrences recorded, want 2", skips)
	}
	if n := len(listRange(ana, start)); n != 1 {
		t.Fatalf("%d occurrences left, want 1", n)
	}
}

// Deleting a wallet cascades through recurrences and their occurrences; the skip trigger must not get in the way.
func TestDeletingAWalletWithRecurringOccurrences(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	var w struct {
		ID int64 `json:"id"`
	}
	ana.do("POST", "/api/wallets", map[string]any{"name": "Casa"}, 201, &w)
	var cats []struct {
		ID   int64  `json:"id"`
		Kind string `json:"kind"`
	}
	ana.do("GET", "/api/wallets/"+itoa(w.ID)+"/categories", nil, 200, &cats)
	var cat int64
	for _, c := range cats {
		if c.Kind == "expense" {
			cat = c.ID
			break
		}
	}
	ana.do("POST", "/api/wallets/"+itoa(w.ID)+"/recurrences", map[string]any{
		"kind": "expense", "amountCents": 100, "categoryId": cat, "dayOfMonth": 1, "startDate": monthsAgoFirst(t, app, 2),
	}, 201, nil)
	ana.do("DELETE", "/api/wallets/"+itoa(w.ID), nil, 204, nil)
	var skips, txs int
	app.s.db.QueryRow(`SELECT COUNT(*) FROM recurrence_skips`).Scan(&skips)
	app.s.db.QueryRow(`SELECT COUNT(*) FROM transactions WHERE wallet_id = ?`, w.ID).Scan(&txs)
	if skips != 0 || txs != 0 {
		t.Fatalf("after deleting the wallet: %d skips, %d transactions, want 0 and 0", skips, txs)
	}
}
