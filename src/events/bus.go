package events

import "sync"

type sub struct {
	userID int64
	ch     chan struct{}
}

type EventBus struct {
	mu   sync.RWMutex
	subs map[int64][]*sub
}

var Bus = &EventBus{subs: make(map[int64][]*sub)}

func (b *EventBus) Subscribe(accountID, userID int64) chan struct{} {
	ch := make(chan struct{}, 1)
	b.mu.Lock()
	b.subs[accountID] = append(b.subs[accountID], &sub{userID, ch})
	b.mu.Unlock()
	return ch
}

func (b *EventBus) Unsubscribe(accountID int64, ch chan struct{}) {
	b.mu.Lock()
	subs := b.subs[accountID]
	for i, s := range subs {
		if s.ch == ch {
			b.subs[accountID] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	b.mu.Unlock()
}

func (b *EventBus) Notify(accountID, actorUserID int64) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, s := range b.subs[accountID] {
		if s.userID == actorUserID {
			continue
		}
		select {
		case s.ch <- struct{}{}:
		default:
		}
	}
}
