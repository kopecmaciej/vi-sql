package manager

import "sync"

// TabRegistry maps MCP-assigned tab IDs to getter/setter functions for the
// editor text. It is created in tui.App and shared with both page.Main
// (which registers tabs) and mcp.Server (which reads/writes them).
type TabRegistry struct {
	mu      sync.RWMutex
	getters map[string]func() string
	setters map[string]func(string)
}

func NewTabRegistry() *TabRegistry {
	return &TabRegistry{
		getters: make(map[string]func() string),
		setters: make(map[string]func(string)),
	}
}

func (r *TabRegistry) Register(tabID string, getter func() string) {
	r.mu.Lock()
	r.getters[tabID] = getter
	r.mu.Unlock()
}

func (r *TabRegistry) RegisterSetter(tabID string, setter func(string)) {
	r.mu.Lock()
	r.setters[tabID] = setter
	r.mu.Unlock()
}

func (r *TabRegistry) Unregister(tabID string) {
	r.mu.Lock()
	delete(r.getters, tabID)
	delete(r.setters, tabID)
	r.mu.Unlock()
}

func (r *TabRegistry) Exists(tabID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.setters[tabID]
	return ok
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

func (r *TabRegistry) SetText(tabID, text string) bool {
	r.mu.RLock()
	setter, ok := r.setters[tabID]
	r.mu.RUnlock()
	if !ok {
		return false
	}
	setter(text)
	return true
}
