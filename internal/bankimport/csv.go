package bankimport

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// CSVMapping says which column holds what. Columns are numbered from 1, as in a spreadsheet.
// InvertSign is for card exports that list purchases as positive numbers.
type CSVMapping struct {
	Date, Description, Amount int
	HasHeader                 bool
	InvertSign                bool
}

var errBadMapping = errors.New("invalid column mapping: use three different columns, numbered from 1")

// ParseCSV reads a statement exported as CSV. The delimiter (; , or tab) and the encoding are detected,
// numbers use Brazilian or international formats (see ParseAmount) and dates are dd/mm/yyyy or yyyy-mm-dd.
// Blank rows and zero-amount rows are skipped. Errors name the line of the file.
func ParseCSV(r io.Reader, m CSVMapping) ([]Entry, error) {
	if m.Date < 1 || m.Description < 1 || m.Amount < 1 || m.Date == m.Description || m.Date == m.Amount || m.Description == m.Amount {
		return nil, errBadMapping
	}
	raw, err := io.ReadAll(io.LimitReader(r, MaxFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > MaxFileBytes {
		return nil, ErrTooLarge
	}
	text := decodeText(raw)

	cr := csv.NewReader(strings.NewReader(text))
	cr.Comma = detectDelimiter(text)
	cr.FieldsPerRecord = -1 // rows may have different widths; we check the columns we need ourselves
	cr.LazyQuotes = true
	cr.TrimLeadingSpace = true

	var (
		entries []Entry
		seenID  = map[string]int{}
		seenRow = map[string]int{}
		first   = true
		need    = max(m.Date, m.Description, m.Amount)
	)
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		line, _ := cr.FieldPos(0)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		isHeader := first && m.HasHeader
		first = false
		if isHeader || blank(rec) {
			continue
		}
		if len(rec) < need {
			return nil, fmt.Errorf("line %d: expected at least %d columns, found %d", line, need, len(rec))
		}

		date, err := parseDate(rec[m.Date-1])
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		cents, err := ParseAmount(rec[m.Amount-1])
		if err != nil {
			return nil, fmt.Errorf("line %d: amount %q: %w", line, rec[m.Amount-1], err)
		}
		if cents == 0 {
			continue
		}
		if m.InvertSign {
			cents = -cents
		}
		desc := tidy(rec[m.Description-1])
		if desc == "" {
			desc = NoDescription
		}

		key := fmt.Sprintf("%s|%d|%s", date, cents, desc)
		id := fallbackID(date, cents, desc, seenRow[key])
		seenRow[key]++
		entries = append(entries, Entry{ExternalID: uniqueID(seenID, id), Date: date, AmountCents: cents, Description: desc})
	}
	if len(entries) == 0 {
		return nil, ErrNoTransactions
	}
	return entries, nil
}

// detectDelimiter counts ; tab and , in the first non-empty line, outside quotes, and picks the most frequent.
func detectDelimiter(text string) rune {
	counts := map[rune]int{}
	inQuotes := false
	for _, c := range text {
		if c == '"' {
			inQuotes = !inQuotes
		}
		if !inQuotes && (c == '\n') && len(counts) > 0 {
			break
		}
		if !inQuotes && (c == ';' || c == '\t' || c == ',') {
			counts[c]++
		}
	}
	best, bestN := ';', 0
	for _, d := range []rune{';', '\t', ','} { // on a tie, prefer ; (Brazil), then tab, then comma
		if counts[d] > bestN {
			best, bestN = d, counts[d]
		}
	}
	return best
}

func blank(rec []string) bool {
	for _, f := range rec {
		if strings.TrimSpace(f) != "" {
			return false
		}
	}
	return true
}

var dateLayouts = []string{"2/1/2006", "2006-01-02", "2-1-2006", "2/1/06", "2006/1/2"}

func parseDate(s string) (string, error) {
	s = strings.TrimSpace(s)
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2006-01-02"), nil
		}
	}
	return "", fmt.Errorf("invalid date %q (use dd/mm/yyyy or yyyy-mm-dd)", s)
}
