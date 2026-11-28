package assistant_test

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gastos/internal/anthropic"
	"gastos/internal/api/apitest"
	"gastos/internal/api/assistant"
	"gastos/internal/api/httpx"
	"gastos/internal/api/wallet"
	"gastos/internal/config"
)

type anthropicReq struct {
	Model     string              `json:"model"`
	System    string              `json:"system"`
	MaxTokens int                 `json:"max_tokens"`
	Messages  []anthropic.Message `json:"messages"`
	Tools     []anthropic.Tool    `json:"tools"`
}

// fakeAnthropic answers each chat call with script(call index, request); the key check from PUT /api/me/ai is answered apart.
type fakeAnthropic struct {
	mu   sync.Mutex
	reqs []anthropicReq
}

func (f *fakeAnthropic) requests() []anthropicReq {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]anthropicReq(nil), f.reqs...)
}

func sseFrames(frames ...string) string {
	var b strings.Builder
	for _, f := range frames {
		b.WriteString("data: " + f + "\n\n")
	}
	return b.String()
}

func jsonStr(s string) string { b, _ := json.Marshal(s); return string(b) }

func sseText(text string) string {
	return sseFrames(
		`{"type":"message_start","message":{"usage":{"input_tokens":10}}}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"text"}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":`+jsonStr(text)+`}}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}`,
		`{"type":"message_stop"}`,
	)
}

func sseToolUse(id, name, input string) string {
	return sseFrames(
		`{"type":"message_start","message":{"usage":{"input_tokens":7}}}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"`+id+`","name":"`+name+`"}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":`+jsonStr(input)+`}}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":3}}`,
		`{"type":"message_stop"}`,
	)
}

func newChatApp(t *testing.T, script func(call int, req anthropicReq, w http.ResponseWriter, r *http.Request)) (*apitest.App, *fakeAnthropic) {
	t.Helper()
	f := &fakeAnthropic{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req anthropicReq
		json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "text/event-stream")
		if req.MaxTokens == 1 { // key validation from PUT /api/me/ai
			fmt.Fprint(w, sseFrames(`{"type":"message_stop"}`))
			return
		}
		f.mu.Lock()
		call := len(f.reqs)
		f.reqs = append(f.reqs, req)
		f.mu.Unlock()
		script(call, req, w, r)
	}))
	t.Cleanup(srv.Close)
	app := apitest.NewWith(t, func(c *config.Config) { c.AnthropicURL = srv.URL; c.SecretKey = aiSecret })
	return app, f
}

func replyText(text string) func(int, anthropicReq, http.ResponseWriter, *http.Request) {
	return func(_ int, _ anthropicReq, w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, sseText(text)) }
}

func enableAI(c *apitest.Client) {
	c.Do("PUT", "/api/me/ai", map[string]any{"apiKey": aiTestKey, "enabled": true}, 200, nil)
}

type sseEvent struct {
	Name, Data string
}

func parseSSE(raw string) []sseEvent {
	var out []sseEvent
	for _, frame := range strings.Split(raw, "\n\n") {
		var ev sseEvent
		for _, line := range strings.Split(frame, "\n") {
			if v, ok := strings.CutPrefix(line, "event: "); ok {
				ev.Name = v
			} else if v, ok := strings.CutPrefix(line, "data: "); ok {
				ev.Data = v
			}
		}
		if ev.Name != "" {
			out = append(out, ev)
		}
	}
	return out
}

