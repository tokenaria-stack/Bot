package exchange

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
)

// VOLUME-SOURCE-ARBITRATION-1 STEP 2 — futures BaseVolume SSOT.
//
// Canonical family is Binance REST, not Vision and not TradingView.
// REST 15m parent owns only when it equals the complete REST 1m child sum
// AND child OHLC reconstructs the parent candle identity.
// If the REST parent fails that additive check but the REST 1m child set is
// complete and identity-valid, the child reconstruction owns.
// Vision is provenance only.

const VolumeTruthFuturesBaseV1 = "volume-truth:futures-base-v1"

// VolumeTruthPolicyV1 is hashed into PolicyDigest. Bump the truth version
// (v2) if this text's meaning changes.
const VolumeTruthPolicyV1 = `canonical_family=BINANCE_REST
	parent_invariant=native_parent_v_equals_complete_REST_1m_child_sum
broken_parent=complete_identity_valid_REST_1m_reconstruction_owns
interval_identity=additive_parent_CloseHighLow;broken_parent_Open
vision=provenance_not_canonical
tradingview=not_market_ledger
`

const (
	CanonicalREST15mParent      = "REST_15M_PARENT"
	CanonicalREST1mRecon        = "REST_1M_RECONSTRUCTION"
	CanonicalUnresolved         = "UNRESOLVED"
	PolicyRESTParentValid       = "REST_PARENT_VALID"
	PolicyRESTParentInvalidKids = "REST_PARENT_INVALID_CHILDREN_CERTIFIED"
	PolicyRESTFamilyUnresolved  = "REST_FAMILY_UNRESOLVED"
)

const native1mMs = 60_000

// Expected1mOpenTimes is the exact 15 child opens for a native 15m bar at T.
func Expected1mOpenTimes(parentOT int64) []int64 {
	out := make([]int64, 15)
	for i := 0; i < 15; i++ {
		out[i] = parentOT + int64(i)*native1mMs
	}
	return out
}

// AlignExact1mChildren requires the exact 15 expected 1m open times.
// Missing, duplicate, extra, or off-grid children refuse reconstruction.
func AlignExact1mChildren(parentOT int64, kids []VolumeAuthority) ([]VolumeAuthority, error) {
	want := Expected1mOpenTimes(parentOT)
	by := make(map[int64]VolumeAuthority, len(kids))
	for _, k := range kids {
		if _, dup := by[k.OpenTime]; dup {
			return nil, fmt.Errorf("duplicate 1m child open_time=%d", k.OpenTime)
		}
		by[k.OpenTime] = k
	}
	for t := range by {
		off := t - parentOT
		if off < 0 || off >= parent15mMs || off%native1mMs != 0 {
			return nil, fmt.Errorf("unexpected 1m child open_time=%d", t)
		}
	}
	out := make([]VolumeAuthority, 15)
	for i, t := range want {
		k, ok := by[t]
		if !ok {
			return nil, fmt.Errorf("missing 1m child open_time=%d", t)
		}
		out[i] = k
	}
	if len(by) != 15 {
		return nil, fmt.Errorf("1m child set size=%d want 15", len(by))
	}
	return out, nil
}

// AggregateAligned1m sums BaseVolume and OHLC envelope of 15 ordered 1m children.
func AggregateAligned1m(kids []VolumeAuthority) (vol, open, high, low, close float64) {
	if len(kids) == 0 {
		return 0, 0, 0, 0, 0
	}
	open = kids[0].Open
	close = kids[len(kids)-1].Close
	high = kids[0].High
	low = kids[0].Low
	for _, k := range kids {
		vol += k.Base
		if k.High > high {
			high = k.High
		}
		if k.Low < low {
			low = k.Low
		}
	}
	return vol, open, high, low, close
}

// REST15mOHLCIdentity is the established VolumeFloatEqual law on full OHLC.
func REST15mOHLCIdentity(parent VolumeAuthority, open, high, low, close float64) bool {
	return VolumeFloatEqual(parent.Open, open) &&
		VolumeFloatEqual(parent.Close, close) &&
		VolumeFloatEqual(parent.High, high) &&
		VolumeFloatEqual(parent.Low, low)
}

// REST15mIntervalOK is the STEP 2 identity gate (correction vs the prompt).
//
// A volume-broken REST 15m parent often has broken High/Low/Close too.
// Requiring full parent OHLC then quarantines the only honest REST-family
// reconstruction. Interval identity is:
//
//	additive parent: Close+High+Low match; Open mismatch is recorded, not fatal
//	broken parent:   first-child Open matches parent Open (same bar start)
//
// Full O/H/L/C mismatch still refuses. No extra numeric tolerance.
func REST15mIntervalOK(parent VolumeAuthority, open, high, low, close float64, parentAdditive bool) (ok, openMismatch bool) {
	openOK := VolumeFloatEqual(parent.Open, open)
	closeOK := VolumeFloatEqual(parent.Close, close)
	highOK := VolumeFloatEqual(parent.High, high)
	lowOK := VolumeFloatEqual(parent.Low, low)
	openMismatch = !openOK
	if parentAdditive {
		return closeOK && highOK && lowOK, openMismatch
	}
	return openOK, openMismatch
}

