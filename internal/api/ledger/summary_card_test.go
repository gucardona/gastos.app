package ledger_test

import (
	"testing"
	"time"

	"gastos/internal/api/apitest"
	"gastos/internal/api/ledger"
)

// cardApp freezes today at 2026-12-05 and creates a card that closes on the 20th and is due on the 27th.
func cardApp(t *testing.T) (*apitest.Client, apitest.PaymentMethod) {
	t.Helper()
	app := apitest.New(t)
	loc := app.Server().Cfg.Location
	app.SetNow(func() time.Time { return time.Date(2026, 12, 5, 12, 0, 0, 0, loc) })
	c := app.Register("Ana", "ana@example.com")
	return c, apitest.Card2027(c)
}

// A card purchase is paid when its invoice is due: until then it is still to pay in that month, like a later-dated Pix.
func TestCardPurchaseCountsInInvoiceMonth(t *testing.T) {
	c, card := cardApp(t)
	tx := apitest.Purchase(c, "expense", "2026-11-22", card.ID) // 8700, due 2026-12-27
	check := func(month string, want int64) {
		t.Helper()
		s := getSummary(c, month)
		var cat int64
		for _, b := range s.ByCategory {
			if b.CategoryID == tx.CategoryID {
				cat = b.Cents
			}
		}
		var ess int64
		for _, v := range s.ByEssentiality {
			ess += v
		}
		if s.ExpenseCents != want || cat != want || ess != want {
			t.Errorf("%s: expense %d, category %d, essentiality %d; want %d", month, s.ExpenseCents, cat, ess, want)
		}
	}
	check("2026-11", 0)
	check("2026-12", 0) // today is 2026-12-05: the invoice is not due yet
	dec := getSummary(c, "2026-12")
	if dec.UpcomingExpenseCents != 8700 || len(dec.Upcoming) != 1 || dec.Upcoming[0].TransactionID == nil ||
		*dec.Upcoming[0].TransactionID != tx.ID || dec.Upcoming[0].Date != "2026-12-27" {
		t.Errorf("December to pay: %+v (%d)", dec.Upcoming, dec.UpcomingExpenseCents)
	}

	c.App().Server().Now = func() time.Time { return time.Date(2026, 12, 27, 12, 0, 0, 0, c.App().Server().Cfg.Location) }
	check("2026-12", 8700)
	if d := getSummary(c, "2026-12"); len(d.Upcoming) != 0 {
		t.Errorf("paid on the due day, nothing left: %+v", d.Upcoming)
	}
	if p := getSummary(c, "2027-01").Previous; p.ExpenseCents != 8700 {
		t.Errorf("January's previous month: %d, want 8700", p.ExpenseCents)
	}
}

func TestListByMonth(t *testing.T) {
	c, card := cardApp(t)
	apitest.Purchase(c, "expense", "2026-11-22", card.ID)
	var dec, nov []ledger.Transaction
	c.Do("GET", c.WPath("/transactions?month=2026-12"), nil, 200, &dec)
	c.Do("GET", c.WPath("/transactions?month=2026-11"), nil, 200, &nov)
	if len(dec) != 1 || due(dec[0]) != "2026-12-27" || len(nov) != 0 {
		t.Fatalf("december %+v, november %+v", dec, nov)
	}
	c.Do("GET", c.WPath("/transactions"), nil, 400, nil)
	c.Do("GET", c.WPath("/transactions?month=2026-12&from=2026-12-01&to=2026-12-31"), nil, 400, nil)
	c.Do("GET", c.WPath("/transactions?month=dez"), nil, 400, nil)
}

func TestOldTransactionsKeepTheirMonth(t *testing.T) {
	c, _ := cardApp(t)
	var methods []apitest.PaymentMethod
	c.Do("GET", c.WPath("/payment-methods"), nil, 200, &methods)
	apitest.Purchase(c, "expense", "2026-11-22", methods[0].ID)
	if getSummary(c, "2026-11").ExpenseCents != 8700 || getSummary(c, "2026-12").ExpenseCents != 0 {
		t.Fatal("a purchase on a plain method stays in its own month")
	}
}

func TestUpcomingCardRecurrenceLandsInInvoiceMonth(t *testing.T) {
	c, card := cardApp(t)
	id := insertRecurrence(t, c, c.WalletID, recRow{kind: "expense", description: "Streaming", start: "2026-12-01", amount: 3990, day: 22, enabled: true})
	if _, err := c.App().Server().DB.Exec(`UPDATE recurrences SET payment_method_id = ? WHERE id = ?`, card.ID, id); err != nil {
		t.Fatal(err)
	}
	dec, jan := getSummary(c, "2026-12"), getSummary(c, "2027-01")
	if len(dec.Upcoming) != 0 || dec.UpcomingExpenseCents != 0 {
		t.Errorf("December projects %+v", dec.Upcoming)
	}
	if len(jan.Upcoming) != 1 || jan.Upcoming[0].Date != "2027-01-27" || jan.UpcomingExpenseCents != 3990 {
		t.Errorf("January projects %+v (%d)", jan.Upcoming, jan.UpcomingExpenseCents)
	}
	// January's own occurrence (2027-01-22) is due in February; on a card, what is to pay is dated on the due day
	if feb := getSummary(c, "2027-02"); len(feb.Upcoming) != 1 || feb.Upcoming[0].Date != "2027-02-27" {
		t.Errorf("February projects %+v", feb.Upcoming)
	}
}
