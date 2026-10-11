package events

import "sync"

type sub struct {
	userID int64
	ch     chan struct{}
}

// Bus delivers change notices per wallet to everyone with an SSE connection open.
type Bus struct {
	mu   sync.RWMutex
	subs map[int64][]*sub
}

func New() *Bus { return &Bus{subs: make(map[int64][]*sub)} }

func (b *Bus) Subscribe(walletID, userID int64) chan struct{} {
	ch := make(chan struct{}, 1)
	b.mu.Lock()
	b.subs[walletID] = append(b.subs[walletID], &sub{userID, ch})
	b.mu.Unlock()
	return ch
}

func (b *Bus) Unsubscribe(walletID int64, ch chan struct{}) {
	b.mu.Lock()
	defer b.mu.Unlock()
	subs := b.subs[walletID]
	for i, s := range subs {
		if s.ch == ch {
			b.subs[walletID] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	if len(b.subs[walletID]) == 0 {
		delete(b.subs, walletID)
	}
}

// Notify tells every subscriber of the wallet, except whoever made the change.
// actorUserID = 0 stands for the system (jobs) and notifies everyone.
func (b *Bus) Notify(walletID, actorUserID int64) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, s := range b.subs[walletID] {
		if actorUserID != 0 && s.userID == actorUserID {
			continue
		}
		select {
		case s.ch <- struct{}{}:
		default:
		}
	}
}
