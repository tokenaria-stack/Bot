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

func TestStarSnapshotV2Certification(t *testing.T) {
	if os.Getenv("STAR_SNAPSHOT_V2_CERT") != "1" {
		t.Skip("set STAR_SNAPSHOT_V2_CERT=1 to read the frozen archive")
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	frozen := []string{
		filepath.Join(root, "research", "starstop", "star_stops.json"),
		filepath.Join(root, "research", "starstop", "star_sl_outcome.json"),
		filepath.Join(root, "research", "starstop", "star_sl_buffer_compare.json"),
		filepath.Join(root, "research", "starstop", "star_outcome_dataset.json"),
	}
	before := make(map[string]string, len(frozen))
	for _, path := range frozen {
		sum, err := fileSHA256(path)
		if err != nil {
			t.Fatal(err)
		}
		before[path] = sum
	}
	stopRaw, err := os.ReadFile(frozen[0])
	if err != nil {
		t.Fatal(err)
	}
	var stops starStopFile
	if err := json.Unmarshal(stopRaw, &stops); err != nil {
		t.Fatal(err)
	}
	if stops.Digest != starStopFileDigest || stops.Count != starStopCount {
		t.Fatalf("stops %s %d", stops.Digest, stops.Count)
	}
	baseRaw, err := os.ReadFile(frozen[3])
	if err != nil {
		t.Fatal(err)
	}
	var base datasetFile
	if err := json.Unmarshal(baseRaw, &base); err != nil {
		t.Fatal(err)
	}
	if base.Count != starStopCount || len(base.Rows) != starStopCount {
		t.Fatalf("dataset %d", base.Count)
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
	rsx := ResearchRSXSettings()
	born := testDAGRunnerBorn.Load()
	first, err := ExtractStarSnapshots(k15, k1h, k4h, rsx)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ExtractStarSnapshots(k15, k1h, k4h, rsx)
	if err != nil {
		t.Fatal(err)
	}
	if testDAGRunnerBorn.Load()-born != 6 {
		t.Fatalf("dag runners %d", testDAGRunnerBorn.Load()-born)
	}
	if len(first) != starStopCount || len(second) != starStopCount {
		t.Fatalf("stars %d %d", len(first), len(second))
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("extracts differ")
	}
	d1, d2 := starSnapshotV2Digest(first), starSnapshotV2Digest(second)
	if d1 != d2 || d1 == "" {
		t.Fatalf("digest %s %s", d1, d2)
	}
	up, down := 0, 0
	ok := newStarV2OK()
	for i, row := range first {
		if row.ConfirmedAt != stops.Rows[i].DecisionAt || row.Side != stops.Rows[i].Side || row.AnchorAt != row.ConfirmedAt {
			t.Fatalf("identity %d", i)
		}
		if row.ConfirmedAt != base.Rows[i].DecisionAt || row.Side != base.Rows[i].Side {
			t.Fatalf("dataset identity %d", i)
		}
		got := datasetStateFrom(row)
		if !reflect.DeepEqual(got, base.Rows[i].State) {
			t.Fatalf("v1 state %d", row.ConfirmedAt)
		}
		if err := sameCertifiedTF(row.H1, stops.Rows[i].H1); err != nil {
			t.Fatalf("h1 %d %v", row.ConfirmedAt, err)
		}
		if err := sameCertifiedTF(row.H4, stops.Rows[i].H4); err != nil {
			t.Fatalf("h4 %d %v", row.ConfirmedAt, err)
		}
		switch row.Side {
		case "up":
			up++
		case "down":
			down++
		default:
			t.Fatalf("side %q", row.Side)
		}
		ok.add(row)
	}
	if up != 4392 || down != 4391 {
		t.Fatalf("up %d down %d", up, down)
	}
	m15 := loadBusSeries(t, k15, "15m", rsx)
	h1 := loadBusSeries(t, k1h, "1h", rsx)
	h4 := loadBusSeries(t, k4h, "4h", rsx)
	openAt := make(map[int64]int, len(k15))
	for i, k := range k15 {
		openAt[k.OpenTime] = i
	}
	for _, row := range first {
		i, found := openAt[row.AnchorAt]
		if !found || m15.close[i] != row.M15.CloseTime {
			t.Fatalf("15m index %d", row.AnchorAt)
		}
		assertTFBus(t, "15m", row.M15, m15, i)
		assertRootRSX(t, row, m15, i)
		assertHTFBus(t, "1h", row.M15.CloseTime, row.H1, row.H1RSX, h1)
		assertHTFBus(t, "4h", row.M15.CloseTime, row.H4, row.H4RSX, h4)
	}
	doc := starSnapshotV2File{
		Schema: StarSnapshotSchemaV2, Digest: d1, Count: len(first), Up: up, Down: down,
		MissingValue: "a number is an observation only when its OK flag is true; 0 with a false flag is not a market value",
		OK:           ok, Rows: first,
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(root, "research", "starstop", "star_snapshot_v2.json")
	if err := os.WriteFile(outPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, path := range frozen {
		sum, err := fileSHA256(path)
		if err != nil {
			t.Fatal(err)
		}
		if sum != before[path] {
			t.Fatalf("v1 artifact changed %s", path)
		}
	}
	t.Logf("schema=%d digest=%s stars=%d up=%d down=%d ok=%+v", StarSnapshotSchemaV2, d1, len(first), up, down, ok)
}

func fileSHA256(path string) (string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

type starSnapshotV2File struct {
	Schema       int            `json:"schema"`
	Digest       string         `json:"digest"`
	Count        int            `json:"count"`
	Up           int            `json:"up"`
	Down         int            `json:"down"`
	MissingValue string         `json:"missingValue"`
	OK           starV2OK       `json:"ok"`
	Rows         []StarSnapshot `json:"rows"`
}

type starV2OK struct {
	M15   starTFOK  `json:"m15"`
	H1    starTFOK  `json:"h1"`
	H4    starTFOK  `json:"h4"`
	RSX   starRSXOK `json:"rsx15"`
	H1RSX starRSXOK `json:"rsx1h"`
	H4RSX starRSXOK `json:"rsx4h"`
	TV    int       `json:"tv"`
}

type starTFOK struct {
	Present          int `json:"present"`
	Values           int `json:"values"`
	VwemaSlope       int `json:"vwemaSlope"`
	VwemaAccel       int `json:"vwemaAccel"`
	MidSlope         int `json:"midSlope"`
	MidAccel         int `json:"midAccel"`
	Width            int `json:"width"`
	WidthChange      int `json:"widthChange"`
	Distance         int `json:"distance"`
	Ema5             int `json:"ema5"`
	Ema5Slope        int `json:"ema5Slope"`
	Ema5Accel        int `json:"ema5Accel"`
	Ema12            int `json:"ema12"`
	Ema12Slope       int `json:"ema12Slope"`
	RsiClose         int `json:"rsiClose"`
	RsiCloseSlope    int `json:"rsiCloseSlope"`
	RsiCloseAccel    int `json:"rsiCloseAccel"`
	CloseWidth       int `json:"closeWidth"`
	CloseWidthChange int `json:"closeWidthChange"`
	Ema7             int `json:"ema7"`
	Ema7Slope        int `json:"ema7Slope"`
	Ema7Accel        int `json:"ema7Accel"`
	Macd             int `json:"macd"`
	MacdSlope        int `json:"macdSlope"`
	MacdAccel        int `json:"macdAccel"`
}

type starRSXOK struct {
	Value       int `json:"value"`
	Slope       int `json:"slope"`
	Accel       int `json:"accel"`
	Signal      int `json:"signal"`
	SignalSlope int `json:"signalSlope"`
}

func newStarV2OK() starV2OK { return starV2OK{} }

func (c *starV2OK) add(row StarSnapshot) {
	c.M15.add(row.M15)
	c.H1.add(row.H1)
	c.H4.add(row.H4)
	c.RSX.addRoot(row)
	c.H1RSX.add(row.H1RSX)
	c.H4RSX.add(row.H4RSX)
	if row.TVAgeOK {
		c.TV++
	}
}

func (c *starTFOK) add(tf StarTF) {
	if tf.Present {
		c.Present++
	}
	count := func(ok bool, n *int) {
		if ok {
			*n++
		}
	}
	count(tf.ValuesOK, &c.Values)
	count(tf.SlopeOK, &c.VwemaSlope)
	count(tf.VwemaAccelOK, &c.VwemaAccel)
	count(tf.MidSlopeOK, &c.MidSlope)
	count(tf.MidAccelOK, &c.MidAccel)
	count(tf.WidthOK, &c.Width)
	count(tf.WidthChangeOK, &c.WidthChange)
	count(tf.DistanceOK, &c.Distance)
	count(tf.Ema5OK, &c.Ema5)
	count(tf.Ema5SlopeOK, &c.Ema5Slope)
	count(tf.Ema5AccelOK, &c.Ema5Accel)
	count(tf.Ema12OK, &c.Ema12)
	count(tf.Ema12SlopeOK, &c.Ema12Slope)
	count(tf.RsiCloseOK, &c.RsiClose)
	count(tf.RsiCloseSlopeOK, &c.RsiCloseSlope)
	count(tf.RsiCloseAccelOK, &c.RsiCloseAccel)
	count(tf.CloseWidthOK, &c.CloseWidth)
	count(tf.CloseWidthChangeOK, &c.CloseWidthChange)
	count(tf.Ema7OK, &c.Ema7)
	count(tf.Ema7SlopeOK, &c.Ema7Slope)
	count(tf.Ema7AccelOK, &c.Ema7Accel)
	count(tf.MacdOK, &c.Macd)
	count(tf.MacdSlopeOK, &c.MacdSlope)
	count(tf.MacdAccelOK, &c.MacdAccel)
}

func (c *starRSXOK) add(rsx StarRSX) {
	if rsx.ValueOK {
		c.Value++
	}
	if rsx.SlopeOK {
		c.Slope++
	}
	if rsx.AccelOK {
		c.Accel++
	}
	if rsx.SignalOK {
		c.Signal++
	}
	if rsx.SignalSlopeOK {
		c.SignalSlope++
	}
}

func (c *starRSXOK) addRoot(row StarSnapshot) {
	if row.RSXOK {
		c.Value++
	}
	if row.RSXSlopeOK {
		c.Slope++
	}
	if row.RSXAccelOK {
		c.Accel++
	}
	if row.SignalOK {
		c.Signal++
	}
	if row.SignalSlopeOK {
		c.SignalSlope++
	}
}

func starSnapshotV2Digest(rows []StarSnapshot) string {
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
	putI(int64(StarSnapshotSchemaV2))
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
	}
	return hex.EncodeToString(h.Sum(nil))
}

func hashStarTF(putI func(int64), putF func(float64), putB func(bool), tf StarTF) {
	putB(tf.Present)
	putI(tf.OpenTime)
	putI(tf.CloseTime)
	putB(tf.ValuesOK)
	putF(tf.Vwema)
	putF(tf.ChanMid)
	putF(tf.ChanUp)
	putF(tf.ChanDn)
	putB(tf.SlopeOK)
	putF(tf.Slope)
	putB(tf.WidthOK)
	putF(tf.Width)
	putB(tf.WidthChangeOK)
	putF(tf.WidthChange)
	putB(tf.DistanceOK)
	putF(tf.Distance)
	putB(tf.MidSlopeOK)
	putF(tf.MidSlope)
	putB(tf.MidAccelOK)
	putF(tf.MidAccel)
	putB(tf.Ema5OK)
	putF(tf.Ema5)
	putB(tf.Ema5SlopeOK)
	putF(tf.Ema5Slope)
	putB(tf.Ema5AccelOK)
	putF(tf.Ema5Accel)
	putB(tf.Ema12OK)
	putF(tf.Ema12)
	putB(tf.Ema12SlopeOK)
	putF(tf.Ema12Slope)
	putB(tf.VwemaAccelOK)
	putF(tf.VwemaAccel)
	putB(tf.RsiCloseOK)
	putF(tf.RsiClose)
	putB(tf.RsiCloseSlopeOK)
	putF(tf.RsiCloseSlope)
	putB(tf.RsiCloseAccelOK)
	putF(tf.RsiCloseAccel)
	putB(tf.CloseMidOK)
	putF(tf.CloseMid)
	putB(tf.CloseUpOK)
	putF(tf.CloseUp)
	putB(tf.CloseDnOK)
	putF(tf.CloseDn)
	putB(tf.CloseWidthOK)
	putF(tf.CloseWidth)
	putB(tf.CloseWidthChangeOK)
	putF(tf.CloseWidthChange)
	putB(tf.Ema7OK)
	putF(tf.Ema7)
	putB(tf.Ema7SlopeOK)
	putF(tf.Ema7Slope)
	putB(tf.Ema7AccelOK)
	putF(tf.Ema7Accel)
	putB(tf.MacdOK)
	putF(tf.Macd)
	putB(tf.MacdSlopeOK)
	putF(tf.MacdSlope)
	putB(tf.MacdAccelOK)
	putF(tf.MacdAccel)
}

func hashStarRSX(putF func(float64), putB func(bool), rsx StarRSX) {
	putB(rsx.ValueOK)
	putF(rsx.Value)
	putB(rsx.SlopeOK)
	putF(rsx.Slope)
	putB(rsx.AccelOK)
	putF(rsx.Accel)
	putB(rsx.SignalOK)
	putF(rsx.Signal)
	putB(rsx.SignalSlopeOK)
	putF(rsx.SignalSlope)
}
