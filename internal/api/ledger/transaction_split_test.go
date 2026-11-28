package ledger_test

import (
	"slices"
	"testing"

	"gastos/internal/api/apitest"
	"gastos/internal/api/httpx"
	"gastos/internal/api/ledger"
)

// memberShare is one member's default share, as the members list returns it.
type memberShare struct {
	UserID          int64 `json:"userId"`
	SplitsByDefault bool  `json:"splitsByDefault"`
	DefaultShareBP  int   `json:"defaultShareBp"`
}

// shareOf returns the default share, in basis points, of the member userID.
func shareOf(t *testing.T, c *apitest.Client, userID int64) int {
	t.Helper()
	var list []memberShare
	c.Do("GET", c.WPath("/members"), nil, 200, &list)
	for _, m := range list {
		if m.UserID == userID {
			return m.DefaultShareBP
		}
	}
	t.Fatalf("member %d not listed: %+v", userID, list)
	return 0
}

func TestMemberDefaultSharePatch(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	apitest.ShareWallet(ana, bia, "editor")
	path := ana.WPath("/members/%d", bia.User.ID)

	var m memberShare
	ana.Do("PATCH", path, map[string]any{"defaultShareBp": 6000}, 200, &m)
	if m.DefaultShareBP != 6000 || shareOf(t, ana, bia.User.ID) != 6000 {
		t.Fatalf("set: %+v", m)
	}
	ana.Do("PATCH", path, map[string]any{"splitsByDefault": false}, 200, &m)
	if m.DefaultShareBP != 6000 {
		t.Fatalf("an omitted share changed: %+v", m)
	}
	ana.Do("PATCH", path, map[string]any{"defaultShareBp": 10000}, 200, &m)
	if m.DefaultShareBP != 10000 {
		t.Fatalf("100%%: %+v", m)
	}
	ana.Do("PATCH", path, map[string]any{"defaultShareBp": 0}, 200, &m)
	if m.DefaultShareBP != 0 || shareOf(t, ana, bia.User.ID) != 0 {
		t.Fatalf("clear: %+v", m)
	}
	for _, bad := range []int{10001, -1} {
		var res struct {
			Error httpx.APIError `json:"error"`
		}
		ana.Do("PATCH", path, map[string]any{"defaultShareBp": bad}, 400, &res)
		if res.Error.Fields["defaultShareBp"] != "use entre 0 e 100%" {
			t.Errorf("%d: %+v", bad, res.Error)
		}
	}
	bia.Do("PATCH", ana.WPath("/members/%d", ana.User.ID), map[string]any{"defaultShareBp": 5000}, 403, nil)
	ana.Do("PATCH", ana.WPath("/members/%d", 9999), map[string]any{"defaultShareBp": 5000}, 404, nil)
}

// splitsOf posts an expense without splits and returns share_bp per user.
func splitsOf(t *testing.T, c *apitest.Client, cat int64) map[int64]int64 {
	t.Helper()
	var tx ledger.Transaction
	c.Do("POST", c.WPath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 1000, "description": "x", "categoryId": cat, "date": "2026-10-01", "allowDuplicate": true,
	}, 201, &tx)
	out := map[int64]int64{}
	var cents int64
	for _, sp := range tx.Splits {
		out[sp.UserID] = sp.ShareBP
		cents += sp.AmountCents
	}
	if len(tx.Splits) > 0 && cents != 1000 {
		t.Fatalf("splits do not add up: %+v", tx.Splits)
	}
	return out
}

