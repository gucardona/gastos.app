package api

import "sort"

type transfer struct {
	FromUserID  int64 `json:"fromUserId"`
	ToUserID    int64 `json:"toUserId"`
	AmountCents int64 `json:"amountCents"`
}

// suggestSettlements casa o maior devedor com o maior credor até zerar.
// net: positivo = tem a receber; negativo = deve. A soma de net deve ser 0.
func suggestSettlements(net map[int64]int64) []transfer {
	type bal struct{ id, v int64 }
	var debtors, creditors []bal
	for id, v := range net {
		switch {
		case v < 0:
			debtors = append(debtors, bal{id, -v})
		case v > 0:
			creditors = append(creditors, bal{id, v})
		}
	}
	order := func(b []bal) {
		sort.Slice(b, func(i, j int) bool {
			if b[i].v != b[j].v {
				return b[i].v > b[j].v
			}
			return b[i].id < b[j].id
		})
	}
	order(debtors)
	order(creditors)

	out := []transfer{}
	for i, j := 0, 0; i < len(debtors) && j < len(creditors); {
		amt := min(debtors[i].v, creditors[j].v)
		out = append(out, transfer{debtors[i].id, creditors[j].id, amt})
		debtors[i].v -= amt
		creditors[j].v -= amt
		if debtors[i].v == 0 {
			i++
		}
		if creditors[j].v == 0 {
			j++
		}
	}
	return out
}
