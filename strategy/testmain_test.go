package strategy

import (
	"os"
	"testing"
)

// Strategy unit tests exercise Falcon/Score math — run under Live.
// ChartOnly gate behavior is covered explicitly in engine_mode_test.go.
func TestMain(m *testing.M) {
	SetEngineMode(EngineModeLive)
	os.Exit(m.Run())
}
