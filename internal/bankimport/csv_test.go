package bankimport

import (
	"slices"
	"strings"
	"testing"
)

func parse(t *testing.T, file string, m CSVMapping) []Entry {
	t.Helper()
	got, err := ParseCSV(strings.NewReader(file), m)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func noIDs(entries []Entry) []Entry {
	out := slices.Clone(entries)
	for i := range out {
		out[i].ExternalID = ""
	}
	return out
}

var brazilian = CSVMapping{Date: 1, Description: 2, Amount: 3, HasHeader: true}

func TestParseCSVBrazilianFormat(t *testing.T) {
	file := "Data;Descrição;Valor\n01/10/2026;COMPRA MERCADO;-48,90\n05/10/2026;SALARIO;8.500,00\n"
	got := noIDs(parse(t, file, brazilian))
	want := []Entry{
		{Date: "2026-10-01", AmountCents: -4890, Description: "COMPRA MERCADO"},
		{Date: "2026-10-05", AmountCents: 850000, Description: "SALARIO"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestParseCSVCommaDelimitedWithQuotes(t *testing.T) {
	file := "date,description,amount\n2026-10-01,\"MERCADO, BAIRRO\",-48.90\n2026-10-02,\"CAFE \"\"CENTRAL\"\"\",\"R$ -3,50\"\n"
	got := noIDs(parse(t, file, brazilian))
	want := []Entry{
		{Date: "2026-10-01", AmountCents: -4890, Description: "MERCADO, BAIRRO"},
		{Date: "2026-10-02", AmountCents: -350, Description: `CAFE "CENTRAL"`},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestParseCSVTabDelimitedWithoutHeader(t *testing.T) {
	m := CSVMapping{Date: 1, Description: 2, Amount: 3}
	got := noIDs(parse(t, "01/10/2026\tPADARIA\t-12,50\n", m))
	if len(got) != 1 || got[0].AmountCents != -1250 || got[0].Description != "PADARIA" {
		t.Fatalf("got %+v", got)
	}
}

func TestParseCSVColumnsInAnyOrder(t *testing.T) {
	m := CSVMapping{Amount: 1, Date: 3, Description: 2, HasHeader: true}
	got := noIDs(parse(t, "valor;historico;data\n-9,99;STREAMING;03/10/2026\n", m))
	if len(got) != 1 || got[0].Date != "2026-10-03" || got[0].AmountCents != -999 || got[0].Description != "STREAMING" {
		t.Fatalf("got %+v", got)
	}
}

func TestParseCSVInvertSignForCardExports(t *testing.T) {
	// many card exports list purchases as positive numbers
	m := CSVMapping{Date: 1, Description: 2, Amount: 3, HasHeader: true, InvertSign: true}
	got := noIDs(parse(t, "d;h;v\n01/10/2026;LOJA;129,90\n02/10/2026;ESTORNO;-30,00\n", m))
	if got[0].AmountCents != -12990 || got[1].AmountCents != 3000 {
		t.Fatalf("got %+v", got)
	}
}

func TestParseCSVDateFormats(t *testing.T) {
	for in, want := range map[string]string{
		"01/10/2026": "2026-10-01", "1/10/2026": "2026-10-01", "2026-10-01": "2026-10-01",
		"01-10-2026": "2026-10-01", "01/10/26": "2026-10-01", "2026/10/01": "2026-10-01",
	} {
		got := parse(t, in+";X;-1,00\n", CSVMapping{Date: 1, Description: 2, Amount: 3})
		if got[0].Date != want {
			t.Errorf("date %q parsed as %q, want %q", in, got[0].Date, want)
		}
	}
}

func TestParseCSVDecodesLatin1(t *testing.T) {
	file := "data;historico;valor\n01/10/2026;A\xc7A\xcd S\xc3O JO\xc3O;-10,00\n"
	got := parse(t, file, brazilian)
	if got[0].Description != "AÇAÍ SÃO JOÃO" {
		t.Fatalf("got %q", got[0].Description)
	}
}

func TestParseCSVSkipsBlankAndZeroRows(t *testing.T) {
	file := "d;h;v\n\n01/10/2026;TARIFA ISENTA;0,00\n;;\n02/10/2026;CAFE;-5,00\n\n"
	got := noIDs(parse(t, file, brazilian))
	if len(got) != 1 || got[0].Description != "CAFE" {
		t.Fatalf("got %+v", got)
	}
}

func TestParseCSVIDsAreStableAndDistinctForRepeatedRows(t *testing.T) {
	file := "d;h;v\n01/10/2026;CAFE;-5,00\n01/10/2026;CAFE;-5,00\n02/10/2026;CAFE;-5,00\n"
	first := parse(t, file, brazilian)
	again := parse(t, file, brazilian)
	ids := map[string]bool{}
	for i, e := range first {
		if e.ExternalID == "" || ids[e.ExternalID] {
			t.Fatalf("row %d: id %q is empty or repeated", i+1, e.ExternalID)
		}
		ids[e.ExternalID] = true
		if e.ExternalID != again[i].ExternalID {
			t.Fatalf("row %d: id changed between parses of the same file", i+1)
		}
	}
}

func TestParseCSVIDDoesNotDependOnRowOrderOfOtherDays(t *testing.T) {
	// re-importing next month's overlapping export must recognize yesterday's rows even though the file now starts later
	a := parse(t, "d;h;v\n01/10/2026;CAFE;-5,00\n02/10/2026;PAO;-3,00\n", brazilian)
	b := parse(t, "d;h;v\n02/10/2026;PAO;-3,00\n", brazilian)
	if a[1].ExternalID != b[0].ExternalID {
		t.Fatalf("the same line got different ids in two exports: %q vs %q", a[1].ExternalID, b[0].ExternalID)
	}
}

func TestParseCSVErrors(t *testing.T) {
	cases := map[string]struct {
		file string
		m    CSVMapping
	}{
		"empty file":                   {"", brazilian},
		"only a header":                {"d;h;v\n", brazilian},
		"a column beyond the file":     {"d;h\n01/10/2026;X\n", brazilian},
		"an invalid date":              {"d;h;v\n32/10/2026;X;-1,00\n", brazilian},
		"an unreadable amount":         {"d;h;v\n01/10/2026;X;abc\n", brazilian},
		"column zero":                  {"d;h;v\n01/10/2026;X;-1,00\n", CSVMapping{Date: 0, Description: 2, Amount: 3}},
		"two roles on the same column": {"d;h;v\n01/10/2026;X;-1,00\n", CSVMapping{Date: 1, Description: 1, Amount: 3}},
	}
	for name, tc := range cases {
		if got, err := ParseCSV(strings.NewReader(tc.file), tc.m); err == nil {
			t.Errorf("%s: got %+v, want an error", name, got)
		}
	}
}

func TestParseCSVErrorsNameTheLine(t *testing.T) {
	file := "d;h;v\n01/10/2026;OK;-1,00\n02/10/2026;BAD;nope\n"
	_, err := ParseCSV(strings.NewReader(file), brazilian)
	if err == nil || !strings.Contains(err.Error(), "line 3") {
		t.Fatalf("the error should point at line 3, got %v", err)
	}
}

func TestParseCSVRejectsAHugeFile(t *testing.T) {
	huge := "d;h;v\n" + strings.Repeat("01/10/2026;X;-1,00\n", MaxFileBytes/10)
	if _, err := ParseCSV(strings.NewReader(huge), brazilian); err != ErrTooLarge {
		t.Fatalf("got %v, want ErrTooLarge", err)
	}
}
