//go:build wezterm

package harness

import (
	"fmt"
	"strings"
)

// Encode converts a human-readable key name to the raw byte sequence that
// wezterm cli send-text --no-paste should inject to reproduce that keypress.
//
// Names mirror vi-sql's keybindings.json format ("Enter", "Ctrl+t", "/", etc.).
// Single rune strings (e.g. "j", "/", "E") are returned as-is.
func Encode(name string) (string, error) {
	if v, ok := specialKeys[name]; ok {
		return v, nil
	}
	// Ctrl+<letter>: strip bit 6 of the letter to get the control byte.
	if strings.HasPrefix(name, "Ctrl+") && len(name) == 6 {
		ch := name[5]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') {
			return string([]byte{ch & 0x1f}), nil
		}
	}
	// Alt+<single-char>: ESC prefix.
	if strings.HasPrefix(name, "Alt+") && len(name) == 5 {
		return "\x1b" + string(name[4]), nil
	}
	// Single rune (letter, digit, punctuation).
	if len([]rune(name)) == 1 {
		return name, nil
	}
	return "", fmt.Errorf("unknown key: %q", name)
}

// specialKeys maps key names to their terminal byte sequences.
// Ctrl+/ sends \x1f (same code as Ctrl+_, KeyCtrlUnderscore in tcell).
// If Ctrl+/ focus-schema-tree doesn't respond, try replacing \x1f with \x0f.
var specialKeys = map[string]string{
	"Enter":     "\r",
	"Esc":       "\x1b",
	"Tab":       "\t",
	"Backtab":   "\x1b[Z",
	"Space":     " ",
	"Backspace": "\x7f",
	"Up":        "\x1b[A",
	"Down":      "\x1b[B",
	"Right":     "\x1b[C",
	"Left":      "\x1b[D",
	// Ctrl + non-letter specials
	"Ctrl+/":     "\x1f",
	"Ctrl+Space": "\x00",
	// Function keys (xterm/VT100 sequences)
	"F1":  "\x1bOP",
	"F2":  "\x1bOQ",
	"F3":  "\x1bOR",
	"F4":  "\x1bOS",
	"F5":  "\x1b[15~",
	"F6":  "\x1b[17~",
	"F7":  "\x1b[18~",
	"F8":  "\x1b[19~",
	"F9":  "\x1b[20~",
	"F10": "\x1b[21~",
	"F11": "\x1b[23~",
	"F12": "\x1b[24~",
}
