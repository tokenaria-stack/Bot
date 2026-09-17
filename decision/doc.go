// Package decision holds DECISION-CONTRACT-1 (ForecastEvidence → DirectionalIntent).
//
// Decision contract ≠ Score engine ≠ Strategy Book ≠ ExecutionPolicy.
// DirectionalIntent is a research opinion, not TradeIntent and not an order.
//
// MUST NOT import package market. Live path is exchange → market → server/web.
// Research opinion path is forecast → decision → decisionresearch.
package decision
