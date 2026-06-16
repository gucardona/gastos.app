package events

import (
	"sync"
	"testing"
	"time"
)

func newBus() *EventBus {
	return &EventBus{subs: make(map[int64][]*sub)}
}

func TestBusNotifyExcludesActor(t *testing.T) {
	b := newBus()

	actorCh := b.Subscribe(10, 1)
	otherCh := b.Subscribe(10, 2)

	b.Notify(10, 1) // actor = userID 1

	select {
	case <-actorCh:
		t.Error("actor should not receive its own notification")
	default:
	}

	select {
	case <-otherCh:
		// expected
	default:
		t.Error("non-actor subscriber should receive notification")
	}
}

func TestBusUnsubscribe(t *testing.T) {
	b := newBus()

	ch := b.Subscribe(10, 1)
	b.Unsubscribe(10, ch)

	b.Notify(10, 99) // actor=99 so ch (userID=1) would normally receive

	select {
	case <-ch:
		t.Error("unsubscribed channel should not receive notifications")
	default:
	}
}

func TestBusNotifyFullChannelDoesNotBlock(t *testing.T) {
	b := newBus()

	ch := b.Subscribe(10, 2)
	ch <- struct{}{} // fill the channel (capacity 1)

	done := make(chan struct{})
	go func() {
		b.Notify(10, 1) // actor=1, subscriber userID=2 would receive but channel is full
		close(done)
	}()

	select {
	case <-done:
		// Notify returned without blocking
	case <-time.After(time.Second):
		t.Error("Notify blocked on a full channel")
	}
}

func TestBusConcurrentRace(t *testing.T) {
	b := newBus()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ch := b.Subscribe(10, int64(i))
			b.Notify(10, int64(i))
			b.Unsubscribe(10, ch)
		}(i)
	}
	wg.Wait()
}