func TestDefaultSharesDriveSplits(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	cleo := app.Register("Cleo", "cleo@example.com")
	apitest.ShareWallet(ana, bia, "editor")
	apitest.ShareWallet(ana, cleo, "editor")
	ana.Do("PATCH", ana.WPath(""), map[string]any{"splittingEnabled": true}, 200, nil)
	// Cleo is not flagged to split by default: excluded even with a share.
	ana.Do("PATCH", ana.WPath("/members/%d", cleo.User.ID), map[string]any{"splitsByDefault": false, "defaultShareBp": 5000}, 200, nil)
	cat := apitest.FirstCategory(t, ana, "expense")
	set := func(a, b int) {
		ana.Do("PATCH", ana.WPath("/members/%d", ana.User.ID), map[string]any{"defaultShareBp": a}, 200, nil)
		ana.Do("PATCH", ana.WPath("/members/%d", bia.User.ID), map[string]any{"defaultShareBp": b}, 200, nil)
	}
	for _, tc := range []struct{ a, b, wantA, wantB int64 }{
		{6000, 4000, 6000, 4000},
		{1, 1, 5000, 5000},
		{3000, 3000, 5000, 5000},
		{7000, 2000, 7778, 2222},
		{6000, 0, 5000, 5000}, // Bia has no share: equal split
	} {
		set(int(tc.a), int(tc.b))
		got := splitsOf(t, ana, cat)
		if len(got) != 2 || got[ana.User.ID] != tc.wantA || got[bia.User.ID] != tc.wantB {
			t.Errorf("shares %d/%d: splits %v, want %d/%d", tc.a, tc.b, got, tc.wantA, tc.wantB)
		}
	}

	// 1 against 10000 and 10000 rounds to 0%: that member drops out instead of breaking the insert.
	ana.Do("PATCH", ana.WPath("/members/%d", cleo.User.ID), map[string]any{"splitsByDefault": true, "defaultShareBp": 10000}, 200, nil)
	set(1, 10000)
	if got := splitsOf(t, ana, cat); len(got) != 2 || got[bia.User.ID] != 5000 || got[cleo.User.ID] != 5000 {
		t.Errorf("rounded-to-zero share: splits %v", got)
	}
	ana.Do("PATCH", ana.WPath("/members/%d", cleo.User.ID), map[string]any{"splitsByDefault": false}, 200, nil)

	set(6000, 4000)
	var tx ledger.Transaction
	ana.Do("POST", ana.WPath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 1000, "description": "x", "categoryId": cat, "date": "2026-10-01", "allowDuplicate": true,
		"splits": []map[string]any{{"userId": ana.User.ID, "shareBp": 3000}, {"userId": bia.User.ID, "shareBp": 7000}},
	}, 201, &tx)
	if len(tx.Splits) != 2 || tx.Splits[0].ShareBP+tx.Splits[1].ShareBP != 10000 || tx.Splits[0].ShareBP == 6000 {
		t.Fatalf("explicit splits should win: %+v", tx.Splits)
	}

	start := apitest.MonthsAgoFirst(t, app, 0)
	var rec ledger.RecurrenceDTO
	ana.Do("POST", ana.WPath("/recurrences"), map[string]any{
		"kind": "expense", "amountCents": 1000, "description": "Aluguel", "categoryId": cat, "dayOfMonth": 1, "startDate": start,
	}, 201, &rec)
	var n int
	for _, tx := range apitest.ListRange(ana, start) {
		if tx.RecurrenceID == nil || *tx.RecurrenceID != rec.ID {
			continue
		}
		n++
		got := map[int64]int64{}
		for _, sp := range tx.Splits {
			got[sp.UserID] = sp.ShareBP
		}
		if len(got) != 2 || got[ana.User.ID] != 6000 || got[bia.User.ID] != 4000 {
			t.Errorf("materialized recurrence splits: %+v", tx.Splits)
		}
	}
	if n != 1 {
		t.Fatalf("%d materialized transactions, want 1", n)
	}
}

func TestWithAmountsSumsToTotal(t *testing.T) {
	got := ledger.WithAmounts(10001, []ledger.Split{{UserID: 1, ShareBP: 3334}, {UserID: 2, ShareBP: 3333}, {UserID: 3, ShareBP: 3333}})
	var sum int64
	var amounts []int64
	for _, s := range got {
		sum += s.AmountCents
		amounts = append(amounts, s.AmountCents)
	}
	if sum != 10001 {
		t.Fatalf("the sum of the parts (%v) must be the total", amounts)
	}
	// exact: 3334.33 / 3333.33 / 3333.33; the leftover cent goes to the largest remainder
	if !slices.Equal(amounts, []int64{3335, 3333, 3333}) {
		t.Errorf("unexpected allocation: %v", amounts)
	}
	if got[0].UserID != 1 || got[1].ShareBP != 3333 {
		t.Errorf("the order and the original shares must be preserved: %+v", got)
	}
	if len(ledger.WithAmounts(100, nil)) != 0 {
		t.Error("no shares, no allocation")
	}
}
