package starlens

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func fixture() []Row {
	return []Row{
		{
			Index: 0, DecisionAt: 1000, Side: "up", Status: "SURVIVED_WINDOW",
			MFEATR: Observed{Value: 0, OK: true},
			R:      Observed{Value: 10, OK: true},
			R1:     NotReachedValidR,
			Stop:   StopNotTouched,
		},
		{
			Index: 1, DecisionAt: 2000, Side: "down", Status: "STOP_FIRST",
			MFEATR: Observed{},
			R:      Observed{Value: 12, OK: true},
			R1:     Reached,
			R2:     Reached,
			Stop:   StopTouched,
		},
		{
			Index: 2, DecisionAt: 3000, Side: "up", Status: "INVALID_GEOMETRY",
			MFEATR:   Observed{Value: 0, OK: false},
			MFEPrice: Observed{Value: 4, OK: true},
			R1:       NoR,
			Stop:     NoStop,
		},
		{
			Index: 3, DecisionAt: 4000, Side: "up", Status: "SURVIVED_WINDOW",
			MFEATR: Observed{Value: 6, OK: true},
			R:      Observed{Value: 8, OK: true},
			R1:     Reached,
			Stop:   StopNotTouched,
		},
		{
			Index: 4, DecisionAt: 5000, Side: "down", Status: "INCOMPLETE_WINDOW",
			MFEATR: Observed{Value: 2, OK: true},
			R:      Observed{Value: 9, OK: true},
			R1:     NotReachedValidR,
			Stop:   StopNotTouched,
		},
	}
}

func fixtureParent(t *testing.T) Population {
	t.Helper()
	pop, err := OutcomeUniverse(fixture())
	if err != nil {
		t.Fatal(err)
	}
	return pop
}

func TestRealZeroIsObservedAndMissingZeroIsNot(t *testing.T) {
	parent := fixtureParent(t)
	table := fixture()
	got, err := Evaluate(parent, table, []Clause{Continuous(FieldMFEATR, CmpLTE, 0)})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Pass) != 1 || got.Pass[0].Index != 0 {
		t.Fatalf("pass %+v", got.Pass)
	}
	pic, err := got.NumberPicture(FieldMFEATR)
	if err != nil {
		t.Fatal(err)
	}
	if pic.Missing != 2 {
		t.Fatalf("missing %d", pic.Missing)
	}
	zeros := 0
	for _, v := range pic.Values {
		if v == 0 {
			zeros++
		}
	}
	if zeros != 1 || len(pic.Values) != 3 {
		t.Fatalf("values %v missing %d", pic.Values, pic.Missing)
	}
}

func TestUndefinedRIsNotNotReached(t *testing.T) {
	parent := fixtureParent(t)
	table := fixture()
	notReached, err := Evaluate(parent, table, []Clause{REvent(1, "not_reached")})
	if err != nil {
		t.Fatal(err)
	}
	for _, member := range notReached.Pass {
		if member.Index == 2 {
			t.Fatal("undefined R passed not_reached")
		}
	}
	if len(notReached.Pass) != 2 {
		t.Fatalf("not reached %+v", notReached.Pass)
	}
	reached, err := Evaluate(parent, table, []Clause{REvent(1, "reached")})
	if err != nil {
		t.Fatal(err)
	}
	if len(reached.Pass) != 2 {
		t.Fatalf("reached %+v", reached.Pass)
	}
	pic, err := reached.RPicture(1)
	if err != nil {
		t.Fatal(err)
	}
	if pic.Reached != 2 || pic.NotReached != 2 || pic.Undefined != 1 || pic.Base != 5 {
		t.Fatalf("picture %+v", pic)
	}
	if pic.Rate == 1 {
		t.Fatal("active reached clause collapsed the rate")
	}
}

func TestStopFirstAndSurvivedBothReach1R(t *testing.T) {
	parent := fixtureParent(t)
	table := fixture()
	stop, err := Evaluate(parent, table, []Clause{Status("stop first"), REvent(1, "reached")})
	if err != nil {
		t.Fatal(err)
	}
	if len(stop.Pass) != 1 || stop.Pass[0].Index != 1 {
		t.Fatalf("stop first %+v", stop.Pass)
	}
	lived, err := Evaluate(parent, table, []Clause{Status("survived"), REvent(1, "reached")})
	if err != nil {
		t.Fatal(err)
	}
	if len(lived.Pass) != 1 || lived.Pass[0].Index != 3 {
		t.Fatalf("survived %+v", lived.Pass)
	}
}

