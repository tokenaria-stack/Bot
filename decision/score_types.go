package decision

// ActionType defines the final signal direction.
type ActionType string

const (
	BuyAction  ActionType = "BUY"
	SellAction ActionType = "SELL"
	WaitAction ActionType = "WAIT"
)

// ScoreFactor holds an isolated score contribution from one indicator or timeframe.
// Wire fossil until SCORE-WIRE-1 (dashboard JSON zeros these maps).
type ScoreFactor struct {
	Name      string     `json:"name"`
	Direction ActionType `json:"direction"`
	Score     int        `json:"score"`
	Reason    string     `json:"reason,omitempty"`
}

// ScoreDecision is a Phase F dashboard/telemetry DTO.
// It is not produced by a ScoreEngine (removed SOCKET-CLEAN-1).
// It is not DECISION-CONTRACT-1 (see ApplyDecision).
// It is not TradeIntent. Kept until SCORE-WIRE-1 because server JSON still embeds it.
type ScoreDecision struct {
	RawAction      ActionType             `json:"rawAction"`   // indicator math (BUY/SELL/WAIT)
	FinalAction    ActionType             `json:"finalAction"` // after decision-layer vetoes
	IsVetoed       bool                   `json:"isVetoed"`
	VetoReason     string                 `json:"vetoReason,omitempty"`
	LongScore      int                    `json:"longScore"`
	ShortScore     int                    `json:"shortScore"`
	Factors        map[string]ScoreFactor `json:"factors"`
	ActiveFactors  []string               `json:"activeFactors,omitempty"`
	StrategySource string                 `json:"strategySource,omitempty"`
	Reason         string                 `json:"reason"`
	LotMod         float64                `json:"lotMod"`
	StopDist       float64                `json:"stopDist"`
}
