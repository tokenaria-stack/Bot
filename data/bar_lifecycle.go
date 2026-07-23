package data

// IsFormingCloseTime reports whether a bar is still open under Cap/Tip Ownership.
// Forming predicate (ADR-009 / ADR-016): CloseTime > 0 && nowMs <= CloseTime.
// CloseTime == 0 is treated as closed (unknown boundary → do not invent forming).
func IsFormingCloseTime(closeTimeMs, nowMs int64) bool {
	return closeTimeMs > 0 && nowMs <= closeTimeMs
}
