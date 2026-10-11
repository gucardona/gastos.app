package bankimport

import "testing"

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{
		"  PADARIA   São João ": "padaria sao joao",
		"AÇAÍ do Zé":            "acai do ze",
		"IFD*IFOOD 123":         "ifd*ifood 123",
		"Pão de Açúcar":         "pao de acucar",
		"":                      "",
	} {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func entry(desc string, cents int64, date string) Entry {
	return Entry{ExternalID: "x", Date: date, AmountCents: cents, Description: desc}
}

func TestClassifyAppliesTheBestRule(t *testing.T) {
	rules := []Rule{
		{ID: 1, Pattern: "uber", CategoryID: 10},
		{ID: 2, Pattern: "uber eats", CategoryID: 20, RenameTo: "Uber Eats"},
		{ID: 3, Pattern: "mercado", CategoryID: 30, Priority: 5},
	}
	tests := []struct {
		name     string
		desc     string
		wantCat  int64
		wantName string
	}{
		{"a rule matches by substring", "UBER *TRIP 123", 10, ""},
		{"the longer pattern wins a tie in priority", "UBER EATS PEDIDO", 20, "Uber Eats"},
		{"a rename is suggested with the category", "uber eats", 20, "Uber Eats"},
		{"accents and case are ignored", "Compra MERCADO São João", 30, ""},
		{"a higher priority wins over a longer pattern", "UBER EATS MERCADO", 30, ""},
		{"no rule, no suggestion", "FARMACIA CENTRAL", 0, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(entry(tc.desc, -1000, "2026-10-01"), rules, nil)
			if got.CategoryID != tc.wantCat || got.Description != tc.wantName {
				t.Errorf("got %+v, want category %d and name %q", got, tc.wantCat, tc.wantName)
			}
		})
	}
}

func TestClassifyEqualRulesAreDeterministic(t *testing.T) {
	// same priority and same length: the oldest rule (lowest id) wins, whatever order they arrive in
	a := []Rule{{ID: 7, Pattern: "loja", CategoryID: 1}, {ID: 3, Pattern: "casa", CategoryID: 2}}
	b := []Rule{a[1], a[0]}
	e := entry("LOJA E CASA", -100, "2026-10-01")
	if x, y := Classify(e, a, nil), Classify(e, b, nil); x != y || x.CategoryID != 2 {
		t.Fatalf("got %+v and %+v, want the same result with category 2", x, y)
	}
}

func TestClassifyIgnoresEmptyPatterns(t *testing.T) {
	// an empty pattern would match everything
	if got := Classify(entry("QUALQUER COISA", -100, "2026-10-01"), []Rule{{ID: 1, Pattern: "   ", CategoryID: 9}}, nil); got.CategoryID != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestClassifySuggestsAnExistingTransaction(t *testing.T) {
	existing := []Candidate{
		{ID: 1, Kind: "expense", AmountCents: 4890, Date: "2026-10-03"},
		{ID: 2, Kind: "expense", AmountCents: 4890, Date: "2026-10-01"}, // exact day: closer
		{ID: 3, Kind: "income", AmountCents: 4890, Date: "2026-10-01"},  // same value, wrong kind
		{ID: 4, Kind: "expense", AmountCents: 5000, Date: "2026-10-01"}, // other value
		{ID: 5, Kind: "expense", AmountCents: 4890, Date: "2026-10-09"}, // too far
	}
	got := Classify(entry("COMPRA MERCADO", -4890, "2026-10-01"), nil, existing)
	if got.MatchID != 2 {
		t.Fatalf("got match %d, want 2 (same value, closest date, same kind)", got.MatchID)
	}
}

func TestClassifyMatchWindowIsTwoDays(t *testing.T) {
	cand := func(date string) []Candidate {
		return []Candidate{{ID: 1, Kind: "expense", AmountCents: 1000, Date: date}}
	}
	for date, want := range map[string]int64{
		"2026-09-28": 0, // 3 days before
		"2026-09-29": 1, // 2 days before
		"2026-10-03": 1, // 2 days after
		"2026-10-04": 0, // 3 days after
	} {
		if got := Classify(entry("X", -1000, "2026-10-01"), nil, cand(date)).MatchID; got != want {
			t.Errorf("candidate on %s: match %d, want %d", date, got, want)
		}
	}
}

func TestClassifyMatchesIncomeToo(t *testing.T) {
	got := Classify(entry("SALARIO", 850000, "2026-10-05"), nil, []Candidate{{ID: 9, Kind: "income", AmountCents: 850000, Date: "2026-10-05"}})
	if got.MatchID != 9 {
		t.Fatalf("got %+v", got)
	}
}

func TestClassifyMatchTieGoesToTheLowestID(t *testing.T) {
	existing := []Candidate{
		{ID: 8, Kind: "expense", AmountCents: 100, Date: "2026-10-02"},
		{ID: 6, Kind: "expense", AmountCents: 100, Date: "2026-09-30"}, // also one day away, on the other side
	}
	e := entry("X", -100, "2026-10-01")
	if got := Classify(e, nil, existing).MatchID; got != 6 {
		t.Fatalf("got %d, want 6", got)
	}
}

func TestClassifyCombinesCategoryAndMatch(t *testing.T) {
	got := Classify(entry("MERCADO", -4890, "2026-10-01"),
		[]Rule{{ID: 1, Pattern: "mercado", CategoryID: 3}},
		[]Candidate{{ID: 4, Kind: "expense", AmountCents: 4890, Date: "2026-10-01"}})
	if got.CategoryID != 3 || got.MatchID != 4 {
		t.Fatalf("got %+v", got)
	}
}

func TestClassifyWithBadDates(t *testing.T) {
	// a malformed candidate date must not match or panic
	got := Classify(entry("X", -100, "2026-10-01"), nil, []Candidate{{ID: 1, Kind: "expense", AmountCents: 100, Date: "garbage"}})
	if got.MatchID != 0 {
		t.Fatalf("got %+v", got)
	}
}
