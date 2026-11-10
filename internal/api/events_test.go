package api

import (
	"bufio"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

const changedLine = `data: {"type":"changed"}`

// openStream opens the user's SSE stream and delivers every line read on the channel.
// The server subscribes before sending the headers, so the stream is live once Do returns.
func openStream(t *testing.T, c *client) <-chan string {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, "GET", c.app.srv.URL+"/api/events", nil)
	res, err := c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("status %d, content-type %q", res.StatusCode, res.Header.Get("Content-Type"))
	}
	lines := make(chan string, 16)
	go func() {
		defer res.Body.Close()
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	return lines
}

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

func postExpense(t *testing.T, c *client) {
	t.Helper()
	c.do("POST", c.wpath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 100, "description": "x", "categoryId": firstCategory(t, c, "expense"), "date": "2026-10-01",
	}, 201, nil)
}

func TestSSEReachesTheSameUsersOtherTabs(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	tab1, tab2 := openStream(t, ana), openStream(t, ana)

	postExpense(t, ana)
	expectChanged(t, tab1, "the tab that made the change")
	expectChanged(t, tab2, "Ana's other tab")
}

func TestSSEReachesTheWalletMembersOnly(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	caio := app.register("Caio", "caio@example.com")
	shareWallet(ana, bia, "editor")
	biaStream, caioStream := openStream(t, bia), openStream(t, caio)

	postExpense(t, ana)
	expectChanged(t, biaStream, "Bia, a member")
	expectQuiet(t, caioStream, "Caio, outside the wallet")
}

func TestSSECreatingAWalletReachesTheCreatorsOtherTabs(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	anaStream, biaStream := openStream(t, ana), openStream(t, bia)

	ana.do("POST", "/api/wallets", map[string]any{"name": "Viagem"}, 201, nil)
	expectChanged(t, anaStream, "Ana")
	expectQuiet(t, biaStream, "Bia")
}

func TestSSEDeletingAWalletReachesItsFormerMembers(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	shareWallet(ana, bia, "reader")
	anaStream, biaStream := openStream(t, ana), openStream(t, bia)

	ana.do("DELETE", ana.wpath(""), nil, 204, nil)
	expectChanged(t, anaStream, "Ana")
	expectChanged(t, biaStream, "Bia")
}

func TestSSERenamingAWalletReachesItsMembers(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	shareWallet(ana, bia, "reader")
	biaStream := openStream(t, bia)

	ana.do("PATCH", ana.wpath(""), map[string]any{"name": "Casa"}, 200, nil)
	expectChanged(t, biaStream, "Bia")
}

func TestSSEAddingAMemberReachesTheNewMember(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	biaStream := openStream(t, bia)

	shareWallet(ana, bia, "editor")
	expectChanged(t, biaStream, "Bia, just added")
}

func TestSSEChangingAMemberReachesThatMember(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	shareWallet(ana, bia, "editor")
	biaStream := openStream(t, bia)

	ana.do("PATCH", ana.wpath("/members/%d", bia.user.ID), map[string]any{"role": "reader"}, 200, nil)
	expectChanged(t, biaStream, "Bia, whose role changed")
}

func TestSSERemovingAMemberReachesTheRemovedUser(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	shareWallet(ana, bia, "editor")
	anaStream, biaStream := openStream(t, ana), openStream(t, bia)

	ana.do("DELETE", ana.wpath("/members/%d", bia.user.ID), nil, 204, nil)
	expectChanged(t, biaStream, "Bia, just removed")
	expectChanged(t, anaStream, "Ana, the owner")
}

func TestSSERenamingYourselfReachesYourOtherTabsAndCoMembers(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	caio := app.register("Caio", "caio@example.com")
	shareWallet(ana, bia, "editor")
	anaStream, biaStream, caioStream := openStream(t, ana), openStream(t, bia), openStream(t, caio)

	ana.do("PATCH", "/api/me", map[string]string{"name": "Ana Maria"}, 200, nil)
	expectChanged(t, anaStream, "Ana's other tab")
	expectChanged(t, biaStream, "Bia, who shares a wallet")
	expectQuiet(t, caioStream, "Caio, who shares nothing")
}

func TestSSENeedsASession(t *testing.T) {
	app := newTestApp(t)
	if status := app.anon().status("GET", "/api/events", nil); status != 401 {
		t.Fatalf("status %d, want 401", status)
	}
}

func TestSSEOldPerWalletRouteIsGone(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	ana.do("GET", ana.wpath("/events"), nil, 404, nil)
}

func TestSSEAssistantSettingsReachTheUsersOtherTabs(t *testing.T) {
	app, _ := newAIApp(t, true)
	ana := app.register("Ana", "ana@example.com")
	stream := openStream(t, ana)

	ana.do("PUT", "/api/me/ai", map[string]any{"apiKey": aiTestKey, "enabled": true}, 200, nil)
	expectChanged(t, stream, "Ana's other tab, after saving the key")
	ana.do("DELETE", "/api/me/ai/key", nil, 204, nil)
	expectChanged(t, stream, "Ana's other tab, after removing the key")
}

// The change is saved before the notice goes out: a client that hangs up right after must not keep the others in the dark.
func TestNotifyWalletSurvivesACancelledRequest(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	shareWallet(ana, bia, "editor")
	ch := app.s.bus.Subscribe(bia.user.ID)
	defer app.s.bus.Unsubscribe(bia.user.ID, ch)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	app.s.notifyWallet(ctx, ana.walletID)
	select {
	case <-ch:
	default:
		t.Fatal("Bia was not notified")
	}
}
