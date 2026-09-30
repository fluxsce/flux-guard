package service

import (
	"context"
	"sync"
	"time"
)

type hub struct {
	mu    sync.Mutex
	slots map[string][]chan struct{}
}

func (h *hub) wait(agentID string, ctx context.Context, hold time.Duration) {
	ch := make(chan struct{})
	h.mu.Lock()
	if h.slots == nil {
		h.slots = map[string][]chan struct{}{}
	}
	h.slots[agentID] = append(h.slots[agentID], ch)
	h.mu.Unlock()
	timer := time.NewTimer(hold)
	defer timer.Stop()
	select {
	case <-ch:
	case <-timer.C:
	case <-ctx.Done():
	}
	h.remove(agentID, ch)
}

func (h *hub) wake(agentID string) {
	h.mu.Lock()
	list := h.slots[agentID]
	delete(h.slots, agentID)
	h.mu.Unlock()
	for _, ch := range list {
		close(ch)
	}
}

func (h *hub) remove(agentID string, ch chan struct{}) {
	h.mu.Lock()
	defer h.mu.Unlock()
	list := h.slots[agentID]
	for i, item := range list {
		if item == ch {
			h.slots[agentID] = append(list[:i], list[i+1:]...)
			return
		}
	}
}
