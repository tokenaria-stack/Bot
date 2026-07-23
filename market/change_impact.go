package market

// ChangeImpact classifies how an indicator configuration change must rebuild runtime (ADR-013).
// Engine-owned; the browser never decides replay behavior.
//
// Severity order (highest wins when multiple fields change):
//
//	ProjectionOnly < AnnotationOnly < IndicatorReplay < GraphReplay
type ChangeImpact int

const (
	// ChangeImpactProjectionOnly — pure visual (color, width, visibility). No engine mutation.
	ChangeImpactProjectionOnly ChangeImpact = iota
	// ChangeImpactAnnotationOnly — derived overlays (div method, pivots, lookback). Jurik untouched.
	ChangeImpactAnnotationOnly
	// ChangeImpactIndicatorReplay — walk-forward math (length, source, signal). Set* then Replay only.
	ChangeImpactIndicatorReplay
	// ChangeImpactGraphReplay — reserved (enable/disable, DAG membership). Do not implement in B1.
	ChangeImpactGraphReplay
)

// MaxChangeImpact returns the higher-severity impact.
func MaxChangeImpact(a, b ChangeImpact) ChangeImpact {
	if a > b {
		return a
	}
	return b
}

// String returns a stable label for logs/tests.
func (c ChangeImpact) String() string {
	switch c {
	case ChangeImpactProjectionOnly:
		return "ProjectionOnly"
	case ChangeImpactAnnotationOnly:
		return "AnnotationOnly"
	case ChangeImpactIndicatorReplay:
		return "IndicatorReplay"
	case ChangeImpactGraphReplay:
		return "GraphReplay"
	default:
		return "ChangeImpact(?)"
	}
}
