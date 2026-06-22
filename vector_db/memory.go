package vector_db

import (
	"context"
	"fmt"

	"github.com/qdrant/go-client/qdrant"
)

// MemoryStore wraps DBClient for AI-assisted scalp memory lookups.
type MemoryStore struct {
	db             *DBClient
	collectionName string
}

// NewMemoryStore creates a MemoryStore bound to a Qdrant collection.
func NewMemoryStore(db *DBClient, collectionName string) *MemoryStore {
	return &MemoryStore{
		db:             db,
		collectionName: collectionName,
	}
}

// PredictWinRate searches similar historical trade states and returns win rate among neighbors.
func (m *MemoryStore) PredictWinRate(ctx context.Context, snapshot ReportSnapshot, k uint64) (float64, int, error) {
	if m == nil || m.db == nil {
		return 0, 0, fmt.Errorf("memory store is not configured")
	}

	vector := VectorizeReport(snapshot)
	results, err := m.db.SearchSimilarPatterns(ctx, m.collectionName, vector, k)
	if err != nil {
		return 0, 0, err
	}

	count := len(results)
	if count == 0 {
		return 0, 0, nil
	}

	wins := 0
	for _, point := range results {
		if payloadBool(point.GetPayload(), "is_win") {
			wins++
		}
	}

	return float64(wins) / float64(count), count, nil
}

func payloadBool(payload map[string]*qdrant.Value, key string) bool {
	if payload == nil {
		return false
	}

	value, ok := payload[key]
	if !ok || value == nil {
		return false
	}

	return value.GetBoolValue()
}
