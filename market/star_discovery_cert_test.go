package market

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"trading_bot/forecast"
)

const (
	discoveryOutcomeDigest = "6a11ed6121ae16bb39de247afabb39c454d6985b751fc995410ba5b7cb0cfe62"
	discoveryMatrixDigest  = "8c08c2a643023ee04985d2e5fed6d1cbbf285a4148e216681088f4ec49c3ec8a"
	discoveryMatrixSHA256  = "514d513dd173bba6d62b05aa0d1b3c15bf1a4ca3216aaf1d8db4ec8550c109c6"
)

func TestStarDiscoveryV1Certification(t *testing.T) {
	if os.Getenv("STAR_DISCOVERY_V1_CERT") != "1" {
		t.Skip("set STAR_DISCOVERY_V1_CERT=1 to read the frozen artifacts")
	}
	if forecast.StarStopK != 2 || forecast.StarStopBufferATR != 0.15 || forecast.StarOutcomeWindow != DiscoveryWindow {
		t.Fatalf("measurement constants k=%d buffer=%v window=%d", forecast.StarStopK, forecast.StarStopBufferATR, forecast.StarOutcomeWindow)
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	v3Path := filepath.Join(root, "research", "starstop", "star_snapshot_v3.json")
	matrixPath := filepath.Join(root, "research", "starstop", "star_relative_v1.json")
	outcomePath := filepath.Join(root, "research", "starstop", "star_sl_outcome.json")
	v3Before, err := fileSHA256(v3Path)
	if err != nil {
		t.Fatal(err)
	}
	matrixBefore, err := fileSHA256(matrixPath)
	if err != nil {
		t.Fatal(err)
	}
	outcomeBefore, err := fileSHA256(outcomePath)
	if err != nil {
		t.Fatal(err)
	}
	if v3Before != starRelativeSchema3SHA256 || matrixBefore != discoveryMatrixSHA256 {
		t.Fatalf("frozen inputs changed")
	}

	var snaps starSnapshotV3File
	if err := readJSON(v3Path, &snaps); err != nil {
		t.Fatal(err)
	}
	var matrix starRelativeFile
	if err := readJSON(matrixPath, &matrix); err != nil {
		t.Fatal(err)
	}
	var outcome outcomeFile
	if err := readJSON(outcomePath, &outcome); err != nil {
		t.Fatal(err)
	}
	if snaps.Digest != starRelativeSchema3Digest || snaps.Count != starStopCount || len(snaps.Rows) != starStopCount {
		t.Fatalf("schema3 %s %d", snaps.Digest, snaps.Count)
	}
	if matrix.Digest != discoveryMatrixDigest || matrix.Schema3Digest != starRelativeSchema3Digest || len(matrix.Rows) != starStopCount {
		t.Fatalf("matrix %s", matrix.Digest)
	}
	if outcome.Digest != discoveryOutcomeDigest || outcome.Count != starStopCount || outcome.Window != DiscoveryWindow || outcome.BufferATR != 0.15 || len(outcome.Rows) != starStopCount {
		t.Fatalf("outcome %s window %d buffer %v", outcome.Digest, outcome.Window, outcome.BufferATR)
	}

	facts := make([]PathFact, starStopCount)
	discovery := make([]bool, starStopCount)
	times := make([]int64, starStopCount)
	var holdout, up, down int
	for i := range snaps.Rows {
		snap := snaps.Rows[i]
		rel := matrix.Rows[i]
		got := outcome.Rows[i]
		if snap.ConfirmedAt != got.DecisionAt || snap.Side != got.Side || snap.AnchorAt != snap.ConfirmedAt {
			t.Fatalf("schema3 identity %d", i)
		}
		if rel.ConfirmedAt != snap.ConfirmedAt || rel.Side != snap.Side || rel.AnchorAt != snap.AnchorAt {
			t.Fatalf("matrix identity %d", i)
		}
		if i > 0 && snap.ConfirmedAt <= snaps.Rows[i-1].ConfirmedAt {
			t.Fatalf("order %d", i)
		}
		class, survivalOK, survived, excursion := ClassifyPath(got.Status, got.ExcursionOK, got.ATRExcursion, got.MFEATR, got.MAEATR)
		fact := PathFact{
			DecisionAt: snap.ConfirmedAt, Side: snap.Side, StopClass: class,
			SurvivalOK: survivalOK, Survived: survived,
			FavorableOK: excursion, AdverseOK: excursion,
		}
		if excursion {
			fact.Favorable = got.MFEATR
			fact.Adverse = got.MAEATR
		}
		facts[i] = fact
		times[i] = snap.ConfirmedAt
		if snap.ConfirmedAt >= DiscoveryHoldoutAt {
			holdout++
		} else {
			discovery[i] = true
		}
		switch snap.Side {
		case "up":
			up++
		case "down":
			down++
		default:
			t.Fatalf("side %q", snap.Side)
		}
	}
	if up != 4392 || down != 4391 {
		t.Fatalf("up %d down %d", up, down)
	}

	var discoveryFacts, upFacts, downFacts []PathFact
	for i, fact := range facts {
		if !discovery[i] {
			continue
		}
		discoveryFacts = append(discoveryFacts, fact)
		if fact.Side == "up" {
			upFacts = append(upFacts, fact)
		} else {
			downFacts = append(downFacts, fact)
		}
	}

	rawN := len(RawObservations(snaps.Rows[0]))
	if rawN != 142 {
		t.Fatalf("raw %d", rawN)
	}
	relN := len(RelationObservations(matrix.Rows[0]))
	if relN != 49 {
		t.Fatalf("relations %d", relN)
	}
	rawValue := make([][]float64, rawN)
	rawOK := make([][]bool, rawN)
	rawName := make([]string, rawN)
	for c := range rawValue {
		rawValue[c] = make([]float64, starStopCount)
		rawOK[c] = make([]bool, starStopCount)
	}
	relValue := make([][]float64, relN)
	relOK := make([][]bool, relN)
	relName := make([]string, relN)
	for c := range relValue {
		relValue[c] = make([]float64, starStopCount)
		relOK[c] = make([]bool, starStopCount)
	}
	for i := range snaps.Rows {
		raw := RawObservations(snaps.Rows[i])
		if len(raw) != rawN {
			t.Fatalf("raw width %d", i)
		}
		for c, obs := range raw {
			if i == 0 {
				rawName[c] = obs.Name
			} else if obs.Name != rawName[c] {
				t.Fatalf("raw name %s", obs.Name)
			}
			rawValue[c][i] = obs.Value
			rawOK[c][i] = obs.OK
		}
		rels := RelationObservations(matrix.Rows[i])
		if len(rels) != relN {
			t.Fatalf("relation width %d", i)
		}
		for c, obs := range rels {
			if i == 0 {
				relName[c] = obs.Name
			}
			relValue[c][i] = obs.Value
			relOK[c][i] = obs.OK
		}
	}
	coordinates := make([]CoordPicture, rawN)
	for c := range rawName {
		coordinates[c] = DescribeCoordinate(rawName[c], facts, rawValue[c], rawOK[c], discovery)
	}
	relations := make([]CoordPicture, relN)
	for c := range relName {
		relations[c] = DescribeCoordinate(relName[c], facts, relValue[c], relOK[c], discovery)
	}

	doc := discoveryFile{
		Schema:         1,
		Schema3Digest:  starRelativeSchema3Digest,
		Schema3Commit:  "975da0d",
		MatrixDigest:   discoveryMatrixDigest,
		MatrixCommit:   "0389023",
		OutcomeDigest:  discoveryOutcomeDigest,
		Measurement:    "stop survival, full-window favorable ATR, full-window adverse ATR; 0.15 ATR; Williams k=2; 96 bars",
		SliceLaw:       "five discovery-period quintiles; OK false is not a slice; the 2026 wall is not used to place edges or to name a region",
		NamedRegions:   []string{},
		Count:          starStopCount,
		Up:             up,
		Down:           down,
		Holdout:        holdout,
		DiscoveryCount: len(discoveryFacts),
		All:            SummarizePath(facts),
		Discovery:      SummarizePath(discoveryFacts),
		UpPath:         SummarizePath(upFacts),
		DownPath:       SummarizePath(downFacts),
		Episodes:       DescribeEpisodes(times),
		Coordinates:    coordinates,
		Relations:      relations,
		Rows:           facts,
	}
	doc.Digest = discoveryDigest(doc)
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(root, "research", "starstop", "star_discovery_v1.json")
	if err := os.WriteFile(outPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if sum, err := fileSHA256(v3Path); err != nil || sum != v3Before {
		t.Fatal("schema3 changed")
	}
	if sum, err := fileSHA256(matrixPath); err != nil || sum != matrixBefore {
		t.Fatal("matrix changed")
	}
	if sum, err := fileSHA256(outcomePath); err != nil || sum != outcomeBefore {
		t.Fatal("outcome changed")
	}
	t.Logf("digest=%s stars=%d discovery=%d holdout=%d up=%d down=%d episodes=%+v all=%+v discoveryPath=%+v upPath=%+v downPath=%+v",
		doc.Digest, doc.Count, doc.DiscoveryCount, doc.Holdout, up, down, doc.Episodes, doc.All, doc.Discovery, doc.UpPath, doc.DownPath)
}

func readJSON(path string, dest any) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, dest)
}

