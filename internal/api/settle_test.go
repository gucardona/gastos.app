package api

import (
	"slices"
	"testing"
)

func TestSuggestSettlements(t *testing.T) {
	tests := []struct {
		name string
		net  map[int64]int64
		want []transfer
	}{
		{"dois", map[int64]int64{1: 1500, 2: -1500}, []transfer{{2, 1, 1500}}},
		{"um credor, dois devedores", map[int64]int64{1: 3000, 2: -1000, 3: -2000}, []transfer{{3, 1, 2000}, {2, 1, 1000}}},
		{"zerado", map[int64]int64{1: 0, 2: 0}, []transfer{}},
		{"um devedor, dois credores", map[int64]int64{1: 1000, 2: 500, 3: -1500}, []transfer{{3, 1, 1000}, {3, 2, 500}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := suggestSettlements(tc.net)
			if !slices.Equal(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
