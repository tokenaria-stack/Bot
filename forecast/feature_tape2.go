package forecast

import (
	"crypto/sha256"
	"fmt"
	"math"
)

// FeatureTapeFormatV2 is the Brain V2 sensory tape. Distinct from feature-tape-v1.
const FeatureTapeFormatV2 = "feature-tape-v2"

// FeatureVector2 is the Ready-path Spec-2 vector. Wrong width cannot exist.
type FeatureVector2 [Spec2FeatureWidth]float64

func (v FeatureVector2) Slice() []float64 {
	out := make([]float64, Spec2FeatureWidth)
	copy(out, v[:])
	return out
}

func FeatureVector2FromSlice(values []float64) (FeatureVector2, error) {
	var v FeatureVector2
	if len(values) != Spec2FeatureWidth {
		return v, fmt.Errorf("forecast: feature-tape-v2 vector len %d != 64", len(values))
	}
	copy(v[:], values)
	if err := ValidateFeatureVector(v[:]); err != nil {
		return FeatureVector2{}, err
	}
	return v, nil
}

// NotReadyReason is a tiny frozen taxonomy. Not a diagnostics framework.
type NotReadyReason string

const (
	ReasonPrimaryWarmup NotReadyReason = "PRIMARY_WARMUP"
	ReasonHTF1hWarmup   NotReadyReason = "HTF_1H_WARMUP"
	ReasonHTF4hWarmup   NotReadyReason = "HTF_4H_WARMUP"
	ReasonPriceNotReady NotReadyReason = "PRICE_NOT_READY"
)

func (r NotReadyReason) Validate() error {
	switch r {
	case ReasonPrimaryWarmup, ReasonHTF1hWarmup, ReasonHTF4hWarmup, ReasonPriceNotReady:
		return nil
	default:
		return fmt.Errorf("forecast: feature-tape-v2 unknown NotReady reason %q", r)
	}
}

// FeatureRow2 is one primary closed-bar observation.
type FeatureRow2 struct {
	At     int64
	Ready  Ready
	Reason NotReadyReason
	Values FeatureVector2
}

// Tape2Header is file-level identity for one immutable feature-tape-v2.
type Tape2Header struct {
	FormatVersion  string
	SpecDigest     Digest
	PlanDigest     Digest
	FeaturesDigest Digest
	AnalysisDigest Digest
	TargetDigest   Digest
	Primary        MarketKey
	HTF1h          MarketKey
	HTF4h          MarketKey
	FeatureIDs     []FeatureID
	VectorLen      int
	Q              int
	Demand         HistoryDemand
	PrimarySource  Digest
	HTF1hSource    Digest
	HTF4hSource    Digest
}

// Tape2Footer closes a feature-tape-v2.
type Tape2Footer struct {
	RowCount      int
	ReadyCount    int
	NotReadyCount int
	FirstAt       int64
	LastAt        int64
	ContentDigest Digest
}

func validateTape2Header(h Tape2Header) error {
	if h.FormatVersion != FeatureTapeFormatV2 {
		return fmt.Errorf("forecast: unknown feature-tape format %q", h.FormatVersion)
	}
	if err := h.Primary.Validate(); err != nil {
		return err
	}
	if err := h.HTF1h.Validate(); err != nil {
		return err
	}
	if err := h.HTF4h.Validate(); err != nil {
		return err
	}
	if !h.Primary.SameFamily(h.HTF1h) || !h.Primary.SameFamily(h.HTF4h) {
		return fmt.Errorf("forecast: feature-tape-v2 HTF not SameFamily")
	}
	if h.Primary.Timeframe != Spec2PrimaryTF || h.HTF1h.Timeframe != Spec2HTF1h || h.HTF4h.Timeframe != Spec2HTF4h {
		return fmt.Errorf("forecast: feature-tape-v2 native timeframes must be 15m/1h/4h")
	}
	if h.VectorLen != Spec2FeatureWidth {
		return fmt.Errorf("forecast: feature-tape-v2 VectorLen must be 64")
	}
	if len(h.FeatureIDs) != Spec2FeatureWidth {
		return fmt.Errorf("forecast: feature-tape-v2 FeatureIDs len")
	}
	want := FeatureSpec2IDs()
	for i := range want {
		if h.FeatureIDs[i] != want[i] {
			return fmt.Errorf("forecast: feature-tape-v2 FeatureID order mismatch at %d", i)
		}
	}
	var zero Digest
	if h.SpecDigest == zero || h.PlanDigest == zero || h.FeaturesDigest == zero ||
		h.AnalysisDigest == zero || h.TargetDigest == zero {
		return fmt.Errorf("forecast: feature-tape-v2 identity digests required")
	}
	if h.PrimarySource == zero || h.HTF1hSource == zero || h.HTF4hSource == zero {
		return fmt.Errorf("forecast: feature-tape-v2 source digests required")
	}
	if h.Q <= 0 || !h.Demand.IIRFromSourceStart {
		return fmt.Errorf("forecast: feature-tape-v2 HistoryDemand IIRFromSourceStart required")
	}
	return nil
}

