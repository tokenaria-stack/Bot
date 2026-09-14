package market

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"trading_bot/forecast"
)

func TestSetupValidation1_FromLabelSet(t *testing.T) {
	if os.Getenv("SETUP_VALIDATION_1") != "1" {
		t.Skip("set SETUP_VALIDATION_1=1 to compile SETUP-VALIDATION-1 from setup-labelset-1.json")
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "research", "setuplabels", "setup-labelset-1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var ls forecast.SetupLabelSet1
	if err := json.Unmarshal(raw, &ls); err != nil {
		t.Fatal(err)
	}
	rep, err := forecast.CompileSetupValidation1(ls)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Folds) != forecast.SetupValidationFoldCount {
		t.Fatalf("folds %d", len(rep.Folds))
	}
	dir := filepath.Join(root, "research", "setuplabels")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "setup-validation-1.json"), out, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "setup-validation-1.txt"), []byte(rep.Text), 0o644); err != nil {
		t.Fatal(err)
	}
	fmt.Fprint(os.Stderr, rep.Text)
	fmt.Fprint(os.Stderr, "SETUP-VALIDATION-1 GREEN no CatBoost 2026=OOF\n")
}
