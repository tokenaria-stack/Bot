package starlens

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"trading_bot/market"
)

const (
	OutcomeFieldCount = 11
	Schema3CoordCount = 142
	MatrixCoordCount  = 49
	ContinuousIDCount = OutcomeFieldCount + Schema3CoordCount + MatrixCoordCount
)

var (
	continuousIDs map[string]struct{}
	matrixIDs     map[string]struct{}
	schema3IDs    map[string]struct{}
	outcomeIDs    = []string{
		FieldEntryPrice, FieldMFEPrice, FieldMAEPrice, FieldMFEATR, FieldMAEATR,
		FieldMFEPercent, FieldMAEPercent, FieldStopPrice, FieldStopDistance,
		FieldStopDistanceATR, FieldR,
	}
)

func init() {
	continuousIDs = make(map[string]struct{}, ContinuousIDCount)
	matrixIDs = make(map[string]struct{}, MatrixCoordCount)
	schema3IDs = make(map[string]struct{}, Schema3CoordCount)
	for _, id := range outcomeIDs {
		continuousIDs[id] = struct{}{}
	}
	for _, o := range market.RawObservations(market.StarSnapshotV3{}) {
		schema3IDs[o.Name] = struct{}{}
		continuousIDs[o.Name] = struct{}{}
	}
	for _, id := range market.RelationNames() {
		matrixIDs[id] = struct{}{}
		continuousIDs[id] = struct{}{}
	}
}

func clausesUseCoordinates(clauses []Clause) bool {
	for _, c := range clauses {
		if c.Kind != KindContinuous {
			continue
		}
		if _, ok := schema3IDs[c.Field]; ok {
			return true
		}
		if _, ok := matrixIDs[c.Field]; ok {
			return true
		}
	}
	return false
}

func requireCoordinatesAttached(table []Row, clauses []Clause) error {
	if !clausesUseCoordinates(clauses) {
		return nil
	}
	if len(table) == 0 {
		return fmt.Errorf("starlens: coordinates are not attached")
	}
	for i := range table {
		if table[i].coords == nil {
			return fmt.Errorf("starlens: coordinates are not attached")
		}
	}
	return nil
}

func requireMatrixProvenance(prov Provenance, clauses []Clause) error {
	if !clausesUseMatrix(clauses) {
		return nil
	}
	if prov.MatrixDigest == "" || prov.MatrixCommit == "" {
		return fmt.Errorf("starlens: matrix digest and commit are required")
	}
	return nil
}

func clausesUseMatrix(clauses []Clause) bool {
	for _, c := range clauses {
		if c.Kind == KindContinuous {
			if _, ok := matrixIDs[c.Field]; ok {
				return true
			}
		}
	}
	return false
}

type schema3File struct {
	Digest string                  `json:"digest"`
	Rows   []market.StarSnapshotV3 `json:"rows"`
}

type matrixFile struct {
	Digest string                 `json:"digest"`
	Rows   []market.StarRelations `json:"rows"`
}

// ReadSchema3File reads the frozen Schema 3 artifact. It does not rewrite it.
func ReadSchema3File(path string) (digest string, rows []market.StarSnapshotV3, err error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	var doc schema3File
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", nil, err
	}
	return doc.Digest, doc.Rows, nil
}

// ReadMatrixFile reads the frozen Matrix artifact. It does not rewrite it.
func ReadMatrixFile(path string) (digest string, rows []market.StarRelations, err error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	var doc matrixFile
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", nil, err
	}
	return doc.Digest, doc.Rows, nil
}

// AttachCoordinates projects Schema 3 and Matrix onto rows by Star identity.
// It stores observed numbers, not the Schema 3 structs.
func AttachCoordinates(rows []Row, snaps []market.StarSnapshotV3, rels []market.StarRelations) error {
	if len(snaps) != len(rows) || len(rels) != len(rows) {
		return fmt.Errorf("starlens: coordinate tables %d %d %d", len(rows), len(snaps), len(rels))
	}
	for i := range rows {
		snap := snaps[i]
		rel := rels[i]
		if snap.ConfirmedAt != rows[i].DecisionAt || snap.Side != rows[i].Side {
			return fmt.Errorf("starlens: schema 3 identity %d", i)
		}
		if rel.ConfirmedAt != rows[i].DecisionAt || rel.Side != rows[i].Side {
			return fmt.Errorf("starlens: matrix identity %d", i)
		}
		coords := make(map[string]Observed, Schema3CoordCount+MatrixCoordCount)
		for _, o := range market.RawObservations(snap) {
			coords[o.Name] = Observed{Value: o.Value, OK: o.OK}
		}
		for _, o := range market.RelationObservations(rel) {
			coords[o.Name] = Observed{Value: o.Value, OK: o.OK}
		}
		rows[i].coords = coords
	}
	return nil
}

// AttachFrozenCoordinates loads the certified Schema 3 and Matrix files
// and projects them onto rows. It does not rewrite those files.
func AttachFrozenCoordinates(rows []Row) error {
	s3Path, err := FindSchema3File()
	if err != nil {
		return err
	}
	digest, snaps, err := ReadSchema3File(s3Path)
	if err != nil {
		return err
	}
	if digest != Schema3Digest {
		return fmt.Errorf("starlens: schema 3 digest %s", digest)
	}
	mxPath, err := FindMatrixFile()
	if err != nil {
		return err
	}
	mdigest, rels, err := ReadMatrixFile(mxPath)
	if err != nil {
		return err
	}
	if mdigest != MatrixDigest {
		return fmt.Errorf("starlens: matrix digest %s", mdigest)
	}
	return AttachCoordinates(rows, snaps, rels)
}

func FindSchema3File() (string, error) {
	return findStarstop("star_snapshot_v3.json")
}

func FindMatrixFile() (string, error) {
	return findStarstop("star_relative_v1.json")
}

func findStarstop(name string) (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		p := filepath.Join(dir, "research", "starstop", name)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("starlens: %s not found from %s", name, wd)
		}
		dir = parent
	}
}
