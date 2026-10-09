package money

import (
	"slices"
	"testing"
)

func TestAllocate(t *testing.T) {
	tests := []struct {
		total  int64
		shares []int64
		want   []int64
	}{
		{100, []int64{5000, 5000}, []int64{50, 50}},
		{101, []int64{5000, 5000}, []int64{51, 50}},
		{100, []int64{3334, 3333, 3333}, []int64{34, 33, 33}},
		{1, []int64{3334, 3333, 3333}, []int64{1, 0, 0}},
		{12990, []int64{7000, 3000}, []int64{9093, 3897}},
		{5, []int64{10000}, []int64{5}},
		{0, []int64{5000, 5000}, []int64{0, 0}},
	}
	for _, tc := range tests {
		got := Allocate(tc.total, tc.shares)
		if !slices.Equal(got, tc.want) {
			t.Errorf("Allocate(%d, %v) = %v, want %v", tc.total, tc.shares, got, tc.want)
		}
		var sum int64
		for _, v := range got {
			sum += v
		}
		if sum != tc.total {
			t.Errorf("Allocate(%d, %v) soma %d", tc.total, tc.shares, sum)
		}
	}
}

func TestEqualShares(t *testing.T) {
	for n, want := range map[int][]int64{
		1: {10000},
		2: {5000, 5000},
		3: {3334, 3333, 3333},
		7: {1432, 1428, 1428, 1428, 1428, 1428, 1428},
	} {
		if got := EqualShares(n); !slices.Equal(got, want) {
			t.Errorf("EqualShares(%d) = %v, want %v", n, got, want)
		}
	}
}
