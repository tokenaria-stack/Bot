package starlens

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"trading_bot/market"
)

func TestContinuousCatalogCount(t *testing.T) {
	if len(outcomeIDs) != OutcomeFieldCount {
		t.Fatalf("outcome %d", len(outcomeIDs))
	}
	if len(schema3IDs) != Schema3CoordCount {
		t.Fatalf("schema3 %d", len(schema3IDs))
	}
	if len(matrixIDs) != MatrixCoordCount {
		t.Fatalf("matrix %d", len(matrixIDs))
	}
	if len(continuousIDs) != ContinuousIDCount {
		t.Fatalf("continuous %d", len(continuousIDs))
	}
	if _, ok := schema3IDs["RSXMinusSignal"]; !ok {
		t.Fatal("missing root RSXMinusSignal")
	}
	for id := range matrixIDs {
		if strings.HasPrefix(id, "M15H4") || strings.HasPrefix(id, "M15D1") || id == "M15RsxMinusSignal" {
			t.Fatalf("forbidden matrix id %s", id)
		}
	}
	raw := market.RawObservations(market.StarSnapshotV3{})
	if len(raw) != 142 {
		t.Fatalf("raw %d", len(raw))
	}
	if len(market.RelationObservations(market.StarRelations{})) != 49 {
		t.Fatal("relations")
	}
	if _, ok := knownField("M15H4Vwema"); ok {
		t.Fatal("M15H4Vwema accepted")
	}
	if _, ok := knownField("not-a-field"); ok {
		t.Fatal("unknown accepted")
	}
	if _, ok := knownField(FieldMFEATR); !ok {
		t.Fatal("mfeAtr missing")
	}
}

func TestSameContinuousHoldsPath(t *testing.T) {
	row := Row{
		Index: 0, DecisionAt: 1, Side: "up",
		MFEATR: Observed{Value: 4, OK: true},
		coords: map[string]Observed{
			"M15.Vwema":   {Value: 1, OK: true},
			"M15H1Vwema":  {Value: 2, OK: true},
			"H4RSX.Value": {Value: 40, OK: true},
			"TVAge":       {Value: 0, OK: true},
		},
	}
	fields := []string{FieldMFEATR, "M15.Vwema", "M15H1Vwema", "H4RSX.Value", "TVAge"}
	for _, field := range fields {
		n, ok := row.Number(field)
		if !ok {
			t.Fatalf("number %s", field)
		}
		clause := Continuous(field, CmpGTE, 0)
		got, err := clause.holds(row)
		if err != nil {
			t.Fatal(err)
		}
		want, err := holdContinuous(n, CmpGTE, 0)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("%s holds %v want %v", field, got, want)
		}
	}
}

func TestVwemaZeroObservation(t *testing.T) {
	rows := []Row{{Index: 0, DecisionAt: 10, Side: "up"}}
	snap := market.StarSnapshotV3{}
	snap.Side = "up"
	snap.ConfirmedAt = 10
	snap.M15.Present = true
	snap.M15.Vwema = 0
	snap.M15.ValuesOK = false
	rel := market.StarRelations{Side: "up", ConfirmedAt: 10}
	if err := AttachCoordinates(rows, []market.StarSnapshotV3{snap}, []market.StarRelations{rel}); err != nil {
		t.Fatal(err)
	}
	n, ok := rows[0].Number("M15.Vwema")
	if !ok || !n.OK || n.Value != 0 {
		t.Fatalf("observed zero %+v %v", n, ok)
	}
	parent, err := OutcomeUniverse(rows)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Evaluate(parent, rows, []Clause{Continuous("M15.Vwema", CmpLTE, 0)})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Pass) != 1 {
		t.Fatalf("pass %+v", got.Pass)
	}
	pic, err := got.NumberPicture("M15.Vwema")
	if err != nil {
		t.Fatal(err)
	}
	if pic.Missing != 0 || len(pic.Values) != 1 || pic.Values[0] != 0 {
		t.Fatalf("picture %+v", pic)
	}

	rows[0].coords = nil
	snap.M15.Present = false
	if err := AttachCoordinates(rows, []market.StarSnapshotV3{snap}, []market.StarRelations{rel}); err != nil {
		t.Fatal(err)
	}
	n, _ = rows[0].Number("M15.Vwema")
	if n.OK {
		t.Fatal("absent became observed")
	}
	fail, err := Evaluate(parent, rows, []Clause{Continuous("M15.Vwema", CmpLTE, 0)})
	if err != nil {
		t.Fatal(err)
	}
	if len(fail.Pass) != 0 {
		t.Fatal("missing passed")
	}
	pic, err = fail.NumberPicture("M15.Vwema")
	if err != nil {
		t.Fatal(err)
	}
	if pic.Missing != 1 || len(pic.Values) != 0 {
		t.Fatalf("missing picture %+v", pic)
	}
}

