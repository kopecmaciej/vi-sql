package widget

import (
	"testing"

	"github.com/kopecmaciej/tview"
	"github.com/stretchr/testify/assert"
)

type mockPrimitive struct{ tview.Box }

func (m *mockPrimitive) Render() {}

func TestTabBar_SwitchToTabByName(t *testing.T) {
	tests := []struct {
		name     string
		tabName  string
		tabKind  TabKind
		wantFind bool
	}{
		{"exact match", "users", KindStructure, true},
		{"same name wrong kind", "users", KindView, false},
		{"different name", "orders", KindStructure, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bar := NewTabBar()
			bar.AddTab("users", &mockPrimitive{}, KindStructure, true)
			bar.AddTab("posts", &mockPrimitive{}, KindTable, false)

			found := bar.SwitchToTabByName(tt.tabName, tt.tabKind)
			assert.Equal(t, tt.wantFind, found)
		})
	}
}
