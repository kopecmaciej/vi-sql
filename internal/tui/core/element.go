package core

import (
	"sync"

	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
)

// BaseElement is a base struct for all visible elements.
// It contains all the basic fields and functions that are used by all visible elements.
type BaseElement struct {
	// enabled is a flag that indicates if the view is enabled.
	enabled bool

	// App is a pointer to the main App.
	App *App

	// Driver is a pointer to the database driver.
	Driver database.Driver

	// afterInitFunc is a function that is called when the view is initialized.
	afterInitFunc func() error

	// Listener is a channel that is used to receive events from the app.
	Listener chan manager.EventMsg

	// mutex is a mutex that is used to synchronize the view.
	mutex sync.Mutex
}

func NewBaseElement() *BaseElement {
	return &BaseElement{}
}

// Init initializes the element with the app reference.
func (b *BaseElement) Init(app *App) error {
	if b.App != nil {
		return nil
	}

	b.App = app
	if app.GetDriver() != nil {
		b.Driver = app.GetDriver()
	}

	if b.afterInitFunc != nil {
		return b.afterInitFunc()
	}

	return nil
}

// UpdateDriver updates the driver in the element.
func (b *BaseElement) UpdateDriver(driver database.Driver) {
	b.Driver = driver
}

// Enable sets the enabled flag.
func (b *BaseElement) Enable() {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.enabled = true
}

// Disable unsets the enabled flag.
func (b *BaseElement) Disable() {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.enabled = false
}

// Toggle toggles the enabled flag.
func (b *BaseElement) Toggle() {
	if b.IsEnabled() {
		b.Disable()
	} else {
		b.Enable()
	}
}

// IsEnabled returns the enabled flag.
func (b *BaseElement) IsEnabled() bool {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.enabled
}

// SetAfterInitFunc sets the optional function that will be run at the end of Init.
func (b *BaseElement) SetAfterInitFunc(afterInitFunc func() error) {
	b.afterInitFunc = afterInitFunc
}

// Subscribe subscribes to view events.
func (b *BaseElement) Subscribe(identifier tview.Identifier) {
	b.Listener = b.App.GetManager().Subscribe(identifier)
}

// HandleEvents handles events from the manager.
func (b *BaseElement) HandleEvents(identifier tview.Identifier, handler func(event manager.EventMsg)) {
	if b.Listener == nil {
		b.Listener = b.App.GetManager().Subscribe(identifier)
	}
	done := b.App.GetManager().Done()
	for {
		select {
		case event := <-b.Listener:
			handler(event)
		case <-done:
			return
		}
	}
}
