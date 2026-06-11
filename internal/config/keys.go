package config

import (
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/vi-sql/internal/util"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

type (
	Key struct {
		Keys        []string `yaml:"keys,omitempty,flow"`
		Runes       []string `yaml:"runes,omitempty,flow"`
		Sequences   []string `yaml:"sequences,omitempty,flow"`
		Description string   `yaml:"description"`
	}

	OrderedKeys struct {
		Element string
		Keys    []Key
	}

	KeyBindings struct {
		sequenceState  `yaml:"-"`
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
		Confirm Key `yaml:"confirm"`
		Filter  Key `yaml:"filter"`
		Select  Key `yaml:"select"`
		Refresh Key `yaml:"refresh"`
		Clear   Key `yaml:"clear"`
		Paste   Key `yaml:"paste"`
	}

	NavigationKeys struct {
		MoveUp             Key `yaml:"moveUp"`
		MoveDown           Key `yaml:"moveDown"`
		MoveLeft           Key `yaml:"moveLeft"`
		MoveRight          Key `yaml:"moveRight"`
		GoTop              Key `yaml:"goTop"`
		GoBottom           Key `yaml:"goBottom"`
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
		RenameTab       Key `yaml:"renameTab"`
		FocusSchemaTree Key `yaml:"focusSchemaTree"`
		OpenActions     Key `yaml:"openActions"`
		ImportData      Key `yaml:"importData"`
		GoToTable       Key `yaml:"goToTable"`
	}

	SchemaKeys struct {
		ExpandAll     Key `yaml:"expandAll"`
		CollapseAll   Key `yaml:"collapseAll"`
		RenameTable   Key `yaml:"renameTable"`
		ExpandTable   Key `yaml:"expandTable"`
		OpenStructure Key `yaml:"openStructure"`
		OpenIndexes   Key `yaml:"openIndexes"`
	}

	DataKeys struct {
		PeekRow            Key `yaml:"peekRow"`
		FullPagePeek       Key `yaml:"fullPagePeek"`
		EditRow            Key `yaml:"editRow"`
		DuplicateRow       Key `yaml:"duplicateRow"`
		CopyCell           Key `yaml:"copyCell"`
		CopyRow            Key `yaml:"copyRow"`
		CopyRowJSON        Key `yaml:"copyRowJSON"`
		CopyRowCSV         Key `yaml:"copyRowCSV"`
		ToggleOrderBar     Key `yaml:"toggleOrderBar"`
		OrderByColumn      Key `yaml:"orderByColumn"`
		HideColumn         Key `yaml:"hideColumn"`
		ResetHiddenColumns Key `yaml:"resetHiddenColumns"`
		MultipleSelect     Key `yaml:"multipleSelect"`
		ClearSelection     Key `yaml:"clearSelection"`
		ExplainQuery       Key `yaml:"explainQuery"`
		ExportData         Key `yaml:"exportData"`
		FollowForeignKey   Key `yaml:"followForeignKey"`
		FindReferences     Key `yaml:"findReferences"`
	}

	ExplainViewerKeys struct {
		ToggleMode Key `yaml:"toggleMode"`
	}

	PeekerKeys struct {
		CopyHighlight    Key `yaml:"copyHighlight"`
		ExpandRow        Key `yaml:"expandRow"`
		OpenValueViewer  Key `yaml:"openValueViewer"`
		ToggleFullScreen Key `yaml:"toggleFullScreen"`
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
		RenameColumn  Key `yaml:"renameColumn"`
		ToggleDDLPane Key `yaml:"toggleDDLPane"`
	}

	SQLQueryEditorKeys struct {
		Fullscreen  Key `yaml:"fullscreen"`
		OpenHistory Key `yaml:"openHistory"`
		TermEditor  Key `yaml:"termEditor"`
	}
)

// DataKeysSplit returns DataKeys divided for help display: queryMode keys work
// in both QueryMode and TableMode; tableOnly are exclusive to TableMode.
func (kb *KeyBindings) DataKeysSplit() (queryMode, tableOnly []Key) {
	d := kb.Data
	queryMode = []Key{d.PeekRow, d.FullPagePeek, d.CopyCell, d.CopyRow, d.CopyRowJSON, d.CopyRowCSV, d.MultipleSelect, d.ClearSelection, d.ExplainQuery, d.ExportData}
	tableOnly = []Key{d.EditRow, d.DuplicateRow, d.ToggleOrderBar, d.OrderByColumn, d.HideColumn, d.ResetHiddenColumns, d.FollowForeignKey, d.FindReferences}
	return
}

// DataKeysForQueryMode returns the subset of DataKeys that are meaningful in
// QueryMode (read-only results table — no CRUD, no filter/sort bars).
func (kb *KeyBindings) DataKeysForQueryMode() []Key {
	d := kb.Data
	return []Key{
		d.PeekRow, d.FullPagePeek,
		d.CopyCell, d.CopyRow, d.CopyRowJSON, d.CopyRowCSV,
		kb.Common.Refresh,
		d.MultipleSelect, d.ClearSelection,
		d.ExplainQuery,
		d.ExportData,
	}
}

func (kb *KeyBindings) ReloadKeybidings(newKeys *KeyBindings) {
	onPendingChanged := kb.OnPendingChanged
	sequencesDisabled := kb.SequencesDisabled
	*kb = *newKeys
	kb.OnPendingChanged = onPendingChanged
	kb.SequencesDisabled = sequencesDisabled
}

// keyGroupParents defines optional single-parent inheritance for key groups.
// GetKeysForElement prepends the parent's keys before the child's own keys.
var keyGroupParents = map[string]string{} // eg: "ChildKeys": "ParentKeys"

// componentCommonKeys lists which CommonKeys fields each component exposes in
// the footer. Only the named fields are shown — this avoids displaying
// irrelevant common keys (e.g. Confirm in the schema tree).
var componentCommonKeys = map[string][]string{
	"Data":            {"Add", "Edit", "Delete", "Filter", "Refresh"},
	"Schema":          {"Add", "Delete", "Copy", "Filter", "Refresh"},
	"Peeker":          {"Copy", "Close"},
	"Index":           {"Add", "Delete"},
	"IndexAddForm":    {"Confirm", "Close"},
	"Structure":       {"Refresh", "Copy"},
	"History":         {"Select", "Delete", "Close"},
	"SQLQueryEditor":  {"Confirm", "Clear", "Paste"},
	"InputBar":        {"Confirm", "Clear", "Paste", "Close"},
	"InlineEditModal": {"Confirm", "Close"},
	"ServerInfo":      {"Close", "Refresh"},
	"CreateTable":     {"Add", "Delete", "Copy", "Confirm", "Close"},
}

const keybindingsFileHeaderVim = `# Profile: vim
# runes:  literal characters, case-sensitive (e.g. [a], [A])
# keys:   named/combo keys (e.g. [Enter], [Esc], [Tab], [Space], [Ctrl+Space])
#         Ctrl+<letter>: case-insensitive in config, use lowercase (e.g. Ctrl+l)
#         Alt+<char>:    case-sensitive, both upper and lower work (e.g. Alt+a)
# sequences: multi-rune vim sequences (e.g. [gg, gd, yrj]) — only active when vim mode is on

`

const keybindingsFileHeaderNormal = `# Profile: normal
# runes:  literal characters, case-sensitive (e.g. [a], [A])
# keys:   named/combo keys (e.g. [Enter], [Esc], [Tab], [Space], [Ctrl+Space])
#         Ctrl+<letter>: case-insensitive in config, use lowercase (e.g. Ctrl+l)
#         Alt+<char>:    case-sensitive, both upper and lower work (e.g. Alt+a)

`

func LoadKeybindings(vimMode bool) (*KeyBindings, error) {
	defaultKeybindings := &KeyBindings{}
	defaultKeybindings.loadDefaults(vimMode)
	defaultKeybindings.vimMode = vimMode

	if os.Getenv("ENV") == "vi-dev" {
		defaultKeybindings.buildSequencePrefixes()
		return defaultKeybindings, nil
	}

	keybindingsPath, err := getKeybindingsPath(vimMode)
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(keybindingsPath); os.IsNotExist(err) {
		if err := writeKeybindingsWithHeader(defaultKeybindings, keybindingsPath); err != nil {
			return nil, err
		}
		defaultKeybindings.buildSequencePrefixes()
		return defaultKeybindings, nil
	}

	loaded, err := util.LoadConfigFile(defaultKeybindings, keybindingsPath)
	if err != nil {
		return nil, err
	}
	loaded.vimMode = vimMode
	loaded.buildSequencePrefixes()
	return loaded, nil
}

func writeKeybindingsWithHeader(kb *KeyBindings, path string) error {
	data, err := yaml.Marshal(kb)
	if err != nil {
		return fmt.Errorf("failed to marshal keybindings: %w", err)
	}
	header := keybindingsFileHeaderNormal
	if kb.vimMode {
		header = keybindingsFileHeaderVim
	}
	content := append([]byte(header), data...)
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
		if field.Kind() != reflect.Struct {
			continue
		}
		fieldName := t.Field(i).Name

		extracted := extractKeysFromStruct(field)
		if len(extracted) == 0 {
			continue
		}
		keys = append(keys, OrderedKeys{Element: fieldName, Keys: extracted})
	}

	return keys
}

