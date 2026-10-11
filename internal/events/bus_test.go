package events

import (
	"sync"
	"testing"
	"time"
)

func TestBusNotifyExcludesActor(t *testing.T) {
	b := New()
	actorCh := b.Subscribe(10, 1)
	otherCh := b.Subscribe(10, 2)

	b.Notify(10, 1)

	select {
	case <-actorCh:
		t.Error("whoever made the change must not be notified")
	default:
	}
	select {
	case <-otherCh:
	default:
		t.Error("the other subscribers must be notified")
	}
}

func TestBusSystemActorNotifiesEveryone(t *testing.T) {
	b := New()
	ch := b.Subscribe(10, 1)
	b.Notify(10, 0)
	select {
	case <-ch:
	default:
		t.Error("a system notification must reach everyone")
	}
}

func TestBusUnsubscribe(t *testing.T) {
	b := New()
	ch := b.Subscribe(10, 1)
	b.Unsubscribe(10, ch)
	b.Notify(10, 99)
	select {
	case <-ch:
		t.Error("a removed channel must not receive")
	default:
	}
}

func TestBusNotifyFullChannelDoesNotBlock(t *testing.T) {
	b := New()
	ch := b.Subscribe(10, 2)
	ch <- struct{}{}

	done := make(chan struct{})
	go func() { b.Notify(10, 1); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("Notify bloqueou num canal cheio")
	}
}

func TestBusConcurrentRace(t *testing.T) {
	b := New()
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
