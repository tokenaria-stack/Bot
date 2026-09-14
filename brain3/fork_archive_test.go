package brain3

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestTPStopInformationFork1_Archive(t *testing.T) {
	if os.Getenv("TPSTOP_FORK") != "1" {
		t.Skip("set TPSTOP_FORK=1 to freeze spec, audit, and one binary OOF probe")
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
	got, err := RunTPStopFork(ds, val, defaultPython(root), root, work)
	if err != nil {
		t.Fatal(err)
	}
	if got.PreDigest != got.PostDigest {
		t.Fatal("SPEC_MUTATED_AFTER_AUDIT")
	}
	if got.Probe.OOFRows == 0 || got.Probe.MaxOOFAt >= val.HoldoutStartAt {
		t.Fatal("oof")
	}
	if err := writeForkArtifacts(root, got); err != nil {
		t.Fatal(err)
	}
	fmt.Fprint(os.Stderr, got.Text)
}
