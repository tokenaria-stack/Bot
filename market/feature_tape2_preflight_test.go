package market

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"trading_bot/data"
	"trading_bot/forecast"
)

func TestDumpFeatureTape2_PreflightArchive(t *testing.T) {
	if os.Getenv("FEATURE_TAPE2_PREFLIGHT") != "1" {
		t.Skip("set FEATURE_TAPE2_PREFLIGHT=1 to dump/read canonical BTCUSDT FUTURES_PERP 15m/1h/4h")
	}
	s, err := ResearchFeatureSpec2()
	if err != nil {
		t.Fatal(err)
	}
	p := loadResearchClosedBarsTF(t, "15m")
	h1 := loadResearchClosedBarsTF(t, "1h")
	h4 := loadResearchClosedBarsTF(t, "4h")
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "research", "tapes2")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	name, err := ResearchFeatureTape2FileName(s)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	t0 := time.Now()
	if err := DumpFeatureTape2(path, s, p, h1, h4); err != nil {
		t.Fatal(err)
	}
	hdr, rows, foot, err := forecast.ReadTape2(path)
	if err != nil {
		t.Fatal(err)
	}
	firstReady := int64(0)
	for _, r := range rows {
		if r.Ready {
			firstReady = r.At
			break
		}
	}
	t.Logf("FEATURE-TAPE-2 PREFLIGHT elapsed=%s", time.Since(t0))
	t.Logf("primary start=%d 1h start=%d 4h start=%d", p[0].OpenTime, h1[0].OpenTime, h4[0].OpenTime)
	t.Logf("rows=%d ready=%d not_ready=%d firstReady=%d last=%d", foot.RowCount, foot.ReadyCount, foot.NotReadyCount, firstReady, foot.LastAt)
	t.Logf("src15=%s src1h=%s src4h=%s content=%s", hdr.PrimarySource, hdr.HTF1hSource, hdr.HTF4hSource, foot.ContentDigest)
	if foot.RowCount != len(p) {
		t.Fatalf("row count")
	}
	if data.IsFormingCloseTime(p[len(p)-1].CloseTime, time.Now().UnixMilli()) {
		t.Fatal("forming primary")
	}
	pass2 := path + ".pass2"
	if err := DumpFeatureTape2(pass2, s, p, h1, h4); err != nil {
		t.Fatal(err)
	}
	_, _, f2, err := forecast.ReadTape2(pass2)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(pass2)
	if f2.ContentDigest != foot.ContentDigest {
		t.Fatal("preflight determinism")
	}
}
