package starlens

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOutcomeArchiveCounts(t *testing.T) {
	path, err := FindOutcomeFile()
	if err != nil {
		t.Skip(err)
	}
	before, err := fileSHA(path)
	if err != nil {
		t.Fatal(err)
	}
	digest, rows, err := ReadOutcomeFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if digest != OutcomeDigest {
		t.Fatalf("digest %s", digest)
	}
	if len(rows) != UniverseStars {
		t.Fatalf("rows %d", len(rows))
	}
	var reached, notReached, noR, reached2, reached3 int
	var stopFirst1, survived1 int
	for _, row := range rows {
		if row.R2 == Reached && row.R1 != Reached {
			t.Fatalf("2R without 1R at %d", row.Index)
		}
		if row.R3 == Reached && row.R2 != Reached {
			t.Fatalf("3R without 2R at %d", row.Index)
		}
		switch row.R1 {
		case Reached:
			reached++
		case NotReachedValidR:
			notReached++
		case NoR:
			noR++
		}
		if row.R2 == Reached {
			reached2++
		}
		if row.R3 == Reached {
			reached3++
		}
		if row.Status == "STOP_FIRST" && row.R1 == Reached {
			stopFirst1++
		}
		if row.Status == "SURVIVED_WINDOW" && row.R1 == Reached {
			survived1++
		}
	}
	if reached != 4160 || notReached != 4576 || noR != 47 {
		t.Fatalf("1R reached %d not %d none %d", reached, notReached, noR)
	}
	if reached2 != 2486 || reached3 != 1642 {
		t.Fatalf("2R %d 3R %d", reached2, reached3)
	}
	if stopFirst1 != 1879 || survived1 != 2267 {
		t.Fatalf("stop-first 1R %d survived 1R %d", stopFirst1, survived1)
	}

	parent, err := OutcomeUniverse(rows)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Evaluate(parent, rows, []Clause{REvent(1, "reached")})
	if err != nil {
		t.Fatal(err)
	}
	twice, err := Evaluate(parent, rows, []Clause{REvent(1, "reached")})
	if err != nil {
		t.Fatal(err)
	}
	if again.LensPass != 4160 || again.Population != UniverseStars || !sameRefs(again.Pass, twice.Pass) {
		t.Fatalf("pass %d", again.LensPass)
	}
	pic, err := again.RPicture(1)
	if err != nil {
		t.Fatal(err)
	}
	if pic.Base != UniverseStars || pic.Reached != 4160 || pic.Undefined != 47 || pic.Rate == 1 {
		t.Fatalf("picture %+v", pic)
	}
	mfe, err := again.NumberPicture(FieldMFEATR)
	if err != nil {
		t.Fatal(err)
	}
	zeros := 0
	for _, v := range mfe.Values {
		if v == 0 {
			zeros++
		}
	}
	if mfe.Missing != 1 || zeros != 20 {
		t.Fatalf("mfe atr missing %d zeros %d", mfe.Missing, zeros)
	}

	dir := t.TempDir()
	child, err := SaveChild(parent, rows, []Clause{REvent(1, "reached")}, filepath.Join(dir, "r1.json"), time.Unix(20, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if parent.Operation != "universe" || len(parent.Members) != UniverseStars {
		t.Fatal("archive parent changed")
	}
	if err := Certify(parent, rows, child); err != nil {
		t.Fatal(err)
	}
	after, err := fileSHA(path)
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatal("outcome file changed")
	}
}

func fileSHA(path string) (string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}
