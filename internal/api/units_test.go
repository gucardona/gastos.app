package api

import (
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"gastos/internal/api/httpx"
)

func TestRateLimiter(t *testing.T) {
	l := httpx.NewRateLimiter(3, 50*time.Millisecond)
	for i := 0; i < 3; i++ {
		if !l.Allow("a") {
			t.Fatalf("attempt %d should pass", i+1)
		}
	}
	if l.Allow("a") {
		t.Fatal("the 4th attempt inside the window should be blocked")
	}
	if !l.Allow("b") {
		t.Fatal("another key is not affected")
	}
	time.Sleep(70 * time.Millisecond)
	if !l.Allow("a") {
		t.Fatal("after the window the key passes again")
	}
}

func TestParseRoleAndOrdering(t *testing.T) {
	for s, want := range map[string]httpx.Role{"owner": httpx.RoleOwner, "editor": httpx.RoleEditor, "reader": httpx.RoleReader} {
		got, ok := httpx.ParseRole(s)
		if !ok || got != want || got.String() != s {
			t.Errorf("parseRole(%q) = %v, %v", s, got, ok)
		}
	}
	for _, bad := range []string{"", "admin", "OWNER"} {
		if _, ok := httpx.ParseRole(bad); ok {
			t.Errorf("parseRole(%q) should fail", bad)
		}
	}
	//nolint:staticcheck // moved unchanged, see docs/security.md
	if !(httpx.RoleReader < httpx.RoleEditor && httpx.RoleEditor < httpx.RoleOwner) {
		t.Error("role order defines permissions: reader < editor < owner")
	}
	if httpx.Role(0).String() != "" {
		t.Error("the zero role has no name")
	}
}

func TestValidDate(t *testing.T) {
	for _, ok := range []string{"2026-10-09", "2028-02-29"} {
		if !httpx.ValidDate(ok) {
			t.Errorf("%q should be valid", ok)
		}
	}
	for _, bad := range []string{"", "2026-02-30", "2027-02-29", "09/10/2026", "2026-10-9", "hoje"} {
		if httpx.ValidDate(bad) {
			t.Errorf("%q should be invalid", bad)
		}
	}
}

func TestFieldErrors(t *testing.T) {
	fe := httpx.FieldErrors{}
	if fe.Err() != nil {
		t.Fatal("with no errors, err() is nil")
	}
	fe.Add("amount", "first")
	fe.Add("amount", "second") // the first error of each field wins
	fe.Add("date", "invalid")
	var ae *httpx.APIError
	if !errors.As(fe.Err(), &ae) || ae.Status != 400 || ae.Code != "validation" {
		t.Fatalf("err() = %v", fe.Err())
	}
	if ae.Fields["amount"] != "first" || ae.Fields["date"] != "invalid" || len(ae.Fields) != 2 {
		t.Errorf("fields: %+v", ae.Fields)
	}
	if ae.Error() == "" || !httpx.AsAPIError(ae, &ae) {
		t.Error("apiError must implement error and be recognizable by errors.As")
	}
}

func TestPathID(t *testing.T) {
	for value, want := range map[string]int64{"1": 1, "42": 42} {
		r := httptest.NewRequest("GET", "/", nil)
		r.SetPathValue("id", value)
		if got, err := httpx.PathID(r, "id"); err != nil || got != want {
			t.Errorf("pathID(%q) = %d, %v", value, got, err)
		}
	}
	for _, bad := range []string{"", "0", "-3", "abc", "1.5"} {
		r := httptest.NewRequest("GET", "/", nil)
		r.SetPathValue("id", bad)
		if _, err := httpx.PathID(r, "id"); err == nil {
			t.Errorf("pathID(%q) should fail with 404", bad)
		}
	}
}
