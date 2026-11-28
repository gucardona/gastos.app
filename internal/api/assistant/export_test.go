package assistant

import (
	"context"
	"encoding/json"
)

// ChartDTO exposes chartDTO to the tests.
type ChartDTO = chartDTO

// CostMicroBRL exposes costMicroBRL to the tests.
var CostMicroBRL = costMicroBRL

// RunTool exposes runTool to the tests.
func (s *Handler) RunTool(ctx context.Context, walletIDs []int64, name string, input json.RawMessage) (string, *ChartDTO, error) {
	return s.runTool(ctx, walletIDs, name, input)
}

// Busy reports whether an answer is being generated for userID.
func (s *Handler) Busy(userID int64) bool {
	s.chatMu.Lock()
	defer s.chatMu.Unlock()
	return s.chatInFlight[userID]
}

// FXRate exposes fxRate to the tests.
func (s *Handler) FXRate(ctx context.Context) (micro int64, fetchedAt string, ok bool) {
	return s.fxRate(ctx)
}

// AssistantTools exposes assistantTools to the tests.
var AssistantTools = assistantTools

// WalletInfo exposes walletInfo to the tests.
type WalletInfo = walletInfo

// SystemPrompt exposes systemPrompt to the tests.
var SystemPrompt = systemPrompt

// FormatBRL exposes formatBRL to the tests.
var FormatBRL = formatBRL