// chat posts to /api/chat and returns the status, the raw body and the parsed events. Safe to call from goroutines.
func chat(c *apitest.Client, body any) (int, string, []sseEvent) {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", c.App().URL()+"/api/chat", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	res, err := c.HTTP().Do(req)
	if err != nil {
		c.App().T().Error(err)
		return 0, "", nil
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(raw), parseSSE(string(raw))
}

func ask(c *apitest.Client, text string, wallets ...int64) (int, string, []sseEvent) {
	if len(wallets) == 0 {
		wallets = []int64{c.WalletID}
	}
	return chat(c, map[string]any{"wallet": c.WalletID, "wallets": wallets, "text": text})
}

type chatMsg struct {
	ID        int64                `json:"id"`
	Role      string               `json:"role"`
	Content   string               `json:"content"`
	Charts    []assistant.ChartDTO `json:"charts"`
	WalletIDs []int64              `json:"walletIds"`
	CreatedAt string               `json:"createdAt"`
}

func history(c *apitest.Client, wallet int64) []chatMsg {
	c.App().T().Helper()
	var out []chatMsg
	c.Do("GET", "/api/chat?wallet="+apitest.Itoa(wallet), nil, 200, &out)
	return out
}

func eventsNamed(evs []sseEvent, name string) []sseEvent {
	var out []sseEvent
	for _, e := range evs {
		if e.Name == name {
			out = append(out, e)
		}
	}
	return out
}

func errorEvent(t *testing.T, evs []sseEvent) string {
	t.Helper()
	errs := eventsNamed(evs, "error")
	if len(errs) != 1 {
		t.Fatalf("want one error event, got %+v", evs)
	}
	var e struct {
		Message string `json:"message"`
	}
	json.Unmarshal([]byte(errs[0].Data), &e)
	return e.Message
}

func TestChatStreamsAnswer(t *testing.T) {
	app, fake := newChatApp(t, replyText("R$ 10,00 no mês."))
	ana := app.Register("Ana", "ana@example.com")
	enableAI(ana)

	st, _, evs := ask(ana, "quanto gastei?")
	if st != 200 {
		t.Fatalf("status %d", st)
	}
	var text strings.Builder
	for _, e := range eventsNamed(evs, "delta") {
		var d struct {
			Text string `json:"text"`
		}
		json.Unmarshal([]byte(e.Data), &d)
		text.WriteString(d.Text)
	}
	if text.String() != "R$ 10,00 no mês." {
		t.Fatalf("deltas = %q (%+v)", text.String(), evs)
	}
	last := evs[len(evs)-1]
	if last.Name != "done" || last.Data != `{"inputTokens":10,"outputTokens":5}` {
		t.Fatalf("last event %+v", last)
	}

	reqs := fake.requests()
	if len(reqs) != 1 {
		t.Fatalf("%d calls", len(reqs))
	}
	r := reqs[0]
	if r.Model != "claude-sonnet-5-5" || r.MaxTokens != 1024 || len(r.Tools) != len(assistant.AssistantTools()) || !strings.Contains(r.System, "Pessoal") {
		t.Fatalf("request %+v", r)
	}
	if n := len(r.Messages); n != 1 || r.Messages[0].Role != "user" || r.Messages[0].Content[0].Text != "quanto gastei?" {
		t.Fatalf("messages %+v", r.Messages)
	}

	h := history(ana, ana.WalletID)
	if len(h) != 2 || h[0].Role != "user" || h[0].Content != "quanto gastei?" || h[1].Role != "assistant" || h[1].Content != "R$ 10,00 no mês." {
		t.Fatalf("history %+v", h)
	}
	if len(h[0].WalletIDs) != 1 || h[0].WalletIDs[0] != ana.WalletID || h[1].Charts == nil {
		t.Fatalf("history %+v", h)
	}
}

func firstExpenseCategory(t *testing.T, c *apitest.Client) int64 {
	t.Helper()
	var cats []categoryDTO
	c.Do("GET", c.WPath("/categories"), nil, 200, &cats)
	for _, cat := range cats {
		if cat.Kind == "expense" {
			return cat.ID
		}
	}
	t.Fatal("no expense category")
	return 0
}

func TestChatToolLoop(t *testing.T) {
	app, fake := newChatApp(t, func(call int, _ anthropicReq, w http.ResponseWriter, _ *http.Request) {
		if call == 0 {
			fmt.Fprint(w, sseToolUse("tu_1", "resumo_por_categoria", `{"de":"2026-10-01","ate":"2026-10-31"}`))
			return
		}
		fmt.Fprint(w, sseText("R$ 12,34."))
	})
	ana := app.Register("Ana", "ana@example.com")
	enableAI(ana)
	ana.Do("POST", ana.WPath("/transactions"), map[string]any{
		"kind": "expense", "amountCents": 1234, "description": "Mercado", "date": "2026-10-05", "categoryId": firstExpenseCategory(t, ana),
	}, 201, nil)

	st, _, evs := ask(ana, "onde gastei?")
	if st != 200 || len(eventsNamed(evs, "status")) != 1 || evs[len(evs)-1].Name != "done" {
		t.Fatalf("status %d events %+v", st, evs)
	}
	if evs[len(evs)-1].Data != `{"inputTokens":17,"outputTokens":8}` {
		t.Fatalf("usage %s", evs[len(evs)-1].Data)
	}
	reqs := fake.requests()
	if len(reqs) != 2 {
		t.Fatalf("%d calls", len(reqs))
	}
	msgs := reqs[1].Messages
	asst, res := msgs[len(msgs)-2], msgs[len(msgs)-1]
	if asst.Role != "assistant" || asst.Content[0].Type != "tool_use" || asst.Content[0].ID != "tu_1" {
		t.Fatalf("assistant turn %+v", asst)
	}
	if res.Role != "user" || res.Content[0].Type != "tool_result" || res.Content[0].ToolUseID != "tu_1" || res.Content[0].IsError ||
		!strings.Contains(res.Content[0].Content, `"total_despesas":1234`) {
		t.Fatalf("tool result %+v", res)
	}
	if h := history(ana, ana.WalletID); len(h) != 2 || h[1].Content != "R$ 12,34." {
		t.Fatalf("history %+v", h)
	}
}

// toolResultOf runs one tool call through the chat and returns the tool_result the model got back.
func toolResultOf(t *testing.T, input string, before func(*apitest.App)) anthropic.Block {
	t.Helper()
	app, fake := newChatApp(t, func(call int, _ anthropicReq, w http.ResponseWriter, _ *http.Request) {
		if call == 0 {
			fmt.Fprint(w, sseToolUse("tu_1", "resumo_por_categoria", input))
			return
		}
		fmt.Fprint(w, sseText("ok"))
	})
	ana := app.Register("Ana", "ana@example.com")
	enableAI(ana)
	if before != nil {
		before(app)
	}
	if _, _, evs := ask(ana, "oi"); evs[len(evs)-1].Name != "done" {
		t.Fatalf("events %+v", evs)
	}
	reqs := fake.requests()
	if len(reqs) != 2 {
		t.Fatalf("%d calls", len(reqs))
	}
	msgs := reqs[1].Messages
	return msgs[len(msgs)-1].Content[0]
}

func TestChatToolInputErrorReachesModel(t *testing.T) {
	res := toolResultOf(t, `{"de":"ontem","ate":"hoje"}`, nil)
	if !res.IsError || res.Content != "'de' e 'ate' são obrigatórios no formato YYYY-MM-DD" {
		t.Fatalf("tool result %+v", res)
	}
}

func TestChatToolInternalErrorIsHidden(t *testing.T) {
	res := toolResultOf(t, `{"de":"2026-10-01","ate":"2026-10-31"}`, func(app *apitest.App) {
		if _, err := app.Server().DB.Exec(`ALTER TABLE transactions RENAME TO transactions_gone`); err != nil {
			t.Fatal(err)
		}
	})
	if !res.IsError || res.Content != "erro interno ao consultar os dados" {
		t.Fatalf("tool result %+v", res)
	}
}

func TestChatSeparatesTextFromTurns(t *testing.T) {
	app, _ := newChatApp(t, func(call int, _ anthropicReq, w http.ResponseWriter, _ *http.Request) {
		if call == 1 {
			fmt.Fprint(w, sseText("R$ 1,00."))
			return
		}
		fmt.Fprint(w, sseFrames(
			`{"type":"message_start","message":{"usage":{"input_tokens":1}}}`,
			`{"type":"content_block_start","index":0,"content_block":{"type":"text"}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Vou ver."}}`,
			`{"type":"content_block_stop","index":0}`,
			`{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"tu_1","name":"resumo_por_categoria"}}`,
			`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"de\":\"2026-10-01\",\"ate\":\"2026-10-31\"}"}}`,
			`{"type":"content_block_stop","index":1}`,
			`{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":1}}`,
			`{"type":"message_stop"}`,
		))
	})
	ana := app.Register("Ana", "ana@example.com")
	enableAI(ana)
	_, raw, _ := ask(ana, "oi")
	if !strings.Contains(raw, `data: {"text":"\n\n"}`) {
		t.Fatalf("no separator delta: %s", raw)
	}
	if h := history(ana, ana.WalletID); len(h) != 2 || h[1].Content != "Vou ver.\n\nR$ 1,00." {
		t.Fatalf("history %+v", h)
	}
}

func TestChatMaxTokensNotesCutAndSkipsTools(t *testing.T) {
	app, fake := newChatApp(t, func(_ int, _ anthropicReq, w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, strings.Replace(sseText("Resposta longa"), `"end_turn"`, `"max_tokens"`, 1))
	})
	ana := app.Register("Ana", "ana@example.com")
	enableAI(ana)
	_, _, evs := ask(ana, "oi")
	if evs[len(evs)-1].Name != "done" || len(fake.requests()) != 1 {
		t.Fatalf("events %+v", evs)
	}
	if h := history(ana, ana.WalletID); len(h) != 2 || h[1].Content != "Resposta longa\n\n(resposta cortada pelo limite de tamanho)" {
		t.Fatalf("history %+v", h)
	}

	// A tool_use block without stop_reason tool_use is not run.
	app2, fake2 := newChatApp(t, func(_ int, _ anthropicReq, w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, strings.Replace(sseToolUse("tu_1", "resumo_por_categoria", `{"de":"2026-10-01","ate":"2026-10-31"}`),
			`"tool_use"}`, `"end_turn"}`, 1))
	})
	bia := app2.Register("Bia", "bia@example.com")
	enableAI(bia)
	if _, _, evs := ask(bia, "oi"); len(eventsNamed(evs, "status")) != 0 || evs[len(evs)-1].Name != "done" || len(fake2.requests()) != 1 {
		t.Fatalf("events %+v", evs)
	}
}

