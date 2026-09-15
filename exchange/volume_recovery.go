package exchange

import "fmt"

// RecoveryClass is VOLUME-TRUTH-RECOVERY-1 reclassification of a dirty row.
type RecoveryClass string

const (
	RecoveryLiveVCollision RecoveryClass = "LIVE_V_COLLISION"
	RecoveryZeroHole       RecoveryClass = "ZERO_HOLE"
	RecoveryAuthMismatch   RecoveryClass = "AUTHORITY_CONFIRMED_MISMATCH"
	RecoverySourceConflict RecoveryClass = "SOURCE_CONFLICT"
	RecoveryUnknown        RecoveryClass = "UNKNOWN"
)

// RepairEligible reports whether a recovery class may mutate Volume.
func RepairEligible(c RecoveryClass) bool {
	switch c {
	case RecoveryLiveVCollision, RecoveryZeroHole, RecoveryAuthMismatch:
		return true
	default:
		return false
	}
}

// CandleIdentityOK proves stored and authority describe the same native candle.
// Same open_time plus Open and Close. High/Low are not identity: MAX envelopes
// and under-indexed extremes must not reclassify a known-v mismatch as conflict.
func CandleIdentityOK(ot int64, o, h, l, c float64, auth VolumeAuthority) bool {
	_ = h
	_ = l
	if ot != auth.OpenTime {
		return false
	}
	return VolumeFloatEqual(o, auth.Open) && VolumeFloatEqual(c, auth.Close)
}

// ClassifyRecoveryRow assigns a recovery class. vision may be nil.
// prevBase/nextBase are adjacent REST BaseVolume when hasPrev/hasNext.
func ClassifyRecoveryRow(ot int64, o, h, l, c, stored float64, rest VolumeAuthority, vision *VolumeAuthority, prevBase, nextBase float64, hasPrev, hasNext bool) (class RecoveryClass, indexShift bool) {
	if rest.OpenTime != ot {
		return RecoveryUnknown, false
	}
	if VolumeFloatEqual(stored, rest.Base) {
		return RecoveryUnknown, false
	}
	liveV := VolumeFloatEqual(stored, rest.TakerBuyBase) && !VolumeFloatEqual(stored, rest.Base)
	idREST := CandleIdentityOK(ot, o, h, l, c, rest)

	if vision != nil {
		if !VolumeFloatEqual(rest.Base, vision.Base) {
			return RecoverySourceConflict, false
		}
		if vision.OpenTime != rest.OpenTime || !VolumeFloatEqual(vision.Open, rest.Open) || !VolumeFloatEqual(vision.Close, rest.Close) {
			return RecoverySourceConflict, false
		}
	}

	if liveV && idREST {
		if hasPrev && VolumeFloatEqual(stored, prevBase) || hasNext && VolumeFloatEqual(stored, nextBase) {
			return RecoveryLiveVCollision, true
		}
		return RecoveryLiveVCollision, false
	}

	if vision == nil {
		return RecoveryUnknown, indexShiftHint(stored, prevBase, nextBase, hasPrev, hasNext)
	}

	idVision := VolumeFloatEqual(o, vision.Open) && VolumeFloatEqual(c, vision.Close) && ot == vision.OpenTime
	if !idREST || !idVision {
		return RecoverySourceConflict, false
	}

	if VolumeFloatEqual(stored, 0) && !VolumeFloatEqual(rest.Base, 0) {
		return RecoveryZeroHole, indexShiftHint(stored, prevBase, nextBase, hasPrev, hasNext)
	}
	if VolumeFloatEqual(stored, rest.TakerBuyBase) || VolumeFloatEqual(stored, rest.Quote) || VolumeFloatEqual(stored, rest.TakerBuyQuote) {
		// V without live-id already handled; quote would be conflict-ish but still
		// canonical v is known when REST==Vision.
		return RecoveryAuthMismatch, indexShiftHint(stored, prevBase, nextBase, hasPrev, hasNext)
	}
	return RecoveryAuthMismatch, indexShiftHint(stored, prevBase, nextBase, hasPrev, hasNext)
}

func indexShiftHint(stored, prev, next float64, hasPrev, hasNext bool) bool {
	if hasPrev && VolumeFloatEqual(stored, prev) {
		return true
	}
	if hasNext && VolumeFloatEqual(stored, next) {
		return true
	}
	return false
}

// RepairNewVolume is the canonical BaseVolume for an eligible class.
func RepairNewVolume(class RecoveryClass, rest VolumeAuthority) (float64, error) {
	if !RepairEligible(class) {
		return 0, fmt.Errorf("refuse mutation class %s", class)
	}
	return rest.Base, nil
}
