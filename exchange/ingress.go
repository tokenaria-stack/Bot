package exchange

// Ingress SSOT (Core 5.0 Phase A): the single place where candle trust and merge
// rules live. Consumers never decide "who wins" — they declare Authority and
// the pipeline decides. Kline itself stays authority-free (ledger is Edge-only).
//
// Contract: only CLOSED canonical bars enter this layer. Forming ticks
// (Binance k.x == false) are Marker telemetry and must bypass ingress.

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Authority is the trust level of a candle observation. Higher wins outright.
type Authority uint8

const (
	// AuthorityEstimated — bar inside the exchange settle window (grace);
	// may still be under-indexed. Should normally never reach merge.
	AuthorityEstimated Authority = iota
	// AuthoritySettled — closed bar from REST or SQLite archive after grace.
	AuthoritySettled
	// AuthorityFinal — bar confirmed by live WS (k.x == true). Immutable truth.
	AuthorityFinal
)

// RejectReason is a typed cause for dropping a candle at the ingress boundary.
type RejectReason string

const (
	RejectInvalidTime    RejectReason = "invalid_time"    // OpenTime <= 0 or CloseTime <= OpenTime
	RejectInvalidRange   RejectReason = "invalid_range"   // Low > High or OHLC outside [Low, High]
	RejectNegativeVolume RejectReason = "negative_volume" // Volume < 0
	RejectFutureBar      RejectReason = "future_bar"      // OpenTime ahead of wall clock beyond skew
)

// futureBarSkewMs tolerates minor clock skew before declaring a bar "from the future".
const futureBarSkewMs = 2 * 60 * 1000

// RejectError carries the typed reason; never silently fixes data.
type RejectError struct {
	Reason RejectReason
	Kline  Kline
}

func (e *RejectError) Error() string {
	return fmt.Sprintf("ingress reject: %s (open=%d)", e.Reason, e.Kline.OpenTime)
}

// IngressMetrics — cheap atomic observability (pattern: PersistenceQueue.Dropped).
type IngressMetrics struct {
	Accepted      atomic.Uint64 // bars that entered the canonical series
	Rejected      atomic.Uint64 // bars dropped by Validate
	Merged        atomic.Uint64 // equal-authority heuristic merges
	AuthConflicts atomic.Uint64 // lower-authority bars discarded on conflict
}

// defaultLedgerCap bounds the Edge ledger: authority only matters for fresh
// bars where REST/WS race; older bars are settled by definition.
const defaultLedgerCap = 4096

// IngressPipeline owns metrics and the Edge authority ledger.
// The ledger (openTime → Authority) is private RAM state: it is never persisted
// to SQLite and never leaks into Kline (Jeweler Protocol: zero core mutation).
type IngressPipeline struct {
	Metrics IngressMetrics

	mu         sync.Mutex
	ledger     map[int64]Authority
	ledgerFIFO []int64
	ledgerCap  int
}

// NewIngressPipeline creates a pipeline; cap <= 0 → defaultLedgerCap.
func NewIngressPipeline(ledgerCap int) *IngressPipeline {
	if ledgerCap <= 0 {
		ledgerCap = defaultLedgerCap
	}
	return &IngressPipeline{
		ledger:    make(map[int64]Authority, ledgerCap),
		ledgerCap: ledgerCap,
	}
}

// DefaultIngress is the process-wide pipeline used by package-level helpers,
// so metrics accumulate in one place until Phase C wires explicit ownership.
var DefaultIngress = NewIngressPipeline(0)

// Observe records the authority of a fresh bar (FIFO-evicted at capacity).
func (p *IngressPipeline) Observe(openTime int64, auth Authority) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, exists := p.ledger[openTime]; !exists {
		p.ledgerFIFO = append(p.ledgerFIFO, openTime)
		if len(p.ledgerFIFO) > p.ledgerCap {
			evict := p.ledgerFIFO[0]
			p.ledgerFIFO = p.ledgerFIFO[1:]
			delete(p.ledger, evict)
		}
	}
	if auth > p.ledger[openTime] {
		p.ledger[openTime] = auth
	}
}