func TestChatWalletRequired(t *testing.T) {
	app, _ := newChatApp(t, replyText("ok"))
	ana := app.Register("Ana", "ana@example.com")
	enableAI(ana)
	for _, body := range []map[string]any{
		{"wallets": []int64{ana.WalletID}, "text": "oi"},
		{"wallet": 0, "wallets": []int64{ana.WalletID}, "text": "oi"},
	} {
		if st, raw, _ := chat(ana, body); st != 400 {
			t.Fatalf("%v: %d %s", body, st, raw)
		}
	}
	for _, m := range []string{"GET", "DELETE"} {
		for _, q := range []string{"", "?wallet=0"} {
			if st := ana.Status(m, "/api/chat"+q, nil); st != 400 {
				t.Fatalf("%s %s: %d", m, q, st)
			}
		}
	}
}

func TestChatChartEvent(t *testing.T) {
	app, _ := newChatApp(t, func(call int, _ anthropicReq, w http.ResponseWriter, _ *http.Request) {
		if call == 0 {
			fmt.Fprint(w, sseToolUse("tu_1", "mostrar_grafico", `{"tipo":"bars","titulo":"Gastos","pontos":[{"rotulo":"Mercado","valor":1234}]}`))
			return
		}
		fmt.Fprint(w, sseText("Veja o gráfico."))
	})
	ana := app.Register("Ana", "ana@example.com")
	enableAI(ana)
	_, _, evs := ask(ana, "mostra um gráfico")
	charts := eventsNamed(evs, "chart")
	want := `{"kind":"bars","title":"Gastos","points":[{"label":"Mercado","value":1234}]}`
	if len(charts) != 1 || charts[0].Data != want {
		t.Fatalf("events %+v", evs)
	}
	h := history(ana, ana.WalletID)
	if len(h) != 2 || len(h[1].Charts) != 1 || h[1].Charts[0].Points[0].Value != 1234 {
		t.Fatalf("history %+v", h)
	}
}

