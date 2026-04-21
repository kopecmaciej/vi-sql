package manager

import "sync"

// TabRegistry maps MCP-assigned tab IDs to getter functions that return the
// current editor text for that tab. It is created in tui.App and shared with
// both page.Main (which registers tabs) and mcp.Server (which reads them).
type TabRegistry struct {
	mu      sync.RWMutex
	getters map[string]func() string
}

func NewTabRegistry() *TabRegistry {
	return &TabRegistry{getters: make(map[string]func() string)}
}

func (r *TabRegistry) Register(tabID string, getter func() string) {
	r.mu.Lock()
	r.getters[tabID] = getter
	r.mu.Unlock()
}

func (r *TabRegistry) Unregister(tabID string) {
	r.mu.Lock()
	delete(r.getters, tabID)
	r.mu.Unlock()
}

func (r *TabRegistry) GetText(tabID string) (string, bool) {
	r.mu.RLock()
	getter, ok := r.getters[tabID]
	r.mu.RUnlock()
	if !ok {
		return "", false
	}
	return getter(), true
}