// AuthorityOf reports the recorded authority for a bar. Bars unknown to the
// ledger default to AuthoritySettled: anything old enough to fall out of the
// Edge window is settled by definition.
func (p *IngressPipeline) AuthorityOf(openTime int64) Authority {
	p.mu.Lock()
	defer p.mu.Unlock()
	if a, ok := p.ledger[openTime]; ok {
		return a
	}
	return AuthoritySettled
}

// Validate rejects malformed candles with a typed reason. No silent repair:
// a broken bar must reach neither SQLite, nor Marker, nor DAG.
func (p *IngressPipeline) Validate(k Kline) error {
	k = NormalizeKline(k)
	if k.OpenTime <= 0 || (k.CloseTime > 0 && k.CloseTime <= k.OpenTime) {
		return p.reject(RejectInvalidTime, k)
	}
	if k.Low > k.High ||
		k.Open < k.Low || k.Open > k.High ||
		k.Close < k.Low || k.Close > k.High {
		return p.reject(RejectInvalidRange, k)
	}
	if k.Volume < 0 {
		return p.reject(RejectNegativeVolume, k)
	}
	if k.OpenTime > time.Now().UnixMilli()+futureBarSkewMs {
		return p.reject(RejectFutureBar, k)
	}
	return nil
}

func (p *IngressPipeline) reject(reason RejectReason, k Kline) error {
	p.Metrics.Rejected.Add(1)
	return &RejectError{Reason: reason, Kline: k}
}

// MergeCandle is the sole decision point for "whose bar wins".
//   - higher authority: incoming replaces existing entirely (no field heuristics);
//   - lower authority: existing is kept, incoming discarded (AuthConflicts++);
//   - equal authority: deterministic union — High=MAX, Low=MIN, Volume=MAX
//     (exchange totals only grow on honest re-reads), Open/Close from incoming.
func (p *IngressPipeline) MergeCandle(existing, incoming Kline, existAuth, incAuth Authority) Kline {
	switch {
	case incAuth > existAuth:
		return incoming
	case incAuth < existAuth:
		p.Metrics.AuthConflicts.Add(1)
		return existing
	default:
		p.Metrics.Merged.Add(1)
		merged := incoming
		merged.OpenTime = existing.OpenTime
		if existing.High > merged.High {
			merged.High = existing.High
		}
		if existing.Low < merged.Low {
			merged.Low = existing.Low
		}
		if existing.Volume > merged.Volume {
			merged.Volume = existing.Volume
		}
		if merged.CloseTime <= 0 {
			merged.CloseTime = existing.CloseTime
		}
		return merged
	}
}

// MergeKlineSeries unions two closed-bar series by OpenTime through the
// authority rules above. Bars are normalized (sec→ms) and validated; invalid
// bars are dropped with a typed metric. Output is ascending by OpenTime.
func (p *IngressPipeline) MergeKlineSeries(primary, overlay []Kline, priAuth, overlayAuth Authority) []Kline {
	merged := make(map[int64]Kline, len(primary)+len(overlay))
	auth := make(map[int64]Authority, len(primary)+len(overlay))

	ingest := func(series []Kline, a Authority) {
		for _, k := range series {
			k = NormalizeKline(k)
			if err := p.Validate(k); err != nil {
				continue
			}
			if existing, ok := merged[k.OpenTime]; ok {
				merged[k.OpenTime] = p.MergeCandle(existing, k, auth[k.OpenTime], a)
				if a > auth[k.OpenTime] {
					auth[k.OpenTime] = a
				}
				continue
			}
			merged[k.OpenTime] = k
			auth[k.OpenTime] = a
			p.Metrics.Accepted.Add(1)
		}
	}
	ingest(primary, priAuth)
	ingest(overlay, overlayAuth)

	out := make([]Kline, 0, len(merged))
	for _, k := range merged {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OpenTime < out[j].OpenTime })
	return out
}

// MergeKlineSeries is the package-level entry over DefaultIngress —
// the single replacement for every legacy mergeKlinesByOpenTime copy (debt #19).
func MergeKlineSeries(primary, overlay []Kline, priAuth, overlayAuth Authority) []Kline {
	return DefaultIngress.MergeKlineSeries(primary, overlay, priAuth, overlayAuth)
}
