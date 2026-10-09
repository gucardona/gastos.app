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
		{"mensal simples", 10, "2026-01-01", "", "2026-03-15",
			[]string{"2026-01-10", "2026-02-10", "2026-03-10"}},
		{"até antes do dia no mês corrente", 20, "2026-01-01", "", "2026-03-15",
			[]string{"2026-01-20", "2026-02-20"}},
		{"início depois do dia pula o 1º mês", 5, "2026-01-10", "", "2026-03-31",
			[]string{"2026-02-05", "2026-03-05"}},
		{"dia 31 limitado ao fim do mês", 31, "2026-01-01", "", "2026-04-30",
			[]string{"2026-01-31", "2026-02-28", "2026-03-31", "2026-04-30"}},
		{"ano bissexto", 30, "2028-02-01", "", "2028-02-29",
			[]string{"2028-02-29"}},
		{"fim inclusivo", 10, "2026-01-01", "2026-02-10", "2026-12-31",
			[]string{"2026-01-10", "2026-02-10"}},
		{"until antes do início", 10, "2026-05-01", "", "2026-04-30", nil},
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
					t.Errorf("Month %q não bate com Date %q", o.Month, o.Date)
				}
			}
		})
	}
}

func TestOccurrencesInvalid(t *testing.T) {
	if _, err := Occurrences(10, "2026-13-01", "", "2026-12-31"); err == nil {
		t.Error("start inválido deveria falhar")
	}
	if _, err := Occurrences(0, "2026-01-01", "", "2026-12-31"); err == nil {
		t.Error("dia 0 deveria falhar")
	}
}
