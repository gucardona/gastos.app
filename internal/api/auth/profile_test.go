package auth_test

import (
	"strings"
	"testing"
	"time"

	"gastos/internal/api/apitest"
)

type meResponse struct {
	User struct {
		ID    int64  `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"user"`
}

func TestRenameYourself(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")

	var out struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	ana.Do("PATCH", "/api/me", map[string]string{"name": "  Aninha  "}, 200, &out)
	if out.Name != "Aninha" || out.Email != "ana@example.com" {
		t.Fatalf("response: %+v", out)
	}
	var me meResponse
	ana.Do("GET", "/api/me", nil, 200, &me)
	if me.User.Name != "Aninha" {
		t.Fatalf("GET /api/me after the rename: %q", me.User.Name)
	}
}

func TestRenameIsValidated(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	for name, in := range map[string]string{"empty": "", "only spaces": "   ", "too long": strings.Repeat("x", 61)} {
		if status := ana.Status("PATCH", "/api/me", map[string]string{"name": in}); status != 400 {
			t.Errorf("%s: status %d, want 400", name, status)
		}
	}
	var me meResponse
	ana.Do("GET", "/api/me", nil, 200, &me)
	if me.User.Name != "Ana" {
		t.Fatalf("a refused rename changed the name to %q", me.User.Name)
	}
	// the longest name that is allowed
	ana.Do("PATCH", "/api/me", map[string]string{"name": strings.Repeat("é", 60)}, 200, nil)
}

func TestRenameNeedsASession(t *testing.T) {
	app := apitest.New(t)
	if status := app.Anon().Status("PATCH", "/api/me", map[string]string{"name": "x"}); status != 401 {
		t.Fatalf("status %d, want 401", status)
	}
}

func TestRenameOnlyChangesYourself(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	ana.Do("PATCH", "/api/me", map[string]string{"name": "Ana Maria"}, 200, nil)
	var me meResponse
	bia.Do("GET", "/api/me", nil, 200, &me)
	if me.User.Name != "Bia" {
		t.Fatalf("Bia is now %q", me.User.Name)
	}
}

func TestRenameShowsUpForOtherMembers(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	apitest.ShareWallet(ana, bia, "editor")
	stream := apitest.OpenStream(t, bia)

	ana.Do("PATCH", "/api/me", map[string]string{"name": "Ana Maria"}, 200, nil)

	var members []struct {
		Name string `json:"name"`
	}
	bia.Do("GET", bia.WPath("/members"), nil, 200, &members)
	seen := false
	for _, m := range members {
		seen = seen || m.Name == "Ana Maria"
	}
	if !seen {
		t.Fatalf("the members list still shows the old name: %+v", members)
	}
	deadline := time.After(2 * time.Second)
	for {
		select {
		case l := <-stream:
			if l == `data: {"type":"changed"}` {
				return // Bia's screen is told to reload
			}
		case <-deadline:
			t.Fatal("the other member was not notified")
		}
	}
}
