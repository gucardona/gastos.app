package httpx_test

import (
	"testing"
	"time"

	"gastos/internal/api/httpx"
)

func TestRateLimiter(t *testing.T) {
	l := httpx.NewRateLimiter(3, 50*time.Millisecond)
	for i := 0; i < 3; i++ {
		if !l.Allow("a") {
			t.Fatalf("attempt %d should pass", i+1)
		}
	}
	if l.Allow("a") {
		t.Fatal("the 4th attempt inside the window should be blocked")
	}
	if !l.Allow("b") {
		t.Fatal("another key is not affected")
	}
	time.Sleep(70 * time.Millisecond)
	if !l.Allow("a") {
		t.Fatal("after the window the key passes again")
	}
}
