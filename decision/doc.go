// Package decision holds DECISION-CONTRACT-1 (ForecastEvidence → DirectionalIntent)
// and leftover ScoreDecision / ScoreFactor wire DTOs until SCORE-WIRE-1.
//
// Decision contract ≠ Score engine ≠ Strategy Book ≠ ExecutionPolicy.
// DirectionalIntent is a research opinion, not TradeIntent and not an order.
//
// MUST NOT import package market. See docs/ARCHITECTURE.md (Import DAG).
package decision
