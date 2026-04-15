package config

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/vi-sql/internal/util"
	"gopkg.in/yaml.v3"
)

type (
	Key struct {
		Keys        []string `yaml:"keys,omitempty,flow"`
		Runes       []string `yaml:"runes,omitempty,flow"`
		Description string   `yaml:"description"`
	}

	OrderedKeys struct {
		Element string
		Keys    []Key
	}

	KeyBindings struct {
		Navigation     NavigationKeys     `yaml:"navigation"`
		Common         CommonKeys         `yaml:"common"`
		Global         GlobalKeys         `yaml:"global"`
		Main           MainKeys           `yaml:"main"`
		Schema         SchemaKeys         `yaml:"schema"`
		Data           DataKeys           `yaml:"data"`
		Peeker         PeekerKeys         `yaml:"peeker"`
		SQLQueryEditor SQLQueryEditorKeys `yaml:"sqlQueryEditor"`
		IndexAddForm   IndexAddFormKeys   `yaml:"indexAddForm"`
		Structure      StructureKeys      `yaml:"structure"`
		History        HistoryKeys        `yaml:"history"`
		ExplainViewer  ExplainViewerKeys  `yaml:"explainViewer"`
	}

	CommonKeys struct {
		Close   Key `yaml:"close"`
		Delete  Key `yaml:"delete"`
		Add     Key `yaml:"add"`
		Edit    Key `yaml:"edit"`
		Copy    Key `yaml:"copy"`
		Save    Key `yaml:"save"`
		Filter  Key `yaml:"filter"`
		Select  Key `yaml:"select"`
		Refresh Key `yaml:"refresh"`
		Execute Key `yaml:"execute"`
		Clear   Key `yaml:"clear"`
		Paste   Key `yaml:"paste"`
	}

	NavigationKeys struct {
		MoveUp             Key `yaml:"moveUp"`
		MoveDown           Key `yaml:"moveDown"`
		MoveLeft           Key `yaml:"moveLeft"`
		MoveRight          Key `yaml:"moveRight"`
		FocusUp            Key `yaml:"focusUp"`
		FocusDown          Key `yaml:"focusDown"`
		FocusLeft          Key `yaml:"focusLeft"`
		FocusRight         Key `yaml:"focusRight"`
		AutocompleteUp     Key `yaml:"autocompleteUp"`
		AutocompleteDown   Key `yaml:"autocompleteDown"`
		AutocompleteAccept Key `yaml:"autocompleteAccept"`
	}

	GlobalKeys struct {
		CloseApp       Key `yaml:"closeApp"`
		FullScreenHelp Key `yaml:"fullScreenHelp"`
		OpenConnection Key `yaml:"openConnection"`
		ChangeStyle    Key `yaml:"changeStyle"`
		ToggleFooter   Key `yaml:"togglefooter"`
	}

	MainKeys struct {
		ServerInfo      Key `yaml:"serverInfo"`
		HideSchema      Key `yaml:"hideSchema"`
		NewTab          Key `yaml:"newTab"`
		CloseTab        Key `yaml:"closeTab"`
		FocusSchemaTree Key `yaml:"focusSchemaTree"`
		OpenActions     Key `yaml:"openActions"`
		ImportData      Key `yaml:"importData"`
	}

	SchemaKeys struct {
		ExpandAll   Key `yaml:"expandAll"`
		CollapseAll Key `yaml:"collapseAll"`
		RenameTable Key `yaml:"renameTable"`
		ExpandTable Key `yaml:"expandTable"`
	}

	DataKeys struct {
		PeekRow            Key `yaml:"peekRow"`
		FullPagePeek       Key `yaml:"fullPagePeek"`
		TermEditor         Key `yaml:"termEditor"`
		EditRow            Key `yaml:"editRow"`
		DuplicateRow       Key `yaml:"duplicateRow"`
		CopyRow            Key `yaml:"copyRow"`
		NextPage           Key `yaml:"nextPage"`
		PreviousPage       Key `yaml:"previousPage"`
		ToggleSortBar      Key `yaml:"toggleSortBar"`
		SortByColumn       Key `yaml:"sortByColumn"`
		HideColumn         Key `yaml:"hideColumn"`
		ResetHiddenColumns Key `yaml:"resetHiddenColumns"`
		MultipleSelect     Key `yaml:"multipleSelect"`
		ClearSelection     Key `yaml:"clearSelection"`
		ExplainQuery       Key `yaml:"explainQuery"`
		ExportData         Key `yaml:"exportData"`
	}

	ExplainViewerKeys struct {
		ToggleMode Key `yaml:"toggleMode"`
	}

	PeekerKeys struct {
		CopyHighlight    Key `yaml:"copyHighlight"`
		CopyValue        Key `yaml:"copyValue"`
		ExpandRow        Key `yaml:"expandRow"`
		OpenValueViewer  Key `yaml:"openValueViewer"`
		ToggleFullScreen Key `yaml:"toggleFullScreen"`
		Exit             Key `yaml:"exit"`
		MoveToTop        Key `yaml:"moveToTop"`
		MoveToBottom     Key `yaml:"moveToBottom"`
	}

	HistoryKeys struct {
		PurgeHistory Key `yaml:"purgeHistory"`
		CopyQuery    Key `yaml:"copyQuery"`
	}

	IndexAddFormKeys struct {
		ToggleSQLMode Key `yaml:"toggleSQLMode"`
		AddColumn     Key `yaml:"addColumn"`
	}

	StructureKeys struct {
		RenameColumn Key `yaml:"renameColumn"`
	}

	SQLQueryEditorKeys struct {
		Expand      Key `yaml:"expand"`
		OpenHistory Key `yaml:"openHistory"`
	}
)

