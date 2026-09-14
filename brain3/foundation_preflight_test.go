package brain3

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestBrain3Foundation1_ArchivePreflight(t *testing.T) {
	if os.Getenv("BRAIN3_FOUNDATION") != "1" {
		t.Skip("set BRAIN3_FOUNDATION=1 to consume frozen SETUP-DATASET-1 + SETUP-VALIDATION-1")
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
	c, err := ResolveOuterFolds(ds, val)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "research", "brain3")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "foundation-1.txt"), []byte(c.Text), 0o644); err != nil {
		t.Fatal(err)
	}
	fmt.Fprint(os.Stderr, c.Text)
	fmt.Fprint(os.Stderr, "BRAIN3-FOUNDATION-1 GREEN no CatBoost fit\n")
}
