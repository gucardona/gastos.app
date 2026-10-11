package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	l := newRateLimiter(3, 50*time.Millisecond)
	for i := 0; i < 3; i++ {
		if !l.allow("a") {
			t.Fatalf("attempt %d should pass", i+1)
		}
	}
	if l.allow("a") {
		t.Fatal("the 4th attempt inside the window should be blocked")
	}
	if !l.allow("b") {
		t.Fatal("another key is not affected")
	}
	time.Sleep(70 * time.Millisecond)
	if !l.allow("a") {
		t.Fatal("after the window the key passes again")
	}
}

func TestClientIP(t *testing.T) {
	req := func(remote, xff string) *http.Request {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = remote
		if xff != "" {
			r.Header.Set("X-Forwarded-For", xff)
		}
		return r
	}
	tests := []struct{ name, remote, xff, want string }{
		{"direct, no proxy", "203.0.113.9:5555", "", "203.0.113.9"},
		{"a trusted local proxy uses XFF", "127.0.0.1:4000", "198.51.100.7, 10.0.0.1", "198.51.100.7"},
		{"XFF forged by an external client is ignored", "203.0.113.9:5555", "1.2.3.4", "203.0.113.9"},
		{"IPv6 loopback is trusted too", "[::1]:4000", "198.51.100.7", "198.51.100.7"},
		{"loopback without XFF", "127.0.0.1:4000", "", "127.0.0.1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := clientIP(req(tc.remote, tc.xff)); got != tc.want {
				t.Errorf("clientIP = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseRoleAndOrdering(t *testing.T) {
	for s, want := range map[string]role{"owner": roleOwner, "editor": roleEditor, "reader": roleReader} {
		got, ok := parseRole(s)
		if !ok || got != want || got.String() != s {
			t.Errorf("parseRole(%q) = %v, %v", s, got, ok)
		}
	}
	for _, bad := range []string{"", "admin", "OWNER"} {
		if _, ok := parseRole(bad); ok {
			t.Errorf("parseRole(%q) should fail", bad)
		}
	}
	if !(roleReader < roleEditor && roleEditor < roleOwner) {
		t.Error("role order defines permissions: reader < editor < owner")
	}
	if role(0).String() != "" {
		t.Error("the zero role has no name")
	}
}

func TestMonthRange(t *testing.T) {
	for _, tc := range []struct{ month, from, to string }{
		{"2026-10", "2026-10-01", "2026-10-31"},
		{"2026-02", "2026-02-01", "2026-02-28"},
		{"2028-02", "2028-02-01", "2028-02-29"},
		{"2026-12", "2026-12-01", "2026-12-31"},
	} {
		from, to, ok := monthRange(tc.month)
		if !ok || from != tc.from || to != tc.to {
			t.Errorf("monthRange(%q) = %q, %q, %v", tc.month, from, to, ok)
		}
	}
	for _, bad := range []string{"", "2026-13", "2026-1", "outubro", "2026-10-01"} {
		if _, _, ok := monthRange(bad); ok {
			t.Errorf("monthRange(%q) should fail", bad)
		}
	}
}

func TestValidDate(t *testing.T) {
	for _, ok := range []string{"2026-10-09", "2028-02-29"} {
		if !validDate(ok) {
			t.Errorf("%q should be valid", ok)
		}
	}
	for _, bad := range []string{"", "2026-02-30", "2027-02-29", "09/10/2026", "2026-10-9", "hoje"} {
		if validDate(bad) {
			t.Errorf("%q should be invalid", bad)
		}
	}
}

func TestFieldErrors(t *testing.T) {
	fe := fieldErrors{}
	if fe.err() != nil {
		t.Fatal("with no errors, err() is nil")
	}
	fe.add("amount", "first")
	fe.add("amount", "second") // the first error of each field wins
	fe.add("date", "invalid")
	var ae *apiError
	if !errors.As(fe.err(), &ae) || ae.Status != 400 || ae.Code != "validation" {
		t.Fatalf("err() = %v", fe.err())
	}
	if ae.Fields["amount"] != "first" || ae.Fields["date"] != "invalid" || len(ae.Fields) != 2 {
		t.Errorf("campos: %+v", ae.Fields)
	}
	if ae.Error() == "" || !asAPIError(ae, &ae) {
		t.Error("apiError must implement error and be recognizable by errors.As")
	}
}

func TestPathID(t *testing.T) {
	for value, want := range map[string]int64{"1": 1, "42": 42} {
		r := httptest.NewRequest("GET", "/", nil)
		r.SetPathValue("id", value)
		if got, err := pathID(r, "id"); err != nil || got != want {
			t.Errorf("pathID(%q) = %d, %v", value, got, err)
		}
	}
	for _, bad := range []string{"", "0", "-3", "abc", "1.5"} {
		r := httptest.NewRequest("GET", "/", nil)
		r.SetPathValue("id", bad)
		if _, err := pathID(r, "id"); err == nil {
			t.Errorf("pathID(%q) should fail with 404", bad)
		}
	}
}

func TestWithAmountsSumsToTotal(t *testing.T) {
	got := withAmounts(10001, []split{{UserID: 1, ShareBP: 3334}, {UserID: 2, ShareBP: 3333}, {UserID: 3, ShareBP: 3333}})
	var sum int64
	var amounts []int64
	for _, s := range got {
		sum += s.AmountCents
		amounts = append(amounts, s.AmountCents)
	}
	if sum != 10001 {
		t.Fatalf("the sum of the parts (%v) must be the total", amounts)
	}
	// exact: 3334.33 / 3333.33 / 3333.33; the leftover cent goes to the largest remainder
	if !slices.Equal(amounts, []int64{3335, 3333, 3333}) {
		t.Errorf("rateio inesperado: %v", amounts)
	}
	if got[0].UserID != 1 || got[1].ShareBP != 3333 {
		t.Errorf("the order and the original shares must be preserved: %+v", got)
	}
	if len(withAmounts(100, nil)) != 0 {
		t.Error("no shares, no allocation")
	}
}