func TestChatStopsAfterSixTurns(t *testing.T) {
	app, fake := newChatApp(t, func(call int, _ anthropicReq, w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, sseToolUse(fmt.Sprintf("tu_%d", call), "resumo_por_categoria", `{"de":"2026-10-01","ate":"2026-10-31"}`))
	})
	ana := app.Register("Ana", "ana@example.com")
	enableAI(ana)
	st, _, evs := ask(ana, "loop")
	if st != 200 || len(fake.requests()) != 6 {
		t.Fatalf("status %d, %d calls", st, len(fake.requests()))
	}
	if msg := errorEvent(t, evs); msg == "" || evs[len(evs)-1].Name != "error" {
		t.Fatalf("events %+v", evs)
	}
	if h := history(ana, ana.WalletID); len(h) != 1 || h[0].Role != "user" {
		t.Fatalf("history %+v", h)
	}
}

func TestChatRejectsOtherUsersWallet(t *testing.T) {
	app, fake := newChatApp(t, replyText("x"))
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	enableAI(ana)
	for _, body := range []map[string]any{
		{"wallet": ana.WalletID, "wallets": []int64{ana.WalletID, bia.WalletID}, "text": "oi"},
		{"wallet": bia.WalletID, "wallets": []int64{ana.WalletID}, "text": "oi"},
		{"wallet": ana.WalletID, "wallets": []int64{999999}, "text": "oi"},
	} {
		if st, raw, _ := chat(ana, body); st != 403 {
			t.Fatalf("%v: status %d %s", body, st, raw)
		}
	}
	if n := len(fake.requests()); n != 0 {
		t.Fatalf("%d calls to Anthropic", n)
	}
	if h := history(ana, ana.WalletID); len(h) != 0 {
		t.Fatalf("history %+v", h)
	}
	if st := ana.Status("GET", "/api/chat?wallet="+apitest.Itoa(bia.WalletID), nil); st != 403 {
		t.Fatalf("GET other wallet: %d", st)
	}
	if st := ana.Status("DELETE", "/api/chat?wallet="+apitest.Itoa(bia.WalletID), nil); st != 403 {
		t.Fatalf("DELETE other wallet: %d", st)
	}
}

func TestChatNeedsEnabledAssistant(t *testing.T) {
	app, fake := newChatApp(t, replyText("x"))
	ana := app.Register("Ana", "ana@example.com")
	var e struct {
		Error httpx.APIError `json:"error"`
	}
	st, raw, _ := ask(ana, "oi")
	json.Unmarshal([]byte(raw), &e)
	if st != 409 || e.Error.Message == "" {
		t.Fatalf("no key: %d %s", st, raw)
	}
	ana.Do("PUT", "/api/me/ai", map[string]any{"apiKey": aiTestKey, "enabled": false}, 200, nil)
	if st, raw, _ := ask(ana, "oi"); st != 409 {
		t.Fatalf("disabled: %d %s", st, raw)
	}
	if len(fake.requests()) != 0 || len(history(ana, ana.WalletID)) != 0 {
		t.Fatal("called Anthropic or saved a message")
	}
}

func manyIDs(n int) []int64 {
	out := make([]int64, n)
	for i := range out {
		out[i] = int64(i + 1)
	}
	return out
}

func TestChatValidatesText(t *testing.T) {
	app, _ := newChatApp(t, replyText("ok"))
	ana := app.Register("Ana", "ana@example.com")
	enableAI(ana)
	w := ana.WalletID
	for _, body := range []map[string]any{
		{"wallet": w, "wallets": []int64{w}, "text": ""},
		{"wallet": w, "wallets": []int64{w}, "text": "   "},
		{"wallet": w, "wallets": []int64{w}, "text": strings.Repeat("a", 4001)},
		{"wallet": w, "wallets": []int64{}, "text": "oi"},
		{"wallet": w, "wallets": []int64{w, w}, "text": "oi"},
		{"wallet": w, "wallets": manyIDs(101), "text": "oi"},
	} {
		if st, raw, _ := chat(ana, body); st != 400 {
			t.Fatalf("%.60v: status %d %s", body, st, raw)
		}
	}
	if st, raw, _ := ask(ana, strings.Repeat("é", 4000)); st != 200 {
		t.Fatalf("4000 runes: %d %s", st, raw)
	}
}

func TestChatRateLimit(t *testing.T) {
	app, _ := newChatApp(t, replyText("ok"))
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	enableAI(ana)
	enableAI(bia)
	for i := range 20 {
		if st, raw, _ := ask(ana, "oi"); st != 200 {
			t.Fatalf("send %d: %d %s", i, st, raw)
		}
	}
	if st, _, _ := ask(ana, "oi"); st != 429 {
		t.Fatalf("21st: %d", st)
	}
	if st, _, _ := ask(bia, "oi"); st != 200 {
		t.Fatalf("bia: %d", st)
	}
}

func TestChatOneAtATime(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	app, _ := newChatApp(t, func(call int, _ anthropicReq, w http.ResponseWriter, _ *http.Request) {
		if call == 0 {
			close(started)
			<-release
		}
		fmt.Fprint(w, sseText("ok"))
	})
	ana := app.Register("Ana", "ana@example.com")
	enableAI(ana)
	done := make(chan int)
	go func() { st, _, _ := ask(ana, "primeira"); done <- st }()
	<-started
	for range 5 {
		if st, _, _ := ask(ana, "segunda"); st != 409 {
			t.Fatalf("second while busy: %d", st)
		}
	}
	close(release)
	if st := <-done; st != 200 {
		t.Fatalf("first: %d", st)
	}
	// The 409s did not use the quota: 19 more sends fit in the 20/min limit.
	for i := range 19 {
		if st, _, _ := ask(ana, "depois"); st != 200 {
			t.Fatalf("after %d: %d", i, st)
		}
	}
}

