package recurrence

import (
	"fmt"
	"time"
)

const dateLayout = "2006-01-02"

type Occurrence struct {
	Month string // YYYY-MM
	Date  string // YYYY-MM-DD
}

// Occurrences lists a recurrence's monthly dates between start and until (inclusive),
// honoring end (inclusive, empty = no end). The day is clamped to the last day of each month.
func Occurrences(day int, start, end, until string) ([]Occurrence, error) {
	if day < 1 || day > 31 {
		return nil, fmt.Errorf("invalid day of month: %d", day)
	}
	s, err := time.Parse(dateLayout, start)
	if err != nil {
		return nil, fmt.Errorf("start: %w", err)
	}
	u, err := time.Parse(dateLayout, until)
	if err != nil {
		return nil, fmt.Errorf("until: %w", err)
	}
	if end != "" {
		e, err := time.Parse(dateLayout, end)
		if err != nil {
			return nil, fmt.Errorf("end: %w", err)
		}
		if e.Before(u) {
			u = e
		}
	}

	var out []Occurrence
	for m := time.Date(s.Year(), s.Month(), 1, 0, 0, 0, 0, time.UTC); !m.After(u); m = m.AddDate(0, 1, 0) {
		last := time.Date(m.Year(), m.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
		date := time.Date(m.Year(), m.Month(), min(day, last), 0, 0, 0, 0, time.UTC)
		if date.Before(s) || date.After(u) {
			continue
		}
		out = append(out, Occurrence{Month: date.Format("2006-01"), Date: date.Format(dateLayout)})
	}
	return out, nil
}
