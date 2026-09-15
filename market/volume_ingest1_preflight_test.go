package market

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVolumeIngest1_ArchiveCensus(t *testing.T) {
	if os.Getenv("VOLUME_INGEST") != "1" {
		t.Skip("set VOLUME_INGEST=1 to census history.db vs Binance REST (slow, networked)")
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(root, "history.db")
	outDir := filepath.Join(root, "research", "volume")
	rep, err := RunVolumeIngest1(dbPath, outDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("\n" + FormatVolumeIngest1(rep))
	switch rep.Verdict {
	case "VOLUME_TRUTH_GREEN_NO_REPAIR", "VOLUME_TRUTH_GREEN_REPAIRED":
	default:
		t.Fatalf("verdict %s", rep.Verdict)
	}
	if rep.Repair != "NONE" && rep.Verdict == "VOLUME_TRUTH_GREEN_NO_REPAIR" {
		t.Fatal("repair field")
	}
}
