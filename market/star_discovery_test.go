package market

import (
	"os"
	"strings"
	"testing"
)

func TestDiscoveryLaws(t *testing.T) {
	body, err := os.ReadFile("star_discovery.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)
	for _, forbidden := range []string{
		"ReplayClosedBars", "replayStarSeries", "FeatureRuntime2",
		"NewWozduhNode", "NewRSXNode", "kmeans", "RandomForest",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("discovery contains %s", forbidden)
		}
	}

	class, survivalOK, survived, excursion := ClassifyPath("SURVIVED_WINDOW", true, true, 1, 2)
	if class != "survived" || !survivalOK || !survived || !excursion {
		t.Fatalf("survived %+v %v %v %v", class, survivalOK, survived, excursion)
	}
	class, survivalOK, survived, excursion = ClassifyPath("STOP_FIRST", true, true, 1, 2)
	if class != "stop_first" || !survivalOK || survived {
		t.Fatalf("stop %+v %v %v", class, survivalOK, survived)
	}
	class, survivalOK, survived, _ = ClassifyPath("UNORDERED_BAR", true, true, 1, 2)
	if class != "unordered" || !survivalOK || survived {
		t.Fatalf("unordered %+v", class)
	}
	class, survivalOK, _, excursion = ClassifyPath("INCOMPLETE_WINDOW", true, true, 1, 2)
	if class != "absent" || survivalOK || excursion {
		t.Fatalf("incomplete %+v %v %v", class, survivalOK, excursion)
	}
	_, _, _, excursion = ClassifyPath("SURVIVED_WINDOW", true, false, 0, 0)
	if excursion {
		t.Fatal("missing atr excursion became a reading")
	}

	edges, ok := SliceEdges([]float64{1, 2, 3, 4, 5})
	if !ok {
		t.Fatal("edges")
	}
	if idx, in := SliceIndex(edges, 0, true); !in || idx != 0 {
		t.Fatalf("low %d %v", idx, in)
	}
	if _, in := SliceIndex(edges, 0, false); in {
		t.Fatal("missing entered a slice")
	}
	if idx, in := SliceIndex(edges, 0, true); !in || idx != 0 {
		t.Fatalf("real zero %d", idx)
	}
	if _, ok := SliceEdges([]float64{1, 2, 3, 4}); ok {
		t.Fatal("short sample invented slices")
	}

	runs := DescribeEpisodes([]int64{0, 10 * DiscoveryBarMs, 200 * DiscoveryBarMs})
	if runs.Runs != 2 || runs.StarsAfterPrev != 1 || runs.MaxRun != 2 {
		t.Fatalf("episodes %+v", runs)
	}

	var row StarSnapshotV3
	raw := RawObservations(row)
	if len(raw) != 142 {
		t.Fatalf("raw %d", len(raw))
	}
	if len(RelationObservations(StarRelations{})) != 49 {
		t.Fatal("relations")
	}
	row.M15.Present = true
	row.M15.Vwema = 0
	row.M15.ValuesOK = false
	raw = RawObservations(row)
	if !raw[0].OK || raw[0].Value != 0 || raw[0].Name != "M15.Vwema" {
		t.Fatalf("zero vwema %+v", raw[0])
	}
}
