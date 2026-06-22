package indicators

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

const (
	maxDivSnapshots = 5
	maxMicroTicks   = 5

	macroJurikWeight      = 15
	macroOrangeWeight     = 10
	macroBlueVolumeWeight = 25
	macroBurgundyWeight   = 20
	macroAOMACDWeight     = 20
	macroHiddenLongWeight = 30

	saucerRSIThreshold = 30.0
	saucerScore          = 15
	vSpikeScore          = 20

	cascadeDoubleMult = 1.5
	cascadeTripleMult = 2.0
)

// Snapshot captures market state at a confirmed ZigZag swing node.
type Snapshot struct {
	Index      int
	IsHigh     bool
	Price      float64
	Jurik      float64
	OrangeRSI  float64
	RedRSI     float64
	BlueVolume float64
	BurgundyAD float64
	AO         float64
	MACD       float64
	Stoch      float64
}

// DivSignal is the output of macro/micro divergence analysis.
type DivSignal struct {
	Score       int
	Description string
}

// SmartDivergenceEngine tracks ZigZag snapshots and micro tick history.
type SmartDivergenceEngine struct {
	snapshots [maxDivSnapshots]Snapshot
	snapIdx   int
	snapCount int

	orangeTicks [maxMicroTicks]float64
	redTicks    [maxMicroTicks]float64
	tickIdx     int
	tickCount   int
}

// NewSmartDivergenceEngine creates an empty divergence engine.
func NewSmartDivergenceEngine() *SmartDivergenceEngine {
	return &SmartDivergenceEngine{}
}

// UpdateSnapshot adds a ZigZag peak snapshot to the ring buffer.
func (e *SmartDivergenceEngine) UpdateSnapshot(snap Snapshot) {
	e.snapshots[e.snapIdx] = snap
	e.snapIdx = (e.snapIdx + 1) % maxDivSnapshots
	if e.snapCount < maxDivSnapshots {
		e.snapCount++
	}
}

// UpdateMicroTick records the latest Orange/Red RSI values for micro analysis.
func (e *SmartDivergenceEngine) UpdateMicroTick(orangeRSI, redRSI float64) {
	e.orangeTicks[e.tickIdx] = orangeRSI
	e.redTicks[e.tickIdx] = redRSI
	e.tickIdx = (e.tickIdx + 1) % maxMicroTicks
	if e.tickCount < maxMicroTicks {
		e.tickCount++
	}
}

// AnalyzeMacro scans stored snapshots for multi-indicator divergence confluence.
func (e *SmartDivergenceEngine) AnalyzeMacro() DivSignal {
	highs := e.filterSnapshots(true)
	lows := e.filterSnapshots(false)

	score := 0
	var parts []string

	if len(highs) >= 2 {
		s, desc := e.analyzeBearishHighs(highs)
		score += s
		if desc != "" {
			parts = append(parts, desc)
		}
	}

	if len(lows) >= 2 {
		s, desc := e.analyzeBullishLows(lows)
		score += s
		if desc != "" {
			parts = append(parts, desc)
		}
	}

	if len(lows) >= 2 {
		s, desc := e.analyzeHiddenContinuationLows(lows)
		score += s
		if desc != "" {
			parts = append(parts, desc)
		}
	}

	return DivSignal{
		Score:       clampDivScore(score),
		Description: strings.Join(parts, "; "),
	}
}

func (e *SmartDivergenceEngine) analyzeBearishHighs(highs []Snapshot) (int, string) {
	h0, h1 := highs[0], highs[1]
	if h0.Price <= h1.Price {
		return 0, ""
	}

	score := 0
	var found []string

	type macroCheck struct {
		name   string
		weight int
		get    func(Snapshot) float64
	}

	checks := []macroCheck{
		{"Jurik", macroJurikWeight, func(s Snapshot) float64 { return s.Jurik }},
		{"Orange", macroOrangeWeight, func(s Snapshot) float64 { return s.OrangeRSI }},
		{"BlueVol", macroBlueVolumeWeight, func(s Snapshot) float64 { return s.BlueVolume }},
		{"BurgundyAD", macroBurgundyWeight, func(s Snapshot) float64 { return s.BurgundyAD }},
		{"AO", macroAOMACDWeight, func(s Snapshot) float64 { return s.AO }},
		{"MACD", macroAOMACDWeight, func(s Snapshot) float64 { return s.MACD }},
	}

	for _, chk := range checks {
		if h0.Price > h1.Price && chk.get(h0) < chk.get(h1) {
			w := float64(chk.weight) * e.cascadeMultiplier(highs, chk.get)
			score -= int(math.Round(w))
			found = append(found, chk.name)
		}
	}

	if len(found) == 0 {
		return 0, ""
	}
	return score, fmt.Sprintf("Bearish Div [%s]", strings.Join(found, ", "))
}

