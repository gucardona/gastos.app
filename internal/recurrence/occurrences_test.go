package recurrence

import (
	"slices"
	"testing"
)

func dates(occ []Occurrence) []string {
	var out []string
	for _, o := range occ {
		out = append(out, o.Date)
	}
	return out
}

func TestOccurrences(t *testing.T) {
	tests := []struct {
		name              string
		day               int
		start, end, until string
		want              []string
	}{
		{"simple monthly", 10, "2026-01-01", "", "2026-03-15",
			[]string{"2026-01-10", "2026-02-10", "2026-03-10"}},
		{"stops before the day in the current month", 20, "2026-01-01", "", "2026-03-15",
			[]string{"2026-01-20", "2026-02-20"}},
		{"a start after the day skips the first month", 5, "2026-01-10", "", "2026-03-31",
			[]string{"2026-02-05", "2026-03-05"}},
		{"day 31 clamped to the end of the month", 31, "2026-01-01", "", "2026-04-30",
			[]string{"2026-01-31", "2026-02-28", "2026-03-31", "2026-04-30"}},
		{"leap year", 30, "2028-02-01", "", "2028-02-29",
			[]string{"2028-02-29"}},
		{"inclusive end", 10, "2026-01-01", "2026-02-10", "2026-12-31",
			[]string{"2026-01-10", "2026-02-10"}},
		{"until before the start", 10, "2026-05-01", "", "2026-04-30", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Occurrences(tc.day, tc.start, tc.end, tc.until)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(dates(got), tc.want) {
				t.Errorf("got %v, want %v", dates(got), tc.want)
			}
			for _, o := range got {
				if o.Month != o.Date[:7] {
					t.Errorf("Month %q does not match Date %q", o.Month, o.Date)
				}
			}
		})
	}
}

func TestOccurrencesInvalid(t *testing.T) {
	if _, err := Occurrences(10, "2026-13-01", "", "2026-12-31"); err == nil {
		t.Error("an invalid start should fail")
	}
	if _, err := Occurrences(0, "2026-01-01", "", "2026-12-31"); err == nil {
		t.Error("day 0 should fail")
	}
}
