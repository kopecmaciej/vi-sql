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
		done      chan struct{}
		stopOnce  sync.Once
	}
)

func NewElementManager() *ElementManager {
	return &ElementManager{
		mutex:     sync.Mutex{},
		listeners: make(map[tview.Identifier]chan EventMsg),
		done:      make(chan struct{}),
	}
}

func (em *ElementManager) Subscribe(element tview.Identifier) chan EventMsg {
	em.mutex.Lock()
	defer em.mutex.Unlock()
	listener := make(chan EventMsg, 1)
	em.listeners[element] = listener
	return listener
}

func (em *ElementManager) Broadcast(event EventMsg) {
	em.mutex.Lock()
	channels := make([]chan EventMsg, 0, len(em.listeners))
	for _, ch := range em.listeners {
		channels = append(channels, ch)
	}
	em.mutex.Unlock()

	for _, ch := range channels {
		go func(c chan EventMsg) {
			select {
			case c <- event:
			case <-em.done:
			}
		}(ch)
	}
}

func (em *ElementManager) SendTo(element tview.Identifier, event EventMsg) {
	em.mutex.Lock()
	ch, exists := em.listeners[element]
	em.mutex.Unlock()
	if exists {
		go func() { ch <- event }()
	}
}

func (em *ElementManager) Done() chan struct{} {
	return em.done
}

func (em *ElementManager) Stop() {
	em.stopOnce.Do(func() {
		close(em.done)
	})
}
