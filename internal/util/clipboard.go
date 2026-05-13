package util

import (
	"strings"

	"github.com/atotto/clipboard"
	"github.com/rs/zerolog/log"
)

var (
	ClipboardWrite func(string) error       = clipboard.WriteAll
	ClipboardRead  func() (string, error)   = clipboard.ReadAll
)

func Copy(text string) {
	if err := ClipboardWrite(text); err != nil {
		log.Error().Err(err).Msg("Error writing to clipboard")
	}
}

func Paste() string {
	text, err := ClipboardRead()
	if err != nil {
		log.Error().Err(err).Msg("Error reading from clipboard")
		return ""
	}
	return strings.TrimSpace(text)
}

// GetClipboard returns copy/paste funcs for use with tview's SetClipboard API.
func GetClipboard() (func(string), func() string) {
	return Copy, Paste
}
