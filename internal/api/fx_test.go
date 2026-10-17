package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"gastos/internal/config"
)

// awesomeBody is AwesomeAPI's real response shape for /last/USD-BRL, with the ask price replaced.
func awesomeBody(ask string) string {
	return `{"USDBRL":{"code":"USD","codein":"BRL","name":"Dólar Americano/Real Brasileiro","high":"5.098","low":"5.0613","varBid":"-0.0353","pctChange":"-0.692429","bid":"5.0627","ask":"` +
		ask + `","timestamp":"1792184472","create_date":"2026-10-16 18:01:12"}}`
}

// newFXApp starts a server whose FX URL points at a fake AwesomeAPI answering reply(); hits counts its requests.
func newFXApp(t *testing.T, reply *atomic.Value) (*testApp, *atomic.Int32) {
	t.Helper()
	hits := &atomic.Int32{}
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		body := reply.Load().(string)
		if body == "500" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	}))
	t.Cleanup(fake.Close)
	return newTestAppWith(t, func(c *config.Config) { c.FXURL = fake.URL }), hits
}

func storedFX(t *testing.T, app *testApp) (int64, string) {
	t.Helper()
	micro, at, _ := app.s.fxRate(t.Context())
	return micro, at
}

func TestRefreshFXStoresAsk(t *testing.T) {
	reply := &atomic.Value{}
	reply.Store(awesomeBody("5.0631"))
	app, _ := newFXApp(t, reply)
	app.s.now = func() time.Time { return time.Date(2026, 10, 17, 9, 30, 0, 0, time.UTC) }
	if err := app.s.refreshFX(t.Context()); err != nil {
		t.Fatal(err)
	}
	micro, at, ok := app.s.fxRate(t.Context())
	if !ok || micro != 5063100 || at != "2026-10-17T09:30:00Z" {
		t.Fatalf("stored %d at %q (ok %v)", micro, at, ok)
	}
}

func TestRefreshFXKeepsPreviousRateOnBadAnswers(t *testing.T) {
	reply := &atomic.Value{}
	reply.Store(awesomeBody("5.0631"))
	app, _ := newFXApp(t, reply)
	if err := app.s.refreshFX(t.Context()); err != nil {
		t.Fatal(err)
	}
	_, before := storedFX(t, app)
	for _, body := range []string{"500", "not json", `{"USDBRL":{}}`, awesomeBody("0"), awesomeBody("-1"), awesomeBody("abc"), awesomeBody("100.5"), awesomeBody("0.5"), awesomeBody("25"), awesomeBody("NaN"), awesomeBody("1e300")} {
		reply.Store(body)
		if err := app.s.refreshFX(t.Context()); err == nil {
			t.Errorf("%.40s: want an error", body)
		}
		if micro, at := storedFX(t, app); micro != 5063100 || at != before {
			t.Errorf("%.40s: rate changed to %d at %s", body, micro, at)
		}
	}
}

func TestFXRateUnknownAtFirst(t *testing.T) {
	app := newTestApp(t)
	if _, _, ok := app.s.fxRate(t.Context()); ok {
		t.Fatal("a fresh database has no rate")
	}
}

func TestHousekeepingFetchesFXOnlyWhenStale(t *testing.T) {
	reply := &atomic.Value{}
	reply.Store(awesomeBody("5.0631"))
	app, hits := newFXApp(t, reply)
	hasRate := func() bool { _, _, ok := app.s.fxRate(t.Context()); return ok }
	runHousekeeping(t, app, hasRate)
	if !hasRate() || hits.Load() != 1 {
		t.Fatalf("first run: rate %v, %d requests", hasRate(), hits.Load())
	}

	// A second run (a restart, or the hourly tick) within 20 hours does not fetch again.
	settle := time.Now().Add(300 * time.Millisecond)
	runHousekeeping(t, app, func() bool { return time.Now().After(settle) })
	if hits.Load() != 1 {
		t.Fatalf("fresh rate refetched: %d requests", hits.Load())
	}

	// After 21 hours the rate is stale and is fetched again.
	reply.Store(awesomeBody("5.5"))
	later := time.Now().Add(21 * time.Hour)
	app.s.now = func() time.Time { return later }
	runHousekeeping(t, app, func() bool { micro, _ := storedFX(t, app); return micro == 5500000 })
	if micro, at := storedFX(t, app); micro != 5500000 || hits.Load() != 2 || at != later.UTC().Format(tsLayout) {
		t.Fatalf("stale: rate %d at %s, %d requests", micro, at, hits.Load())
	}
}

func TestHousekeepingSkipsFXWithoutURL(t *testing.T) {
	app := newTestApp(t) // tests leave FXURL empty: never reach the real AwesomeAPI
	runHousekeeping(t, app, func() bool { return len(backups(app)) == 1 })
	if _, _, ok := app.s.fxRate(t.Context()); ok {
		t.Fatal("fetched a rate without an FX URL")
	}
}
