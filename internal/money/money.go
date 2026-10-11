package money

import "sort"

// FullShare is 100% in basis points.
const FullShare = 10000

// Allocate splits total cents proportionally to shares (largest remainder).
// The result always sums to total; remainder ties favor the lowest index.
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
		out[i] = total * s / sum
		rems[i] = rem{i, total * s % sum}
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
