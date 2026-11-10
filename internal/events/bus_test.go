package events

import (
	"sync"
	"testing"
	"time"
)

func received(ch chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

func TestBusNotifyReachesEveryConnectionOfTheUser(t *testing.T) {
	b := New()
	tab1 := b.Subscribe(1)
	tab2 := b.Subscribe(1)
	other := b.Subscribe(2)

	b.Notify(1)

	if !received(tab1) || !received(tab2) {
		t.Error("every connection of the user must be notified, including the one that made the change")
	}
	if received(other) {
		t.Error("another user must not be notified")
	}
}

func TestBusNotifySeveralUsers(t *testing.T) {
	b := New()
	ana, bia, caio := b.Subscribe(1), b.Subscribe(2), b.Subscribe(3)
	b.Notify(1, 2, 1) // a repeated id is harmless
	if !received(ana) || !received(bia) {
		t.Error("both users must be notified")
	}
	if received(caio) {
		t.Error("a user not named must not be notified")
	}
}

func TestBusUnsubscribe(t *testing.T) {
	b := New()
	ch := b.Subscribe(1)
	kept := b.Subscribe(1)
	b.Unsubscribe(1, ch)
	b.Notify(1)
	if received(ch) {
		t.Error("a removed channel must not receive")
	}
	if !received(kept) {
		t.Error("the user's other connection must still receive")
	}
	b.Unsubscribe(1, kept)
	if len(b.subs) != 0 {
		t.Errorf("subs left behind: %v", b.subs)
	}
}

func TestBusNotifyFullChannelDoesNotBlock(t *testing.T) {
	b := New()
	ch := b.Subscribe(2)
	ch <- struct{}{}

	done := make(chan struct{})
	go func() { b.Notify(2); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("Notify blocked on a full channel")
	}
}

func TestBusConcurrentRace(t *testing.T) {
	b := New()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ch := b.Subscribe(int64(i % 3))
			b.Notify(int64(i%3), 99)
			b.Unsubscribe(int64(i%3), ch)
		}(i)
	}
	wg.Wait()
}
