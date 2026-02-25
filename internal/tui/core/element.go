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
func (c *BaseElement) Init(app *App) error {
	if c.App != nil {
		return nil
	}

	c.App = app
	if app.GetDriver() != nil {
		c.Driver = app.GetDriver()
	}

	if c.afterInitFunc != nil {
		return c.afterInitFunc()
	}

	return nil
}

// UpdateDriver updates the driver in the element.
func (c *BaseElement) UpdateDriver(driver database.Driver) {
	c.Driver = driver
}

// Enable sets the enabled flag.
func (c *BaseElement) Enable() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.enabled = true
}

// Disable unsets the enabled flag.
func (c *BaseElement) Disable() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.enabled = false
}

// Toggle toggles the enabled flag.
func (c *BaseElement) Toggle() {
	if c.IsEnabled() {
		c.Disable()
	} else {
		c.Enable()
	}
}

// IsEnabled returns the enabled flag.
func (c *BaseElement) IsEnabled() bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.enabled
}

// BroadcastEvent sends an event to all listeners.
func (c *BaseElement) BroadcastEvent(event manager.EventMsg) {
	c.App.GetManager().Broadcast(event)
}

// SendToElement sends an event to a specific element.
func (c *BaseElement) SendToElement(element tview.Identifier, event manager.EventMsg) {
	c.App.GetManager().SendTo(element, event)
}

// SetAfterInitFunc sets the optional function that will be run at the end of Init.
func (c *BaseElement) SetAfterInitFunc(afterInitFunc func() error) {
	c.afterInitFunc = afterInitFunc
}

// Subscribe subscribes to view events.
func (c *BaseElement) Subscribe(identifier tview.Identifier) {
	c.Listener = c.App.GetManager().Subscribe(identifier)
}

// HandleEvents handles events from the manager.
func (c *BaseElement) HandleEvents(identifier tview.Identifier, handler func(event manager.EventMsg)) {
	if c.Listener == nil {
		c.Listener = c.App.GetManager().Subscribe(identifier)
	}
	for event := range c.Listener {
		handler(event)
	}
}