// DataKeysForQueryMode returns the subset of DataKeys that are meaningful in
// QueryMode (read-only results table — no CRUD, no filter/sort bars).
func (kb *KeyBindings) DataKeysForQueryMode() []Key {
	d := kb.Data
	return []Key{
		d.PeekRow, d.FullPagePeek,
		kb.Common.Copy, d.CopyRow,
		kb.Common.Refresh, d.NextPage, d.PreviousPage,
		d.ExplainQuery,
		d.ExportData,
	}
}

// keyGroupParents defines optional single-parent inheritance for key groups.
// GetKeysForElement prepends the parent's keys before the child's own keys.
var keyGroupParents = map[string]string{} // eg: "ChildKeys": "ParentKeys"

// componentCommonKeys lists which CommonKeys fields each component exposes in
// the footer. Only the named fields are shown — this avoids displaying
// irrelevant common keys (e.g. Execute in the schema tree).
var componentCommonKeys = map[string][]string{
	"Data":           {"Add", "Edit", "Delete", "Copy", "Filter", "Refresh"},
	"Schema":         {"Add", "Delete", "Filter"},
	"Peeker":         {"Copy", "Close"},
	"Index":          {"Add", "Delete", "Save", "Close"},
	"Structure":      {"Refresh"},
	"History":        {"Select", "Delete", "Close"},
	"SQLQueryEditor": {"Execute", "Clear", "Paste"},
	"InputBar":       {"Execute", "Clear", "Paste", "Close"},
}

const keybindingsFileHeader = `# runes: literal characters, case-sensitive (e.g. [a], [A])
# keys:  named/combo keys (e.g. [Enter], [Esc], [Tab], [Space])
#        Ctrl+<letter>: case-insensitive in config, but no Ctrl+Shift — use lowercase (e.g. Ctrl+l)
#        Alt+<char>:    case-sensitive, both upper and lower work (e.g. Alt+a, Alt+A)

`

func LoadKeybindings() (*KeyBindings, error) {
	defaultKeybindings := &KeyBindings{}
	defaultKeybindings.loadDefaults()

	if os.Getenv("ENV") == "vi-dev" {
		return defaultKeybindings, nil
	}

	keybindingsPath, err := getKeybindingsPath()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(keybindingsPath); os.IsNotExist(err) {
		if err := writeKeybindingsWithHeader(defaultKeybindings, keybindingsPath); err != nil {
			return nil, err
		}
		return defaultKeybindings, nil
	}

	return util.LoadConfigFile(defaultKeybindings, keybindingsPath)
}

func writeKeybindingsWithHeader(kb *KeyBindings, path string) error {
	data, err := yaml.Marshal(kb)
	if err != nil {
		return fmt.Errorf("failed to marshal keybindings: %w", err)
	}
	content := append([]byte(keybindingsFileHeader), data...)
	return os.WriteFile(path, content, FileMode)
}

func extractKeysFromStruct(val reflect.Value) []Key {
	var keys []Key

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		if field.Type() == reflect.TypeOf(Key{}) {
			keys = append(keys, field.Interface().(Key))
		} else if field.Kind() == reflect.Struct {
			keys = append(keys, extractKeysFromStruct(field)...)
		}
	}

	return keys
}

func (kb KeyBindings) GetAvailableKeys() []OrderedKeys {
	var keys []OrderedKeys

	v := reflect.ValueOf(kb)
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldName := t.Field(i).Name

		orderedKeys := OrderedKeys{
			Element: fieldName,
			Keys:    extractKeysFromStruct(field),
		}

		keys = append(keys, orderedKeys)
	}

	return keys
}

