package server

import (
	"math"
	"testing"

	"trading_bot/market"
)

func TestInspectionColorLaws(t *testing.T) {
	sample := []inspectionSample{
		{Value: -10, OK: true, Discovery: true},
		{Value: -1, OK: true, Discovery: true},
		{Value: 0, OK: true, Discovery: true},
		{Value: 1, OK: true, Discovery: true},
		{Value: 10, OK: true, Discovery: true},
		{Value: 1000, OK: true, Discovery: false},
		{Value: 0, OK: false, Discovery: true},
	}
	sat := signedSaturation(sample)
	if sat != 10 {
		t.Fatalf("saturation %v", sat)
	}
	if signedPosition(10, sat) != 1 || signedPosition(-10, sat) != 0 || signedPosition(0, sat) != 0.5 {
		t.Fatal("signed ends")
	}
	if signedPosition(100, sat) != 1 || signedPosition(-100, sat) != 0 {
		t.Fatal("saturation clamp")
	}
	again := signedSaturation(append(sample, inspectionSample{Value: -5000, OK: true, Discovery: false}))
	if again != sat {
		t.Fatal("holdout moved the range")
	}
	levels := []inspectionSample{
		{Value: 10, OK: true, Discovery: true},
		{Value: 50, OK: true, Discovery: true},
		{Value: 90, OK: true, Discovery: true},
		{Value: 0, OK: true, Discovery: false},
	}
	rank := unsignedRank(levels)
	low := unsignedPosition(10, rank)
	mid := unsignedPosition(50, rank)
	high := unsignedPosition(90, rank)
	if !(low < mid && mid < high) {
		t.Fatalf("level order %v %v %v", low, mid, high)
	}
	if math.Abs(low-1.0/6) > 1e-9 || math.Abs(high-5.0/6) > 1e-9 {
		t.Fatalf("level ends %v %v", low, high)
	}
	held := unsignedPosition(0, rank)
	if held != 0 {
		t.Fatalf("holdout value uses the discovery rank, got %v", held)
	}
	rank2 := unsignedRank(levels)
	if unsignedPosition(50, rank) != unsignedPosition(50, rank2) {
		t.Fatal("same value changed color position")
	}
}

func TestInspectionRelationIsStored(t *testing.T) {
	snap := market.StarSnapshotV3{}
	snap.Side = "up"
	snap.ConfirmedAt = 10
	snap.M15.Present = true
	snap.M15.CloseTime = 20
	snap.M15.Vwema = 0
	rel := market.StarRelations{Side: "up", ConfirmedAt: 10}
	rel.M15H1Vwema = market.Rel{Value: 3.41, OK: true}
	fact := market.PathFact{DecisionAt: 10, Side: "up", StopClass: "unordered", FavorableOK: true, Favorable: 4, AdverseOK: false, Adverse: 99}
	rows, _, err := collectInspectionStars([]market.StarSnapshotV3{snap}, []market.StarRelations{rel}, []market.PathFact{fact})
	if err != nil {
		t.Fatal(err)
	}
	var found inspectionReading
	for _, reading := range rows[0].Readings {
		if reading.ID == "rel.m15h1.vwema" {
			found = reading
		}
		if reading.ID == "m15.vwema" && !reading.OK {
			t.Fatal("real zero VWEMA was treated as missing")
		}
	}
	if !found.OK || found.Value != 3.41 {
		t.Fatalf("relation %+v", found)
	}
	if rows[0].PathLabel != "Stop order unresolved" || rows[0].Adverse != nil || rows[0].Favorable == nil {
		t.Fatalf("path %+v", rows[0])
	}
	snap.D1.Present = false
	snap.D1.Vwema = 0
	rows, _, err = collectInspectionStars([]market.StarSnapshotV3{snap}, []market.StarRelations{rel}, []market.PathFact{fact})
	if err != nil {
		t.Fatal(err)
	}
	for _, reading := range rows[0].Readings {
		if reading.ID == "d1.vwema" && reading.OK {
			t.Fatal("absent daily VWEMA painted as a reading")
		}
	}
}

func TestInspectionArchive(t *testing.T) {
	doc, err := readInspection()
	if err != nil {
		t.Skip(err)
	}
	if doc.Count != 8783 || doc.Digest != "934e2e0d584297b50f199cd102511f6ecfb93a034ce94d9b27d6046675d8eee7" {
		t.Fatalf("count %d digest %s", doc.Count, doc.Digest)
	}
	var daily inspectionReading
	var rels int
	for _, reading := range doc.Rows[0].Readings {
		if reading.ID == "d1.vwema" {
			daily = reading
		}
		if len(reading.ID) > 4 && reading.ID[:4] == "rel." {
			rels++
		}
	}
	if daily.OK || daily.Position != nil {
		t.Fatalf("daily missing star painted %+v", daily)
	}
	if rels != 49 {
		t.Fatalf("relations %d", rels)
	}
	second, err := readInspection()
	if err != nil {
		t.Fatal(err)
	}
	if second.Rows[100].Readings[0].Position == nil || doc.Rows[100].Readings[0].Position == nil {
		t.Fatal("missing position")
	}
	if *second.Rows[100].Readings[0].Position != *doc.Rows[100].Readings[0].Position {
		t.Fatal("position changed across reads")
	}
}

func TestInspectionJoinRejectsDrift(t *testing.T) {
	snap := market.StarSnapshotV3{}
	snap.ConfirmedAt = 1
	snap.Side = "up"
	rel := market.StarRelations{ConfirmedAt: 2, Side: "up"}
	fact := market.PathFact{DecisionAt: 1, Side: "up"}
	if _, _, err := collectInspectionStars([]market.StarSnapshotV3{snap}, []market.StarRelations{rel}, []market.PathFact{fact}); err == nil {
		t.Fatal("accepted a drifted join")
	}
}
