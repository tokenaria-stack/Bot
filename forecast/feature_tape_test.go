package forecast

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testTapeHeader() TapeHeader {
	var d Digest
	d[0] = 0xab
	d[31] = 0xcd
	return TapeHeader{
		FormatVersion: FeatureTapeFormatV1,
		Market: MarketKey{
			Venue:      "BINANCE",
			Instrument: "BTCUSDT",
			Contract:   "FUTURES_PERP",
			Timeframe:  "15m",
		},
		PlanDigest: d,
		FeatureIDs: []FeatureID{
			FeatureRSXValue, FeatureRSXSignal, FeatureTVBullPresent, FeatureTVBullAge,
		},
		VectorLen: 4,
	}
}

func writeSampleTape(t *testing.T, path string, rows []TapeRow, bars []CanonicalClosedBar) TapeFooter {
	t.Helper()
	h := testTapeHeader()
	w, err := CreateTapeWriter(path, h)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if err := w.WriteRow(r.At, r.Ready, r.Values); err != nil {
			t.Fatal(err)
		}
	}
	src := SourceRangeDigest(h.Market, bars)
	if err := w.Finish(src); err != nil {
		t.Fatal(err)
	}
	_, _, ft, err := ReadTape(path, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return ft
}

func TestFeatureTape_Float64BitsRoundTrip(t *testing.T) {
	neg0 := math.Copysign(0, -1)
	vals := []float64{
		neg0,
		math.SmallestNonzeroFloat64,
		1e-300,
		0.1,
		54.1,
		-54.1,
		math.MaxFloat64,
	}
	path := filepath.Join(t.TempDir(), "t.featuretape")
	bars := make([]CanonicalClosedBar, len(vals))
	rows := make([]TapeRow, len(vals))
	base := int64(1_700_000_000_000)
	for i, v := range vals {
		vec := []float64{v, 1, 0, 2}
		rows[i] = TapeRow{At: base + int64(i)*60_000, Ready: IsReady, Values: vec}
		bars[i] = CanonicalClosedBar{OpenTime: rows[i].At, Open: 1, High: 2, Low: 0, Close: 1.5, Volume: 10}
	}
	writeSampleTape(t, path, rows, bars)
	_, got, _, err := ReadTape(path, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(rows) {
		t.Fatalf("rows %d", len(got))
	}
	for i := range rows {
		if !got[i].Ready {
			t.Fatalf("row %d not ready", i)
		}
		for j := range rows[i].Values {
			if math.Float64bits(got[i].Values[j]) != math.Float64bits(rows[i].Values[j]) {
				t.Fatalf("bits mismatch i=%d j=%d want=%x got=%x", i, j, math.Float64bits(rows[i].Values[j]), math.Float64bits(got[i].Values[j]))
			}
		}
	}
}

func TestFeatureTape_JSONFloat64BitsIncludingNegZero(t *testing.T) {
	neg0 := math.Copysign(0, -1)
	b, err := json.Marshal(neg0)
	if err != nil {
		t.Fatal(err)
	}
	var got float64
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if math.Float64bits(got) != math.Float64bits(neg0) {
		t.Fatalf("stdlib JSON did not preserve -0.0: json=%s in=%x out=%x", b, math.Float64bits(neg0), math.Float64bits(got))
	}
}

func TestFeatureTape_ReadyFalseDistinctFromPresentZero(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.featuretape")
	rows := []TapeRow{
		{At: 10, Ready: NotReady},
		{At: 20, Ready: IsReady, Values: []float64{50, 49, 0, 0}},
	}
	bars := []CanonicalClosedBar{
		{OpenTime: 10, Close: 1, Volume: 1},
		{OpenTime: 20, Close: 2, Volume: 1},
	}
	writeSampleTape(t, path, rows, bars)
	_, got, _, err := ReadTape(path, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Ready || got[0].Values != nil {
		t.Fatalf("NotReady must omit values: %+v", got[0])
	}
	if !got[1].Ready || got[1].Values[2] != 0 {
		t.Fatalf("Ready present=0: %+v", got[1])
	}
}

func TestFeatureTape_PlanAndMarketMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.featuretape")
	h := testTapeHeader()
	rows := []TapeRow{{At: 1, Ready: NotReady}}
	bars := []CanonicalClosedBar{{OpenTime: 1, Close: 1}}
	writeSampleTape(t, path, rows, bars)
	wrongPlan := h.PlanDigest
	wrongPlan[0]++
	if _, _, _, err := ReadTape(path, nil, &wrongPlan); err == nil {
		t.Fatal("expected PlanDigest mismatch")
	}
	wrongM := h.Market
	wrongM.Timeframe = "1h"
	if _, _, _, err := ReadTape(path, &wrongM, nil); err == nil {
		t.Fatal("expected MarketKey mismatch")
	}
}

func TestFeatureTape_RefuseNonfiniteReady(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.featuretape")
	w, err := CreateTapeWriter(path, testTapeHeader())
	if err != nil {
		t.Fatal(err)
	}
	if err := w.WriteRow(1, IsReady, []float64{math.NaN(), 1, 0, 0}); err == nil {
		t.Fatal("expected NaN refuse")
	}
	w, err = CreateTapeWriter(path, testTapeHeader())
	if err != nil {
		t.Fatal(err)
	}
	if err := w.WriteRow(1, IsReady, []float64{math.Inf(1), 1, 0, 0}); err == nil {
		t.Fatal("expected Inf refuse")
	}
}

func TestFeatureTape_RefuseDuplicateAt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.featuretape")
	w, err := CreateTapeWriter(path, testTapeHeader())
	if err != nil {
		t.Fatal(err)
	}
	if err := w.WriteRow(10, NotReady, nil); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteRow(10, NotReady, nil); err == nil {
		t.Fatal("expected duplicate At refuse")
	}
}

