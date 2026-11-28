package auth_test

import (
	"strings"
	"testing"

	"gastos/internal/api/apitest"
	"gastos/internal/api/httpx"
)

const oldPassword = "senha-segura" // what App.Register signs up with

type pwBody = map[string]string

func changePassword(c *apitest.Client, current, next string) int {
	return c.DoRaw("PUT", "/api/me/password", pwBody{"currentPassword": current, "newPassword": next}, nil)
}

func login(a *apitest.App, email, password string) int {
	return a.Anon().DoRaw("POST", "/api/auth/login", pwBody{"email": email, "password": password}, nil)
}

func TestChangePassword(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	if status := changePassword(ana, oldPassword, "outra-senha-boa"); status != 204 {
		t.Fatalf("status %d, want 204", status)
	}
	if status := login(app, "ana@example.com", "outra-senha-boa"); status != 200 {
		t.Fatalf("login with the new password: %d", status)
	}
	if status := login(app, "ana@example.com", oldPassword); status != 401 {
		t.Fatalf("the old password still works: %d", status)
	}
	ana.Do("GET", "/api/me", nil, 200, nil) // the session that changed it stays signed in
}

func TestChangePasswordIsValidated(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	cases := map[string]struct{ current, next, field string }{
		"wrong current":   {"errada-errada", "outra-senha-boa", "currentPassword"},
		"too short":       {oldPassword, "curta", "newPassword"},
		"same as current": {oldPassword, oldPassword, "newPassword"},
		"too long":        {oldPassword, strings.Repeat("x", 73), "newPassword"}, // bcrypt only reads 72 bytes
	}
	for name, c := range cases {
		var res struct {
			Error httpx.APIError `json:"error"`
		}
		status := ana.DoRaw("PUT", "/api/me/password", pwBody{"currentPassword": c.current, "newPassword": c.next}, &res)
		if status != 400 || res.Error.Fields[c.field] == "" {
			t.Errorf("%s: status %d, fields %+v, want 400 on %s", name, status, res.Error.Fields, c.field)
		}
	}
	if status := login(app, "ana@example.com", oldPassword); status != 200 {
		t.Fatalf("a refused change altered the password: %d", status)
	}
	ana.Do("GET", "/api/me", nil, 200, nil) // a wrong current password must not look like an expired session
}

func TestChangePasswordEndsOtherSessions(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com") // session 1
	other := app.Anon()                           // session 2: the same account on another device
	other.Do("POST", "/api/auth/login", pwBody{"email": "ana@example.com", "password": oldPassword}, 200, nil)
	other.Do("GET", "/api/me", nil, 200, nil)

	if status := changePassword(ana, oldPassword, "outra-senha-boa"); status != 204 {
		t.Fatalf("status %d", status)
	}
	if status := other.DoRaw("GET", "/api/me", nil, nil); status != 401 {
		t.Fatalf("the other device is still signed in: %d", status)
	}
	ana.Do("GET", "/api/me", nil, 200, nil)
}

func TestChangePasswordOnlyTouchesYou(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	changePassword(ana, oldPassword, "outra-senha-boa")
	if status := login(app, "bia@example.com", oldPassword); status != 200 {
		t.Fatalf("Bia's password changed: %d", status)
	}
	bia.Do("GET", "/api/me", nil, 200, nil)
}

func TestChangePasswordNeedsASession(t *testing.T) {
	app := apitest.New(t)
	if status := changePassword(app.Anon(), oldPassword, "outra-senha-boa"); status != 401 {
		t.Fatalf("status %d, want 401", status)
	}
}

func TestChangePasswordIsRateLimited(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	for i := 0; i < 10; i++ {
		if status := changePassword(ana, "errada-errada", "outra-senha-boa"); status != 400 {
			t.Fatalf("attempt %d: status %d", i, status)
		}
	}
	if status := changePassword(ana, oldPassword, "outra-senha-boa"); status != 429 {
		t.Fatalf("guessing the current password is not limited: %d", status)
	}
}