func TestChatClientCancelSavesNoAnswer(t *testing.T) {
	upstreamGone := make(chan struct{})
	app, _ := newChatApp(t, func(_ int, _ anthropicReq, w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, sseFrames(
			`{"type":"message_start","message":{"usage":{"input_tokens":1}}}`,
			`{"type":"content_block_start","index":0,"content_block":{"type":"text"}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"parcial"}}`,
		))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(upstreamGone)
	})
	ana := app.Register("Ana", "ana@example.com")
	enableAI(ana)

	ctx, cancel := context.WithCancel(t.Context())
	b, _ := json.Marshal(map[string]any{"wallet": ana.WalletID, "wallets": []int64{ana.WalletID}, "text": "oi"})
	req, _ := http.NewRequestWithContext(ctx, "POST", app.URL()+"/api/chat", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	res, err := ana.HTTP().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4096)
	var got strings.Builder
	for !strings.Contains(got.String(), "event: delta") {
		n, err := res.Body.Read(buf)
		got.Write(buf[:n])
		if err != nil {
			t.Fatalf("read: %v (%q)", err, got.String())
		}
	}
	cancel()
	res.Body.Close()
	select {
	case <-upstreamGone:
	case <-time.After(5 * time.Second):
		t.Fatal("Anthropic call not cancelled")
	}
	for deadline := time.Now().Add(5 * time.Second); app.Server().Assistant().Busy(ana.User.ID); {
		if time.Now().After(deadline) {
			t.Fatal("handler did not finish")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if h := history(ana, ana.WalletID); len(h) != 1 || h[0].Role != "user" {
		t.Fatalf("history %+v", h)
	}
}

func TestChatAnthropicErrorsBecomeEvents(t *testing.T) {
	cases := []struct {
		status int
		kind   string
		want   string
	}{
		{401, "", "chave"}, {429, "", "limite"}, {529, "", "indisponível"}, {500, "", "indisponível"}, {0, "", "conexão"},
		{402, "", "saldo"}, {400, "billing_error", "saldo"},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprint(tc.status, tc.kind), func(t *testing.T) {
			app, _ := newChatApp(t, func(_ int, _ anthropicReq, w http.ResponseWriter, _ *http.Request) {
				if tc.status == 0 {
					conn, _, _ := w.(http.Hijacker).Hijack()
					conn.Close()
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				fmt.Fprintf(w, `{"error":{"type":%q,"message":"bad key %s"}}`, cmp.Or(tc.kind, "some_error"), aiTestKey)
			})
			ana := app.Register("Ana", "ana@example.com")
			enableAI(ana)
			st, raw, evs := ask(ana, "oi")
			if st != 200 {
				t.Fatalf("status %d", st)
			}
			if msg := errorEvent(t, evs); !strings.Contains(msg, tc.want) {
				t.Fatalf("message %q, want %q", msg, tc.want)
			}
			if strings.Contains(raw, aiTestKey) || strings.Contains(raw, "sk-ant") || strings.Contains(raw, "bad key") {
				t.Fatalf("leaked upstream text: %s", raw)
			}
			if h := history(ana, ana.WalletID); len(h) != 1 || h[0].Role != "user" {
				t.Fatalf("history %+v", h)
			}
		})
	}
}

func TestChatKeyUnreadableAfterRotation(t *testing.T) {
	app, fake := newChatApp(t, replyText("x"))
	ana := app.Register("Ana", "ana@example.com")
	enableAI(ana)
	app.Server().Cfg.SecretKey = bytes.Repeat([]byte{9}, 32)
	st, _, evs := ask(ana, "oi")
	if st != 200 || errorEvent(t, evs) != "A chave do assistente precisa ser informada de novo em Ajustes." {
		t.Fatalf("status %d events %+v", st, evs)
	}
	if len(fake.requests()) != 0 {
		t.Fatal("called Anthropic")
	}
}

func TestChatHistoryIsPerUserAndWallet(t *testing.T) {
	app, _ := newChatApp(t, replyText("ok"))
	ana := app.Register("Ana", "ana@example.com")
	bia := app.Register("Bia", "bia@example.com")
	enableAI(ana)
	enableAI(bia)
	var casa wallet.WalletDTO
	ana.Do("POST", "/api/wallets", map[string]any{"name": "Casa"}, 201, &casa)
	ana.Do("POST", "/api/wallets/"+apitest.Itoa(ana.WalletID)+"/members", map[string]any{"email": "bia@example.com", "role": "reader"}, 201, nil)

	ask(ana, "na pessoal")
	chat(ana, map[string]any{"wallet": casa.ID, "wallets": []int64{casa.ID}, "text": "na casa"})
	// A reader may use the assistant on a shared wallet; her thread is her own.
	if st, raw, _ := chat(bia, map[string]any{"wallet": ana.WalletID, "wallets": []int64{ana.WalletID}, "text": "da bia"}); st != 200 {
		t.Fatalf("reader: %d %s", st, raw)
	}

	if h := history(ana, ana.WalletID); len(h) != 2 || h[0].Content != "na pessoal" {
		t.Fatalf("ana pessoal %+v", h)
	}
	if h := history(ana, casa.ID); len(h) != 2 || h[0].Content != "na casa" {
		t.Fatalf("ana casa %+v", h)
	}
	if h := history(bia, ana.WalletID); len(h) != 2 || h[0].Content != "da bia" {
		t.Fatalf("bia %+v", h)
	}
	if h := history(bia, bia.WalletID); len(h) != 0 {
		t.Fatalf("bia own %+v", h)
	}
	if st := ana.Status("GET", "/api/chat?wallet=abc", nil); st != 400 {
		t.Fatalf("bad wallet param: %d", st)
	}
}

func TestChatDeleteClearsOnlyThatConversation(t *testing.T) {
	app, _ := newChatApp(t, replyText("ok"))
	ana := app.Register("Ana", "ana@example.com")
	enableAI(ana)
	var casa wallet.WalletDTO
	ana.Do("POST", "/api/wallets", map[string]any{"name": "Casa"}, 201, &casa)
	ask(ana, "na pessoal")
	chat(ana, map[string]any{"wallet": casa.ID, "wallets": []int64{casa.ID}, "text": "na casa"})
	ana.Do("DELETE", "/api/chat?wallet="+apitest.Itoa(ana.WalletID), nil, 204, nil)
	if h := history(ana, ana.WalletID); len(h) != 0 {
		t.Fatalf("pessoal %+v", h)
	}
	if h := history(ana, casa.ID); len(h) != 2 {
		t.Fatalf("casa %+v", h)
	}
}

func TestChatSendsOnlyRecentHistory(t *testing.T) {
	app, fake := newChatApp(t, replyText("ok"))
	ana := app.Register("Ana", "ana@example.com")
	enableAI(ana)
	insert := func(n int, content func(i int) string) {
		for i := range n {
			role := "user"
			if i%2 == 1 {
				role = "assistant"
			}
			if _, err := app.Server().DB.Exec(`INSERT INTO chat_messages (user_id, wallet_id, role, content) VALUES (?, ?, ?, ?)`,
				ana.User.ID, ana.WalletID, role, content(i)); err != nil {
				t.Fatal(err)
			}
		}
	}
	texts := func(r anthropicReq) []string {
		var out []string
		for _, m := range r.Messages {
			for _, b := range m.Content {
				out = append(out, b.Text)
			}
		}
		return out
	}

	insert(30, func(i int) string { return fmt.Sprintf("m%d", i) })
	ask(ana, "nova")
	got := texts(fake.requests()[0])
	// The last 20 are m11..m29 plus "nova"; m11 is an assistant turn and cannot open the conversation.
	if len(got) != 19 || got[0] != "m12" || got[len(got)-1] != "nova" || fake.requests()[0].Messages[0].Role != "user" {
		t.Fatalf("sent %v", got)
	}

	ana.Do("DELETE", "/api/chat?wallet="+apitest.Itoa(ana.WalletID), nil, 204, nil)
	insert(6, func(i int) string { return strings.Repeat(string(rune('a'+i)), 10000) })
	ask(ana, "grande")
	total := 0
	for _, s := range texts(fake.requests()[1]) {
		total += len(s)
	}
	if total > 24000 {
		t.Fatalf("sent %d characters", total)
	}
	if got := texts(fake.requests()[1]); got[len(got)-1] != "grande" || len(got) < 2 {
		t.Fatalf("sent %d blocks", len(got))
	}
}

func TestSystemPromptMentionsTodayAndWallets(t *testing.T) {
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	// 02:00 UTC is still the previous day in São Paulo.
	p := assistant.SystemPrompt(time.Date(2026, 10, 15, 2, 0, 0, 0, time.UTC), loc, []assistant.WalletInfo{
		{ID: 1, Name: "Pessoal", Categories: []string{"Mercado", "Lazer"}},
		{ID: 2, Name: "Casa", Categories: []string{"Contas"}},
	})
	for _, want := range []string{"2026-10-14", "Pessoal", "Casa", "Mercado", "Lazer", "Contas", "R$", "ferramentas", "instru"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt lacks %q:\n%s", want, p)
		}
	}
}

