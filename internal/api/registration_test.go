package api

import (
	"net/http"
	"sync"
	"testing"

	"gastos/internal/config"
)

const testInvite = "test-invite-code"

func TestRegistrationMode(t *testing.T) {
	tests := []struct {
		name  string
		users int
		code  string
		want  registrationMode
	}{
		{"empty database is open, even with a code configured", 0, testInvite, modeOpen},
		{"empty database with no code is open", 0, "", modeOpen},
		{"users exist and a code is configured", 1, testInvite, modeCode},
		{"users exist and there is no code", 1, "", modeClosed},
		{"many users and a code", 40, testInvite, modeCode},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := modeFor(tc.users, tc.code); got != tc.want {
				t.Errorf("modeFor(%d, %q) = %q, want %q", tc.users, tc.code, got, tc.want)
			}
		})
	}
}

// signUp posts a registration and returns the status; it does not keep the session.
func signUp(app *testApp, email, code string) (int, apiError) {
	app.t.Helper()
	var res struct {
		Error apiError `json:"error"`
	}
	c := app.anon()
	body := map[string]any{"name": "Pessoa", "email": email, "password": "senha-segura"}
	if code != "" {
		body["inviteCode"] = code
	}
	status := c.doRaw("POST", "/api/auth/register", body, &res)
	return status, res.Error
}

func registrationModeOf(t *testing.T, app *testApp) string {
	t.Helper()
	var res struct {
		Mode string `json:"mode"`
	}
	app.anon().do("GET", "/api/auth/registration", nil, 200, &res)
	return res.Mode
}

func TestRegistrationEndpointReportsTheMode(t *testing.T) {
	app := newTestApp(t) // a code is configured
	if m := registrationModeOf(t, app); m != "open" {
		t.Fatalf("fresh database: mode %q, want open", m)
	}
	app.register("Ana", "ana@example.com")
	if m := registrationModeOf(t, app); m != "code" {
		t.Fatalf("after the first user: mode %q, want code", m)
	}

	noCode := newTestAppWith(t, func(c *config.Config) { c.InviteCode = "" })
	noCode.register("Ana", "ana@example.com")
	if m := registrationModeOf(t, noCode); m != "closed" {
		t.Fatalf("no code configured: mode %q, want closed", m)
	}
}

func TestFirstAccountNeedsNoCode(t *testing.T) {
	app := newTestApp(t)
	if status, _ := signUp(app, "ana@example.com", "a-wrong-code-is-ignored"); status != http.StatusCreated {
		t.Fatalf("first account: status %d, want 201", status)
	}
}

func TestLaterAccountsNeedTheInviteCode(t *testing.T) {
	app := newTestApp(t)
	app.register("Ana", "ana@example.com")

	missing, missingErr := signUp(app, "bia@example.com", "")
	wrong, wrongErr := signUp(app, "bia@example.com", "not-the-right-code")
	if missing != http.StatusForbidden || wrong != http.StatusForbidden {
		t.Fatalf("missing/wrong code: %d/%d, want 403/403", missing, wrong)
	}
	if missingErr.Message != wrongErr.Message || missingErr.Code != "forbidden" {
		t.Errorf("a wrong and a missing code must look the same: %+v vs %+v", missingErr, wrongErr)
	}
	if status, _ := signUp(app, "bia@example.com", testInvite); status != http.StatusCreated {
		t.Fatalf("right code: status %d, want 201", status)
	}
	// the rest of the validation still runs behind the gate
	if status, _ := signUp(app, "bia@example.com", testInvite); status != http.StatusConflict {
		t.Fatalf("duplicate email with the right code: status %d, want 409", status)
	}
}

func TestSignupIsClosedWithoutACode(t *testing.T) {
	app := newTestAppWith(t, func(c *config.Config) { c.InviteCode = "" })
	app.register("Ana", "ana@example.com")
	if status, _ := signUp(app, "bia@example.com", "anything-at-all"); status != http.StatusForbidden {
		t.Fatalf("status %d, want 403", status)
	}
}

func TestOnlyOneFirstAccountWhenTwoSignUpAtOnce(t *testing.T) {
	app := newTestAppWith(t, func(c *config.Config) { c.InviteCode = "" })
	statuses := make([]int, 2)
	var wg sync.WaitGroup
	for i := range statuses {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses[i], _ = signUp(app, []string{"a@example.com", "b@example.com"}[i], "")
		}()
	}
	wg.Wait()
	created, forbidden := 0, 0
	for _, s := range statuses {
		switch s {
		case http.StatusCreated:
			created++
		case http.StatusForbidden:
			forbidden++
		}
	}
	if created != 1 || forbidden != 1 {
		t.Fatalf("statuses %v: want exactly one 201 and one 403", statuses)
	}
	if n := countRows(t, app, `SELECT COUNT(*) FROM users`); n != 1 {
		t.Fatalf("%d users, want 1", n)
	}
}

func TestSignupAttemptsAreRateLimited(t *testing.T) {
	app := newTestApp(t)
	app.register("Ana", "ana@example.com")
	// Ana's own sign-up counted as the first of the 10 allowed attempts
	for i := 0; i < 9; i++ {
		if status, _ := signUp(app, "x@example.com", "wrong-code-12345"); status != http.StatusForbidden {
			t.Fatalf("attempt %d: status %d, want 403", i+1, status)
		}
	}
	if status, _ := signUp(app, "x@example.com", testInvite); status != http.StatusTooManyRequests {
		t.Fatalf("10th failed attempt: status %d, want 429 even with the right code", status)
	}
}
