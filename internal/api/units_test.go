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
			t.Fatalf("tentativa %d deveria passar", i+1)
		}
	}
	if l.allow("a") {
		t.Fatal("a 4ª tentativa dentro da janela deveria ser barrada")
	}
	if !l.allow("b") {
		t.Fatal("outra chave não é afetada")
	}
	time.Sleep(70 * time.Millisecond)
	if !l.allow("a") {
		t.Fatal("depois da janela a chave volta a passar")
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
		{"direto, sem proxy", "203.0.113.9:5555", "", "203.0.113.9"},
		{"proxy local confiável usa o XFF", "127.0.0.1:4000", "198.51.100.7, 10.0.0.1", "198.51.100.7"},
		{"XFF forjado por um cliente externo é ignorado", "203.0.113.9:5555", "1.2.3.4", "203.0.113.9"},
		{"IPv6 loopback também é confiável", "[::1]:4000", "198.51.100.7", "198.51.100.7"},
		{"loopback sem XFF", "127.0.0.1:4000", "", "127.0.0.1"},
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
			t.Errorf("parseRole(%q) deveria falhar", bad)
		}
	}
	if !(roleReader < roleEditor && roleEditor < roleOwner) {
		t.Error("a ordem dos papéis define as permissões: reader < editor < owner")
	}
	if role(0).String() != "" {
		t.Error("papel zerado não tem nome")
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
			t.Errorf("monthRange(%q) deveria falhar", bad)
		}
	}
}

func TestValidDate(t *testing.T) {
	for _, ok := range []string{"2026-10-09", "2028-02-29"} {
		if !validDate(ok) {
			t.Errorf("%q deveria ser válida", ok)
		}
	}
	for _, bad := range []string{"", "2026-02-30", "2027-02-29", "09/10/2026", "2026-10-9", "hoje"} {
		if validDate(bad) {
			t.Errorf("%q deveria ser inválida", bad)
		}
	}
}

func TestFieldErrors(t *testing.T) {
	fe := fieldErrors{}
	if fe.err() != nil {
		t.Fatal("sem erros, err() é nil")
	}
	fe.add("valor", "primeiro")
	fe.add("valor", "segundo") // o primeiro erro de cada campo vence
	fe.add("data", "inválida")
	var ae *apiError
	if !errors.As(fe.err(), &ae) || ae.Status != 400 || ae.Code != "validation" {
		t.Fatalf("err() = %v", fe.err())
	}
	if ae.Fields["valor"] != "primeiro" || ae.Fields["data"] != "inválida" || len(ae.Fields) != 2 {
		t.Errorf("campos: %+v", ae.Fields)
	}
	if ae.Error() == "" || !asAPIError(ae, &ae) {
		t.Error("apiError deve implementar error e ser reconhecível por errors.As")
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
			t.Errorf("pathID(%q) deveria falhar com 404", bad)
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
		t.Fatalf("a soma das partes (%v) deve ser o total", amounts)
	}
	// exato: 3334,33 / 3333,33 / 3333,33; o centavo que sobra vai para quem tem o maior resto
	if !slices.Equal(amounts, []int64{3335, 3333, 3333}) {
		t.Errorf("rateio inesperado: %v", amounts)
	}
	if got[0].UserID != 1 || got[1].ShareBP != 3333 {
		t.Errorf("a ordem e as partes originais devem ser preservadas: %+v", got)
	}
	if len(withAmounts(100, nil)) != 0 {
		t.Error("sem partes, sem rateio")
	}
}