func TestChatInStreamAnthropicErrorsBecomeEvents(t *testing.T) {
	for kind, want := range map[string]string{
		"overloaded_error": "indisponível", "rate_limit_error": "limite", "authentication_error": "chave", "billing_error": "saldo",
	} {
		t.Run(kind, func(t *testing.T) {
			app, _ := newChatApp(t, func(_ int, _ anthropicReq, w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprintf(w, "event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":%q,\"message\":\"bad key %s\"}}\n\n", kind, aiTestKey)
			})
			ana := app.Register("Ana", "ana@example.com")
			enableAI(ana)
			st, raw, evs := ask(ana, "oi")
			if st != 200 {
				t.Fatalf("status %d", st)
			}
			if msg := errorEvent(t, evs); !strings.Contains(msg, want) {
				t.Fatalf("message %q, want %q", msg, want)
			}
			if strings.Contains(raw, aiTestKey) || strings.Contains(raw, "bad key") {
				t.Fatalf("leaked upstream text: %s", raw)
			}
		})
	}
}

// ---- Spending limits ----

var spLoc, _ = time.LoadLocation("America/Sao_Paulo")

// limitApp is a chat app frozen at the given time, with Ana's assistant enabled.
func limitApp(t *testing.T, at time.Time, script func(int, anthropicReq, http.ResponseWriter, *http.Request)) (*apitest.App, *fakeAnthropic, *apitest.Client) {
	t.Helper()
	app, fake := newChatApp(t, script)
	app.SetNow(func() time.Time { return at })
	ana := app.Register("Ana", "ana@example.com")
	enableAI(ana)
	return app, fake, ana
}

