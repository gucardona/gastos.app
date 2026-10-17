package api

import "testing"

func TestCostMicroBRL(t *testing.T) {
	const rate = 5063100 // R$ 5,0631 per dollar
	for _, tc := range []struct {
		name    string
		model   string
		in, out int
		rate    int64
		want    int64
	}{
		// 1000 in + 1000 out on Haiku = 600000 nano-USD; x 5.0631 = 3037.86 micro-BRL; +5% = 3189.753, rounded up.
		{"haiku", "claude-haiku-5-5", 1000, 1000, rate, 3190},
		{"sonnet", "claude-sonnet-5-5", 1000, 1000, rate, 63796}, // 63795.06
		{"opus", "claude-opus-5-5", 1000, 1000, rate, 127591},    // 127590.12
		{"unknown model costs as opus", "gpt-4", 1000, 1000, rate, 127591},
		{"margin is 5%", "claude-haiku-5-5", 1_000_000, 0, 1_000_000, 105_000}, // $0.10 at R$ 1,00 = 100000 micro-BRL
		{"zero tokens", "claude-opus-5-5", 0, 0, rate, 0},
		{"no overflow at 1e7 tokens", "claude-opus-5-5", 10_000_000, 10_000_000, 100_000_000, 25_200_000_000},
	} {
		if got := costMicroBRL(tc.model, tc.in, tc.out, tc.rate); got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestFormatBRL(t *testing.T) {
	for cents, want := range map[int64]string{0: "R$ 0,00", 5: "R$ 0,05", 1234: "R$ 12,34", 123456: "R$ 1.234,56", 10000000: "R$ 100.000,00", 123456789: "R$ 1.234.567,89"} {
		if got := formatBRL(cents); got != want {
			t.Errorf("formatBRL(%d) = %q, want %q", cents, got, want)
		}
	}
}
