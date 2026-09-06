package forecast

import (
	"crypto/sha256"
	"fmt"
	"hash"
	"math"

	"trading_bot/data"
	"trading_bot/indicators"
)

// LabelSetFormatV1 is the first LabelSet JSONL format. It pins record kinds,
// ContentDigest hashing, and LabelSourceRangeDigest hashing. Unknown versions
// are refused.
const LabelSetFormatV1 = "label-set-v1"

// LabelSetFormatV2 adds FinerMarketKey / FinerSourceDigest identity for
// resolve_finer_history LabelSets.
const LabelSetFormatV2 = "label-set-v2"

// LabelLogicFirstPassagePrimaryV1 identifies HOW a resolved TargetSpec is
// converted into primary-TF first-passage facts. It is not TargetDigest and
// is not TargetSpec.Logic (FORECAST-SPEC identity of the target question).
const LabelLogicFirstPassagePrimaryV1 LogicVersion = "label:first-passage-primary-v1"

// LabelLogicFirstPassageFinerV1 is 1A primary physics plus pinned-finer dual-hit resolution.
const LabelLogicFirstPassageFinerV1 LogicVersion = "label:first-passage-finer-v1"

const (
	labelKindHeader = "header"
	labelKindRow    = "row"
	labelKindFooter = "footer"
)

// LabelReason is factual metadata for AMBIGUOUS rows (and NONE otherwise).
// It is not a model class and not TargetSpec identity. It is part of ContentDigest.
type LabelReason string

const (
	ReasonNone              LabelReason = "NONE"
	ReasonATRZero           LabelReason = "ATR_ZERO"
	ReasonDualHit           LabelReason = "DUAL_HIT"
	ReasonTruncatedHorizon  LabelReason = "TRUNCATED_HORIZON"
	ReasonPrimaryGap        LabelReason = "PRIMARY_GAP"
	ReasonFinerMissing      LabelReason = "FINER_MISSING"
	ReasonFinerGap          LabelReason = "FINER_GAP"
	ReasonFinerDualHit      LabelReason = "FINER_DUAL_HIT"
	ReasonFinerInconsistent LabelReason = "FINER_INCONSISTENT"
)

// LabelHeader is file-level identity for one immutable LabelSet.
type LabelHeader struct {
	FormatVersion                string
	Market                       MarketKey
	TargetDigest                 Digest
	LabelLogicVersion            LogicVersion
	FeatureTapePlanDigest        Digest
	FeatureTapeSourceRangeDigest Digest
	FeatureTapeContentDigest     Digest
	FinerMarket                  MarketKey // v2 only
}

// LabelRow is one factual outcome for one FeatureTape candidate At.
type LabelRow struct {
	At      int64
	Outcome TargetOutcome
	HitAt   int64
	Reason  LabelReason
}

// LabelFooter closes a LabelSet. RowCount equals FeatureTape row count.
type LabelFooter struct {
	LabelSourceRangeDigest Digest
	RowCount               int
	FirstAt                int64
	LastAt                 int64
	ContentDigest          Digest
	FinerSourceDigest      Digest // v2
	FinerWindowCount       int    // v2; dual-hit resolution attempts
}

// LabelExpect is optional fail-closed identity the caller already knows.
type LabelExpect struct {
	Market      *MarketKey
	Target      *Digest
	TapePlan    *Digest
	TapeSource  *Digest
	TapeContent *Digest
}

type labelHeaderJSON struct {
	Kind                         string          `json:"kind"`
	FormatVersion                string          `json:"format_version"`
	Market                       tapeMarketJSON  `json:"market"`
	TargetDigest                 string          `json:"target_digest"`
	LabelLogicVersion            string          `json:"label_logic_version"`
	FeatureTapePlanDigest        string          `json:"feature_tape_plan_digest"`
	FeatureTapeSourceRangeDigest string          `json:"feature_tape_source_range_digest"`
	FeatureTapeContentDigest     string          `json:"feature_tape_content_digest"`
	FinerMarket                  *tapeMarketJSON `json:"finer_market,omitempty"`
}

