package market

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const (
	starRelativeSchema3Digest = "934e2e0d584297b50f199cd102511f6ecfb93a034ce94d9b27d6046675d8eee7"
	starRelativeSchema3SHA256 = "18f1e5ec79ee2ec4f2ab2afd5d5fbb213acd332df4e1396681bddd8ec652463a"
	starRelativeAbsentAt      = 1567965600000
)

var starRelativeZeroVwemaAt = []int64{1567997100000, 1568028600000, 1568047500000, 1568067300000}

func TestStarRelativeV1Certification(t *testing.T) {
	if os.Getenv("STAR_RELATIVE_V1_CERT") != "1" {
		t.Skip("set STAR_RELATIVE_V1_CERT=1 to read the frozen schema-3 artifact")
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	v2Path := filepath.Join(root, "research", "starstop", "star_snapshot_v2.json")
	v3Path := filepath.Join(root, "research", "starstop", "star_snapshot_v3.json")
	v2Before, err := fileSHA256(v2Path)
	if err != nil {
		t.Fatal(err)
	}
	v3Before, err := fileSHA256(v3Path)
	if err != nil {
		t.Fatal(err)
	}
	if v2Before != starSnapshotV2FileSHA256 {
		t.Fatalf("schema-2 file %s", v2Before)
	}
	if v3Before != starRelativeSchema3SHA256 {
		t.Fatalf("schema-3 file %s", v3Before)
	}
	raw, err := os.ReadFile(v3Path)
	if err != nil {
		t.Fatal(err)
	}
	var frozen starSnapshotV3File
	if err := json.Unmarshal(raw, &frozen); err != nil {
		t.Fatal(err)
	}
	if frozen.Schema != StarSnapshotSchemaV3 || frozen.Digest != starRelativeSchema3Digest || frozen.Count != starStopCount || len(frozen.Rows) != starStopCount {
		t.Fatalf("schema-3 header %d %s %d", frozen.Schema, frozen.Digest, frozen.Count)
	}

	first := BuildStarRelations(frozen.Rows)
	second := BuildStarRelations(frozen.Rows)
	if len(first) != starStopCount || !reflect.DeepEqual(first, second) {
		t.Fatal("builds differ")
	}
	d1, d2 := starRelativeDigest(starRelativeSchema3Digest, first), starRelativeDigest(starRelativeSchema3Digest, second)
	if d1 != d2 || d1 == "" || d1 == starRelativeSchema3Digest {
		t.Fatalf("digest %s %s", d1, d2)
	}

	up, down := 0, 0
	var absent int
	seenZero := map[int64]bool{}
	for i, rel := range first {
		row := frozen.Rows[i]
		if rel.ConfirmedAt != row.ConfirmedAt || rel.AnchorAt != row.AnchorAt || rel.Side != row.Side {
			t.Fatalf("identity %d", i)
		}
		if rel.ConfirmedAt != row.M15.OpenTime {
			t.Fatalf("decision %d", rel.ConfirmedAt)
		}
		switch rel.Side {
		case "up":
			up++
		case "down":
			down++
		default:
			t.Fatalf("side %q", rel.Side)
		}
		if got, want := rel.Relations(), wantRelations(row); got != want {
			t.Fatalf("arithmetic %d", rel.ConfirmedAt)
		}
		if rel.ConfirmedAt == starRelativeAbsentAt {
			absent++
			if row.D1.Present {
				t.Fatal("absent star has a daily bar")
			}
			for _, g := range dailyRelations(rel) {
				if g.OK || g.Value != 0 {
					t.Fatalf("daily relation on absent star %+v", g)
				}
			}
		}
		if row.D1.Present && row.D1.ValuesOK && row.D1.Vwema == 0 {
			seenZero[rel.ConfirmedAt] = true
			if !rel.H4D1Vwema.OK || rel.H4D1Vwema.Value != row.H4.Vwema-row.D1.Vwema {
				t.Fatalf("zero vwema relation %d %+v", rel.ConfirmedAt, rel.H4D1Vwema)
			}
			if row.D1.SlopeOK {
				t.Fatalf("zero vwema star has a slope %d", rel.ConfirmedAt)
			}
			if rel.D1VwemaSlopeMinusMidSlope.OK || rel.D1VwemaSlopeMinusMidSlope.Value != 0 {
				t.Fatalf("missing slope stored as a gap %d", rel.ConfirmedAt)
			}
		}
	}
	if up != 4392 || down != 4391 || absent != 1 {
		t.Fatalf("up %d down %d absent %d", up, down, absent)
	}
	if len(seenZero) != len(starRelativeZeroVwemaAt) {
		t.Fatalf("zero vwema stars %d", len(seenZero))
	}
	for _, at := range starRelativeZeroVwemaAt {
		if !seenZero[at] {
			t.Fatalf("missing zero vwema star %d", at)
		}
	}

	doc := starRelativeFile{
		Schema:        1,
		Digest:        d1,
		Schema3Digest: starRelativeSchema3Digest,
		Schema3Commit: "975da0d",
		Count:         len(first),
		Relations:     49,
		MissingValue:  "a relation is a reading only when OK is true; 0 with OK false is not a relation",
		Rows:          first,
	}
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(root, "research", "starstop", "star_relative_v1.json")
	if err := os.WriteFile(outPath, out, 0o644); err != nil {
		t.Fatal(err)
	}
	v2After, err := fileSHA256(v2Path)
	if err != nil {
		t.Fatal(err)
	}
	v3After, err := fileSHA256(v3Path)
	if err != nil {
		t.Fatal(err)
	}
	if v2After != v2Before || v3After != v3Before {
		t.Fatal("frozen snapshot changed")
	}
	t.Logf("digest=%s stars=%d relations=49 schema3=%s", d1, len(first), starRelativeSchema3Digest)
}

type starRelativeFile struct {
	Schema        int             `json:"schema"`
	Digest        string          `json:"digest"`
	Schema3Digest string          `json:"schema3Digest"`
	Schema3Commit string          `json:"schema3Commit"`
	Count         int             `json:"count"`
	Relations     int             `json:"relations"`
	MissingValue  string          `json:"missingValue"`
	Rows          []StarRelations `json:"rows"`
}

func dailyRelations(rel StarRelations) []Rel {
	return []Rel{
		rel.D1Ema7MinusMacd, rel.D1Ema7MinusCloseMid, rel.D1RsiCloseMinusCloseMid, rel.D1VwemaSlopeMinusMidSlope,
		rel.D1RsxMinusSignal,
		rel.H4D1OrangeMid, rel.H4D1Vwema, rel.H4D1Ema5, rel.H4D1Ema12, rel.H4D1RsiClose, rel.H4D1Ema7, rel.H4D1Macd, rel.H4D1Rsx,
		rel.H4D1Width, rel.H4D1CloseWidth,
	}
}

func wantRelations(row StarSnapshotV3) [49]Rel {
	var out [49]Rel
	s0, s1, s2, s3 := wantSame(row.M15), wantSame(row.H1), wantSame(row.H4), wantSame(row.D1)
	copy(out[0:4], s0[:])
	copy(out[4:8], s1[:])
	copy(out[8:12], s2[:])
	copy(out[12:16], s3[:])
	out[16] = wantRsxMinus(row.H1, row.H1RSX)
	out[17] = wantRsxMinus(row.H4, row.H4RSX)
	out[18] = wantRsxMinus(row.D1, row.D1RSX)
	l0 := wantLevel(row.M15, row.H1, row.M15.Present && row.RSXOK, row.RSX, row.H1.Present && row.H1RSX.ValueOK, row.H1RSX.Value)
	l1 := wantLevel(row.H1, row.H4, row.H1.Present && row.H1RSX.ValueOK, row.H1RSX.Value, row.H4.Present && row.H4RSX.ValueOK, row.H4RSX.Value)
	l2 := wantLevel(row.H4, row.D1, row.H4.Present && row.H4RSX.ValueOK, row.H4RSX.Value, row.D1.Present && row.D1RSX.ValueOK, row.D1RSX.Value)
	copy(out[19:27], l0[:])
	copy(out[27:35], l1[:])
	copy(out[35:43], l2[:])
	w0, w1, w2 := wantWidth(row.M15, row.H1), wantWidth(row.H1, row.H4), wantWidth(row.H4, row.D1)
	copy(out[43:45], w0[:])
	copy(out[45:47], w1[:])
	copy(out[47:49], w2[:])
	return out
}

func wantSame(tf StarTF) [4]Rel {
	p := tf.Present
	return [4]Rel{
		wantSub(tf.Ema7, p && tf.Ema7OK, tf.Macd, p && tf.MacdOK),
		wantSub(tf.Ema7, p && tf.Ema7OK, tf.CloseMid, p && tf.CloseMidOK),
		wantSub(tf.RsiClose, p && tf.RsiCloseOK, tf.CloseMid, p && tf.CloseMidOK),
		wantSub(tf.Slope, p && tf.SlopeOK, tf.MidSlope, p && tf.MidSlopeOK),
	}
}

func wantLevel(lo, hi StarTF, loRsxOK bool, loRsx float64, hiRsxOK bool, hiRsx float64) [8]Rel {
	lv, lok := wantFinite(lo, lo.Vwema)
	hv, hok := wantFinite(hi, hi.Vwema)
	lm, lmok := wantFinite(lo, lo.ChanMid)
	hm, hmok := wantFinite(hi, hi.ChanMid)
	return [8]Rel{
		wantSub(lm, lmok, hm, hmok),
		wantSub(lv, lok, hv, hok),
		wantSub(lo.Ema5, lo.Present && lo.Ema5OK, hi.Ema5, hi.Present && hi.Ema5OK),
		wantSub(lo.Ema12, lo.Present && lo.Ema12OK, hi.Ema12, hi.Present && hi.Ema12OK),
		wantSub(lo.RsiClose, lo.Present && lo.RsiCloseOK, hi.RsiClose, hi.Present && hi.RsiCloseOK),
		wantSub(lo.Ema7, lo.Present && lo.Ema7OK, hi.Ema7, hi.Present && hi.Ema7OK),
		wantSub(lo.Macd, lo.Present && lo.MacdOK, hi.Macd, hi.Present && hi.MacdOK),
		wantSub(loRsx, loRsxOK, hiRsx, hiRsxOK),
	}
}

func wantWidth(lo, hi StarTF) [2]Rel {
	return [2]Rel{
		wantSub(lo.Width, lo.Present && lo.WidthOK, hi.Width, hi.Present && hi.WidthOK),
		wantSub(lo.CloseWidth, lo.Present && lo.CloseWidthOK, hi.CloseWidth, hi.Present && hi.CloseWidthOK),
	}
}

func wantRsxMinus(tf StarTF, rsx StarRSX) Rel {
	return wantSub(rsx.Value, tf.Present && rsx.ValueOK, rsx.Signal, tf.Present && rsx.SignalOK)
}

func wantFinite(tf StarTF, v float64) (float64, bool) {
	if !tf.Present || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}

func wantSub(a float64, aOK bool, b float64, bOK bool) Rel {
	if !aOK || !bOK {
		return Rel{}
	}
	return Rel{Value: a - b, OK: true}
}

func starRelativeDigest(schema3 string, rows []StarRelations) string {
	h := sha256.New()
	var buf [8]byte
	putI := func(v int64) {
		binary.LittleEndian.PutUint64(buf[:], uint64(v))
		_, _ = h.Write(buf[:])
	}
	putF := func(v float64) {
		binary.LittleEndian.PutUint64(buf[:], math.Float64bits(v))
		_, _ = h.Write(buf[:])
	}
	_, _ = h.Write([]byte(schema3))
	putI(int64(len(rows)))
	for _, row := range rows {
		_, _ = h.Write([]byte(row.Side))
		putI(row.AnchorAt)
		putI(row.ConfirmedAt)
		for _, rel := range row.Relations() {
			if rel.OK {
				_, _ = h.Write([]byte{1})
			} else {
				_, _ = h.Write([]byte{0})
			}
			putF(rel.Value)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}
