package strategy

// ActionType defines the final signal direction.
type ActionType string

const (
	BuyAction  ActionType = "BUY"
	SellAction ActionType = "SELL"
	WaitAction ActionType = "WAIT"
)

// ScoreFactor holds an isolated score contribution from one indicator or timeframe.
type ScoreFactor struct {
	Name      string     `json:"name"`
	Direction ActionType `json:"direction"`
	Score     int        `json:"score"`
	Reason    string     `json:"reason,omitempty"`
}

// ScoreDecision is the per-bar verdict from ScoreEngine.
type ScoreDecision struct {
	RawAction   ActionType             `json:"rawAction"`   // indicator math (BUY/SELL/WAIT)
	FinalAction ActionType             `json:"finalAction"` // after analyst/chief vetoes
	IsVetoed    bool                   `json:"isVetoed"`
	VetoReason  string                 `json:"vetoReason,omitempty"`
	LongScore      int                    `json:"longScore"`
	ShortScore     int                    `json:"shortScore"`
	Factors        map[string]ScoreFactor `json:"factors"`
	ActiveFactors  []string               `json:"activeFactors,omitempty"`
	StrategySource string                 `json:"strategySource,omitempty"`
	Reason         string                 `json:"reason"`
	LotMod      float64                `json:"lotMod"`
	StopDist    float64                `json:"stopDist"`
}

// WinningScore returns the higher of LongScore and ShortScore.
func (d ScoreDecision) WinningScore() int {
	if d.ShortScore > d.LongScore {
		return d.ShortScore
	}
	return d.LongScore
}

// HasRawSignal reports whether indicators produced a directional raw signal.
func (d ScoreDecision) HasRawSignal() bool {
	return d.RawAction == BuyAction || d.RawAction == SellAction
}

// HasFinalSignal reports whether execution is allowed after vetoes.
func (d ScoreDecision) HasFinalSignal() bool {
	return d.FinalAction == BuyAction || d.FinalAction == SellAction
}

// ScoreEngine evaluates Marker state against a ScoringMatrix.
type ScoreEngine struct{}

// DefaultScoreEngine is the shared scoring calculator.
var DefaultScoreEngine = &ScoreEngine{}