func TestEmptyLensAndMissingFailsClause(t *testing.T) {
	parent := fixtureParent(t)
	table := fixture()
	empty, err := Evaluate(parent, table, nil)
	if err != nil {
		t.Fatal(err)
	}
	if empty.LensPass != len(table) || empty.Population != len(table) {
		t.Fatalf("empty %+v", empty.LensPass)
	}
	missing, err := Evaluate(parent, table, []Clause{Continuous(FieldMFEATR, CmpGTE, 0)})
	if err != nil {
		t.Fatal(err)
	}
	for _, member := range missing.Pass {
		if member.Index == 1 || member.Index == 2 {
			t.Fatalf("missing passed %+v", missing.Pass)
		}
	}
}

func TestSelfExclusionKeepsBothSides(t *testing.T) {
	parent := fixtureParent(t)
	table := fixture()
	got, err := Evaluate(parent, table, []Clause{
		Status("survived"),
		Continuous(FieldMFEATR, CmpGTE, 5),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Pass) != 1 || got.Pass[0].Index != 3 {
		t.Fatalf("pass %+v", got.Pass)
	}
	pic, err := got.NumberPicture(FieldMFEATR)
	if err != nil {
		t.Fatal(err)
	}
	if pic.Base != 2 || len(pic.Values) != 2 {
		t.Fatalf("picture %+v", pic)
	}
	sawLow := false
	for _, v := range pic.Values {
		if v == 0 {
			sawLow = true
		}
	}
	if !sawLow {
		t.Fatal("threshold removed the low side of its own picture")
	}
}

func TestNoReadingAndRawStatus(t *testing.T) {
	parent := fixtureParent(t)
	table := fixture()
	none, err := Evaluate(parent, table, []Clause{Status("no reading")})
	if err != nil {
		t.Fatal(err)
	}
	if len(none.Pass) != 2 {
		t.Fatalf("no reading %+v", none.Pass)
	}
	inc, err := Evaluate(parent, table, []Clause{Status("INCOMPLETE_WINDOW")})
	if err != nil {
		t.Fatal(err)
	}
	if len(inc.Pass) != 1 || inc.Pass[0].Index != 4 {
		t.Fatalf("incomplete %+v", inc.Pass)
	}
	if _, err := Evaluate(parent, table, []Clause{Status("win")}); err == nil {
		t.Fatal("win was accepted")
	}
}

func TestUnattachedCoordinateFailsClosed(t *testing.T) {
	parent := fixtureParent(t)
	table := fixture()
	_, err := Evaluate(parent, table, []Clause{Continuous("M15.Vwema", CmpGTE, 0)})
	if err == nil || !strings.Contains(err.Error(), "coordinates are not attached") {
		t.Fatalf("want attach error, got %v", err)
	}
	_, err = Evaluate(parent, table, []Clause{Continuous(FieldMFEATR, CmpGTE, 0)})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBinsAndViewportStayOutsideTheLens(t *testing.T) {
	parent := fixtureParent(t)
	table := fixture()
	got, err := Evaluate(parent, table, []Clause{Side("up")})
	if err != nil {
		t.Fatal(err)
	}
	before := append([]StarRef(nil), got.Pass...)
	bins, err := CountBins([]float64{0, 5, 6}, []float64{0, 5})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(bins, []int{1, 1, 1}) {
		t.Fatalf("bins %v", bins)
	}
	view := ViewCount(got.Pass, 1000, 4000)
	if view != 2 {
		t.Fatalf("view %d", view)
	}
	if !reflect.DeepEqual(got.Pass, before) || got.LensPass != len(before) {
		t.Fatal("presentation changed membership")
	}
}

func TestSaveReplaysAndLeavesParent(t *testing.T) {
	parent := fixtureParent(t)
	table := fixture()
	before := parent.cloneMembers()
	dir := t.TempDir()
	path := filepath.Join(dir, "child.json")
	clauses := []Clause{Status("survived"), REvent(1, "reached")}
	child, err := SaveChild(parent, table, clauses, path, time.Unix(10, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if !parent.sameMembers(before) || parent.Operation != "universe" || len(parent.Clauses) != 0 {
		t.Fatal("parent changed")
	}
	loaded, err := LoadPopulation(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := Certify(parent, table, loaded); err != nil {
		t.Fatal(err)
	}
	loaded.Members = []StarRef{table[0].Ref()}
	if err := Certify(parent, table, loaded); err == nil {
		t.Fatal("tampered child certified")
	}
	again, err := LoadPopulation(path)
	if err != nil {
		t.Fatal(err)
	}
	if !sameRefs(again.Members, child.Members) {
		t.Fatal("certify rewrote the file")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, part := range []string{"viewport", "timeToStop", "capture", "\"bins\""} {
		if strings.Contains(text, part) {
			t.Fatalf("saved file holds %s", part)
		}
	}
	if child.Provenance.OutcomeDigest != OutcomeDigest || child.Provenance.Schema3Commit != Schema3Commit {
		t.Fatalf("provenance %+v", child.Provenance)
	}
}