func TestFeatureTape_RefuseDecreasingAt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.featuretape")
	w, err := CreateTapeWriter(path, testTapeHeader())
	if err != nil {
		t.Fatal(err)
	}
	if err := w.WriteRow(20, NotReady, nil); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteRow(10, NotReady, nil); err == nil {
		t.Fatal("expected decreasing At refuse")
	}
}

func TestFeatureTape_RefuseExistingPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.featuretape")
	writeSampleTape(t, path, []TapeRow{{At: 1, Ready: NotReady}}, []CanonicalClosedBar{{OpenTime: 1}})
	if _, err := CreateTapeWriter(path, testTapeHeader()); err == nil {
		t.Fatal("expected refuse overwrite")
	}
}

func TestFeatureTape_MissingFooter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.featuretape")
	h := testTapeHeader()
	body := `{"kind":"header","format_version":"feature-tape-v1","market":{"venue":"BINANCE","instrument":"BTCUSDT","contract":"FUTURES_PERP","timeframe":"15m"},"plan_digest":"` + h.PlanDigest.String() + `","feature_ids":["rsx_value","rsx_signal","tv_bull_present","tv_bull_age"],"vector_len":4}
{"kind":"row","at":1,"ready":false}
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := ReadTape(path, nil, nil); err == nil {
		t.Fatal("expected missing footer")
	}
}

func TestFeatureTape_UnknownFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.featuretape")
	h := testTapeHeader()
	body := `{"kind":"header","format_version":"feature-tape-v9","market":{"venue":"BINANCE","instrument":"BTCUSDT","contract":"FUTURES_PERP","timeframe":"15m"},"plan_digest":"` + h.PlanDigest.String() + `","feature_ids":["rsx_value"],"vector_len":1}
{"kind":"footer","source_range_digest":"` + h.PlanDigest.String() + `","row_count":0,"first_at":0,"last_at":0,"content_digest":"` + h.PlanDigest.String() + `"}
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := ReadTape(path, nil, nil); err == nil {
		t.Fatal("expected unknown format")
	}
}

func TestFeatureTape_RecordAfterFooter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.featuretape")
	writeSampleTape(t, path, []TapeRow{{At: 1, Ready: NotReady}}, []CanonicalClosedBar{{OpenTime: 1}})
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, []byte("{\"kind\":\"row\",\"at\":2,\"ready\":false}\n")...)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := ReadTape(path, nil, nil); err == nil {
		t.Fatal("expected record after footer")
	}
}

func TestFeatureTape_SourceRangeDigestChangesWithBar(t *testing.T) {
	m := testTapeHeader().Market
	a := []CanonicalClosedBar{{OpenTime: 1, Open: 1, High: 2, Low: 0, Close: 1.5, Volume: 10}}
	b := []CanonicalClosedBar{{OpenTime: 1, Open: 1, High: 2, Low: 0, Close: 1.6, Volume: 10}}
	if SourceRangeDigest(m, a) == SourceRangeDigest(m, b) {
		t.Fatal("OHLC change must change SourceRangeDigest")
	}
	if SourceRangeDigest(m, a) != SourceRangeDigest(m, a) {
		t.Fatal("identical bars must match")
	}
}

func TestFeatureTape_ContentDigestDetectsValueTamper(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.featuretape")
	writeSampleTape(t, path, []TapeRow{
		{At: 1, Ready: IsReady, Values: []float64{54.1, 50, 1, 0}},
	}, []CanonicalClosedBar{{OpenTime: 1, Close: 1}})
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tampered := bytes.Replace(raw, []byte("54.1"), []byte("64.1"), 1)
	if bytes.Equal(raw, tampered) {
		t.Fatal("tamper did not change file")
	}
	if err := os.WriteFile(path, tampered, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := ReadTape(path, nil, nil); err == nil {
		t.Fatal("expected ContentDigest mismatch")
	}
}

func TestFeatureTape_SchemaVectorLenMismatch(t *testing.T) {
	h := testTapeHeader()
	h.VectorLen = 3
	path := filepath.Join(t.TempDir(), "t.featuretape")
	if _, err := CreateTapeWriter(path, h); err == nil {
		t.Fatal("expected schema/vector refuse")
	}
}

func TestFeatureTape_WrongReadyLength(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.featuretape")
	w, err := CreateTapeWriter(path, testTapeHeader())
	if err != nil {
		t.Fatal(err)
	}
	if err := w.WriteRow(1, IsReady, []float64{1, 2}); err == nil {
		t.Fatal("expected wrong length refuse")
	}
}

func TestFeatureTape_GapsAllowed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.featuretape")
	writeSampleTape(t, path, []TapeRow{
		{At: 10, Ready: NotReady},
		{At: 40, Ready: NotReady},
	}, []CanonicalClosedBar{{OpenTime: 10}, {OpenTime: 40}})
}

func TestFeatureTape_JSONUsesKind(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.featuretape")
	writeSampleTape(t, path, []TapeRow{{At: 1, Ready: NotReady}}, []CanonicalClosedBar{{OpenTime: 1}})
	raw, _ := os.ReadFile(path)
	s := string(raw)
	if !strings.Contains(s, `"kind":"header"`) || !strings.Contains(s, `"kind":"row"`) || !strings.Contains(s, `"kind":"footer"`) {
		t.Fatalf("missing kinds:\n%s", s)
	}
	if strings.Contains(s, `"values"`) {
		t.Fatalf("NotReady must omit values:\n%s", s)
	}
}
