package forecast

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"hash"
	"math"
)

// FeatureTapeFormatV1 is the first FeatureTape JSONL format. It pins record
// kinds, stdlib JSON float64 storage, SourceRangeDigest hashing, and
// ContentDigest hashing. Unknown versions are refused.
const FeatureTapeFormatV1 = "feature-tape-v1"

const (
	tapeKindHeader = "header"
	tapeKindRow    = "row"
	tapeKindFooter = "footer"
)

// CanonicalClosedBar is the normalized primary closed-bar payload hashed into
// SourceRangeDigest. These are the fields actually fed into Frame analysis
// (OpenTime identity + DAG OHLCV). CloseTime is omitted: Frame may synthesize
// it from the timeframe and Jurik/TV do not consume it.
type CanonicalClosedBar struct {
	OpenTime int64
	Open     float64
	High     float64
	Low      float64
	Close    float64
	Volume   float64
}

// TapeHeader is file-level identity for one immutable FeatureTape.
type TapeHeader struct {
	FormatVersion string
	Market        MarketKey
	PlanDigest    Digest
	FeatureIDs    []FeatureID
	VectorLen     int
}

// TapeRow is one observation for one consumed source closed bar.
// Values is nil when Ready is false.
type TapeRow struct {
	At     int64
	Ready  Ready
	Values []float64
}

// TapeFooter closes a FeatureTape. RowCount equals consumed source bars.
type TapeFooter struct {
	SourceRangeDigest Digest
	RowCount          int
	FirstAt           int64
	LastAt            int64
	ContentDigest     Digest
}

type tapeMarketJSON struct {
	Venue      string `json:"venue"`
	Instrument string `json:"instrument"`
	Contract   string `json:"contract"`
	Timeframe  string `json:"timeframe"`
}

type tapeHeaderJSON struct {
	Kind          string         `json:"kind"`
	FormatVersion string         `json:"format_version"`
	Market        tapeMarketJSON `json:"market"`
	PlanDigest    string         `json:"plan_digest"`
	FeatureIDs    []FeatureID    `json:"feature_ids"`
	VectorLen     int            `json:"vector_len"`
}

type tapeRowJSON struct {
	Kind   string    `json:"kind"`
	At     int64     `json:"at"`
	Ready  bool      `json:"ready"`
	Values []float64 `json:"values,omitempty"`
}

type tapeFooterJSON struct {
	Kind              string `json:"kind"`
	SourceRangeDigest string `json:"source_range_digest"`
	RowCount          int    `json:"row_count"`
	FirstAt           int64  `json:"first_at"`
	LastAt            int64  `json:"last_at"`
	ContentDigest     string `json:"content_digest"`
}

func marketJSON(k MarketKey) tapeMarketJSON {
	return tapeMarketJSON{Venue: k.Venue, Instrument: k.Instrument, Contract: k.Contract, Timeframe: k.Timeframe}
}

func marketFromJSON(j tapeMarketJSON) MarketKey {
	return MarketKey{Venue: j.Venue, Instrument: j.Instrument, Contract: j.Contract, Timeframe: j.Timeframe}
}

func validateHeader(h TapeHeader) error {
	if h.FormatVersion != FeatureTapeFormatV1 {
		return fmt.Errorf("forecast: unknown feature-tape format %q", h.FormatVersion)
	}
	if err := h.Market.Validate(); err != nil {
		return err
	}
	if h.VectorLen <= 0 {
		return fmt.Errorf("forecast: feature-tape VectorLen must be > 0")
	}
	if len(h.FeatureIDs) != h.VectorLen {
		return fmt.Errorf("forecast: feature-tape FeatureIDs len %d != VectorLen %d", len(h.FeatureIDs), h.VectorLen)
	}
	var zero Digest
	if h.PlanDigest == zero {
		return fmt.Errorf("forecast: feature-tape PlanDigest is required")
	}
	return nil
}

