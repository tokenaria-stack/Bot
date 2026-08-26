package exchange

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNativeBinanceSet(t *testing.T) {
	t.Parallel()
	want := []string{
		"1m", "3m", "5m", "15m", "30m",
		"1h", "2h", "4h", "6h", "8h", "12h",
		"1d", "1w", "1M",
	}
	got := NativeBinanceIDs()
	if len(got) != len(want) {
		t.Fatalf("NativeBinanceIDs len=%d want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("NativeBinanceIDs[%d]=%q want %q", i, got[i], want[i])
		}
	}
	for _, id := range want {
		e, ok := TimeframeByName(id)
		if !ok {
			t.Fatalf("missing catalog row %q", id)
		}
		if e.Class != TFClassNative {
			t.Fatalf("%s class=%s want native", id, e.Class)
		}
		if !e.Persist {
			t.Fatalf("%s persist=false want true", id)
		}
		if e.LiveSource != LiveBinanceKlineWS {
			t.Fatalf("%s live=%s", id, e.LiveSource)
		}
		if e.Parent != "" {
			t.Fatalf("%s native must have empty parent", id)
		}
		if !IsNativeBinance(id) {
			t.Fatalf("IsNativeBinance(%q)=false", id)
		}
	}
}

func TestMissingNativesAreNative(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"2h", "6h", "8h", "12h"} {
		if !IsNativeBinance(id) {
			t.Fatalf("%s must be native", id)
		}
	}
}

func TestThreeDayIsUnsupported(t *testing.T) {
	t.Parallel()
	if _, ok := TimeframeByName("3d"); ok {
		t.Fatal("3d must not be in the project catalog")
	}
	if IsNativeBinance("3d") || IsLiveChartTF("3d") {
		t.Fatal("3d must not be native or live-chart")
	}
}

func TestTwoMinuteIsDerivedNotNative(t *testing.T) {
	t.Parallel()
	e, ok := TimeframeByName("2m")
	if !ok {
		t.Fatal("2m must exist as derived")
	}
	if e.Class != TFClassDerived {
		t.Fatalf("2m class=%s want derived", e.Class)
	}
	if e.Persist {
		t.Fatal("2m must not persist")
	}
	if e.Parent != "1m" {
		t.Fatalf("2m parent=%q want 1m", e.Parent)
	}
	if IsNativeBinance("2m") {
		t.Fatal("2m must not be Binance-native")
	}
	if e.MenuGroup != "MINUTES" {
		t.Fatalf("2m MenuGroup=%q want MINUTES", e.MenuGroup)
	}
}

func TestInactiveClassesNotActivated(t *testing.T) {
	t.Parallel()
	natives := map[string]struct{}{}
	for _, id := range NativeBinanceIDs() {
		natives[id] = struct{}{}
	}
	for _, e := range timeframeCatalog {
		if e.Class == TFClassNative {
			continue
		}
		if _, live := natives[e.Name]; live {
			t.Fatalf("%s class=%s must not be in NativeBinanceIDs", e.Name, e.Class)
		}
		if e.Persist {
			t.Fatalf("%s persist=true; only native persists", e.Name)
		}
		if e.Class == TFClassDerived && e.MenuGroup != "" {
			continue // TF-B live derived views
		}
		if e.Class == TFClassSeconds && e.MenuGroup != "" && e.Parent == "" {
			continue // MICRO-1 live 1s
		}
		if e.Class == TFClassSeconds && e.MenuGroup != "" && e.Parent == SecondTF {
			continue // SPARSE-SECONDS-1 activated 5s
		}
		if e.MenuGroup != "" {
			t.Fatalf("%s MenuGroup=%q; seconds/ticks stay hidden", e.Name, e.MenuGroup)
		}
		if e.LiveSource == LiveBinanceKlineWS {
			t.Fatalf("%s must not use kline WS until a builder exists", e.Name)
		}
	}
}

func TestMonthIsInCatalogMenuGroup(t *testing.T) {
	t.Parallel()
	e, ok := TimeframeByName("1M")
	if !ok || e.Class != TFClassNative || e.MenuGroup != "DAYS" {
		t.Fatalf("1M catalog=%v ok=%v", e, ok)
	}
}

func TestCombinedKlineStreamsCoverEveryNative(t *testing.T) {
	t.Parallel()
	streams := CombinedKlineStreamNames("BTCUSDT")
	ids := NativeBinanceIDs()
	if len(streams) != len(ids) {
		t.Fatalf("streams=%d ids=%d", len(streams), len(ids))
	}
	joined := strings.Join(streams, " ")
	if strings.Contains(joined, "2m") || strings.Contains(joined, "kline_1s") || strings.Contains(joined, "kline_3d") {
		t.Fatalf("inactive TF leaked into WS: %v", streams)
	}
	for _, id := range ids {
		want := "btcusdt@kline_" + id
		found := false
		for _, s := range streams {
			if s == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing WS stream %s in %v", want, streams)
		}
	}
}

func TestBootAndWSUseNativeCatalog(t *testing.T) {
	t.Parallel()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Dir(filepath.Dir(file))
	mainSrc, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mainSrc), "exchange.NativeBinanceIDs()") {
		t.Fatal("main.go must boot Frames from NativeBinanceIDs")
	}
	if !strings.Contains(string(mainSrc), "HydrateDerivedFrames") {
		t.Fatal("main.go must hydrate derived Frames after native boot")
	}
	if !strings.Contains(string(mainSrc), "ShouldPersist") {
		t.Fatal("main.go must gate persistence on catalog Persist")
	}
	if strings.Contains(string(mainSrc), `"1m", "3m", "5m"`) {
		t.Fatal("main.go still has a duplicated native TF list")
	}
	wsSrc, err := os.ReadFile(filepath.Join(root, "exchange", "ws.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(wsSrc), "CombinedLiveStreamNames(") {
		t.Fatal("ws.go must subscribe via CombinedLiveStreamNames")
	}
	if strings.Contains(string(wsSrc), "@kline_1m") {
		t.Fatal("ws.go still hardcodes kline stream names")
	}
	if !strings.Contains(string(mainSrc), "AttachLiveSecondFrames") {
		t.Fatal("main.go must attach live 1s Frame")
	}
	if !strings.Contains(string(mainSrc), "HydrateSparseSecondFrames") {
		t.Fatal("main.go must hydrate sparse-second children after 1s")
	}
}