func TestWidthAndCloseWidthGates(t *testing.T) {
	rows := []Row{{Index: 0, DecisionAt: 10, Side: "up"}}
	snap := market.StarSnapshotV3{}
	snap.Side = "up"
	snap.ConfirmedAt = 10
	snap.M15.Present = true
	snap.M15.Vwema = 1
	snap.M15.ValuesOK = false
	snap.M15.Width = 9
	snap.M15.WidthOK = false
	snap.M15.CloseWidth = 3
	snap.M15.CloseWidthOK = true
	rel := market.StarRelations{Side: "up", ConfirmedAt: 10}
	if err := AttachCoordinates(rows, []market.StarSnapshotV3{snap}, []market.StarRelations{rel}); err != nil {
		t.Fatal(err)
	}
	w, _ := rows[0].Number("M15.Width")
	if w.OK {
		t.Fatal("width used stored number without WidthOK")
	}
	cw, _ := rows[0].Number("M15.CloseWidth")
	if !cw.OK || cw.Value != 3 {
		t.Fatalf("close width %+v", cw)
	}
}

func TestMatrixZeroOK(t *testing.T) {
	rows := []Row{{Index: 0, DecisionAt: 10, Side: "up"}}
	snap := market.StarSnapshotV3{StarSnapshot: market.StarSnapshot{Side: "up", ConfirmedAt: 10}}
	missing := market.StarRelations{Side: "up", ConfirmedAt: 10, M15H1Vwema: market.Rel{Value: 0, OK: false}}
	if err := AttachCoordinates(rows, []market.StarSnapshotV3{snap}, []market.StarRelations{missing}); err != nil {
		t.Fatal(err)
	}
	n, _ := rows[0].Number("M15H1Vwema")
	if n.OK {
		t.Fatal("0 OK false observed")
	}
	tie := market.StarRelations{Side: "up", ConfirmedAt: 10, M15H1Vwema: market.Rel{Value: 0, OK: true}}
	if err := AttachCoordinates(rows, []market.StarSnapshotV3{snap}, []market.StarRelations{tie}); err != nil {
		t.Fatal(err)
	}
	n, _ = rows[0].Number("M15H1Vwema")
	if !n.OK || n.Value != 0 {
		t.Fatalf("tie %+v", n)
	}
}

func TestDailyAbsenceDoesNotPoisonLowerTF(t *testing.T) {
	rows := []Row{{Index: 0, DecisionAt: 10, Side: "up"}}
	snap := market.StarSnapshotV3{}
	snap.Side = "up"
	snap.ConfirmedAt = 10
	snap.M15.Present = true
	snap.M15.Vwema = 5
	snap.H1.Present = true
	snap.H1.Vwema = 4
	snap.H4.Present = true
	snap.H4.Vwema = 3
	snap.D1.Present = false
	rel := market.StarRelations{Side: "up", ConfirmedAt: 10}
	if err := AttachCoordinates(rows, []market.StarSnapshotV3{snap}, []market.StarRelations{rel}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"M15.Vwema", "H1.Vwema", "H4.Vwema"} {
		n, _ := rows[0].Number(id)
		if !n.OK {
			t.Fatalf("%s missing", id)
		}
	}
	d1, _ := rows[0].Number("D1.Vwema")
	if d1.OK {
		t.Fatal("daily present")
	}
}

