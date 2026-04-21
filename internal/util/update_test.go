package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"v0.0.2", "0.0.2"},
		{"v0.1.0", "0.1.0"},
		{"v1.2.3", "1.2.3"},
		// dirty dev builds — everything after the first '-' is stripped
		{"v0.0.2-dirty", "0.0.2"},
		{"v0.0.1-15-g75060dc-dirty", "0.0.1"},
		{"v0.0.2-3-gabcdef0", "0.0.2"},
		// no 'v' prefix (e.g. stored in config after normalisation)
		{"0.0.2", "0.0.2"},
		{"0.1.0", "0.1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, NormalizeVersion(tt.input))
		})
	}
}

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
		want    bool
	}{
		// normal upgrades
		{"patch upgrade", "0.0.1", "0.0.2", true},
		{"minor upgrade", "0.0.2", "0.1.0", true},
		{"major upgrade", "0.9.9", "1.0.0", true},
		// no change
		{"equal", "0.0.2", "0.0.2", false},
		// downgrade
		{"downgrade patch", "0.0.2", "0.0.1", false},
		{"downgrade minor", "0.1.0", "0.0.9", false},
		// 'v' prefix handling
		{"v prefix current", "v0.0.1", "0.0.2", true},
		{"v prefix latest", "0.0.1", "v0.0.2", true},
		{"v prefix both", "v0.0.1", "v0.0.2", true},
		// dirty dev builds stripped before compare
		{"dirty current older", "v0.0.1-15-g75060dc-dirty", "0.0.2", true},
		{"dirty current equal", "v0.0.2-dirty", "0.0.2", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsNewerVersion(tt.current, tt.latest))
		})
	}
}

func TestChangelogVersionFlow(t *testing.T) {
	// Simulates the real-world upgrade scenario end-to-end using only util functions.
	// A user upgrades from configVersion to buildVersion; the changelog has one entry.

	tests := []struct {
		name          string
		configVersion string // what is stored in config.yaml
		buildVersion  string // injected via ldflags at build time
		entryVersion  string // version field in changelog.yaml
		shouldShow    bool   // should the changelog modal appear?
		storedAfter   string // what NormalizeVersion(buildVersion) produces
	}{
		{
			name:          "normal upgrade: 0.0.1 → 0.0.2",
			configVersion: "0.0.1",
			buildVersion:  "v0.0.2",
			entryVersion:  "0.0.2",
			shouldShow:    true,
			storedAfter:   "0.0.2",
		},
		{
			name:          "dirty build on new tag",
			configVersion: "0.0.1",
			buildVersion:  "v0.0.2-dirty",
			entryVersion:  "0.0.2",
			shouldShow:    true,
			storedAfter:   "0.0.2",
		},
		{
			name:          "already up to date",
			configVersion: "0.0.2",
			buildVersion:  "v0.0.2",
			entryVersion:  "0.0.2",
			shouldShow:    false,
			storedAfter:   "0.0.2",
		},
		{
			name:          "dev build below entry — shows every run (expected)",
			configVersion: "0.0.1",
			buildVersion:  "v0.0.1-14-gabcdef0-dirty",
			entryVersion:  "0.0.2",
			shouldShow:    true,
			storedAfter:   "0.0.1",
		},
		{
			name:          "after acknowledging on dev build — still shows (expected)",
			configVersion: "0.0.1", // stored after previous dev run
			buildVersion:  "v0.0.1-15-g1234567-dirty",
			entryVersion:  "0.0.2",
			shouldShow:    true,  // dev below entry, keeps showing until tag is cut
			storedAfter:   "0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shows := IsNewerVersion(tt.configVersion, tt.entryVersion)
			assert.Equal(t, tt.shouldShow, shows, "modal visibility mismatch")

			stored := NormalizeVersion(tt.buildVersion)
			assert.Equal(t, tt.storedAfter, stored, "stored version mismatch")

			// After acknowledging: verify no second show
			if tt.shouldShow {
				showsAgain := IsNewerVersion(stored, tt.entryVersion)
				if tt.storedAfter >= tt.entryVersion {
					assert.False(t, showsAgain, "modal should not show again after acknowledging")
				}
			}
		})
	}
}
