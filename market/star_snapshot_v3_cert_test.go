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
	starSnapshotV2DigestFrozen = "1d86391613f56ad82895d6d98e3f16673c2c3b8ff3e230cc74400c7ac36196cb"
	starSnapshotV2FileSHA256   = "0f7e68fa942092449cc910f6c2bdba2c10a79fb769ac5fffa544a16adc1c102c"
	starStopD1A                = 1567900800000
	// Last daily open whose close is at or before starStopAsOf.
	// A later open is still forming at the frozen cutoff.
	starStopD1B = 1790208000000
)

func TestStarSnapshotV3Certification(t *testing.T) {
	if os.Getenv("STAR_SNAPSHOT_V3_CERT") != "1" {
		t.Skip("set STAR_SNAPSHOT_V3_CERT=1 to read the frozen archive")
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	v2Path := filepath.Join(root, "research", "starstop", "star_snapshot_v2.json")
	before, err := fileSHA256(v2Path)
	if err != nil {
		t.Fatal(err)
	}
	if before != starSnapshotV2FileSHA256 {
		t.Fatalf("schema-2 file %s", before)
	}
	v2Raw, err := os.ReadFile(v2Path)
	if err != nil {
		t.Fatal(err)
	}
	var frozen starSnapshotV2File
	if err := json.Unmarshal(v2Raw, &frozen); err != nil {
		t.Fatal(err)
	}
	if frozen.Schema != StarSnapshotSchemaV2 || frozen.Digest != starSnapshotV2DigestFrozen || frozen.Count != starStopCount {
		t.Fatalf("schema-2 header %d %s %d", frozen.Schema, frozen.Digest, frozen.Count)
	}
	if len(frozen.Rows) != starStopCount {
		t.Fatalf("schema-2 rows %d", len(frozen.Rows))
	}

	db := filepath.Join(root, "history.db")
	k15, err := loadClosedRange(db, "15m", starStopM15A, starStopM15B, starStopAsOf)
	if err != nil {
		t.Fatal(err)
	}
	k1h, err := loadClosedRange(db, "1h", starStopH1A, starStopH1B, starStopAsOf)
	if err != nil {
		t.Fatal(err)
	}
	k4h, err := loadClosedRange(db, "4h", starStopH4A, starStopH4B, starStopAsOf)
	if err != nil {
		t.Fatal(err)
	}
	k1d, err := loadClosedRange(db, "1d", starStopD1A, starStopD1B, starStopAsOf)
	if err != nil {
		t.Fatal(err)
	}
	if len(k1d) == 0 || k1d[0].OpenTime != starStopD1A || k1d[len(k1d)-1].OpenTime != starStopD1B {
		t.Fatalf("daily %d first %d last %d", len(k1d), k1d[0].OpenTime, k1d[len(k1d)-1].OpenTime)
	}
	if k1d[len(k1d)-1].CloseTime > starStopAsOf {
		t.Fatalf("daily tail still open %d", k1d[len(k1d)-1].CloseTime)
	}
	rsx := ResearchRSXSettings()
	born := testDAGRunnerBorn.Load()
	first, err := ExtractStarSnapshotsV3(k15, k1h, k4h, k1d, rsx)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ExtractStarSnapshotsV3(k15, k1h, k4h, k1d, rsx)
	if err != nil {
		t.Fatal(err)
	}
	if testDAGRunnerBorn.Load()-born != 8 {
		t.Fatalf("dag runners %d", testDAGRunnerBorn.Load()-born)
	}
	if len(first) != starStopCount || len(second) != starStopCount {
		t.Fatalf("stars %d %d", len(first), len(second))
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("extracts differ")
	}
	d1 := starSnapshotV3Digest(first)
	d2 := starSnapshotV3Digest(second)
	if d1 != d2 || d1 == "" || d1 == starSnapshotV2DigestFrozen {
		t.Fatalf("digest %s %s", d1, d2)
	}

	schema2 := make([]StarSnapshot, len(first))
	up, down := 0, 0
	var present, absent, exact, beforeClose, zeroObserved int
	for i, row := range first {
		schema2[i] = row.StarSnapshot
		frozenRow := frozen.Rows[i]
		if !reflect.DeepEqual(row.StarSnapshot, frozenRow) {
			t.Fatalf("schema-2 field %d %d", i, row.ConfirmedAt)
		}
		if row.ConfirmedAt != frozenRow.ConfirmedAt || row.Side != frozenRow.Side || row.AnchorAt != frozenRow.AnchorAt {
			t.Fatalf("identity %d", i)
		}
		switch row.Side {
		case "up":
			up++
		case "down":
			down++
		default:
			t.Fatalf("side %q", row.Side)
		}
		if row.D1.Present && row.D1.CloseTime > row.M15.CloseTime {
			t.Fatalf("daily after star %d", row.ConfirmedAt)
		}
		if !row.D1.Present && (row.D1 != (StarTF{}) || row.D1RSX != (StarRSX{})) {
			t.Fatalf("missing daily stored a number at %d", row.ConfirmedAt)
		}
		if row.D1.Present && row.D1.ValuesOK && row.D1.Vwema == 0 {
			zeroObserved++
		}
		switch {
		case !row.D1.Present:
			absent++
		case row.D1.CloseTime == row.M15.CloseTime:
			exact++
			present++
		case row.D1.CloseTime < row.M15.CloseTime:
			beforeClose++
			present++
		default:
			t.Fatalf("daily close %d star %d", row.D1.CloseTime, row.M15.CloseTime)
		}
	}
	if up != 4392 || down != 4391 {
		t.Fatalf("up %d down %d", up, down)
	}
	if starSnapshotV2Digest(schema2) != starSnapshotV2DigestFrozen {
		t.Fatal("schema-2 digest moved")
	}
	if present+absent != starStopCount || present == 0 {
		t.Fatalf("daily present %d absent %d exact %d", present, absent, exact)
	}

	bus := loadBusSeries(t, k1d, "1d", rsx)
	for _, row := range first {
		assertHTFBus(t, "1d", row.M15.CloseTime, row.D1, row.D1RSX, bus)
	}

	doc := starSnapshotV3File{
		Schema:        StarSnapshotSchemaV3,
		Digest:        d1,
		Schema2Digest: starSnapshotV2DigestFrozen,
		Count:         len(first),
		Up:            up,
		Down:          down,
		DailyBars:     len(k1d),
		DailyFirst:    k1d[0].OpenTime,
		DailyLast:     k1d[len(k1d)-1].OpenTime,
		DailyPresent:  present,
		DailyAbsent:   absent,
		DailyExact:    exact,
		MissingValue:  "a number is an observation only when its OK flag is true; 0 with a false flag is not a market value",
		Causal:        "latest 1d bar whose BarCloseTimeMs is at or before the Star 15m close",
		Rows:          first,
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(root, "research", "starstop", "star_snapshot_v3.json")
	if err := os.WriteFile(outPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	after, err := fileSHA256(v2Path)
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatal("schema-2 file changed")
	}
	t.Logf("schema2=%s schema3=%s stars=%d up=%d down=%d dailyBars=%d first=%d last=%d present=%d absent=%d exact=%d after=%d zeroVwema=%d",
		starSnapshotV2DigestFrozen, d1, len(first), up, down, len(k1d), k1d[0].OpenTime, k1d[len(k1d)-1].OpenTime, present, absent, exact, beforeClose, zeroObserved)
}

type starSnapshotV3File struct {
	Schema        int              `json:"schema"`
	Digest        string           `json:"digest"`
	Schema2Digest string           `json:"schema2Digest"`
	Count         int              `json:"count"`
	Up            int              `json:"up"`
	Down          int              `json:"down"`
	DailyBars     int              `json:"dailyBars"`
	DailyFirst    int64            `json:"dailyFirst"`
	DailyLast     int64            `json:"dailyLast"`
	DailyPresent  int              `json:"dailyPresent"`
	DailyAbsent   int              `json:"dailyAbsent"`
	DailyExact    int              `json:"dailyExact"`
	MissingValue  string           `json:"missingValue"`
	Causal        string           `json:"causal"`
	Rows          []StarSnapshotV3 `json:"rows"`
}

func starSnapshotV3Digest(rows []StarSnapshotV3) string {
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
	putB := func(v bool) {
		if v {
			_, _ = h.Write([]byte{1})
			return
		}
		_, _ = h.Write([]byte{0})
	}
	putI(int64(StarSnapshotSchemaV3))
	putI(int64(len(rows)))
	for _, row := range rows {
		_, _ = h.Write([]byte(row.Side))
		putI(row.AnchorAt)
		putI(row.ConfirmedAt)
		hashStarTF(putI, putF, putB, row.M15)
		hashStarTF(putI, putF, putB, row.H1)
		hashStarTF(putI, putF, putB, row.H4)
		putB(row.RSXOK)
		putF(row.RSX)
		putB(row.RSXSlopeOK)
		putF(row.RSXSlope)
		putB(row.RSXAccelOK)
		putF(row.RSXAccel)
		putB(row.SignalOK)
		putF(row.Signal)
		putB(row.SignalSlopeOK)
		putF(row.SignalSlope)
		putB(row.RSXMinusOK)
		putF(row.RSXMinusSignal)
		putB(row.TVAgeOK)
		_, _ = h.Write([]byte(row.TVDirection))
		putI(row.TVConfirmedAt)
		putI(int64(row.TVAge))
		hashStarRSX(putF, putB, row.H1RSX)
		hashStarRSX(putF, putB, row.H4RSX)
		hashStarTF(putI, putF, putB, row.D1)
		hashStarRSX(putF, putB, row.D1RSX)
	}
	return hex.EncodeToString(h.Sum(nil))
}
