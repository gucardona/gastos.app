// Package anthropic is a minimal streaming client for the Messages API.
package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Client calls the Anthropic Messages API with an API key.
type Client struct {
	BaseURL, APIKey string
	HTTP            *http.Client
}

// Block is one content block of a message: text, a tool call or a tool result.
type Block struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

// Message is one turn of the conversation, from the user or the assistant.
type Message struct {
	Role    string  `json:"role"`
	Content []Block `json:"content"`
}

// Tool is a tool the model may call, described by a JSON schema for its input.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// Request is one call to the Messages API.
type Request struct {
	Model, System string
	Messages      []Message
	Tools         []Tool
	MaxTokens     int
}

// Result is what one streamed call returned: the content blocks, why it stopped and the tokens used.
type Result struct {
	Blocks                    []Block
	StopReason                string
	InputTokens, OutputTokens int
}

// APIError never carries the API key: Message comes from the server body only.
type APIError struct {
	Status        int
	Kind, Message string
}

// Error formats the API error with its HTTP status and error type.
func (e *APIError) Error() string {
	return fmt.Sprintf("anthropic: %s (%d %s)", e.Message, e.Status, e.Kind)
}

// apiErrBody is the JSON body the API returns with an error status.
type apiErrBody struct {
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// Check makes the smallest possible call with model, to tell whether the key and the model work.
func (c *Client) Check(ctx context.Context, model string) error {
	_, err := c.Stream(ctx, Request{
		Model: model, MaxTokens: 1,
		Messages: []Message{{Role: "user", Content: []Block{{Type: "text", Text: "hi"}}}},
	}, nil)
	return err
}

// Stream sends req with streaming on, calls onText with each piece of text as it arrives
// (onText may be nil) and returns the assembled result.
func (c *Client) Stream(ctx context.Context, req Request, onText func(string)) (Result, error) {
	var res Result
	body, err := json.Marshal(struct {
		Model     string    `json:"model"`
		System    string    `json:"system,omitempty"`
		MaxTokens int       `json:"max_tokens"`
		Messages  []Message `json:"messages"`
		Tools     []Tool    `json:"tools,omitempty"`
		Stream    bool      `json:"stream"`
	}{req.Model, req.System, req.MaxTokens, req.Messages, req.Tools, true})
	if err != nil {
		return res, err
	}
	hr, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return res, err
	}
	hr.Header.Set("x-api-key", c.APIKey)
	hr.Header.Set("anthropic-version", "2023-06-01")
	hr.Header.Set("content-type", "application/json")
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(hr)
	if err != nil {
		if ctx.Err() != nil {
			return res, ctx.Err()
		}
		// url.Error never includes headers, but keep only the cause to be safe.
		return res, fmt.Errorf("anthropic: request failed: %w", unwrapURL(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		var eb apiErrBody
		_ = json.Unmarshal(raw, &eb)
		msg := eb.Error.Message
		if msg == "" {
			msg = http.StatusText(resp.StatusCode)
		}
		return res, &APIError{Status: resp.StatusCode, Kind: c.redact(eb.Error.Type), Message: c.redact(msg)}
	}

	stopped := false
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64<<10), 8<<20)
	var inputs = map[int]*strings.Builder{}
	for sc.Scan() {
		line := sc.Text()
		data, ok := strings.CutPrefix(line, "data:")
		if !ok || strings.TrimSpace(data) == "" {
			continue // "event:" lines are redundant with the data's own type
		}
		var ev struct {
			Type         string `json:"type"`
			Index        int    `json:"index"`
			ContentBlock Block  `json:"content_block"`
			Delta        struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				PartialJSON string `json:"partial_json"`
				StopReason  string `json:"stop_reason"`
			} `json:"delta"`
			Message struct {
				Usage struct {
					InputTokens int `json:"input_tokens"`
				} `json:"usage"`
			} `json:"message"`
			Usage struct {
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
			Error struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(data)), &ev); err != nil {
			return res, fmt.Errorf("anthropic: bad stream event: %w", err)
		}
		switch ev.Type {
		case "message_start":
			res.InputTokens = ev.Message.Usage.InputTokens
		case "content_block_start":
			res.Blocks = append(res.Blocks, Block{Type: ev.ContentBlock.Type, ID: ev.ContentBlock.ID, Name: ev.ContentBlock.Name})
		case "content_block_delta":
			if ev.Index < 0 || ev.Index >= len(res.Blocks) {
				continue
			}
			switch ev.Delta.Type {
			case "text_delta":
				res.Blocks[ev.Index].Text += ev.Delta.Text
				if onText != nil {
					onText(ev.Delta.Text)
				}
			case "input_json_delta":
				if inputs[ev.Index] == nil {
					inputs[ev.Index] = &strings.Builder{}
				}
				inputs[ev.Index].WriteString(ev.Delta.PartialJSON)
			}
		case "content_block_stop":
			if ev.Index >= 0 && ev.Index < len(res.Blocks) && res.Blocks[ev.Index].Type == "tool_use" {
				in := "{}" // a tool without arguments streams no deltas
				if b := inputs[ev.Index]; b != nil && b.Len() > 0 {
					in = b.String()
				}
				if !json.Valid([]byte(in)) {
					return res, fmt.Errorf("anthropic: tool input was cut off (response truncated, likely max_tokens)")
				}
				res.Blocks[ev.Index].Input = json.RawMessage(in)
			}
		case "message_delta":
			res.StopReason = ev.Delta.StopReason
			res.OutputTokens = ev.Usage.OutputTokens
		case "message_stop":
			stopped = true
		case "error":
			return res, &APIError{Status: resp.StatusCode, Kind: c.redact(ev.Error.Type), Message: c.redact(ev.Error.Message)}
		}
	}
	if ctx.Err() != nil {
		return res, ctx.Err()
	}
	if err := sc.Err(); err != nil {
		return res, fmt.Errorf("anthropic: reading stream: %w", err)
	}
	if !stopped {
		return res, fmt.Errorf("anthropic: stream ended without message_stop: %w", io.ErrUnexpectedEOF)
	}
	return res, nil
}

// redact replaces the API key in s, so it never reaches a log or an error message.
func (c *Client) redact(s string) string {
	if c.APIKey == "" {
		return s
	}
	return strings.ReplaceAll(s, c.APIKey, "[redacted]")
}

// unwrapURL returns the error err wraps, if any; for a *url.Error that drops the URL from the message.
func unwrapURL(err error) error {
	if u, ok := err.(interface{ Unwrap() error }); ok && u.Unwrap() != nil {
		return u.Unwrap()
	}
	return err
}