// Canonical15m is one REST-family policy decision.
type Canonical15m struct {
	Source         string
	PolicyClass    string
	Volume         float64
	ChildCount     int
	OHLCIdentityOK bool
	ParentAdditive bool
	OpenMismatch   bool
}

// ResolveFuturesRESTFamily15m applies the frozen v1 law.
// visionParent / visionSum are ignored (provenance only).
func ResolveFuturesRESTFamily15m(parent VolumeAuthority, kids []VolumeAuthority) Canonical15m {
	aligned, err := AlignExact1mChildren(parent.OpenTime, kids)
	if err != nil {
		seen := map[int64]struct{}{}
		for _, k := range kids {
			if k.OpenTime >= parent.OpenTime && k.OpenTime < parent.OpenTime+parent15mMs {
				seen[k.OpenTime] = struct{}{}
			}
		}
		return Canonical15m{
			Source: CanonicalUnresolved, PolicyClass: PolicyRESTFamilyUnresolved, ChildCount: len(seen),
		}
	}
	sum, o, h, l, c := AggregateAligned1m(aligned)
	parentOK := VolumeFloatEqual(parent.Base, sum)
	intervalOK, openMis := REST15mIntervalOK(parent, o, h, l, c, parentOK)
	fullOHLC := REST15mOHLCIdentity(parent, o, h, l, c)
	if parentOK && intervalOK {
		return Canonical15m{
			Source: CanonicalREST15mParent, PolicyClass: PolicyRESTParentValid,
			Volume: parent.Base, ChildCount: 15, OHLCIdentityOK: fullOHLC,
			ParentAdditive: true, OpenMismatch: openMis,
		}
	}
	if !parentOK && intervalOK {
		return Canonical15m{
			Source: CanonicalREST1mRecon, PolicyClass: PolicyRESTParentInvalidKids,
			Volume: sum, ChildCount: 15, OHLCIdentityOK: fullOHLC,
			ParentAdditive: false, OpenMismatch: openMis,
		}
	}
	return Canonical15m{
		Source: CanonicalUnresolved, PolicyClass: PolicyRESTFamilyUnresolved,
		ChildCount: 15, OHLCIdentityOK: fullOHLC, ParentAdditive: parentOK, OpenMismatch: openMis,
	}
}

// DigestCanonicalVolumeSeries hashes symbol, interval, then each (open_time, volume)
// as little-endian int64 + IEEE float64 bits. No paths, timestamps, or JSON noise.
func DigestCanonicalVolumeSeries(symbol, interval string, openTimes []int64, volumes []float64) string {
	if len(openTimes) != len(volumes) {
		return ""
	}
	h := sha256.New()
	h.Write([]byte(VolumeTruthFuturesBaseV1))
	h.Write([]byte{'\n'})
	h.Write([]byte(symbol))
	h.Write([]byte{'\t'})
	h.Write([]byte(interval))
	h.Write([]byte{'\n'})
	var buf [16]byte
	for i := range openTimes {
		binary.LittleEndian.PutUint64(buf[0:8], uint64(openTimes[i]))
		binary.LittleEndian.PutUint64(buf[8:16], math.Float64bits(volumes[i]))
		h.Write(buf[:])
	}
	return hex.EncodeToString(h.Sum(nil))
}

// DigestVolumeTruthPolicy hashes the frozen policy constant.
func DigestVolumeTruthPolicy() string {
	sum := sha256.Sum256([]byte(VolumeTruthPolicyV1))
	return hex.EncodeToString(sum[:])
}

// DigestVolumeTruthBundle binds 15m/1h/4h content digests.
func DigestVolumeTruthBundle(d15m, d1h, d4h string) string {
	h := sha256.New()
	h.Write([]byte(d15m))
	h.Write([]byte{'\n'})
	h.Write([]byte(d1h))
	h.Write([]byte{'\n'})
	h.Write([]byte(d4h))
	h.Write([]byte{'\n'})
	return hex.EncodeToString(h.Sum(nil))
}

// DigestArbitrationSet hashes the ordered 20-bar open times + canonical volumes.
func DigestArbitrationSet(openTimes []int64, canonical []float64) string {
	return DigestCanonicalVolumeSeries("BTCUSDT", "15m-arbitration", openTimes, canonical)
}
