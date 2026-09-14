// Package brain3 is the isolated learner: certified matrices in, future OOF out.
//
// It must not import market, Brain V2 CatBoost orchestration, or trading semantics.
package brain3

const (
	ClassTPFirst   = "TP_FIRST"
	ClassStopFirst = "STOP_FIRST"
	ClassTimeout   = "TIMEOUT"
)

// SetupClassSchema is the bound schema of SETUP-DATASET-1. Kernels see only 0..2.
func SetupClassSchema() []string {
	return []string{ClassTPFirst, ClassStopFirst, ClassTimeout}
}