func (e *SmartDivergenceEngine) analyzeBullishLows(lows []Snapshot) (int, string) {
	l0, l1 := lows[0], lows[1]
	if l0.Price >= l1.Price {
		return 0, ""
	}

	score := 0
	var found []string

	type macroCheck struct {
		name   string
		weight int
		get    func(Snapshot) float64
	}

	checks := []macroCheck{
		{"Jurik", macroJurikWeight, func(s Snapshot) float64 { return s.Jurik }},
		{"Orange", macroOrangeWeight, func(s Snapshot) float64 { return s.OrangeRSI }},
		{"BlueVol", macroBlueVolumeWeight, func(s Snapshot) float64 { return s.BlueVolume }},
		{"BurgundyAD", macroBurgundyWeight, func(s Snapshot) float64 { return s.BurgundyAD }},
		{"AO", macroAOMACDWeight, func(s Snapshot) float64 { return s.AO }},
		{"MACD", macroAOMACDWeight, func(s Snapshot) float64 { return s.MACD }},
	}

	for _, chk := range checks {
		if l0.Price < l1.Price && chk.get(l0) > chk.get(l1) {
			w := float64(chk.weight) * e.cascadeMultiplier(lows, chk.get)
			score += int(math.Round(w))
			found = append(found, chk.name)
		}
	}

	if len(found) == 0 {
		return 0, ""
	}
	return score, fmt.Sprintf("Bullish Div [%s]", strings.Join(found, ", "))
}

func (e *SmartDivergenceEngine) analyzeHiddenContinuationLows(lows []Snapshot) (int, string) {
	l0, l1 := lows[0], lows[1]
	if l0.Price <= l1.Price {
		return 0, ""
	}

	confirmations := 0
	if l0.Jurik < l1.Jurik {
		confirmations++
	}
	if l0.OrangeRSI < l1.OrangeRSI {
		confirmations++
	}
	if l0.RedRSI < l1.RedRSI {
		confirmations++
	}
	if l0.MACD < l1.MACD {
		confirmations++
	}

	if confirmations == 0 {
		return 0, ""
	}
	return macroHiddenLongWeight, "Hidden Bullish Continuation"
}

func (e *SmartDivergenceEngine) cascadeMultiplier(snaps []Snapshot, get func(Snapshot) float64) float64 {
	if len(snaps) < 2 {
		return 1
	}

	s0, s1 := snaps[0], snaps[1]
	if !(s0.Price > s1.Price && get(s0) < get(s1)) && !(s0.Price < s1.Price && get(s0) > get(s1)) {
		return 1
	}

	mult := 1.0
	if len(snaps) >= 3 {
		s2 := snaps[2]
		priceExtends := (s0.Price > s1.Price && s0.Price > s2.Price) || (s0.Price < s1.Price && s0.Price < s2.Price)
		indDiverges := (s0.Price > s1.Price && get(s0) < get(s2)) || (s0.Price < s1.Price && get(s0) > get(s2))
		if priceExtends && indDiverges {
			mult = cascadeDoubleMult
		}
		if len(snaps) >= 3 && s0.Price > s1.Price && s1.Price > s2.Price && get(s0) < get(s1) && get(s1) < get(s2) {
			mult = cascadeTripleMult
		}
		if len(snaps) >= 3 && s0.Price < s1.Price && s1.Price < s2.Price && get(s0) > get(s1) && get(s1) > get(s2) {
			mult = cascadeTripleMult
		}
	}
	return mult
}

// AnalyzeMicro inspects the last three ticks of one oscillator for Saucer / V-Spike patterns.
func (e *SmartDivergenceEngine) AnalyzeMicro(currentTick, prevTick, prevPrevTick float64) int {
	return analyzeMicroSeries(currentTick, prevTick, prevPrevTick)
}

// AnalyzeMicroCombined runs micro analysis on stored Orange and Red RSI ticks.
func (e *SmartDivergenceEngine) AnalyzeMicroCombined() int {
	if e.tickCount < 3 {
		return 0
	}

	score := 0
	orange := e.lastNTicks(e.orangeTicks, 3)
	red := e.lastNTicks(e.redTicks, 3)

	score += analyzeMicroSeries(orange[0], orange[1], orange[2])
	score += analyzeMicroSeries(red[0], red[1], red[2])
	return score
}

func analyzeMicroSeries(current, prev, prevPrev float64) int {
	score := 0
	if detectSaucer(current, prev, prevPrev) {
		score += saucerScore
	}
	if detectVSpike(current, prev, prevPrev) {
		score += vSpikeScore
	}
	return score
}

func detectSaucer(current, prev, prevPrev float64) bool {
	if current >= saucerRSIThreshold || prev >= saucerRSIThreshold || prevPrev >= saucerRSIThreshold {
		return false
	}

	v1 := prev - prevPrev
	v2 := current - prev
	a := v2 - v1

	return v1 < 0 && v2 > v1 && a > 0
}

func detectVSpike(current, prev, prevPrev float64) bool {
	vPrev := prev - prevPrev
	vCurr := current - prev
	a := vCurr - vPrev

	strongNeg := vPrev < -2
	strongPos := vCurr > 2
	extremeAccel := math.Abs(a) > 3

	return strongNeg && strongPos && extremeAccel && vPrev*vCurr < 0
}

func (e *SmartDivergenceEngine) filterSnapshots(high bool) []Snapshot {
	out := make([]Snapshot, 0, e.snapCount)
	for i := 0; i < e.snapCount; i++ {
		s := e.snapshots[i]
		if s.IsHigh == high {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Index > out[j].Index
	})
	return out
}

func (e *SmartDivergenceEngine) lastNTicks(buf [maxMicroTicks]float64, n int) []float64 {
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		pos := (e.tickIdx - 1 - i + maxMicroTicks) % maxMicroTicks
		out[i] = buf[pos]
	}
	return out
}

func clampDivScore(score int) int {
	if score > 100 {
		return 100
	}
	if score < -100 {
		return -100
	}
	return score
}