type labelRowJSON struct {
	Kind    string `json:"kind"`
	At      int64  `json:"at"`
	Outcome string `json:"outcome"`
	HitAt   int64  `json:"hit_at"`
	Reason  string `json:"reason"`
}

type labelFooterJSON struct {
	Kind                   string `json:"kind"`
	LabelSourceRangeDigest string `json:"label_source_range_digest"`
	RowCount               int    `json:"row_count"`
	FirstAt                int64  `json:"first_at"`
	LastAt                 int64  `json:"last_at"`
	ContentDigest          string `json:"content_digest"`
	FinerSourceDigest      string `json:"finer_source_digest,omitempty"`
	FinerWindowCount       int    `json:"finer_window_count,omitempty"`
}

func validateLabelHeader(h LabelHeader) error {
	switch h.FormatVersion {
	case LabelSetFormatV1:
		if h.LabelLogicVersion != LabelLogicFirstPassagePrimaryV1 {
			return fmt.Errorf("forecast: unknown LabelLogicVersion %q", h.LabelLogicVersion)
		}
		if h.FinerMarket != (MarketKey{}) {
			return fmt.Errorf("forecast: label-set-v1 forbids FinerMarketKey")
		}
	case LabelSetFormatV2:
		if h.LabelLogicVersion != LabelLogicFirstPassageFinerV1 {
			return fmt.Errorf("forecast: unknown LabelLogicVersion %q", h.LabelLogicVersion)
		}
		if err := h.FinerMarket.Validate(); err != nil {
			return fmt.Errorf("forecast: label-set-v2 FinerMarketKey: %w", err)
		}
	default:
		return fmt.Errorf("forecast: unknown label-set format %q", h.FormatVersion)
	}
	if err := h.Market.Validate(); err != nil {
		return err
	}
	var zero Digest
	if h.TargetDigest == zero {
		return fmt.Errorf("forecast: label-set TargetDigest is required")
	}
	if h.FeatureTapePlanDigest == zero || h.FeatureTapeSourceRangeDigest == zero || h.FeatureTapeContentDigest == zero {
		return fmt.Errorf("forecast: label-set FeatureTape identity is required")
	}
	return nil
}

func validateLabelRow(r LabelRow) error {
	switch r.Outcome {
	case OutcomeUpFirst, OutcomeDownFirst, OutcomeTimeout, OutcomeAmbiguous:
	default:
		return fmt.Errorf("forecast: illegal label outcome %q", r.Outcome)
	}
	switch r.Reason {
	case ReasonNone, ReasonATRZero, ReasonDualHit, ReasonTruncatedHorizon, ReasonPrimaryGap,
		ReasonFinerMissing, ReasonFinerGap, ReasonFinerDualHit, ReasonFinerInconsistent:
	default:
		return fmt.Errorf("forecast: illegal label reason %q", r.Reason)
	}
	switch r.Outcome {
	case OutcomeUpFirst:
		if r.Reason != ReasonNone || r.HitAt <= 0 {
			return fmt.Errorf("forecast: UP_FIRST requires Reason=NONE and HitAt>0")
		}
	case OutcomeDownFirst:
		if r.Reason != ReasonNone || r.HitAt <= 0 {
			return fmt.Errorf("forecast: DOWN_FIRST requires Reason=NONE and HitAt>0")
		}
	case OutcomeTimeout:
		if r.Reason != ReasonNone || r.HitAt != 0 {
			return fmt.Errorf("forecast: TIMEOUT requires Reason=NONE and HitAt=0")
		}
	case OutcomeAmbiguous:
		switch r.Reason {
		case ReasonDualHit, ReasonFinerMissing, ReasonFinerGap, ReasonFinerDualHit, ReasonFinerInconsistent:
			if r.HitAt <= 0 {
				return fmt.Errorf("forecast: %s requires HitAt of the dual-hit primary bar", r.Reason)
			}
		case ReasonATRZero, ReasonTruncatedHorizon, ReasonPrimaryGap:
			if r.HitAt != 0 {
				return fmt.Errorf("forecast: %s requires HitAt=0", r.Reason)
			}
		default:
			return fmt.Errorf("forecast: AMBIGUOUS requires a typed reason")
		}
	}
	return nil
}

