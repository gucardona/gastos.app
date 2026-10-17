package api

import (
	"fmt"
	"math/big"
)

// modelPrices are in nano-USD per token ($0.10 per million tokens = 100). Checked on 2026-10-17 at
// platform.claude.com/docs/en/about-claude/pricing. Haiku's higher tier for prompts over 100K tokens is
// ignored (ours are far smaller) and prompt caching is not used.
var modelPrices = map[string]struct{ in, out int64 }{
	"claude-haiku-5-5":  {100, 500},
	"claude-sonnet-5-5": {2000, 10000},
	"claude-opus-5-5":   {4000, 20000},
}

// costMicroBRL estimates what the tokens cost in micro-reais (1e-6 R$) at usdBrlMicro (R$ per dollar x 1e6),
// plus a 5% safety margin, rounded up. An unknown model is priced as Opus so the estimate never under-counts.
// The arithmetic is done in big.Int: tokens x price x rate x 105 overflows int64 around 1e7 tokens.
func costMicroBRL(model string, inTok, outTok int, usdBrlMicro int64) int64 {
	p, ok := modelPrices[model]
	if !ok {
		p = modelPrices["claude-opus-5-5"]
	}
	nano := new(big.Int).Add(big.NewInt(int64(inTok)*p.in), big.NewInt(int64(outTok)*p.out))
	num := nano.Mul(nano, big.NewInt(usdBrlMicro))
	num.Mul(num, big.NewInt(105))
	den := big.NewInt(1e9 * 100)
	q, r := new(big.Int).QuoRem(num, den, new(big.Int))
	if r.Sign() > 0 {
		q.Add(q, big.NewInt(1))
	}
	return q.Int64()
}

// formatBRL writes centavos as "R$ 1.234,56".
func formatBRL(cents int64) string {
	s := fmt.Sprint(cents / 100)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "." + s[i:]
	}
	return fmt.Sprintf("R$ %s,%02d", s, cents%100)
}

// microToCents rounds micro-reais to centavos, half up.
func microToCents(micro int64) int64 { return (micro + 5_000) / 10_000 }
