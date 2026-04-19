package manager

import (
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
)

const (
	FocusChanged           MessageType = "focus_changed"
	StyleChanged           MessageType = "style_changed"
	UpdateAutocompleteKeys MessageType = "update_autocomplete"
	UpdateQueryBar         MessageType = "update_query_bar"
	FooterHeightChanged    MessageType = "footer_height_changed"
	MCPStateChanged        MessageType = "mcp_state_changed"
	OpenQueryTab           MessageType = "open_query_tab"
)

type (
	MessageType string

	Message struct {
		Type MessageType
		Data any
	}

	EventMsg struct {
		*tcell.EventKey
		Sender  tview.Identifier
		Message Message
	}

	ElementManager struct {
		mutex     sync.Mutex
		listeners map[tview.Identifier]chan EventMsg
	}
)

func NewElementManager() *ElementManager {
	return &ElementManager{
		mutex:     sync.Mutex{},
		listeners: make(map[tview.Identifier]chan EventMsg),
	}
}

func (eh *ElementManager) Subscribe(element tview.Identifier) chan EventMsg {
	eh.mutex.Lock()
	defer eh.mutex.Unlock()
	listener := make(chan EventMsg, 1)
	eh.listeners[element] = listener
	return listener
}

func (eh *ElementManager) Unsubscribe(element tview.Identifier, listener chan EventMsg) {
	eh.mutex.Lock()
	defer eh.mutex.Unlock()
	delete(eh.listeners, element)
}

func (eh *ElementManager) Broadcast(event EventMsg) {
	eh.mutex.Lock()
	channels := make([]chan EventMsg, 0, len(eh.listeners))
	for _, ch := range eh.listeners {
		channels = append(channels, ch)
	}
	eh.mutex.Unlock()

	for _, ch := range channels {
		ch <- event
	}
}

func (eh *ElementManager) SendTo(element tview.Identifier, event EventMsg) {
	eh.mutex.Lock()
	defer eh.mutex.Unlock()
	if listener, exists := eh.listeners[element]; exists {
		listener <- event
	}
}
