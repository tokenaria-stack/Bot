package forecast

// SETUP-TARGET-1 is the first durable setup identity after MOVE-POTENTIAL-1.
// It is not TargetSpec (ATR first-passage), not a Ticket, and not a strategy.
//
// Frozen law:
//
//	LONG RSX 50-cross UP (age==0)
//	entry = 15m close
//	stop  = S0 k=2 Low − 0.15×Canonical ATR14 15m(entry)
//	success = entry + 2R
//	H = 72
//	path = EvaluateBarriers + 1m dual-hit (same as STRUCTURAL-STOP-2)
const (
	SetupTarget1ID       = "setup-target-1"
	SetupTarget1SuccessR = 2.0
	SetupLabelSetFormat  = "setup-labelset-v1"
	SetupClassTP         = "TP_FIRST"
	SetupClassStop       = "STOP_FIRST"
	SetupClassTimeout    = "TIMEOUT"
	SetupClassInvalid    = "INVALID_STRUCTURE"
	SetupClassSkip       = "NOT_EVALUABLE"
)

// SetupTarget1 is the published question. Changing any field is a new target.
type SetupTarget1 struct {
	ID        string
	Side      string
	SuccessR  float64
	Horizon   int
	StopOwner string
	BufferATR float64
	PrimaryTF string
}

// FrozenSetupTarget1 is the reviewed LONG +2R identity. SHORT is not this target.
func FrozenSetupTarget1() SetupTarget1 {
	return SetupTarget1{
		ID:        SetupTarget1ID,
		Side:      GeomSideLong,
		SuccessR:  SetupTarget1SuccessR,
		Horizon:   StructuralStopHorizon,
		StopOwner: StopOwnerPriceK2,
		BufferATR: StructuralStopBufferATR15,
		PrimaryTF: Spec2PrimaryTF,
	}
}

func (s SetupTarget1) validate() error {
	if s.ID != SetupTarget1ID || s.Side != GeomSideLong || s.SuccessR != SetupTarget1SuccessR {
		return errSetupTarget1Identity
	}
	if s.Horizon != StructuralStopHorizon || s.StopOwner != StopOwnerPriceK2 || s.BufferATR != StructuralStopBufferATR15 {
		return errSetupTarget1Identity
	}
	return nil
}