func (kb KeyBindings) GetKeysForElement(elementId string) ([]OrderedKeys, error) {
	if elementId == "" {
		return nil, fmt.Errorf("element is empty")
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

	v := reflect.ValueOf(kb)
	if field := v.FieldByName(elementId); field.IsValid() && field.Kind() == reflect.Struct {
		result = append(result, OrderedKeys{
			Element: elementId,
			Keys:    extractKeysFromStruct(field),
		})
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no keys found for element %s", elementId)
	}

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
		if slices.Contains(names, ct.Field(i).Name) {
			keys = append(keys, cv.Field(i).Interface().(Key))
		}
	}
	return keys
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

// normalizeNamedKey converts tcell's EventKey.Name() to the canonical config
// format. Returns the normalized string and whether it's a bare rune (no modifiers).
// Handles quirks in tcell's Name() output: space arrives as "Rune[ ]", Alt/Ctrl
// combos with runes arrive as "Alt+Rune[x]"/"Ctrl+Rune[x]" instead of "Alt+x"/"Ctrl+x".
func normalizeNamedKey(namedKey string) (normalized string, isRune bool) {
	if namedKey == "Rune[ ]" {
		return "Space", false
	}
	if strings.HasPrefix(namedKey, "Alt+Rune[") && len(namedKey) >= 11 {
		return "Alt+" + namedKey[9:10], false
	}
	if strings.HasPrefix(namedKey, "Ctrl+Rune[") && len(namedKey) >= 12 {
		r := namedKey[10 : len(namedKey)-1]
		if r == " " {
			r = "Space"
		}
		return "Ctrl+" + r, false
	}
	if strings.HasPrefix(namedKey, "Rune[") && len(namedKey) >= 7 {
		return namedKey[5 : len(namedKey)-1], true
	}
	if strings.HasPrefix(namedKey, "Ctrl+") && len(namedKey) == 6 {
		return "Ctrl+" + strings.ToUpper(string(namedKey[5])), false
	}
	return namedKey, false
}

// Match reports whether ev should fire configKey's handler. When a sequence
// prefix is pending, matches the accumulated prefix + incoming rune against
// configKey.Sequences; otherwise checks the Keys/Runes lists. Must be used
// inside a handler wrapped with WrapInputCapture so sequence state is maintained.
func (kb *KeyBindings) Match(configKey Key, ev *tcell.EventKey) bool {
	if ev.Key() == tcell.KeyRune && kb.pending != "" {
		seq := kb.pending + string(ev.Rune())
		return slices.Contains(configKey.Sequences, seq)
	}
	return kb.contains(configKey, ev.Name())
}

// contains reports whether namedKey (tcell EventKey.Name()) matches any
// single-event Keys/Runes entry in configKey. Sequence matching is in Match.
func (kb *KeyBindings) contains(configKey Key, namedKey string) bool {
	normalized, isRune := normalizeNamedKey(namedKey)

	if isRune {
		return slices.Contains(configKey.Runes, normalized)
	}

	for _, k := range configKey.Keys {
		if strings.HasPrefix(k, "Ctrl+") && len(k) == 6 {
			k = "Ctrl+" + strings.ToUpper(string(k[5]))
		}
		if k == normalized {
			return true
		}
	}
	return false
}

// buildSequencePrefixes scans every Key in the bindings and records all proper
// prefixes of every sequence so WrapInputCapture knows which rune strings to
// absorb during prefix extension. Vim mode only — leaves the map nil otherwise.
// Logs a warning for any complete sequence that is also a proper prefix of
// another (shadowing), since the shorter sequence wins at match time.
func (kb *KeyBindings) buildSequencePrefixes() {
	kb.sequencePrefixes = nil
	if !kb.vimMode {
		return
	}
	prefixes := make(map[string]struct{})
	var completeSeqs []string
	for _, group := range kb.GetAvailableKeys() {
		for _, k := range group.Keys {
			completeSeqs = append(completeSeqs, k.Sequences...)
			for _, seq := range k.Sequences {
				runes := []rune(seq)
				for i := 1; i < len(runes); i++ {
					prefixes[string(runes[:i])] = struct{}{}
				}
			}
		}
	}
	for _, seq := range completeSeqs {
		if _, ok := prefixes[seq]; ok {
			log.Warn().Str("sequence", seq).Msg("keybinding sequence is shadowed by a longer sequence")
		}
	}
	kb.sequencePrefixes = prefixes
}

func (k *Key) String() string {
	var parts []string
	parts = append(parts, k.Keys...)
	parts = append(parts, k.Runes...)
	for _, seq := range k.Sequences {
		parts = append(parts, "<"+seq+">")
	}
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
	path, err := getKeybindingsPath(kb.vimMode)
	if err != nil {
		return err
	}
	return writeKeybindingsWithHeader(kb, path)
}

func getKeybindingsPath(vimMode bool) (string, error) {
	configDir, err := util.GetConfigDir()
	if err != nil {
		return "", err
	}
	if vimMode {
		return configDir + "/keybindings-vim.yaml", nil
	}
	return configDir + "/keybindings-normal.yaml", nil
}
