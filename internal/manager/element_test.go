package manager

import (
	"testing"
)

func TestElementManager_StopIdempotent(t *testing.T) {
	em := NewElementManager()
	em.Stop()
	em.Stop()
}
