package vector_db

import (
	"context"
	"fmt"

	"github.com/qdrant/go-client/qdrant"
)

// DBClient wraps the Qdrant gRPC client.
type DBClient struct {
	client *qdrant.Client
}

// NewDBClient connects to a Qdrant instance.
func NewDBClient(host string, port int) (*DBClient, error) {
	client, err := qdrant.NewClient(&qdrant.Config{
		Host: host,
		Port: port,
	})
	if err != nil {
		return nil, fmt.Errorf("create qdrant client: %w", err)
	}

	return &DBClient{client: client}, nil
}

// InitCollection ensures a collection exists with the given vector size and cosine distance.
func (c *DBClient) InitCollection(ctx context.Context, name string, vectorSize uint64) error {
	exists, err := c.client.CollectionExists(ctx, name)
	if err != nil {
		return fmt.Errorf("check collection %q: %w", name, err)
	}
	if exists {
		return nil
	}

	if err := c.client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: name,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     vectorSize,
			Distance: qdrant.Distance_Cosine,
		}),
	}); err != nil {
		return fmt.Errorf("create collection %q: %w", name, err)
	}

	return nil
}

// SearchSimilarPatterns finds the closest stored patterns for the given embedding.
func (c *DBClient) SearchSimilarPatterns(ctx context.Context, collectionName string, vector []float32, limit uint64) ([]*qdrant.ScoredPoint, error) {
	points, err := c.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: collectionName,
		Query:          qdrant.NewQuery(vector...),
		Limit:          qdrant.PtrOf(limit),
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil {
		return nil, fmt.Errorf("query collection %q: %w", collectionName, err)
	}

	return points, nil
}

// SavePattern stores a vectorized candle pattern with fractal metadata in Qdrant.
func (c *DBClient) SavePattern(ctx context.Context, collectionName string, vector []float32, pointID uint64, price float64, isUpFractal bool) error {
	wait := true

	_, err := c.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: collectionName,
		Wait:           &wait,
		Points: []*qdrant.PointStruct{
			{
				Id:      qdrant.NewIDNum(pointID),
				Vectors: qdrant.NewVectorsDense(vector),
				Payload: map[string]*qdrant.Value{
					"price":         qdrant.NewValueDouble(price),
					"is_up_fractal": qdrant.NewValueBool(isUpFractal),
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("save pattern %d: %w", pointID, err)
	}

	return nil
}

// SaveTradeOutcome stores a vectorized market state with trade result metadata in Qdrant.
func (c *DBClient) SaveTradeOutcome(ctx context.Context, collectionName string, vector []float32, pointID uint64, action string, pnl float64, isWin bool) error {
	wait := true

	_, err := c.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: collectionName,
		Wait:           &wait,
		Points: []*qdrant.PointStruct{
			{
				Id:      qdrant.NewIDNum(pointID),
				Vectors: qdrant.NewVectorsDense(vector),
				Payload: map[string]*qdrant.Value{
					"action": qdrant.NewValueString(action),
					"pnl":    qdrant.NewValueDouble(pnl),
					"is_win": qdrant.NewValueBool(isWin),
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("save trade outcome %d: %w", pointID, err)
	}

	return nil
}
