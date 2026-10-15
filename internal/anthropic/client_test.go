package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testKey = "sk-ant-test-key-1234"

func sse(w http.ResponseWriter, events ...string) {
	w.Header().Set("Content-Type", "text/event-stream")
	for _, e := range events {
		name := e[:strings.Index(e, "|")]
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, e[strings.Index(e, "|")+1:])
	}
}

func newClient(h http.HandlerFunc) (*Client, *httptest.Server) {
	srv := httptest.NewServer(h)
	return &Client{BaseURL: srv.URL, APIKey: testKey, HTTP: srv.Client()}, srv
}

func TestStreamTextDeltas(t *testing.T) {
	c, srv := newClient(func(w http.ResponseWriter, r *http.Request) {
		sse(w,
			`message_start|{"type":"message_start","message":{"usage":{"input_tokens":11}}}`,
			`ping|{"type":"ping"}`,
			`content_block_start|{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
			`content_block_delta|{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hel"}}`,
			`content_block_delta|{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"lo"}}`,
			`content_block_stop|{"type":"content_block_stop","index":0}`,
			`message_delta|{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}`,
			`message_stop|{"type":"message_stop"}`)
	})
	defer srv.Close()
	var got []string
	res, err := c.Stream(context.Background(), Request{Model: "m", MaxTokens: 10}, func(s string) { got = append(got, s) })
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, "|") != "Hel|lo" {
		t.Fatalf("deltas = %v", got)
	}
	if len(res.Blocks) != 1 || res.Blocks[0].Type != "text" || res.Blocks[0].Text != "Hello" {
		t.Fatalf("blocks = %+v", res.Blocks)
	}
	if res.StopReason != "end_turn" || res.InputTokens != 11 || res.OutputTokens != 5 {
		t.Fatalf("result = %+v", res)
	}
}

func TestStreamToolUse(t *testing.T) {
	c, srv := newClient(func(w http.ResponseWriter, r *http.Request) {
		sse(w,
			`content_block_start|{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_1","name":"query"}}`,
			`content_block_delta|{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"a\":"}}`,
			`content_block_delta|{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"1}"}}`,
			`content_block_stop|{"type":"content_block_stop","index":0}`,
			`message_delta|{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":7}}`)
	})
	defer srv.Close()
	res, err := c.Stream(context.Background(), Request{Model: "m"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	b := res.Blocks[0]
	if res.StopReason != "tool_use" || b.Type != "tool_use" || b.ID != "toolu_1" || b.Name != "query" || string(b.Input) != `{"a":1}` {
		t.Fatalf("result = %+v", res)
	}
}

func TestStreamSendsHeadersAndBody(t *testing.T) {
	var hdr http.Header
	var path string
	var body map[string]any
	c, srv := newClient(func(w http.ResponseWriter, r *http.Request) {
		hdr, path = r.Header, r.URL.Path
		json.NewDecoder(r.Body).Decode(&body)
		sse(w, `message_stop|{"type":"message_stop"}`)
	})
	defer srv.Close()
	_, err := c.Stream(context.Background(), Request{
		Model: "claude-x", System: "sys", MaxTokens: 99,
		Messages: []Message{
			{Role: "user", Content: []Block{{Type: "text", Text: "hi"}}},
			{Role: "user", Content: []Block{{Type: "tool_result", ToolUseID: "t1", Content: "out", IsError: true}}},
		},
		Tools: []Tool{{Name: "q", Description: "d", InputSchema: json.RawMessage(`{"type":"object"}`)}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if path != "/v1/messages" || hdr.Get("x-api-key") != testKey || hdr.Get("anthropic-version") != "2023-06-01" || hdr.Get("content-type") != "application/json" {
		t.Fatalf("path=%s headers=%v", path, hdr)
	}
	if body["model"] != "claude-x" || body["stream"] != true || body["system"] != "sys" || body["max_tokens"] != float64(99) {
		t.Fatalf("body = %v", body)
	}
	tool := body["tools"].([]any)[0].(map[string]any)
	if tool["name"] != "q" || tool["input_schema"].(map[string]any)["type"] != "object" {
		t.Fatalf("tools = %v", body["tools"])
	}
	msgs := body["messages"].([]any)
	text := msgs[0].(map[string]any)["content"].([]any)[0].(map[string]any)
	if len(text) != 2 {
		t.Fatalf("text block has extra fields: %v", text)
	}
	tr := msgs[1].(map[string]any)["content"].([]any)[0].(map[string]any)
	if tr["tool_use_id"] != "t1" || tr["content"] != "out" || tr["is_error"] != true {
		t.Fatalf("tool_result = %v", tr)
	}
}

func TestStreamHTTPErrors(t *testing.T) {
	for _, status := range []int{401, 429, 529} {
		c, srv := newClient(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			fmt.Fprint(w, `{"type":"error","error":{"type":"some_error","message":"boom"}}`)
		})
		_, err := c.Stream(context.Background(), Request{Model: "m"}, nil)
		srv.Close()
		var ae *APIError
		if !errors.As(err, &ae) || ae.Status != status || ae.Kind != "some_error" || ae.Message != "boom" {
			t.Fatalf("status %d: err = %#v", status, err)
		}
		if strings.Contains(err.Error(), testKey) {
			t.Fatalf("error leaks key: %v", err)
		}
	}
}

func TestStreamErrorEvent(t *testing.T) {
	c, srv := newClient(func(w http.ResponseWriter, r *http.Request) {
		sse(w, `error|{"type":"error","error":{"type":"overloaded_error","message":"busy"}}`)
	})
	defer srv.Close()
	_, err := c.Stream(context.Background(), Request{Model: "m"}, nil)
	var ae *APIError
	if !errors.As(err, &ae) || ae.Kind != "overloaded_error" {
		t.Fatalf("err = %#v", err)
	}
}

func TestStreamCancelStopsRequest(t *testing.T) {
	c, srv := newClient(func(w http.ResponseWriter, r *http.Request) {
		sse(w, `content_block_start|{"type":"content_block_start","index":0,"content_block":{"type":"text"}}`)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(50 * time.Millisecond); cancel() }()
	_, err := c.Stream(ctx, Request{Model: "m"}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}

func TestCheck(t *testing.T) {
	var body map[string]any
	status := 200
	c, srv := newClient(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&body)
		if status != 200 {
			w.WriteHeader(status)
			fmt.Fprint(w, `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`)
			return
		}
		sse(w, `message_stop|{"type":"message_stop"}`)
	})
	defer srv.Close()
	if err := c.Check(context.Background(), "m"); err != nil {
		t.Fatal(err)
	}
	if body["max_tokens"] != float64(1) {
		t.Fatalf("max_tokens = %v", body["max_tokens"])
	}
	status = 401
	var ae *APIError
	if err := c.Check(context.Background(), "m"); !errors.As(err, &ae) || ae.Status != 401 {
		t.Fatalf("err = %#v", err)
	}
}
