package strategy

import "trading_bot/vector_db"

// VectorSnapshot projects Report into a vector_db.ReportSnapshot for Qdrant embeddings.
func (r Report) VectorSnapshot() vector_db.ReportSnapshot {
	fibActive := false
	for _, zone := range r.FibZones {
		if zone.IsActive {
			fibActive = true
			break
		}
	}

	return vector_db.ReportSnapshot{
		JurikValue:      r.JurikValue,
		DivergenceScore: r.Divergence.Score,
		Regime:          string(r.Volatility.Regime),
		FalconRedLine:   r.Falcon.RedLine,
		FalconBlueLine:  r.Falcon.BlueLine,
		FibActive:       fibActive,
	}
}
