package server

import (
	"os"
	"strings"
	"sync/atomic"
)

// Dormant investigation probes (ADR-009 / ADR-015). Default OFF — no runtime spam.
// Enable when diagnosing tip/projection ownership for a new indicator:
//
//	DEBUG_TIP_SSOT=1   → [TipSSOT] on history fetch + /api/debug/tip-ssot
//	DEBUG_PROJ_CONT=1  → projCont JSON on columnar + [ProjCont] server log
//
// Permanent diagnostics (TransportDiag / Self-Healing / MemoryBudget) are unrelated
// and stay always-on in the frontend.

var (
	debugTipSSOT  atomic.Bool
	debugProjCont atomic.Bool
)

func init() {
	debugTipSSOT.Store(envDebugTruthy("DEBUG_TIP_SSOT"))
	debugProjCont.Store(envDebugTruthy("DEBUG_PROJ_CONT"))
}

func envDebugTruthy(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// DebugTipSSOT reports whether TipSSOT continuous probe + debug endpoint are enabled.
func DebugTipSSOT() bool { return debugTipSSOT.Load() }

// DebugProjCont reports whether projection-continuity JSON/log probes are enabled.
func DebugProjCont() bool { return debugProjCont.Load() }

// SetDebugTipSSOT toggles TipSSOT probes (tests).
func SetDebugTipSSOT(v bool) { debugTipSSOT.Store(v) }

// SetDebugProjCont toggles ProjCont probes (tests).
func SetDebugProjCont(v bool) { debugProjCont.Store(v) }
