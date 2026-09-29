package starlens

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// Digests named by POPULATION-FILTER-LENS-CONTRACT-AUDIT-V1.
// The outcome file is not a frozen commit. The digest is its identity.
const (
	Schema3Digest = "934e2e0d584297b50f199cd102511f6ecfb93a034ce94d9b27d6046675d8eee7"
	Schema3Commit = "975da0d"
	MatrixDigest  = "8c08c2a643023ee04985d2e5fed6d1cbbf285a4148e216681088f4ec49c3ec8a"
	MatrixCommit  = "0389023"
	OutcomeDigest = "6a11ed6121ae16bb39de247afabb39c454d6985b751fc995410ba5b7cb0cfe62"
	OutcomeBuffer = 0.15
	UniverseStars = 8783
)

// StarRef is one member. The index is the row in decisionAt order.
// It is meaningful only together with the decision time, the side,
// and the digests on the population.
type StarRef struct {
	Index      int    `json:"index"`
	DecisionAt int64  `json:"decisionAt"`
	Side       string `json:"side"`
}

// Provenance is the files this population was cut from.
// Capture is not part of an outcome-only population.
type Provenance struct {
	Schema3Digest string `json:"schema3Digest"`
	Schema3Commit string `json:"schema3Commit"`
	OutcomeDigest string `json:"outcomeDigest,omitempty"`
	MatrixDigest  string `json:"matrixDigest,omitempty"`
	MatrixCommit  string `json:"matrixCommit,omitempty"`
}

// Population is an immutable set of Star identities.
type Population struct {
	ID         string     `json:"id"`
	ParentID   string     `json:"parentId,omitempty"`
	Operation  string     `json:"operation"`
	SavedAt    string     `json:"savedAt,omitempty"`
	Provenance Provenance `json:"provenance"`
	Clauses    []Clause   `json:"clauses,omitempty"`
	Members    []StarRef  `json:"members"`
}

type idBody struct {
	Parent     string     `json:"parent"`
	Operation  string     `json:"operation"`
	Provenance Provenance `json:"provenance"`
	Clauses    []Clause   `json:"clauses,omitempty"`
}

func populationID(parent, operation string, prov Provenance, clauses []Clause) (string, error) {
	body, err := json.Marshal(idBody{
		Parent: parent, Operation: operation, Provenance: prov, Clauses: clauses,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

// Universe is the whole table, in row order.
func Universe(rows []Row, prov Provenance) (Population, error) {
	if prov.Schema3Digest == "" || prov.Schema3Commit == "" {
		return Population{}, fmt.Errorf("starlens: universe needs the schema 3 digest and commit")
	}
	id, err := populationID("", "universe", prov, nil)
	if err != nil {
		return Population{}, err
	}
	members := make([]StarRef, len(rows))
	for i, row := range rows {
		if row.Index != i {
			return Population{}, fmt.Errorf("starlens: row %d has index %d", i, row.Index)
		}
		members[i] = row.Ref()
	}
	return Population{
		ID: id, Operation: "universe", Provenance: prov, Members: members,
	}, nil
}

// OutcomeUniverse attaches the schema 3 and outcome identities from the audit.
func OutcomeUniverse(rows []Row) (Population, error) {
	return Universe(rows, Provenance{
		Schema3Digest: Schema3Digest,
		Schema3Commit: Schema3Commit,
		OutcomeDigest: OutcomeDigest,
	})
}

func (p Population) cloneMembers() []StarRef {
	out := make([]StarRef, len(p.Members))
	copy(out, p.Members)
	return out
}

func (p Population) sameMembers(other []StarRef) bool {
	if len(p.Members) != len(other) {
		return false
	}
	for i := range p.Members {
		if p.Members[i] != other[i] {
			return false
		}
	}
	return true
}

// SavedAtTime formats the operation time. It is not part of the population id.
func SavedAtTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}