func validateSpecForLabels(spec TargetSpec) error {
	if spec.Family != TargetFamilyATRFirstPassage {
		return fmt.Errorf("forecast: LabelSet supports only %s", TargetFamilyATRFirstPassage)
	}
	switch spec.DualHit {
	case DualHitExcludeAmbiguous:
		if spec.FinerTimeframe != "" {
			return fmt.Errorf("forecast: exclude_ambiguous forbids FinerTimeframe")
		}
	case DualHitResolveFinerHistory:
		if spec.FinerTimeframe == "" {
			return fmt.Errorf("forecast: resolve_finer_history requires FinerTimeframe")
		}
	default:
		return fmt.Errorf("forecast: unknown DualHitPolicy %q", spec.DualHit)
	}
	if spec.HorizonBars <= 0 {
		return fmt.Errorf("forecast: target spec horizon bars must be > 0")
	}
	if spec.UpperATRMultiple <= 0 || spec.LowerATRMultiple <= 0 {
		return fmt.Errorf("forecast: target spec barrier multiples must be > 0")
	}
	return indicators.ValidateATRSpec(spec.ATR)
}

func validatePrimaryBars(bars []CanonicalClosedBar, interval string) error {
	if len(bars) == 0 {
		return fmt.Errorf("forecast: refuse empty primary source")
	}
	if _, err := data.NextBarOpen(bars[0].OpenTime, interval); err != nil {
		return fmt.Errorf("forecast: primary timeframe %q: %w", interval, err)
	}
	var prev int64
	for i, b := range bars {
		if err := validatePrimaryBar(b, interval); err != nil {
			return fmt.Errorf("forecast: primary bar %d: %w", i, err)
		}
		if i > 0 && b.OpenTime <= prev {
			return fmt.Errorf("forecast: primary OpenTime must be strictly increasing (got %d after %d)", b.OpenTime, prev)
		}
		prev = b.OpenTime
	}
	return nil
}

