package bankimport

import (
	"sort"
	"strings"
	"time"
)

// Rule says "a description containing Pattern belongs to CategoryID". RenameTo, when set, replaces the bank's
// cryptic text (for example "IFD*IFOOD 123" becomes "iFood"). Higher Priority wins, then the longer (more specific)
// pattern, then the lowest ID, so the outcome never depends on the order rules arrive in.
type Rule struct {
	ID         int64
	Pattern    string
	CategoryID int64
	RenameTo   string
	Priority   int
}

// Candidate is a transaction already in the wallet that a statement line may be the same thing as
// (for example the lunch someone typed in by hand). AmountCents is positive; Kind is "expense" or "income".
type Candidate struct {
	ID          int64
	Kind        string
	AmountCents int64
	Date        string
}

// Suggestion is what to propose for a line. Zero values mean "no suggestion".
type Suggestion struct {
	CategoryID  int64
	Description string // a rename from the rule; empty keeps the bank's text
	MatchID     int64  // an existing transaction that looks like the same thing
}

// matchWindowDays is how far apart the dates of a line and an existing transaction may be:
// banks post a day or two after the purchase.
const matchWindowDays = 2

var accents = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u",
	"ç", "c", "ñ", "n",
)

// Normalize lowercases, removes Portuguese accents and collapses whitespace, so "PADARIA São João" and
// "padaria sao joao" compare equal.
func Normalize(s string) string {
	return strings.Join(strings.Fields(accents.Replace(strings.ToLower(s))), " ")
}

// Classify proposes a category (and optional rename) from the rules and an existing transaction the line may duplicate.
func Classify(e Entry, rules []Rule, existing []Candidate) Suggestion {
	var s Suggestion
	desc := Normalize(e.Description)

	type ranked struct {
		Rule
		pattern string
	}
	var hits []ranked
	for _, r := range rules {
		p := Normalize(r.Pattern)
		if p != "" && strings.Contains(desc, p) {
			hits = append(hits, ranked{r, p})
		}
	}
	sort.Slice(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		switch {
		case a.Priority != b.Priority:
			return a.Priority > b.Priority
		case len(a.pattern) != len(b.pattern):
			return len(a.pattern) > len(b.pattern)
		default:
			return a.ID < b.ID
		}
	})
	if len(hits) > 0 {
		s.CategoryID = hits[0].CategoryID
		s.Description = strings.TrimSpace(hits[0].RenameTo)
	}

	kind, abs := "expense", -e.AmountCents
	if e.AmountCents > 0 {
		kind, abs = "income", e.AmountCents
	}
	day, err := time.Parse("2006-01-02", e.Date)
	if err != nil {
		return s
	}
	bestGap := matchWindowDays + 1
	for _, c := range existing {
		if c.Kind != kind || c.AmountCents != abs {
			continue
		}
		cd, err := time.Parse("2006-01-02", c.Date)
		if err != nil {
			continue
		}
		gap := int(day.Sub(cd).Hours() / 24)
		if gap < 0 {
			gap = -gap
		}
		if gap < bestGap || (gap == bestGap && gap <= matchWindowDays && c.ID < s.MatchID) {
			bestGap, s.MatchID = gap, c.ID
		}
	}
	return s
}