type discoveryFile struct {
	Schema         int            `json:"schema"`
	Digest         string         `json:"digest"`
	Schema3Digest  string         `json:"schema3Digest"`
	Schema3Commit  string         `json:"schema3Commit"`
	MatrixDigest   string         `json:"matrixDigest"`
	MatrixCommit   string         `json:"matrixCommit"`
	OutcomeDigest  string         `json:"outcomeDigest"`
	Measurement    string         `json:"measurement"`
	SliceLaw       string         `json:"sliceLaw"`
	NamedRegions   []string       `json:"namedRegions"`
	Count          int            `json:"count"`
	Up             int            `json:"up"`
	Down           int            `json:"down"`
	Holdout        int            `json:"holdout"`
	DiscoveryCount int            `json:"discoveryCount"`
	All            PathSummary    `json:"all"`
	Discovery      PathSummary    `json:"discovery"`
	UpPath         PathSummary    `json:"upPath"`
	DownPath       PathSummary    `json:"downPath"`
	Episodes       EpisodeRuns    `json:"episodes"`
	Coordinates    []CoordPicture `json:"coordinates"`
	Relations      []CoordPicture `json:"relations"`
	Rows           []PathFact     `json:"rows"`
}

func discoveryDigest(doc discoveryFile) string {
	h := sha256.New()
	var buf [8]byte
	putI := func(v int64) {
		binary.LittleEndian.PutUint64(buf[:], uint64(v))
		_, _ = h.Write(buf[:])
	}
	putF := func(v float64) {
		binary.LittleEndian.PutUint64(buf[:], math.Float64bits(v))
		_, _ = h.Write(buf[:])
	}
	_, _ = h.Write([]byte(doc.Schema3Digest))
	_, _ = h.Write([]byte(doc.MatrixDigest))
	_, _ = h.Write([]byte(doc.OutcomeDigest))
	putI(int64(len(doc.Rows)))
	for _, row := range doc.Rows {
		_, _ = h.Write([]byte(row.Side))
		_, _ = h.Write([]byte(row.StopClass))
		putI(row.DecisionAt)
		if row.SurvivalOK {
			_, _ = h.Write([]byte{1})
		} else {
			_, _ = h.Write([]byte{0})
		}
		if row.Survived {
			_, _ = h.Write([]byte{1})
		} else {
			_, _ = h.Write([]byte{0})
		}
		if row.FavorableOK {
			_, _ = h.Write([]byte{1})
		} else {
			_, _ = h.Write([]byte{0})
		}
		putF(row.Favorable)
		putF(row.Adverse)
	}
	return hex.EncodeToString(h.Sum(nil))
}
