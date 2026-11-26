package billing

import "testing"

func TestInvoiceDue(t *testing.T) {
	cases := []struct {
		date         string
		closing, due int
		want         string
	}{
		{"2026-11-22", 20, 27, "2026-12-27"},
		{"2026-11-20", 20, 27, "2026-11-27"}, // the closing day itself is in
		{"2026-11-05", 30, 7, "2026-12-07"},
		{"2027-02-15", 31, 10, "2027-03-10"}, // closes on 28/02
		{"2028-02-29", 31, 10, "2028-03-10"}, // leap year: 29/02 is the closing day
		{"2027-03-01", 31, 10, "2027-04-10"},
		{"2026-12-26", 25, 5, "2027-02-05"},
		{"2026-11-10", 15, 15, "2026-12-15"}, // due == closing: next month
	}
	for _, c := range cases {
		got, err := InvoiceDue(c.date, c.closing, c.due)
		if err != nil || got != c.want {
			t.Errorf("InvoiceDue(%s, %d, %d) = %q, %v; want %q", c.date, c.closing, c.due, got, err, c.want)
		}
	}
}

func TestInvoiceDueRejects(t *testing.T) {
	for _, c := range []struct {
		date         string
		closing, due int
	}{{"2026-13-01", 20, 27}, {"2026-11-01", 0, 27}, {"2026-11-01", 20, 32}} {
		if _, err := InvoiceDue(c.date, c.closing, c.due); err == nil {
			t.Errorf("InvoiceDue(%s, %d, %d): want error", c.date, c.closing, c.due)
		}
	}
}

func TestBestPurchaseDay(t *testing.T) {
	if BestPurchaseDay(20) != 21 || BestPurchaseDay(31) != 1 {
		t.Fatal("best purchase day is the day after the closing, 31 wraps to 1")
	}
}
