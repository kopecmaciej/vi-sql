//go:build wezterm

package harness

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/atotto/clipboard"
)

// weztermCheckAvailable returns an error if `wezterm cli list` fails,
// which means we're either not inside wezterm or it's not in PATH.
func weztermCheckAvailable() error {
	cmd := exec.Command("wezterm", "cli", "list")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("wezterm cli unavailable (run inside wezterm): %w", err)
	}
	return nil
}

// weztermSpawn launches binary with args in a new wezterm pane and returns
// the numeric pane-id printed on stdout.
func weztermSpawn(binary string, args []string) (string, error) {
	cmdArgs := []string{"cli", "spawn", "--"}
	cmdArgs = append(cmdArgs, binary)
	cmdArgs = append(cmdArgs, args...)
	out, err := exec.Command("wezterm", cmdArgs...).Output()
	if err != nil {
		return "", fmt.Errorf("wezterm cli spawn: %w", err)
	}
	paneID := strings.TrimSpace(string(out))
	if paneID == "" {
		return "", fmt.Errorf("wezterm cli spawn returned empty pane-id")
	}
	return paneID, nil
}

// weztermSendText injects raw bytes into a pane, bypassing bracketed-paste mode.
// Text is passed via stdin so NUL bytes (e.g. Ctrl+Space → \x00) aren't rejected
// by the OS exec layer, which disallows NUL in argv strings.
func weztermSendText(paneID, text string) error {
	cmd := exec.Command("wezterm", "cli", "send-text", "--pane-id", paneID, "--no-paste")
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("wezterm send-text (pane %s): %w", paneID, err)
	}
	return nil
}

// weztermGetText returns the visible text content of the pane.
func weztermGetText(paneID string) (string, error) {
	var out bytes.Buffer
	cmd := exec.Command("wezterm", "cli", "get-text", "--pane-id", paneID)
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("wezterm get-text (pane %s): %w", paneID, err)
	}
	return out.String(), nil
}

// weztermKillPane closes the pane; errors are ignored (best-effort cleanup).
func weztermKillPane(paneID string) error {
	return exec.Command("wezterm", "cli", "kill-pane", "--pane-id", paneID).Run()
}

// weztermSetClipboard writes text to the system clipboard using atotto/clipboard,
// which handles macOS (pbcopy), Wayland (wl-copy), and X11 (xclip/xsel).
func weztermSetClipboard(text string) error {
	return clipboard.WriteAll(text)
}