func setLimits(c *apitest.Client, limits map[string]any) { c.Do("PUT", "/api/me/ai", limits, 200, nil) }

// askBlocked asks and expects a 429 "limit" refusal with the given message, no Anthropic call and nothing saved.
func askBlocked(t *testing.T, fake *fakeAnthropic, c *apitest.Client, want string) {
	t.Helper()
	calls := len(fake.requests())
	before := len(history(c, c.WalletID))
	st, raw, _ := ask(c, "oi")
	var e struct {
		Error httpx.APIError `json:"error"`
	}
	json.Unmarshal([]byte(raw), &e)
	if st != 429 || e.Error.Code != "limit" || e.Error.Message != want {
		t.Fatalf("status %d, body %s", st, raw)
	}
	if len(fake.requests()) != calls || len(history(c, c.WalletID)) != before {
		t.Fatal("a blocked question reached Anthropic or was saved")
	}
}

func askOK(t *testing.T, c *apitest.Client) {
	t.Helper()
	if st, raw, evs := ask(c, "oi"); st != 200 || len(evs) == 0 || evs[len(evs)-1].Name != "done" {
		t.Fatalf("status %d, body %s", st, raw)
	}
}

const (
	dailyMsg   = "Limite diário do assistente atingido (R$ 1,00). Volta amanhã ou aumente o limite em Ajustes."
	monthlyMsg = "Limite mensal do assistente atingido (R$ 1.234,56). Volta no mês que vem ou aumente o limite em Ajustes."
)

func TestChatDailyLimit(t *testing.T) {
	app, fake, ana := limitApp(t, time.Date(2026, 10, 17, 15, 0, 0, 0, spLoc), replyText("ok"))
	seedFX(t, app, 5_000_000, "2026-10-17T09:00:00Z")
	setLimits(ana, map[string]any{"dailyLimitCents": 100})
	seedUsage(t, app, ana, "2026-10-17", 999_999)
	askOK(t, ana) // under the limit by a fraction of a centavo: allowed, and this answer may overshoot it
	askBlocked(t, fake, ana, dailyMsg)

	// Removing the limit lets her ask again.
	setLimits(ana, map[string]any{"dailyLimitCents": 0})
	askOK(t, ana)
}

func TestChatMonthlyLimit(t *testing.T) {
	app, fake, ana := limitApp(t, time.Date(2026, 10, 17, 15, 0, 0, 0, spLoc), replyText("ok"))
	seedFX(t, app, 5_000_000, "2026-10-17T09:00:00Z")
	setLimits(ana, map[string]any{"monthlyLimitCents": 123456, "dailyLimitCents": 10_000_000})
	seedUsage(t, app, ana, "2026-09-30", 5_000_000_000) // last month does not count
	seedUsage(t, app, ana, "2026-10-01", 1_000_000_000)
	askOK(t, ana)
	seedUsage(t, app, ana, "2026-10-02", 234_560_000) // with the answer above, the month is now past R$ 1.234,56
	askBlocked(t, fake, ana, monthlyMsg)
}

func TestChatDailyLimitIsCheckedFirst(t *testing.T) {
	app, fake, ana := limitApp(t, time.Date(2026, 10, 17, 15, 0, 0, 0, spLoc), replyText("ok"))
	seedFX(t, app, 5_000_000, "2026-10-17T09:00:00Z")
	setLimits(ana, map[string]any{"dailyLimitCents": 100, "monthlyLimitCents": 100})
	seedUsage(t, app, ana, "2026-10-17", 1_000_000)
	askBlocked(t, fake, ana, dailyMsg)
}

func TestChatWithoutLimitsIsNotBlocked(t *testing.T) {
	app, _, ana := limitApp(t, time.Date(2026, 10, 17, 15, 0, 0, 0, spLoc), replyText("ok"))
	seedFX(t, app, 5_000_000, "2026-10-17T09:00:00Z")
	seedUsage(t, app, ana, "2026-10-17", 1_000_000_000_000)
	askOK(t, ana)
}

func TestChatLimitsNeedARate(t *testing.T) {
	app, _, ana := limitApp(t, time.Date(2026, 10, 17, 15, 0, 0, 0, spLoc), replyText("ok"))
	setLimits(ana, map[string]any{"dailyLimitCents": 1, "monthlyLimitCents": 1})
	seedUsage(t, app, ana, "2026-10-17", 1_000_000_000)
	askOK(t, ana) // no USD-BRL rate yet: spend cannot be priced, so limits are not enforced
}

func usageOf(t *testing.T, app *apitest.App, c *apitest.Client, day string) (cost int64, in, out int) {
	t.Helper()
	err := app.Server().DB.QueryRow(`SELECT cost_micro_brl, input_tokens, output_tokens FROM ai_usage WHERE user_id = ? AND day = ?`, c.User.ID, day).
		Scan(&cost, &in, &out)
	if err != nil {
		t.Fatalf("usage %s: %v", day, err)
	}
	return
}