func TestNonFiniteOrangeIsMissing(t *testing.T) {
	rows := []Row{{Index: 0, DecisionAt: 10, Side: "up"}}
	snap := market.StarSnapshotV3{}
	snap.Side = "up"
	snap.ConfirmedAt = 10
	snap.M15.Present = true
	snap.M15.ChanMid = math.NaN()
	rel := market.StarRelations{Side: "up", ConfirmedAt: 10}
	if err := AttachCoordinates(rows, []market.StarSnapshotV3{snap}, []market.StarRelations{rel}); err != nil {
		t.Fatal(err)
	}
	n, _ := rows[0].Number("M15.ChanMid")
	if n.OK {
		t.Fatal("NaN observed")
	}
}

func coordFixture() []Row {
	rows := []Row{
		{Index: 0, DecisionAt: 1000, Side: "up", MFEATR: Observed{Value: 5, OK: true}},
		{Index: 1, DecisionAt: 2000, Side: "down", MFEATR: Observed{Value: 1, OK: true}},
		{Index: 2, DecisionAt: 3000, Side: "up", MFEATR: Observed{}},
		{Index: 3, DecisionAt: 4000, Side: "up", MFEATR: Observed{Value: 6, OK: true}},
	}
	snaps := make([]market.StarSnapshotV3, 4)
	rels := make([]market.StarRelations, 4)
	for i := range rows {
		snaps[i].Side = rows[i].Side
		snaps[i].ConfirmedAt = rows[i].DecisionAt
		snaps[i].M15.Present = true
		snaps[i].M15.Vwema = float64(i)
		snaps[i].H4.Present = i != 2
		snaps[i].H4RSX.Value = 40
		snaps[i].H4RSX.ValueOK = i != 2
		rels[i].Side = rows[i].Side
		rels[i].ConfirmedAt = rows[i].DecisionAt
		rels[i].M15H1Vwema = market.Rel{Value: float64(i) - 1, OK: i != 2}
	}
	if err := AttachCoordinates(rows, snaps, rels); err != nil {
		panic(err)
	}
	return rows
}

func TestCoordinateSelfExclusion(t *testing.T) {
	table := coordFixture()
	parent, err := OutcomeUniverse(table)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Evaluate(parent, table, []Clause{
		Continuous("M15.Vwema", CmpGTE, 2),
		Continuous("M15H1Vwema", CmpGTE, 0),
		Continuous("H4RSX.Value", CmpLT, 50),
		Continuous(FieldMFEATR, CmpGTE, 4),
	})
	if err != nil {
		t.Fatal(err)
	}
	pic, err := got.NumberPicture("M15.Vwema")
	if err != nil {
		t.Fatal(err)
	}
	if pic.Base != 1 {
		t.Fatalf("vwema picture base %d", pic.Base)
	}
	if len(pic.Values) != 1 || pic.Values[0] != 3 {
		t.Fatalf("vwema values %v", pic.Values)
	}
	relPic, err := got.NumberPicture("M15H1Vwema")
	if err != nil {
		t.Fatal(err)
	}
	if relPic.Base != 1 {
		t.Fatalf("matrix picture base %d", relPic.Base)
	}
}

