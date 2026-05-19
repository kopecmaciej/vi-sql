package database

import (
	"fmt"
	"sort"

	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/util"
)

// DriverFactory creates a Driver and ValueFormatter from a connection config.
type DriverFactory func(cfg *config.SQLConfig) (Driver, ValueFormatter, error)

// FieldKind describes the widget type for a form field.
type FieldKind int

const (
	FieldInput    FieldKind = iota // text input with optional clipboard
	FieldPassword                  // masked input with optional clipboard
	FieldTextArea                  // multi-line input with optional clipboard
	FieldDropDown                  // option selector
	FieldLabel                     // read-only text / separator
)

// FieldSpec describes a single form field in a driver's connection form.
type FieldSpec struct {
	Kind      FieldKind
	Label     string
	Default   string   // initial value for input/textarea/password; selected option for dropdown; content for label
	Options   []string // DropDown only
	Clipboard bool     // whether SetClipboard is wired (Input/Password/TextArea)
	Rows      int      // TextArea only; 0 → 3
}

// ConnectorDef bundles everything a driver contributes to the TUI layer.
type ConnectorDef struct {
	// Factory instantiates the runtime driver given a saved config.
	Factory DriverFactory

	// FormSpec describes the driver-specific fields shown in the connection form,
	// between the common Name field and the common Options section.
	FormSpec []FieldSpec

	// BuildConfig turns raw field values collected from the form into a ready-to-save
	// SQLConfig. editConn is nil in add mode; non-nil in edit mode (used for
	// DSN-unchanged detection and password preservation).
	BuildConfig func(fields map[string]string, editConn *config.SQLConfig) (*config.SQLConfig, error)

	// PreFill returns label→value pairs to pre-populate the form in edit mode.
	PreFill func(*config.SQLConfig) map[string]string

	// Quoter is the driver's SQL identifier quoting rule. TUI code that
	// builds SQL fragments (e.g. ad-hoc WHERE clauses) uses this to quote
	// identifiers without needing a driver instance.
	Quoter util.Quoter
}

var registry = map[string]ConnectorDef{}

// RegisterConnector registers a driver with its full TUI definition.
func RegisterConnector(name string, def ConnectorDef) {
	registry[name] = def
}

// GetConnector looks up a registered connector by driver name.
func GetConnector(name string) (ConnectorDef, bool) {
	def, ok := registry[name]
	return def, ok
}

// ListConnectors returns all registered driver names in alphabetical order.
func ListConnectors() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// BuildConfigFromDSN detects the driver from the DSN scheme and runs the
// registered ConnectorDef.BuildConfig to produce a populated SQLConfig — the
// same path the connection form uses when only the DSN field is filled.
// An empty name defaults to the detected driver name.
func BuildConfigFromDSN(name, dsn string) (*config.SQLConfig, error) {
	driver, err := util.DetectDriverFromDSN(dsn)
	if err != nil {
		return nil, err
	}
	if name == "" {
		name = driver
	}
	def, ok := registry[driver]
	if !ok {
		return nil, fmt.Errorf("driver %q not registered", driver)
	}
	return def.BuildConfig(map[string]string{"Name": name, "DSN": dsn}, nil)
}

// NewDriver instantiates the driver registered under cfg.GetDriver().
func NewDriver(cfg *config.SQLConfig) (Driver, ValueFormatter, error) {
	name := cfg.GetDriver()
	def, ok := registry[name]
	if !ok {
		return nil, nil, fmt.Errorf("unknown driver %q — did you import its package?", name)
	}
	return def.Factory(cfg)
}
