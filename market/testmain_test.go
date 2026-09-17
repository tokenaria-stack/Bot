package market

import (
	"os"
	"testing"
)

// Market package tests default to Live so RSX internal Core demand matches
// production Live Frames. ChartOnly gates are covered in engine_mode tests.
func TestMain(m *testing.M) {
	SetEngineMode(EngineModeLive)
	os.Exit(m.Run())
}
