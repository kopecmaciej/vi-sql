//go:build wezterm

package harness

import (
	"os"

	"github.com/kopecmaciej/vi-sql/internal/config"
	"gopkg.in/yaml.v3"
)

func configVimMode(configPath string) bool {
	if configPath == "" {
		return false
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

func loadKeyBindings(vimMode bool) *config.KeyBindings {
	kb, err := config.LoadKeybindings(vimMode)
	if err != nil {
		return nil
	}
	return kb
}

func extractConfigArg(args []string) string {
	for i, a := range args {
		if (a == "--config" || a == "-c") && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

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