func (kb KeyBindings) GetKeysForElement(elementId string) ([]OrderedKeys, error) {
	if elementId == "" {
		return nil, fmt.Errorf("element is empty")
	}

	v := reflect.ValueOf(kb)
	field := v.FieldByName(elementId)

	if !field.IsValid() || field.Kind() != reflect.Struct {
		return nil, fmt.Errorf("field %s not found", elementId)
	}

	var result []OrderedKeys
	if parent, ok := keyGroupParents[elementId]; ok {
		if parentKeys, err := kb.GetKeysForElement(parent); err == nil {
			result = append(result, parentKeys...)
		}
	}

	if commonKeys := kb.GetCommonKeysFor(elementId); len(commonKeys) > 0 {
		result = append(result, OrderedKeys{
			Element: "Common",
			Keys:    commonKeys,
		})
	}

	result = append(result, OrderedKeys{
		Element: elementId,
		Keys:    extractKeysFromStruct(field),
	})

	return result, nil
}

// GetCommonKeysFor returns the Common keys declared for the given component in
// componentCommonKeys. Returns nil if no mapping exists.
func (kb KeyBindings) GetCommonKeysFor(elementId string) []Key {
	names, ok := componentCommonKeys[elementId]
	if !ok {
		return nil
	}
	cv := reflect.ValueOf(kb.Common)
	ct := cv.Type()
	var keys []Key
	for i := 0; i < cv.NumField(); i++ {
		if sliceContains(names, ct.Field(i).Name) {
			keys = append(keys, cv.Field(i).Interface().(Key))
		}
	}
	return keys
}

func sliceContains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func (kb *KeyBindings) ConvertStrKeyToTcellKey(key string) (tcell.Key, bool) {
	// Normalize our config format to tcell's KeyNames format.
	// Config uses "Ctrl+N" (plus), tcell.KeyNames uses "Ctrl-N" (hyphen).
	if strings.HasPrefix(key, "Ctrl+") && len(key) == 6 {
		key = "Ctrl-" + strings.ToUpper(string(key[5]))
	}
	for k, v := range tcell.KeyNames {
		if v == key {
			return k, true
		}
	}
	return -1, false
}

func (kb *KeyBindings) Contains(configKey Key, namedKey string) bool {
	if namedKey == "Rune[ ]" {
		namedKey = "Space"
	}

	// Normalize Ctrl+letter to uppercase since tcell always reports uppercase,
	// allowing config to use lowercase (e.g. "Ctrl+l") for user clarity
	if strings.HasPrefix(namedKey, "Ctrl+") && len(namedKey) == 6 {
		namedKey = "Ctrl+" + strings.ToUpper(string(namedKey[5]))
	}
	if strings.HasPrefix(namedKey, "Alt+Rune[") && len(namedKey) >= 10 {
		runeChar := namedKey[9:10]
		altCombo := "Alt+" + runeChar

		for _, k := range configKey.Keys {
			if k == altCombo {
				return true
			}
		}
		return false
	}

	if strings.HasPrefix(namedKey, "Rune") {
		namedKey = strings.TrimPrefix(namedKey, "Rune")
		for _, k := range configKey.Runes {
			if k == namedKey[1:2] {
				return true
			}
		}
	}

	for _, k := range configKey.Keys {
		// Normalize Ctrl+letter to uppercase to match tcell's key naming
		if strings.HasPrefix(k, "Ctrl+") && len(k) == 6 {
			k = "Ctrl+" + strings.ToUpper(string(k[5]))
		}
		if k == namedKey {
			return true
		}
	}

	return false
}

func (k *Key) String() string {
	var parts []string
	parts = append(parts, k.Keys...)
	parts = append(parts, k.Runes...)
	return strings.Join(parts, ", ")
}

func (kb *KeyBindings) SetKeyAt(element string, idx int, key Key) error {
	v := reflect.ValueOf(kb).Elem()
	field := v.FieldByName(element)
	if !field.IsValid() || field.Kind() != reflect.Struct {
		return fmt.Errorf("element %s not found", element)
	}

	count := 0
	if !setKeyAtIdx(field, idx, key, &count) {
		return fmt.Errorf("key at index %d not found in %s", idx, element)
	}
	return nil
}

// setKeyAtIdx recursively mirrors extractKeysFromStruct so indices always match.
func setKeyAtIdx(val reflect.Value, idx int, key Key, count *int) bool {
	for i := 0; i < val.NumField(); i++ {
		f := val.Field(i)
		if f.Type() == reflect.TypeOf(Key{}) {
			if *count == idx {
				f.Set(reflect.ValueOf(key))
				return true
			}
			(*count)++
		} else if f.Kind() == reflect.Struct {
			if setKeyAtIdx(f, idx, key, count) {
				return true
			}
		}
	}
	return false
}

func (kb *KeyBindings) SaveKeybindings() error {
	path, err := getKeybindingsPath()
	if err != nil {
		return err
	}
	return writeKeybindingsWithHeader(kb, path)
}

func getKeybindingsPath() (string, error) {
	configDir, err := util.GetConfigDir()
	if err != nil {
		return "", err
	}

	return configDir + "/keybindings.yaml", nil
}
