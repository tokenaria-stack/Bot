package brain3

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestBrain3Metalabel1_InnerPreflightArchive(t *testing.T) {
	if os.Getenv("BRAIN3_METALABEL") != "1" && os.Getenv("BRAIN3_METALABEL_PREFLIGHT") != "1" {
		t.Skip("set BRAIN3_METALABEL_PREFLIGHT=1 (or BRAIN3_METALABEL=1) to compile Spec1 inner tails")
	}
	root, err := filepath.Abs("..")
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
	inner, err := PreflightInnerGeometry(ds, cen, Spec1())
	if err != nil {
		t.Fatal(err)
	}
	txt := FormatInnerPreflight(inner)
	dir := filepath.Join(root, "research", "brain3")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "metalabel-1-preflight.txt"), []byte(txt), 0o644); err != nil {
		t.Fatal(err)
	}
	fmt.Fprint(os.Stderr, txt)
}

func TestBrain3Metalabel1_ArchiveOOF(t *testing.T) {
	if os.Getenv("BRAIN3_METALABEL") != "1" {
		t.Skip("set BRAIN3_METALABEL=1 to run one Spec1 archive OOF")
	}
	root, err := filepath.Abs("..")
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
	work := filepath.Join(root, "research", "brain3", "work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := RunMetalabel1(ds, val, Spec1(), defaultPython(root), root, work)
	if err != nil {
		t.Fatal(err)
	}
	if got.OOFRows != 4227 {
		t.Fatalf("oof rows %d", got.OOFRows)
	}
	if got.MaxOOFAt >= val.HoldoutStartAt {
		t.Fatal("holdout leaked")
	}
	out := filepath.Join(root, "research", "brain3", "metalabel-1-oof.jsonl")
	if err := WriteOOF(out, got, ds, val); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "research", "brain3", "metalabel-1.txt"), []byte(got.Text), 0o644); err != nil {
		t.Fatal(err)
	}
	fmt.Fprint(os.Stderr, got.Text)
	fmt.Fprint(os.Stderr, "BRAIN3-METALABEL-1 GREEN one OOF run\n")
}