func validatePrimaryBar(b CanonicalClosedBar, interval string) error {
	if !isFinite(b.Open) || !isFinite(b.High) || !isFinite(b.Low) || !isFinite(b.Close) || !isFinite(b.Volume) {
		return fmt.Errorf("OHLCV is not finite")
	}
	if b.High < b.Low {
		return fmt.Errorf("High < Low")
	}
	open, err := data.CurrentBarOpen(b.OpenTime, interval)
	if err != nil {
		return err
	}
	if open != b.OpenTime {
		return fmt.Errorf("OpenTime %d is not a %s bar open (floor %d)", b.OpenTime, interval, open)
	}
	return nil
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func joinTapeToPrimary(bars []CanonicalClosedBar, rows []TapeRow) ([]int, error) {
	idx := make([]int, len(rows))
	j := 0
	for i, row := range rows {
		for j < len(bars) && bars[j].OpenTime < row.At {
			j++
		}
		if j >= len(bars) || bars[j].OpenTime != row.At {
			return nil, fmt.Errorf("forecast: FeatureTape At %d is missing from primary source", row.At)
		}
		idx[i] = j
		j++
	}
	return idx, nil
}

func consumedEndIndex(lastCandidateIdx, horizon, nBars int) int {
	end := lastCandidateIdx + horizon
	if end >= nBars {
		end = nBars - 1
	}
	return end
}

// validateATRHistoryContinuity refuses a hole anywhere in the IIR input
// (caller prefix through last candidate). Future-horizon holes are not
// this function; they remain per-candidate PRIMARY_GAP.
func validateATRHistoryContinuity(atrBars []CanonicalClosedBar, interval string) error {
	if len(atrBars) == 0 {
		return fmt.Errorf("forecast: refuse empty ATR history")
	}
	for i := 1; i < len(atrBars); i++ {
		expected, err := data.NextBarOpen(atrBars[i-1].OpenTime, interval)
		if err != nil {
			return fmt.Errorf("forecast: ATR history timeframe %q: %w", interval, err)
		}
		if atrBars[i].OpenTime != expected {
			return fmt.Errorf("forecast: ATR history has a primary gap at OpenTime %d (expected %d)", atrBars[i].OpenTime, expected)
		}
	}
	return nil
}

// LabelSourceHasher streams LabelSourceRangeDigest over MarketKey + consumed primary bars.
type LabelSourceHasher struct {
	h hash.Hash
	n int
}

// NewLabelSourceHasher starts the LABEL-SET source-range hash (domain tag LS1S).
func NewLabelSourceHasher(market MarketKey) *LabelSourceHasher {
	s := &LabelSourceHasher{h: sha256.New()}
	hashPutString(s.h, "LS1S")
	hashPutMarket(s.h, market)
	return s
}

func (s *LabelSourceHasher) Add(bar CanonicalClosedBar) {
	if s == nil {
		return
	}
	hashPutBar(s.h, bar)
	s.n++
}

func (s *LabelSourceHasher) Sum() Digest {
	var d Digest
	if s == nil {
		return d
	}
	copy(d[:], s.h.Sum(nil))
	return d
}

// LabelSourceRangeDigest hashes the exact consumed primary sequence.
func LabelSourceRangeDigest(market MarketKey, bars []CanonicalClosedBar) Digest {
	h := NewLabelSourceHasher(market)
	for _, b := range bars {
		h.Add(b)
	}
	return h.Sum()
}

type labelContentHasher struct {
	h hash.Hash
}

func newLabelContentHasher() *labelContentHasher {
	c := &labelContentHasher{h: sha256.New()}
	hashPutString(c.h, "LS1C")
	return c
}

func (c *labelContentHasher) header(h LabelHeader) {
	hashPutString(c.h, h.FormatVersion)
	hashPutMarket(c.h, h.Market)
	hashPutDigest(c.h, h.TargetDigest)
	hashPutString(c.h, string(h.LabelLogicVersion))
	hashPutDigest(c.h, h.FeatureTapePlanDigest)
	hashPutDigest(c.h, h.FeatureTapeSourceRangeDigest)
	hashPutDigest(c.h, h.FeatureTapeContentDigest)
	if h.FormatVersion == LabelSetFormatV2 {
		hashPutMarket(c.h, h.FinerMarket)
	}
}

func (c *labelContentHasher) row(r LabelRow) {
	hashPutI64(c.h, r.At)
	hashPutString(c.h, string(r.Outcome))
	hashPutI64(c.h, r.HitAt)
	hashPutString(c.h, string(r.Reason))
}

func (c *labelContentHasher) meta(src Digest, finer Digest, finerWindows, rowCount int, firstAt, lastAt int64, format string) {
	hashPutDigest(c.h, src)
	if format == LabelSetFormatV2 {
		hashPutDigest(c.h, finer)
		hashPutU32(c.h, uint32(finerWindows))
	}
	hashPutU32(c.h, uint32(rowCount))
	hashPutI64(c.h, firstAt)
	hashPutI64(c.h, lastAt)
}

func (c *labelContentHasher) sum() Digest {
	var d Digest
	copy(d[:], c.h.Sum(nil))
	return d
}

type labelBuild struct {
	Header       LabelHeader
	Rows         []LabelRow
	Source       Digest
	FinerSource  Digest
	FinerWindows int
}

// BuildLabelSet computes one LabelSet from a strict-read FeatureTape and
// materialized primary bars. Resolve_finer_history with no finer slice
// still emits v2 (dual-hits become FINER_MISSING).
func BuildLabelSet(tapePath string, spec TargetSpec, bars []CanonicalClosedBar, expect *LabelExpect) (LabelHeader, []LabelRow, Digest, error) {
	b, err := buildLabels(tapePath, spec, bars, MarketKey{}, nil, expect)
	if err != nil {
		return LabelHeader{}, nil, Digest{}, err
	}
	return b.Header, b.Rows, b.Source, nil
}

// BuildLabelSetWithFiner is BuildLabelSet plus a materialized same-family finer series.
func BuildLabelSetWithFiner(tapePath string, spec TargetSpec, bars []CanonicalClosedBar, finerMarket MarketKey, finer []CanonicalClosedBar, expect *LabelExpect) (LabelHeader, []LabelRow, Digest, error) {
	b, err := buildLabels(tapePath, spec, bars, finerMarket, finer, expect)
	if err != nil {
		return LabelHeader{}, nil, Digest{}, err
	}
	return b.Header, b.Rows, b.Source, nil
}

func buildLabels(tapePath string, spec TargetSpec, bars []CanonicalClosedBar, finerMarket MarketKey, finer []CanonicalClosedBar, expect *LabelExpect) (labelBuild, error) {
	var z labelBuild
	if err := validateSpecForLabels(spec); err != nil {
		return z, err
	}
	tid, err := spec.Identity()
	if err != nil {
		return z, err
	}
	if expect != nil && expect.Target != nil && *expect.Target != tid.Digest {
		return z, fmt.Errorf("forecast: TargetDigest mismatch")
	}

	var expectMarket *MarketKey
	var expectPlan *Digest
	if expect != nil {
		expectMarket = expect.Market
		expectPlan = expect.TapePlan
	}
	th, trows, tf, err := ReadTape(tapePath, expectMarket, expectPlan)
	if err != nil {
		return z, err
	}
	if expect != nil {
		if expect.TapeSource != nil && *expect.TapeSource != tf.SourceRangeDigest {
			return z, fmt.Errorf("forecast: FeatureTape SourceRangeDigest mismatch")
		}
		if expect.TapeContent != nil && *expect.TapeContent != tf.ContentDigest {
			return z, fmt.Errorf("forecast: FeatureTape ContentDigest mismatch")
		}
	}

	if err := validatePrimaryBars(bars, th.Market.Timeframe); err != nil {
		return z, err
	}
	idxs, err := joinTapeToPrimary(bars, trows)
	if err != nil {
		return z, err
	}
	lastCandidateIdx := idxs[len(idxs)-1]
	usedEnd := consumedEndIndex(lastCandidateIdx, spec.HorizonBars, len(bars))
	consumed := bars[:usedEnd+1]
	atrBars := consumed[:lastCandidateIdx+1]
	if err := validateATRHistoryContinuity(atrBars, th.Market.Timeframe); err != nil {
		return z, err
	}

	high := make([]float64, len(atrBars))
	low := make([]float64, len(atrBars))
	close := make([]float64, len(atrBars))
	for i, b := range atrBars {
		high[i], low[i], close[i] = b.High, b.Low, b.Close
	}
	atr, err := indicators.ATRSeries(spec.ATR, high, low, close)
	if err != nil {
		return z, err
	}

	var finerRun *finerResolve
	hdr := LabelHeader{
		Market:                       th.Market,
		TargetDigest:                 tid.Digest,
		FeatureTapePlanDigest:        th.PlanDigest,
		FeatureTapeSourceRangeDigest: tf.SourceRangeDigest,
		FeatureTapeContentDigest:     tf.ContentDigest,
	}
	if spec.DualHit == DualHitResolveFinerHistory {
		if finerMarket == (MarketKey{}) {
			finerMarket = th.Market
			finerMarket.Timeframe = spec.FinerTimeframe
		}
		if finerMarket.Timeframe != spec.FinerTimeframe {
			return z, fmt.Errorf("forecast: FinerMarketKey timeframe %q != TargetSpec.FinerTimeframe %q", finerMarket.Timeframe, spec.FinerTimeframe)
		}
		if !th.Market.SameFamily(finerMarket) {
			return z, fmt.Errorf("forecast: finer MarketKey is not SameFamily as primary")
		}
		if err := finerTilesPrimary(th.Market.Timeframe, spec.FinerTimeframe, bars[0].OpenTime); err != nil {
			return z, err
		}
		if len(finer) > 0 {
			if err := validatePrimaryBars(finer, spec.FinerTimeframe); err != nil {
				return z, err
			}
		}
		hdr.FormatVersion = LabelSetFormatV2
		hdr.LabelLogicVersion = LabelLogicFirstPassageFinerV1
		hdr.FinerMarket = finerMarket
		finerRun = newFinerResolve(finerMarket, th.Market.Timeframe, finer)
	} else {
		if finerMarket != (MarketKey{}) || len(finer) > 0 {
			return z, fmt.Errorf("forecast: exclude_ambiguous LabelSet forbids finer source")
		}
		hdr.FormatVersion = LabelSetFormatV1
		hdr.LabelLogicVersion = LabelLogicFirstPassagePrimaryV1
	}

	out := make([]LabelRow, len(trows))
	for i, tr := range trows {
		row, err := labelCandidate(spec, consumed, atr, idxs[i], th.Market.Timeframe, finerRun)
		if err != nil {
			return z, err
		}
		row.At = tr.At
		if err := validateLabelRow(row); err != nil {
			return z, err
		}
		out[i] = row
	}

	src := LabelSourceRangeDigest(th.Market, consumed)
	if err := validateLabelHeader(hdr); err != nil {
		return z, err
	}
	b := labelBuild{Header: hdr, Rows: out, Source: src}
	if finerRun != nil {
		b.FinerSource = finerRun.h.sum()
		b.FinerWindows = finerRun.windows
	}
	return b, nil
}

func labelCandidate(spec TargetSpec, bars []CanonicalClosedBar, atr []float64, t int, interval string, finer *finerResolve) (LabelRow, error) {
	v := atr[t]
	if !isFinite(v) {
		return LabelRow{}, fmt.Errorf("forecast: nonfinite ATR at %d", bars[t].OpenTime)
	}
	if v <= 0 {
		return LabelRow{Outcome: OutcomeAmbiguous, HitAt: 0, Reason: ReasonATRZero}, nil
	}
	upper := bars[t].Close + spec.UpperATRMultiple*v
	lower := bars[t].Close - spec.LowerATRMultiple*v
	if !isFinite(upper) || !isFinite(lower) {
		return LabelRow{}, fmt.Errorf("forecast: nonfinite barriers at %d", bars[t].OpenTime)
	}
	outcome, hitAt, reason, err := firstPassage(bars, t, upper, lower, spec.HorizonBars, interval)
	if err != nil {
		return LabelRow{}, err
	}
	if outcome == OutcomeAmbiguous && reason == ReasonDualHit && spec.DualHit == DualHitResolveFinerHistory {
		if finer == nil {
			return LabelRow{}, fmt.Errorf("forecast: finer resolver missing")
		}
		o, r, err := finer.resolve(bars[t].OpenTime, hitAt, upper, lower)
		if err != nil {
			return LabelRow{}, err
		}
		if o == OutcomeUpFirst || o == OutcomeDownFirst {
			return LabelRow{Outcome: o, HitAt: hitAt, Reason: ReasonNone}, nil
		}
		return LabelRow{Outcome: OutcomeAmbiguous, HitAt: hitAt, Reason: r}, nil
	}
	return LabelRow{Outcome: outcome, HitAt: hitAt, Reason: reason}, nil
}

// firstPassage is the frozen LABEL-SET-1A primary walker. Dual-hit
// finer resolution happens only in labelCandidate after this returns.
func firstPassage(bars []CanonicalClosedBar, t int, upper, lower float64, horizon int, interval string) (TargetOutcome, int64, LabelReason, error) {
	prev := bars[t].OpenTime
	for n := 0; n < horizon; n++ {
		expected, err := data.NextBarOpen(prev, interval)
		if err != nil {
			return "", 0, "", err
		}
		k := t + 1 + n
		if k >= len(bars) {
			return OutcomeAmbiguous, 0, ReasonTruncatedHorizon, nil
		}
		b := bars[k]
		if b.OpenTime != expected {
			return OutcomeAmbiguous, 0, ReasonPrimaryGap, nil
		}
		up := b.High >= upper
		down := b.Low <= lower
		if up && down {
			return OutcomeAmbiguous, b.OpenTime, ReasonDualHit, nil
		}
		if up {
			return OutcomeUpFirst, b.OpenTime, ReasonNone, nil
		}
		if down {
			return OutcomeDownFirst, b.OpenTime, ReasonNone, nil
		}
		prev = b.OpenTime
	}
	return OutcomeTimeout, 0, ReasonNone, nil
}
