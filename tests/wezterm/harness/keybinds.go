//go:build wezterm

package harness

import (
	"os"

	"github.com/kopecmaciej/vi-sql/internal/config"
	"gopkg.in/yaml.v3"
)

// configVimMode reads UI.VimMode from the vi-sql config YAML at configPath.
// If configPath is empty, config.GetConfigPath() is used (platform-aware).
// Returns false when the file is missing or the field is absent.
func configVimMode(configPath string) bool {
	if configPath == "" {
		var err error
		configPath, err = config.GetConfigPath()
		if err != nil {
			return false
		}
	}
	var c struct {
		UI struct {
			VimMode bool `yaml:"vimMode"`
		} `yaml:"ui"`
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return false
	}
	_ = yaml.Unmarshal(data, &c)
	return c.UI.VimMode
}

// loadKeyBindings returns the resolved KeyBindings for the given vim mode,
// honoring any user remaps in the keybindings file. Returns nil on error.
func loadKeyBindings(vimMode bool) *config.KeyBindings {
	kb, err := config.LoadKeybindings(vimMode)
	if err != nil {
		return nil
	}
	return kb
}

// extractConfigArg scans args for --config/-c and returns the value, or "".
func extractConfigArg(args []string) string {
	for i, a := range args {
		if (a == "--config" || a == "-c") && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// sendableKeys returns the key name(s) to pass to Session.Send for the given
// binding. Keys take priority over Runes; Sequences are split into individual
// rune strings so each rune can be injected separately.
func sendableKeys(k config.Key) []string {
	if len(k.Keys) > 0 {
		return []string{k.Keys[0]}
	}
	if len(k.Runes) > 0 {
		return []string{k.Runes[0]}
	}
	if len(k.Sequences) > 0 {
		runes := []rune(k.Sequences[0])
		out := make([]string, len(runes))
		for i, r := range runes {
			out[i] = string(r)
		}
		return out
	}
	return nil
}
