package server

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"trading_bot/forecast/starlens"
	"trading_bot/market"
)

const (
	schema3FileSHA = "18f1e5ec79ee2ec4f2ab2afd5d5fbb213acd332df4e1396681bddd8ec652463a"
	matrixFileSHA  = "514d513dd173bba6d62b05aa0d1b3c15bf1a4ca3216aaf1d8db4ec8550c109c6"
	absentDailyAt  = 1567965600000
)

func TestStarLensStoreAttachesCoordinates(t *testing.T) {
	before3, beforeM, beforeO := frozenFileHashes(t)
	store, err := loadLens()
	if err != nil {
		t.Skip(err)
	}
	if len(store.rows) != starlens.UniverseStars {
		t.Fatalf("rows %d", len(store.rows))
	}
	uni := store.sources[lensSourceAll]
	if uni.Provenance.MatrixDigest != starlens.MatrixDigest || uni.Provenance.MatrixCommit != starlens.MatrixCommit {
		t.Fatalf("universe provenance %+v", uni.Provenance)
	}
	if uni.Provenance.OutcomeDigest != starlens.OutcomeDigest || uni.Provenance.Schema3Digest != starlens.Schema3Digest {
		t.Fatalf("universe provenance %+v", uni.Provenance)
	}
	for i := range store.rows {
		if !store.rows[i].CoordinatesAttached() {
			t.Fatalf("row %d has no coords", i)
		}
		if store.rows[i].Index != i {
			t.Fatalf("order %d", i)
		}
	}

	empty, err := starlens.Evaluate(uni, store.rows, nil)
	if err != nil {
		t.Fatal(err)
	}
	if empty.LensPass != starlens.UniverseStars {
		t.Fatalf("empty %d", empty.LensPass)
	}
	assertNotAllMissing(t, empty, "M15.Vwema")
	assertNotAllMissing(t, empty, "M15H1Vwema")
	assertNotAllMissing(t, empty, "H4RSX.Value")

	var absent *starlens.Row
	d1Zero := 0
	matrixTie := 0
	matrixMissing := 0
	for i := range store.rows {
		row := store.rows[i]
		if row.DecisionAt == absentDailyAt {
			r := row
			absent = &r
		}
		d1, _ := row.Number("D1.Vwema")
		if d1.OK && d1.Value == 0 {
			d1Zero++
		}
		for _, id := range market.RelationNames() {
			gap, _ := row.Number(id)
			if gap.OK && gap.Value == 0 {
				matrixTie++
			}
			if !gap.OK {
				matrixMissing++
			}
		}
	}
	if absent == nil {
		t.Fatal("absent daily star")
	}
	d1, _ := absent.Number("D1.Vwema")
	if d1.OK {
		t.Fatal("absent daily observed")
	}
	m15, _ := absent.Number("M15.Vwema")
	if !m15.OK {
		t.Fatal("absent star lost M15")
	}
	dailyGap, _ := absent.Number("H4D1Vwema")
	if dailyGap.OK || dailyGap.Value != 0 {
		t.Fatalf("absent daily matrix %+v", dailyGap)
	}
	if d1Zero < 4 {
		t.Fatalf("daily vwema zeros %d", d1Zero)
	}
	if matrixTie == 0 {
		t.Fatal("no matrix 0/true")
	}
	if matrixMissing == 0 {
		t.Fatal("no matrix missing")
	}

	vwema, _ := store.rows[0].Number("M15.Vwema")
	if !vwema.OK {
		t.Fatal("first star M15.Vwema missing")
	}

	dir := t.TempDir()
	child, err := starlens.SaveChild(uni, store.rows, []starlens.Clause{
		starlens.Continuous("M15H1Vwema", starlens.CmpGTE, 0),
	}, filepath.Join(dir, "mx.json"), time.Unix(3, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if child.Provenance.MatrixDigest != starlens.MatrixDigest {
		t.Fatalf("saved %+v", child.Provenance)
	}
	if err := starlens.Certify(uni, store.rows, child); err != nil {
		t.Fatal(err)
	}
	bare := uni
	bare.Provenance.MatrixDigest = ""
	bare.Provenance.MatrixCommit = ""
	if _, err := starlens.SaveChild(bare, store.rows, []starlens.Clause{
		starlens.Continuous("M15H1Vwema", starlens.CmpGTE, 0),
	}, filepath.Join(dir, "bad.json"), time.Unix(3, 0).UTC()); err == nil {
		t.Fatal("empty matrix provenance saved")
	}

	got, err := starlens.Evaluate(uni, store.rows, []starlens.Clause{
		starlens.REvent(1, "reached"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.LensPass != 4160 {
		t.Fatalf("1R %d", got.LensPass)
	}

	after3, afterM, afterO := frozenFileHashes(t)
	if after3 != before3 || afterM != beforeM || afterO != beforeO {
		t.Fatal("frozen files changed")
	}
	if before3 != schema3FileSHA || beforeM != matrixFileSHA {
		t.Fatalf("hashes %s %s", before3, beforeM)
	}
}

func assertNotAllMissing(t *testing.T, got starlens.Result, field string) {
	t.Helper()
	pic, err := got.NumberPicture(field)
	if err != nil {
		t.Fatal(err)
	}
	if pic.Base != starlens.UniverseStars {
		t.Fatalf("%s base %d", field, pic.Base)
	}
	if pic.Missing == starlens.UniverseStars || len(pic.Values) == 0 {
		t.Fatalf("%s all missing values=%d missing=%d", field, len(pic.Values), pic.Missing)
	}
	if pic.Missing+len(pic.Values) != pic.Base {
		t.Fatalf("%s missing+values %d+%d != %d", field, pic.Missing, len(pic.Values), pic.Base)
	}
}

func frozenFileHashes(t *testing.T) (s3, mx, out string) {
	t.Helper()
	s3p, err := starlens.FindSchema3File()
	if err != nil {
		t.Fatal(err)
	}
	mxp, err := starlens.FindMatrixFile()
	if err != nil {
		t.Fatal(err)
	}
	outp, err := starlens.FindOutcomeFile()
	if err != nil {
		t.Fatal(err)
	}
	return fileSHA256(t, s3p), fileSHA256(t, mxp), fileSHA256(t, outp)
}

func fileSHA256(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	_ = path
	return hex.EncodeToString(sum[:])
}
