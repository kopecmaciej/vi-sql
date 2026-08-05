package component

import (
	"testing"

	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/stretchr/testify/assert"
)

func TestFilterRowsBySearch_VisibleColumnsOnly(t *testing.T) {
	cols := []database.ColumnInfo{
		{Name: "id", DataType: "int"},
		{Name: "secret", DataType: "text"},
	}
	rows := []database.Row{
		{"id": "1", "secret": "password123"},
		{"id": "2", "secret": "other"},
		{"id": "3", "secret": "x"},
	}

	got := filterRowsBySearch(rows, "2", cols, []string{"id"})
	assert.Len(t, got, 1)
	assert.Equal(t, "2", got[0]["id"])

	got = filterRowsBySearch(rows, "password123", cols, []string{"id"})
	assert.Len(t, got, 0)

	got = filterRowsBySearch(rows, "password123", cols, []string{"id", "secret"})
	assert.Len(t, got, 1)
}

func TestFilterRowsBySearch_BooleanNormalization(t *testing.T) {
	cols := []database.ColumnInfo{{Name: "active", DataType: "boolean"}}
	rows := []database.Row{{"active": "t"}, {"active": "f"}}
	got := filterRowsBySearch(rows, "true", cols, []string{"active"})
	assert.Len(t, got, 1)
	assert.Equal(t, "t", got[0]["active"])
}
