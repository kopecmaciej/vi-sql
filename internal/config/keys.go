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
		Navigation   NavigationKeys   `yaml:"navigation"`
		Global       GlobalKeys       `yaml:"global"`
		Help         HelpKeys         `yaml:"help"`
		Connection   ConnectionKeys   `yaml:"connection"`
		Schema       SchemaKeys       `yaml:"schema"`
		InputBar     InputBarKeys     `yaml:"inputBar"`
		Content      ContentKeys      `yaml:"content"`
		Peeker       PeekerKeys       `yaml:"peeker"`
		QueryBar     QueryBar         `yaml:"queryBar"`
		Index        IndexKeys        `yaml:"index"`
		IndexAddForm IndexAddFormKeys `yaml:"indexAddForm"`
		Structure    StructureKeys    `yaml:"structure"`
		History      HistoryKeys      `yaml:"history"`
		CreateTable  CreateTableKeys  `yaml:"createTable"`
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
		ServerInfo     Key `yaml:"serverInfo"`
		ToggleHeader   Key `yaml:"toggleHeader"`
		HideSchema     Key `yaml:"hideSchema"`
	}

	SchemaKeys struct {
		FilterBar   Key `yaml:"filterBar"`
		ClearFilter Key `yaml:"clearFilter"`
		ExpandAll   Key `yaml:"expandAll"`
		CollapseAll Key `yaml:"collapseAll"`
		AddTable    Key `yaml:"addTable"`
		DeleteTable Key `yaml:"deleteTable"`
		RenameTable Key `yaml:"renameTable"`
	}

	InputBarKeys struct {
		Exit       Key `yaml:"exit"`
		ClearInput Key `yaml:"clearInput"`
		Paste      Key `yaml:"paste"`
	}

	ContentKeys struct {
		PeekRow             Key `yaml:"peekRow"`
		FullPagePeek        Key `yaml:"fullPagePeek"`
		OpenEditor          Key `yaml:"openEditor"`
		AddRow              Key `yaml:"addRow"`
		EditRow             Key `yaml:"editRow"`
		InlineEdit          Key `yaml:"inlineEdit"`
		DuplicateRow        Key `yaml:"duplicateRow"`
		DeleteRow           Key `yaml:"deleteRow"`
		CopyValue           Key `yaml:"copyValue"`
		CopyRow             Key `yaml:"copyRow"`
		Refresh             Key `yaml:"refresh"`
		ToggleFilterBar     Key `yaml:"toggleFilterBar"`
		ToggleQueryBar      Key `yaml:"toggleQueryBar"`
		NextPage            Key `yaml:"nextPage"`
		PreviousPage        Key `yaml:"previousPage"`
		ToggleSortBar       Key `yaml:"toggleSortBar"`
		SortByColumn        Key `yaml:"sortByColumn"`
		HideColumn          Key `yaml:"hideColumn"`
		ResetHiddenColumns  Key `yaml:"resetHiddenColumns"`
		ToggleFilterOptions Key `yaml:"toggleFilterOptions"`
		MultipleSelect      Key `yaml:"multipleSelect"`
		ClearSelection      Key `yaml:"clearSelection"`
	}

	QueryBar struct {
		ShowHistory Key `yaml:"showHistory"`
	}

	ConnectionKeys struct {
		ConnectionForm ConnectionFormKeys `yaml:"connectionForm"`
		ConnectionList ConnectionListKeys `yaml:"connectionList"`
	}

	ConnectionFormKeys struct {
		SaveConnection Key `yaml:"saveConnection"`
	}

	ConnectionListKeys struct {
		AddConnection    Key `yaml:"addConnection"`
		DeleteConnection Key `yaml:"deleteConnection"`
		EditConnection   Key `yaml:"editConnection"`
		SetConnection    Key `yaml:"setConnection"`
	}

	HelpKeys struct {
		Close   Key `yaml:"close"`
		Search  Key `yaml:"search"`
		EditKey Key `yaml:"editKey"`
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
		ClearHistory Key `yaml:"clearHistory"`
		AcceptEntry  Key `yaml:"acceptEntry"`
		CloseHistory Key `yaml:"closeHistory"`
	}

	IndexKeys struct {
		AddIndex    Key `yaml:"addIndex"`
		DeleteIndex Key `yaml:"deleteIndex"`
	}

	IndexAddFormKeys struct {
		ExitForm      Key `yaml:"exitForm"`
		ToggleSQLMode Key `yaml:"toggleSQLMode"`
		AddColumn     Key `yaml:"addColumn"`
		CreateIndex   Key `yaml:"createIndex"`
	}

	StructureKeys struct {
		Refresh Key `yaml:"refresh"`
	}

	CreateTableKeys struct {
		AddColumn    Key `yaml:"addColumn"`
		DeleteColumn Key `yaml:"deleteColumn"`
		Execute      Key `yaml:"execute"`
		Cancel       Key `yaml:"cancel"`
	}
)

// keyGroupParents defines optional single-parent inheritance for key groups.
// GetKeysForElement prepends the parent's keys before the child's own keys,
// making the header and help page show the full effective key set.
var keyGroupParents = map[string]string{
	// "ChildKeys": "ParentKeys"
	"QueryBar": "InputBar",
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

	result = append(result, OrderedKeys{
		Element: elementId,
		Keys:    extractKeysFromStruct(field),
	})

	return result, nil
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
	if namedKey == "Backspace" {
		namedKey = "Ctrl+H"
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
	for i := 0; i < field.NumField(); i++ {
		f := field.Field(i)
		if f.Type() == reflect.TypeOf(Key{}) {
			if count == idx {
				f.Set(reflect.ValueOf(key))
				return nil
			}
			count++
		}
	}
	return fmt.Errorf("key at index %d not found in %s", idx, element)
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
