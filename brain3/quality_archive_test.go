package brain3

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestSetupQualityCurve1_Archive(t *testing.T) {
	if os.Getenv("SETUP_QUALITY") != "1" {
		t.Skip("set SETUP_QUALITY=1 to analyze frozen metalabel-1-oof.jsonl")
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "research", "brain3", "metalabel-1-oof.jsonl")
	oof, err := LoadOOF(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := RunQualityCurve(oof)
	if err != nil {
		t.Fatal(err)
	}
	if got.Rows != 4227 {
		t.Fatalf("rows %d", got.Rows)
	}
	if got.MaxAt >= got.HoldoutStartAt {
		t.Fatal("holdout")
	}
	if got.Folds[0].Slices[0].N != 1083 || got.Folds[0].Slices[0].TPLiftPP != 0 {
		t.Fatalf("fold0 100%% %+v", got.Folds[0].Slices[0])
	}
	out := filepath.Join(root, "research", "brain3", "setup-quality-curve-1.txt")
	if err := os.WriteFile(out, []byte(got.Text), 0o644); err != nil {
		t.Fatal(err)
	}
	fmt.Fprint(os.Stderr, got.Text)
}
