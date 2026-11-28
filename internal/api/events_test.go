package api_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gastos/internal/api/apitest"
	"gastos/internal/config"
)

const changedLine = `data: {"type":"changed"}`

// expectChanged waits for a change notice on the stream.
func expectChanged(t *testing.T, stream <-chan string, who string) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case l := <-stream:
			if l == changedLine {
				return
			}
		case <-deadline:
			t.Fatalf("%s was not notified", who)
		}
	}
}

// expectQuiet fails if a change notice arrives on the stream within a short while.
func expectQuiet(t *testing.T, stream <-chan string, who string) {
	t.Helper()
	deadline := time.After(300 * time.Millisecond)
	for {
		select {
		case l := <-stream:
			if l == changedLine {
				t.Fatalf("%s should not have been notified", who)
			}
		case <-deadline:
			return
		}
	}
}

func postExpense(t *testing.T, c *apitest.Client) {
	t.Helper()
	c.Do("POST", c.WPath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 100, "description": "x", "categoryId": apitest.FirstCategory(t, c, "expense"), "date": "2026-10-01",
	}, 201, nil)
}

func TestSSEReachesTheSameUsersOtherTabs(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	tab1, tab2 := apitest.OpenStream(t, ana), apitest.OpenStream(t, ana)

	postExpense(t, ana)
	expectChanged(t, tab1, "the tab that made the change")
	expectChanged(t, tab2, "Ana's other tab")
}

func TestSSEReachesTheWalletMembersOnly(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	caio := app.Register("Caio", "caio@example.com")
	apitest.ShareWallet(ana, bia, "editor")
	biaStream, caioStream := apitest.OpenStream(t, bia), apitest.OpenStream(t, caio)

	postExpense(t, ana)
	expectChanged(t, biaStream, "Bia, a member")
	expectQuiet(t, caioStream, "Caio, outside the wallet")
}

func TestSSECreatingAWalletReachesTheCreatorsOtherTabs(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	anaStream, biaStream := apitest.OpenStream(t, ana), apitest.OpenStream(t, bia)

	ana.Do("POST", "/api/wallets", map[string]any{"name": "Viagem"}, 201, nil)
	expectChanged(t, anaStream, "Ana")
	expectQuiet(t, biaStream, "Bia")
}

func TestSSEDeletingAWalletReachesItsFormerMembers(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	apitest.ShareWallet(ana, bia, "reader")
	anaStream, biaStream := apitest.OpenStream(t, ana), apitest.OpenStream(t, bia)

	ana.Do("DELETE", ana.WPath(""), nil, 204, nil)
	expectChanged(t, anaStream, "Ana")
	expectChanged(t, biaStream, "Bia")
}

func TestSSERenamingAWalletReachesItsMembers(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	apitest.ShareWallet(ana, bia, "reader")
	biaStream := apitest.OpenStream(t, bia)

	ana.Do("PATCH", ana.WPath(""), map[string]any{"name": "Casa"}, 200, nil)
	expectChanged(t, biaStream, "Bia")
}

func TestSSEAddingAMemberReachesTheNewMember(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	biaStream := apitest.OpenStream(t, bia)

	apitest.ShareWallet(ana, bia, "editor")
	expectChanged(t, biaStream, "Bia, just added")
}

func TestSSEChangingAMemberReachesThatMember(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	apitest.ShareWallet(ana, bia, "editor")
	biaStream := apitest.OpenStream(t, bia)

	ana.Do("PATCH", ana.WPath("/members/%d", bia.User.ID), map[string]any{"role": "reader"}, 200, nil)
	expectChanged(t, biaStream, "Bia, whose role changed")
}

func TestSSERemovingAMemberReachesTheRemovedUser(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	apitest.ShareWallet(ana, bia, "editor")
	anaStream, biaStream := apitest.OpenStream(t, ana), apitest.OpenStream(t, bia)

	ana.Do("DELETE", ana.WPath("/members/%d", bia.User.ID), nil, 204, nil)
	expectChanged(t, biaStream, "Bia, just removed")
	expectChanged(t, anaStream, "Ana, the owner")
}

func TestSSERenamingYourselfReachesYourOtherTabsAndCoMembers(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	caio := app.Register("Caio", "caio@example.com")
	apitest.ShareWallet(ana, bia, "editor")
	anaStream, biaStream, caioStream := apitest.OpenStream(t, ana), apitest.OpenStream(t, bia), apitest.OpenStream(t, caio)

	ana.Do("PATCH", "/api/me", map[string]string{"name": "Ana Maria"}, 200, nil)
	expectChanged(t, anaStream, "Ana's other tab")
	expectChanged(t, biaStream, "Bia, who shares a wallet")
	expectQuiet(t, caioStream, "Caio, who shares nothing")
}

func TestSSENeedsASession(t *testing.T) {
	app := apitest.New(t)
	if status := app.Anon().Status("GET", "/api/events", nil); status != 401 {
		t.Fatalf("status %d, want 401", status)
	}
}

func TestSSEOldPerWalletRouteIsGone(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	ana.Do("GET", ana.WPath("/events"), nil, 404, nil)
}

func TestSSEAssistantSettingsReachTheUsersOtherTabs(t *testing.T) {
	// The settings endpoint checks the key against Anthropic, so a stub accepts any key.
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"message_stop\"}\n\n") //nolint:errcheck,gosec // test stub
	}))
	t.Cleanup(fake.Close)
	app := apitest.NewWith(t, func(c *config.Config) {
		c.AnthropicURL = fake.URL
		c.SecretKey = bytes.Repeat([]byte{7}, 32)
	})
	const key = "sk-ant-test-key-1234"
	ana := app.Register("Ana", "ana@example.com")
	stream := apitest.OpenStream(t, ana)

	ana.Do("PUT", "/api/me/ai", map[string]any{"apiKey": key, "enabled": true}, 200, nil)
	expectChanged(t, stream, "Ana's other tab, after saving the key")
	ana.Do("DELETE", "/api/me/ai/key", nil, 204, nil)
	expectChanged(t, stream, "Ana's other tab, after removing the key")
}

// The change is saved before the notice goes out: a client that hangs up right after must not keep the others in the dark.
func TestNotifyWalletSurvivesACancelledRequest(t *testing.T) {
	app := apitest.New(t)
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	apitest.ShareWallet(ana, bia, "editor")
	ch := app.Server().Bus.Subscribe(bia.User.ID)
	defer app.Server().Bus.Unsubscribe(bia.User.ID, ch)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	app.Server().NotifyWallet(ctx, ana.WalletID)
	select {
	case <-ch:
	default:
		t.Fatal("Bia was not notified")
	}
}