func validateReadyValues(vectorLen int, ready Ready, values []float64) error {
	if !ready {
		if values != nil {
			return fmt.Errorf("forecast: Ready=false must omit Values")
		}
		return nil
	}
	if len(values) != vectorLen {
		return fmt.Errorf("forecast: Ready vector len %d != VectorLen %d", len(values), vectorLen)
	}
	return ValidateFeatureVector(values)
}

func hashPutU8(h hash.Hash, v byte) {
	_, _ = h.Write([]byte{v})
}

func hashPutU32(h hash.Hash, v uint32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	_, _ = h.Write(b[:])
}

func hashPutU64(h hash.Hash, v uint64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], v)
	_, _ = h.Write(b[:])
}

func hashPutI64(h hash.Hash, v int64) {
	hashPutU64(h, uint64(v))
}

func hashPutBytes(h hash.Hash, p []byte) {
	hashPutU32(h, uint32(len(p)))
	_, _ = h.Write(p)
}

func hashPutString(h hash.Hash, s string) {
	hashPutBytes(h, []byte(s))
}

func hashPutDigest(h hash.Hash, d Digest) {
	_, _ = h.Write(d[:])
}

func hashPutMarket(h hash.Hash, k MarketKey) {
	hashPutString(h, k.Venue)
	hashPutString(h, k.Instrument)
	hashPutString(h, k.Contract)
	hashPutString(h, k.Timeframe)
}

func hashPutF64(h hash.Hash, v float64) {
	hashPutU64(h, math.Float64bits(v))
}

func hashPutBar(h hash.Hash, b CanonicalClosedBar) {
	hashPutI64(h, b.OpenTime)
	hashPutF64(h, b.Open)
	hashPutF64(h, b.High)
	hashPutF64(h, b.Low)
	hashPutF64(h, b.Close)
	hashPutF64(h, b.Volume)
}

// SourceRangeHasher streams SourceRangeDigest over MarketKey + bars in order.
type SourceRangeHasher struct {
	h hash.Hash
	n int
}

// NewSourceRangeHasher starts a v1 source-range hash (FormatVersion pins this law).
func NewSourceRangeHasher(market MarketKey) *SourceRangeHasher {
	s := &SourceRangeHasher{h: sha256.New()}
	hashPutString(s.h, "FT1S")
	hashPutMarket(s.h, market)
	return s
}

func (s *SourceRangeHasher) Add(bar CanonicalClosedBar) {
	if s == nil {
		return
	}
	hashPutBar(s.h, bar)
	s.n++
}

func (s *SourceRangeHasher) Count() int {
	if s == nil {
		return 0
	}
	return s.n
}

func (s *SourceRangeHasher) Sum() Digest {
	var d Digest
	if s == nil {
		return d
	}
	copy(d[:], s.h.Sum(nil))
	return d
}

// SourceRangeDigest hashes a fully materialized bar slice (same law as the streamer).
func SourceRangeDigest(market MarketKey, bars []CanonicalClosedBar) Digest {
	h := NewSourceRangeHasher(market)
	for _, b := range bars {
		h.Add(b)
	}
	return h.Sum()
}

type contentHasher struct {
	h hash.Hash
}

func newContentHasher() *contentHasher {
	c := &contentHasher{h: sha256.New()}
	hashPutString(c.h, "FT1C")
	return c
}

func (c *contentHasher) header(h TapeHeader) {
	hashPutString(c.h, h.FormatVersion)
	hashPutMarket(c.h, h.Market)
	hashPutDigest(c.h, h.PlanDigest)
	hashPutU32(c.h, uint32(len(h.FeatureIDs)))
	for _, id := range h.FeatureIDs {
		hashPutString(c.h, string(id))
	}
	hashPutU32(c.h, uint32(h.VectorLen))
}

func (c *contentHasher) row(at int64, ready Ready, values []float64) {
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
}

func (c *contentHasher) meta(src Digest, rowCount int, firstAt, lastAt int64) {
	hashPutDigest(c.h, src)
	hashPutU32(c.h, uint32(rowCount))
	hashPutI64(c.h, firstAt)
	hashPutI64(c.h, lastAt)
}

func (c *contentHasher) sum() Digest {
	var d Digest
	copy(d[:], c.h.Sum(nil))
	return d
}