func validateTape2Row(ready Ready, reason NotReadyReason, values []float64) error {
	if ready {
		if reason != "" {
			return fmt.Errorf("forecast: Ready=true must omit Reason")
		}
		_, err := FeatureVector2FromSlice(values)
		return err
	}
	if values != nil {
		return fmt.Errorf("forecast: Ready=false must omit Values")
	}
	return reason.Validate()
}

func newTape2ContentHasher() *contentHasher {
	c := &contentHasher{h: sha256.New()}
	hashPutString(c.h, "FT2C")
	return c
}

func (c *contentHasher) header2(h Tape2Header) {
	hashPutString(c.h, h.FormatVersion)
	hashPutDigest(c.h, h.SpecDigest)
	hashPutDigest(c.h, h.PlanDigest)
	hashPutDigest(c.h, h.FeaturesDigest)
	hashPutDigest(c.h, h.AnalysisDigest)
	hashPutDigest(c.h, h.TargetDigest)
	hashPutMarket(c.h, h.Primary)
	hashPutMarket(c.h, h.HTF1h)
	hashPutMarket(c.h, h.HTF4h)
	hashPutU32(c.h, uint32(len(h.FeatureIDs)))
	for _, id := range h.FeatureIDs {
		hashPutString(c.h, string(id))
	}
	hashPutU32(c.h, uint32(h.VectorLen))
	hashPutU32(c.h, uint32(h.Q))
	hashPutU32(c.h, uint32(h.Demand.Primary15mWindowBars))
	hashPutU32(c.h, uint32(h.Demand.HTF1hWindowBars))
	hashPutU32(c.h, uint32(h.Demand.HTF4hWindowBars))
	if h.Demand.IIRFromSourceStart {
		hashPutU8(c.h, 1)
	} else {
		hashPutU8(c.h, 0)
	}
	hashPutDigest(c.h, h.PrimarySource)
	hashPutDigest(c.h, h.HTF1hSource)
	hashPutDigest(c.h, h.HTF4hSource)
}

func (c *contentHasher) row2(at int64, ready Ready, reason NotReadyReason, values []float64) {
	hashPutI64(c.h, at)
	if ready {
		hashPutU8(c.h, 1)
		hashPutU32(c.h, uint32(len(values)))
		for _, v := range values {
			hashPutF64(c.h, v)
		}
		return
	}
	hashPutU8(c.h, 0)
	hashPutString(c.h, string(reason))
}

func (c *contentHasher) meta2(rowCount, readyCount, notReadyCount int, firstAt, lastAt int64) {
	hashPutU32(c.h, uint32(rowCount))
	hashPutU32(c.h, uint32(readyCount))
	hashPutU32(c.h, uint32(notReadyCount))
	hashPutI64(c.h, firstAt)
	hashPutI64(c.h, lastAt)
}

func Vector2Finite(v FeatureVector2) error {
	for i, x := range v {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return fmt.Errorf("forecast: FeatureVector2[%d] is not finite", i)
		}
	}
	return nil
}