func TestChatRecordsUsage(t *testing.T) {
	// 01:00 UTC on the 18th is still the 17th in São Paulo.
	app, _, ana := limitApp(t, time.Date(2026, 10, 18, 1, 0, 0, 0, time.UTC), replyText("ok"))
	seedFX(t, app, 5_000_000, "2026-10-17T09:00:00Z")
	askOK(t, ana)
	// Sonnet, 10 in + 5 out = 70000 nano-USD x R$ 5,00 = 350 micro-BRL, +5% = 367.5, rounded up.
	if cost, in, out := usageOf(t, app, ana, "2026-10-17"); cost != 368 || in != 10 || out != 5 {
		t.Fatalf("usage %d %d %d", cost, in, out)
	}
	askOK(t, ana)
	if cost, in, out := usageOf(t, app, ana, "2026-10-17"); cost != 736 || in != 20 || out != 10 {
		t.Fatalf("accumulated usage %d %d %d", cost, in, out)
	}
}

func TestChatRecordsTokensWithoutRate(t *testing.T) {
	app, _, ana := limitApp(t, time.Date(2026, 10, 17, 15, 0, 0, 0, spLoc), replyText("ok"))
	askOK(t, ana)
	if cost, in, out := usageOf(t, app, ana, "2026-10-17"); cost != 0 || in != 10 || out != 5 {
		t.Fatalf("usage %d %d %d", cost, in, out)
	}
}

func TestChatRecordsUsageOfAFailedAnswer(t *testing.T) {
	app, _, ana := limitApp(t, time.Date(2026, 10, 17, 15, 0, 0, 0, spLoc), func(_ int, _ anthropicReq, w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, sseFrames(`{"type":"message_start","message":{"usage":{"input_tokens":7}}}`))
		fmt.Fprint(w, "event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\",\"message\":\"x\"}}\n\n")
	})
	seedFX(t, app, 5_000_000, "2026-10-17T09:00:00Z")
	if _, _, evs := ask(ana, "oi"); errorEvent(t, evs) == "" {
		t.Fatal("want an error event")
	}
	if cost, in, _ := usageOf(t, app, ana, "2026-10-17"); in != 7 || cost != assistant.CostMicroBRL("claude-sonnet-5-5", 7, 0, 5_000_000) {
		t.Fatalf("usage %d %d", cost, in)
	}
}

func TestChatCancelStillRecordsUsage(t *testing.T) {
	upstreamGone := make(chan struct{})
	app, _, ana := limitApp(t, time.Date(2026, 10, 17, 15, 0, 0, 0, spLoc), func(_ int, _ anthropicReq, w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, sseFrames(
			`{"type":"message_start","message":{"usage":{"input_tokens":42}}}`,
			`{"type":"content_block_start","index":0,"content_block":{"type":"text"}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"parcial"}}`,
		))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(upstreamGone)
	})
	seedFX(t, app, 5_000_000, "2026-10-17T09:00:00Z")
	ctx, cancel := context.WithCancel(t.Context())
	b, _ := json.Marshal(map[string]any{"wallet": ana.WalletID, "wallets": []int64{ana.WalletID}, "text": "oi"})
	req, _ := http.NewRequestWithContext(ctx, "POST", app.URL()+"/api/chat", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	res, err := ana.HTTP().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4096)
	var got strings.Builder
	for !strings.Contains(got.String(), "event: delta") {
		n, err := res.Body.Read(buf)
		got.Write(buf[:n])
		if err != nil {
			t.Fatalf("read: %v (%q)", err, got.String())
		}
	}
	cancel()
	res.Body.Close()
	<-upstreamGone
	for deadline := time.Now().Add(5 * time.Second); app.Server().Assistant().Busy(ana.User.ID); {
		if time.Now().After(deadline) {
			t.Fatal("handler did not finish")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if cost, in, _ := usageOf(t, app, ana, "2026-10-17"); in != 42 || cost == 0 {
		t.Fatalf("usage %d %d", cost, in)
	}
}

func TestChatLimitDayRollover(t *testing.T) {
	app, fake := newChatApp(t, replyText("ok"))
	var now atomic.Int64
	now.Store(time.Date(2026, 10, 17, 23, 30, 0, 0, spLoc).UnixNano())
	app.SetNow(func() time.Time { return time.Unix(0, now.Load()) })
	ana := app.Register("Ana", "ana@example.com")
	enableAI(ana)
	seedFX(t, app, 5_000_000, "2026-10-17T09:00:00Z")
	setLimits(ana, map[string]any{"dailyLimitCents": 100})
	seedUsage(t, app, ana, "2026-10-17", 1_000_000)
	askBlocked(t, fake, ana, dailyMsg)

	now.Store(time.Date(2026, 10, 18, 0, 30, 0, 0, spLoc).UnixNano()) // yesterday's spend does not count today
	askOK(t, ana)
	if _, in, _ := usageOf(t, app, ana, "2026-10-18"); in != 10 {
		t.Fatalf("today's usage: %d input tokens", in)
	}
}

func TestChatLimitMonthRollover(t *testing.T) {
	app, fake := newChatApp(t, replyText("ok"))
	var now atomic.Int64
	now.Store(time.Date(2026, 10, 31, 23, 30, 0, 0, spLoc).UnixNano())
	app.SetNow(func() time.Time { return time.Unix(0, now.Load()) })
	ana := app.Register("Ana", "ana@example.com")
	enableAI(ana)
	seedFX(t, app, 5_000_000, "2026-10-17T09:00:00Z")
	setLimits(ana, map[string]any{"monthlyLimitCents": 123456})
	seedUsage(t, app, ana, "2026-10-05", 1_234_560_000)
	askBlocked(t, fake, ana, monthlyMsg)

	now.Store(time.Date(2026, 11, 1, 0, 30, 0, 0, spLoc).UnixNano())
	askOK(t, ana)
}
