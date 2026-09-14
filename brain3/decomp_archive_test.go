package brain3

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestSetupProbabilityDecomp1_Archive(t *testing.T) {
	if os.Getenv("SETUP_DECOMP") != "1" {
		t.Skip("set SETUP_DECOMP=1 to decompose frozen metalabel-1-oof.jsonl")
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	oof, err := LoadOOF(filepath.Join(root, "research", "brain3", "metalabel-1-oof.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	dsRaw, err := os.ReadFile(filepath.Join(root, "research", "setupdataset", "setup-dataset-1.json"))
	if err != nil {
		t.Fatal(err)
	}
	valRaw, err := os.ReadFile(filepath.Join(root, "research", "setuplabels", "setup-validation-1.json"))
	if err != nil {
		t.Fatal(err)
	}
	ds, err := LoadDataset(dsRaw, SetupClassSchema())
	if err != nil {
		t.Fatal(err)
	}
	val, err := LoadValidation(valRaw)
	if err != nil {
		t.Fatal(err)
	}
	cen, err := ResolveOuterFolds(ds, val)
	if err != nil {
		t.Fatal(err)
	}
	priors, trainC, err := OuterTrainResolvedPriors(ds, cen)
	if err != nil {
		t.Fatal(err)
	}
	got, err := RunDecomp(oof, priors, trainC)
	if err != nil {
		t.Fatal(err)
	}
	if got.Rows != 4227 || got.MaxAt >= got.HoldoutStartAt {
		t.Fatal(got.Rows, got.MaxAt)
	}
	if got.QResFolds[0].Baseline.Timeout != 0 {
		t.Fatal("resolved-only timeout")
	}
	out := filepath.Join(root, "research", "brain3", "setup-probability-decomposition-1.txt")
	if err := os.WriteFile(out, []byte(got.Text), 0o644); err != nil {
		t.Fatal(err)
	}
	fmt.Fprint(os.Stderr, got.Text)
}
