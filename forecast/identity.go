package forecast

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// LogicVersion is an explicit semantic version for one family of
// analytical/feature/label/plan logic (e.g. "analysis:v1", "features:v1",
// "labels:v1"). Bump it whenever the NUMERIC MEANING of that family changes,
// even if published parameters stay byte-identical (e.g. a formula or
// initialization/warmup correctness fix). Hashing resolved config alone
// cannot detect that kind of change — the explicit version is the law
// (FORECAST-SPEC-1 §12).
type LogicVersion string

// Digest is a full SHA-256 identity of a resolved configuration payload.
// Identity comparisons MUST use the full digest. Short() exists for
// filenames/UI only and must never be treated as identity (collision risk).
type Digest [sha256.Size]byte

// String returns the full lowercase hex digest.
func (d Digest) String() string { return hex.EncodeToString(d[:]) }

// Short returns a 16-hex-character (64-bit) display abbreviation. Display
// only — never compare identities with Short().
func (d Digest) Short() string { return hex.EncodeToString(d[:8]) }

// IsZero reports whether this is the unset digest.
func (d Digest) IsZero() bool { return d == Digest{} }

// Identity is a published object's machine identity: a human-readable key
// (for logs/filenames only), the full digest of its resolved payload, and
// the explicit logic version(s) it was resolved under. A friendly Name is
// NEVER part of the digest payload — renaming an object must not change its
// identity (FORECAST-SPEC-1 §18/§37).
type Identity struct {
	HumanKey string
	Digest   Digest
	Logic    []LogicVersion
}

// computeDigest hashes the canonical JSON encoding of payload.
//
// payload MUST contain only resolved (post-default, post-validation) fields
// relevant to numeric identity: never a friendly Name, never a raw
// unresolved draft, never a map whose key type is not string-kind (Go's
// encoding/json sorts string-kind map keys deterministically; struct field
// order is fixed by declaration order). Do not hash Go source files — logic
// changes are tracked via LogicVersion, not by hashing implementation code.
func computeDigest(payload any) (Digest, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return Digest{}, fmt.Errorf("forecast: canonical digest encode: %w", err)
	}
	return sha256.Sum256(b), nil
}

// NewIdentity resolves a full Identity from a human-readable key, a resolved
// payload, and the logic version(s) that produced it.
func NewIdentity(humanKey string, payload any, logic ...LogicVersion) (Identity, error) {
	d, err := computeDigest(payload)
	if err != nil {
		return Identity{}, err
	}
	return Identity{
		HumanKey: humanKey,
		Digest:   d,
		Logic:    append([]LogicVersion(nil), logic...),
	}, nil
}
