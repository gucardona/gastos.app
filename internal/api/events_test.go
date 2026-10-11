package api

import (
	"bufio"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

// openStream abre o SSE da carteira e entrega cada linha lida no canal.
func openStream(t *testing.T, c *client) <-chan string {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, "GET", c.app.srv.URL+c.wpath("/events"), nil)
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

func TestSSEDeliversOthersChangesOnly(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	shareWallet(ana, bia, "editor")
	cat := firstCategory(t, ana, "expense")
	post := func(c *client) {
		c.do("POST", c.wpath("/transactions"), map[string]any{
			"kind": "expense", "amountCents": 100, "description": "x", "categoryId": cat, "date": "2026-10-01",
		}, 201, nil)
	}

	stream := openStream(t, ana)
	time.Sleep(100 * time.Millisecond) // garante a inscrição no bus

	post(ana) // a própria mudança não gera evento
	select {
	case l := <-stream:
		t.Fatalf("não esperava evento da própria mudança, veio %q", l)
	case <-time.After(300 * time.Millisecond):
	}

	post(bia)
	deadline := time.After(2 * time.Second)
	for {
		select {
		case l := <-stream:
			if l == `data: {"type":"changed"}` {
				return
			}
		case <-deadline:
			t.Fatal("evento não chegou")
		}
	}
}

func TestSSERequiresMembership(t *testing.T) {
	app := newTestApp(t)
	ana := app.register("Ana", "ana@example.com")
	bia := app.register("Bia", "bia@example.com")
	bia.walletID = ana.walletID
	bia.do("GET", bia.wpath("/events"), nil, 404, nil)
}
