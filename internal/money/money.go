package money

import (
	"math/bits"
	"sort"
)

// FullShare is 100% in basis points.
const FullShare = 10000

// MaxCents is the largest amount accepted anywhere (R$ 1 billion): far from int64 overflow, even summed.
const MaxCents = 100_000_000_000

// Allocate splits total cents proportionally to shares (largest remainder).
// The result always sums to total; remainder ties favor the lowest index.
// total and shares must not be negative; products are taken in 128 bits, so no total overflows.
func Allocate(total int64, shares []int64) []int64 {
	out := make([]int64, len(shares))
	var sum int64
	for _, s := range shares {
		sum += s
	}
	if sum == 0 {
		return out
	}
	type rem struct {
		i int
		r int64
	}
	rems := make([]rem, len(shares))
	var allocated int64
	for i, s := range shares {
		hi, lo := bits.Mul64(uint64(total), uint64(s))
		q, r := bits.Div64(hi, lo, uint64(sum)) // q <= total since s <= sum, so Div64 cannot overflow
		out[i], rems[i] = int64(q), rem{i, int64(r)}
		allocated += out[i]
	}
	sort.SliceStable(rems, func(a, b int) bool { return rems[a].r > rems[b].r })
	for k := 0; allocated < total; k++ {
		out[rems[k].i]++
		allocated++
	}
	return out
}

// EqualShares divides 100% into n parts; the remainder goes to the first.
func EqualShares(n int) []int64 {
	if n <= 0 {
		return nil
	}
	out := make([]int64, n)
	base := int64(FullShare / n)
	for i := range out {
		out[i] = base
	}
	out[0] += FullShare - base*int64(n)
	return out
}
