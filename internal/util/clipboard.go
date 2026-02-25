package util

import "github.com/atotto/clipboard"

// GetClipboard returns write/read functions compatible with tview's
// SetClipboard(func(string), func() string) signature.
func GetClipboard() (func(string), func() string) {
	writeFunc := func(text string) {
		_ = clipboard.WriteAll(text)
	}
	readFunc := func() string {
		text, _ := clipboard.ReadAll()
		return text
	}
	return writeFunc, readFunc
}
