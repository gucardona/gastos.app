package api

import "testing"

type memberShare struct {
	UserID          int64 `json:"userId"`
	SplitsByDefault bool  `json:"splitsByDefault"`
	DefaultShareBP  int   `json:"defaultShareBp"`
}

func shareOf(t *testing.T, c *client, userID int64) int {
	t.Helper()
	var list []memberShare
	c.do("GET", c.wpath("/members"), nil, 200, &list)
	for _, m := range list {
		if m.UserID == userID {
			return m.DefaultShareBP
		}
	}
	t.Fatalf("member %d not listed: %+v", userID, list)
	return 0
}

func TestMemberDefaultSharePatch(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	shareWallet(ana, bia, "editor")
	path := ana.wpath("/members/%d", bia.user.ID)

	var m memberShare
	ana.do("PATCH", path, map[string]any{"defaultShareBp": 6000}, 200, &m)
	if m.DefaultShareBP != 6000 || shareOf(t, ana, bia.user.ID) != 6000 {
		t.Fatalf("set: %+v", m)
	}
	ana.do("PATCH", path, map[string]any{"splitsByDefault": false}, 200, &m)
	if m.DefaultShareBP != 6000 {
		t.Fatalf("an omitted share changed: %+v", m)
	}
	ana.do("PATCH", path, map[string]any{"defaultShareBp": 10000}, 200, &m)
	if m.DefaultShareBP != 10000 {
		t.Fatalf("100%%: %+v", m)
	}
	ana.do("PATCH", path, map[string]any{"defaultShareBp": 0}, 200, &m)
	if m.DefaultShareBP != 0 || shareOf(t, ana, bia.user.ID) != 0 {
		t.Fatalf("clear: %+v", m)
	}
	for _, bad := range []int{10001, -1} {
		var res struct {
			Error apiError `json:"error"`
		}
		ana.do("PATCH", path, map[string]any{"defaultShareBp": bad}, 400, &res)
		if res.Error.Fields["defaultShareBp"] != "use entre 0,01% e 100%" {
			t.Errorf("%d: %+v", bad, res.Error)
		}
	}
	bia.do("PATCH", ana.wpath("/members/%d", ana.user.ID), map[string]any{"defaultShareBp": 5000}, 403, nil)
	ana.do("PATCH", ana.wpath("/members/%d", 9999), map[string]any{"defaultShareBp": 5000}, 404, nil)
}

// splitsOf posts an expense without splits and returns share_bp per user.
func splitsOf(t *testing.T, c *client, cat int64) map[int64]int64 {
	t.Helper()
	var tx transaction
	c.do("POST", c.wpath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 1000, "description": "x", "categoryId": cat, "date": "2026-10-01",
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
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	cleo := app.register("Cleo", "cleo@example.com")
	shareWallet(ana, bia, "editor")
	shareWallet(ana, cleo, "editor")
	ana.do("PATCH", ana.wpath(""), map[string]any{"splittingEnabled": true}, 200, nil)
	// Cleo is not flagged to split by default: excluded even with a share.
	ana.do("PATCH", ana.wpath("/members/%d", cleo.user.ID), map[string]any{"splitsByDefault": false, "defaultShareBp": 5000}, 200, nil)
	cat := firstCategory(t, ana, "expense")
	set := func(a, b int) {
		ana.do("PATCH", ana.wpath("/members/%d", ana.user.ID), map[string]any{"defaultShareBp": a}, 200, nil)
		ana.do("PATCH", ana.wpath("/members/%d", bia.user.ID), map[string]any{"defaultShareBp": b}, 200, nil)
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
		if len(got) != 2 || got[ana.user.ID] != tc.wantA || got[bia.user.ID] != tc.wantB {
			t.Errorf("shares %d/%d: splits %v, want %d/%d", tc.a, tc.b, got, tc.wantA, tc.wantB)
		}
	}

	// 1 against 10000 and 10000 rounds to 0%: that member drops out instead of breaking the insert.
	ana.do("PATCH", ana.wpath("/members/%d", cleo.user.ID), map[string]any{"splitsByDefault": true, "defaultShareBp": 10000}, 200, nil)
	set(1, 10000)
	if got := splitsOf(t, ana, cat); len(got) != 2 || got[bia.user.ID] != 5000 || got[cleo.user.ID] != 5000 {
		t.Errorf("rounded-to-zero share: splits %v", got)
	}
	ana.do("PATCH", ana.wpath("/members/%d", cleo.user.ID), map[string]any{"splitsByDefault": false}, 200, nil)

	set(6000, 4000)
	var tx transaction
	ana.do("POST", ana.wpath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 1000, "description": "x", "categoryId": cat, "date": "2026-10-01",
		"splits": []map[string]any{{"userId": ana.user.ID, "shareBp": 3000}, {"userId": bia.user.ID, "shareBp": 7000}},
	}, 201, &tx)
	if len(tx.Splits) != 2 || tx.Splits[0].ShareBP+tx.Splits[1].ShareBP != 10000 || tx.Splits[0].ShareBP == 6000 {
		t.Fatalf("explicit splits should win: %+v", tx.Splits)
	}

	start := monthsAgoFirst(t, app, 0)
	var rec recurrenceDTO
	ana.do("POST", ana.wpath("/recurrences"), map[string]any{
		"kind": "expense", "amountCents": 1000, "description": "Aluguel", "categoryId": cat, "dayOfMonth": 1, "startDate": start,
	}, 201, &rec)
	var n int
	for _, tx := range listRange(ana, start) {
		if tx.RecurrenceID == nil || *tx.RecurrenceID != rec.ID {
			continue
		}
		n++
		got := map[int64]int64{}
		for _, sp := range tx.Splits {
			got[sp.UserID] = sp.ShareBP
		}
		if len(got) != 2 || got[ana.user.ID] != 6000 || got[bia.user.ID] != 4000 {
			t.Errorf("materialized recurrence splits: %+v", tx.Splits)
		}
	}
	if n != 1 {
		t.Fatalf("%d materialized transactions, want 1", n)
	}
}
