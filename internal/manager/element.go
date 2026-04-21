package manager

import (
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
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
		go func(c chan EventMsg) { c <- event }(ch)
	}
}

func (eh *ElementManager) SendTo(element tview.Identifier, event EventMsg) {
	eh.mutex.Lock()
	ch, exists := eh.listeners[element]
	eh.mutex.Unlock()
	if exists {
		go func() { ch <- event }()
	}
}