func TestMixedLensANDAndOrder(t *testing.T) {
	table := coordFixture()
	parent, err := OutcomeUniverse(table)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Evaluate(parent, table, []Clause{
		Continuous(FieldMFEATR, CmpGTE, 4),
		Continuous("M15.Vwema", CmpGTE, 0),
		Continuous("M15H1Vwema", CmpGTE, 0),
		Continuous("H4RSX.Value", CmpLT, 50),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Pass) != 1 || got.Pass[0].Index != 3 {
		t.Fatalf("pass %+v", got.Pass)
	}
	if got.Pass[0].DecisionAt != 4000 {
		t.Fatal("order")
	}
}

func TestMatrixProvenanceSave(t *testing.T) {
	table := coordFixture()
	parent, err := OutcomeUniverse(table)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	outcomeOnly := []Clause{Continuous(FieldMFEATR, CmpGTE, 4)}
	if _, err := SaveChild(parent, table, outcomeOnly, filepath.Join(dir, "out.json"), time.Unix(1, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	matrixClause := []Clause{Continuous("M15H1Vwema", CmpGTE, 0)}
	if _, err := SaveChild(parent, table, matrixClause, filepath.Join(dir, "bad.json"), time.Unix(1, 0).UTC()); err == nil {
		t.Fatal("missing matrix provenance accepted")
	}
	parent.Provenance.MatrixDigest = MatrixDigest
	parent.Provenance.MatrixCommit = MatrixCommit
	child, err := SaveChild(parent, table, matrixClause, filepath.Join(dir, "ok.json"), time.Unix(1, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if child.Provenance.MatrixDigest != MatrixDigest || child.Provenance.MatrixCommit != MatrixCommit {
		t.Fatalf("child provenance %+v", child.Provenance)
	}
	if err := Certify(parent, table, child); err != nil {
		t.Fatal(err)
	}
}

func TestUnknownContinuousRejected(t *testing.T) {
	parent := fixtureParent(t)
	if _, err := Evaluate(parent, fixture(), []Clause{Continuous("M15H4Vwema", CmpGTE, 0)}); err == nil {
		t.Fatal("M15H4 accepted")
	}
}

const (
	schema3FileSHA = "18f1e5ec79ee2ec4f2ab2afd5d5fbb213acd332df4e1396681bddd8ec652463a"
	matrixFileSHA  = "514d513dd173bba6d62b05aa0d1b3c15bf1a4ca3216aaf1d8db4ec8550c109c6"
	absentDailyAt  = 1567965600000
)

func TestFrozenCoordinateFiles(t *testing.T) {
	v3Path, err := FindSchema3File()
	if err != nil {
		t.Skip(err)
	}
	mxPath, err := FindMatrixFile()
	if err != nil {
		t.Skip(err)
	}
	before3, err := fileSHA(v3Path)
	if err != nil {
		t.Fatal(err)
	}
	beforeM, err := fileSHA(mxPath)
	if err != nil {
		t.Fatal(err)
	}
	if before3 != schema3FileSHA || beforeM != matrixFileSHA {
		t.Fatalf("frozen hashes %s %s", before3, beforeM)
	}
	digest, snaps, err := ReadSchema3File(v3Path)
	if err != nil {
		t.Fatal(err)
	}
	if digest != Schema3Digest || len(snaps) != UniverseStars {
		t.Fatalf("schema3 %s %d", digest, len(snaps))
	}
	mdigest, rels, err := ReadMatrixFile(mxPath)
	if err != nil {
		t.Fatal(err)
	}
	if mdigest != MatrixDigest || len(rels) != UniverseStars {
		t.Fatalf("matrix %s %d", mdigest, len(rels))
	}
	outPath, err := FindOutcomeFile()
	if err != nil {
		t.Skip(err)
	}
	_, outcomes, err := ReadOutcomeFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := AttachCoordinates(outcomes, snaps, rels); err != nil {
		t.Fatal(err)
	}
	var absent *Row
	zeros := 0
	for i := range outcomes {
		if outcomes[i].DecisionAt == absentDailyAt {
			row := outcomes[i]
			absent = &row
		}
		n, _ := outcomes[i].Number("D1.Vwema")
		if n.OK && n.Value == 0 {
			zeros++
		}
	}
	if absent == nil {
		t.Fatal("absent star missing")
	}
	d1, _ := absent.Number("D1.Vwema")
	if d1.OK {
		t.Fatal("absent daily observed")
	}
	m15, _ := absent.Number("M15.Vwema")
	h1, _ := absent.Number("H1.Vwema")
	if !m15.OK || !h1.OK {
		t.Fatalf("lower tf %+v %+v", m15, h1)
	}
	if zeros < 4 {
		t.Fatalf("daily vwema zeros %d", zeros)
	}
	after3, err := fileSHA(v3Path)
	if err != nil {
		t.Fatal(err)
	}
	afterM, err := fileSHA(mxPath)
	if err != nil {
		t.Fatal(err)
	}
	if after3 != before3 || afterM != beforeM {
		t.Fatal("frozen files changed")
	}
	if _, err := os.Stat(v3Path); err != nil {
		t.Fatal(err)
	}
}

func TestNoBrowserSubtractionInStarlens(t *testing.T) {
	body, err := os.ReadFile("coord.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)
	if strings.Contains(src, "Vwema-") || strings.Contains(src, "lo.Vwema") {
		t.Fatal("coord adapter subtracts raw fields")
	}
}
