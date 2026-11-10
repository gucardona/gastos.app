package events

import "sync"

// Bus delivers change notices per user, to every SSE connection (tab, device) that user has open.
type Bus struct {
	mu   sync.RWMutex
	subs map[int64][]chan struct{}
}

func New() *Bus { return &Bus{subs: make(map[int64][]chan struct{})} }

func (b *Bus) Subscribe(userID int64) chan struct{} {
	ch := make(chan struct{}, 1)
	b.mu.Lock()
	b.subs[userID] = append(b.subs[userID], ch)
	b.mu.Unlock()
	return ch
}

func (b *Bus) Unsubscribe(userID int64, ch chan struct{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	subs := b.subs[userID]
	for i, c := range subs {
		if c == ch {
			b.subs[userID] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	if len(b.subs[userID]) == 0 {
		delete(b.subs, userID)
	}
}

// Notify tells every connection of these users, the one that made the change included
// (its tab just refetches once more), so the same person's other tabs stay in sync.
// It never blocks: a connection with a notice already pending needs no second one.
func (b *Bus) Notify(userIDs ...int64) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, id := range userIDs {
		for _, ch := range b.subs[id] {
			select {
			case ch <- struct{}{}:
			default:
			}
		}
	}
}
