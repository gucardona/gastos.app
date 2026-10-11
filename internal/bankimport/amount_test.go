package bankimport

import "testing"

func TestParseAmount(t *testing.T) {
	tests := []struct {
		in   string
		want int64
	}{
		{"48.90", 4890},
		{"-48.90", -4890},
		{"+48.90", 4890},
		{"-48,90", -4890},
		{"1.234,56", 123456},
		{"-1.234,56", -123456},
		{"1,234.56", 123456}, // the last separator is the decimal one
		{"12", 1200},
		{"12.5", 1250},
		{"0,05", 5},
		{"1.234", 123400}, // exactly three digits after a lone dot: a thousands group
		{"1.234.567,89", 123456789},
		{"R$ 48,90", 4890},
		{"R$ -48,90", -4890},
		{"-R$ 48,90", -4890},
		{"  7,00  ", 700},
		{"(48,90)", -4890}, // accounting style for negatives
	}
	for _, tc := range tests {
		got, err := ParseAmount(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("ParseAmount(%q) = %d, %v; want %d", tc.in, got, err, tc.want)
		}
	}
}

func TestParseAmountRejects(t *testing.T) {
	for _, in := range []string{"", "abc", "1,2,3", "12,345", "--5", "4 8", "1.2.3", "12.345,6789", "R$"} {
		if got, err := ParseAmount(in); err == nil {
			t.Errorf("ParseAmount(%q) = %d, want an error", in, got)
		}
	}
}

func TestParseAmountDoesNotUseFloats(t *testing.T) {
	// 0.1 + 0.2 style traps: these must come out exact
	for in, want := range map[string]int64{"0.29": 29, "1.15": 115, "4.35": 435, "1234567.89": 123456789} {
		if got, err := ParseAmount(in); err != nil || got != want {
			t.Errorf("ParseAmount(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
}
