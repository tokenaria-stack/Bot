package indicators

import "math"

// ZigZagDirection describes the current confirmed swing direction.
type ZigZagDirection int

const (
	ZigZagNeutral ZigZagDirection = 0
	ZigZagUp      ZigZagDirection = 1
	ZigZagDown    ZigZagDirection = -1
)

// ZigZagNode is a confirmed swing high or low.
type ZigZagNode struct {
	Price     float64
	IsHigh    bool
	Confirmed bool
}

// ZigZagUpdate holds the result of a single ZigZag update.
type ZigZagUpdate struct {
	Direction ZigZagDirection
	Node      ZigZagNode
}

// ZigZag is an adaptive swing detector using Williams fractals and ATR filtering.
type ZigZag struct {
	fractals       fractalDetector
	atr            *ATR
	baseMultiplier float64
	direction      ZigZagDirection
	lastNode       ZigZagNode
	hasLastNode    bool

	snapDirection   ZigZagDirection
	snapLastNode    ZigZagNode
	snapHasLastNode bool
}

type fractalDetector interface {
	UpdateCandle(high, low float64) FractalStatus
	SaveState()
	RestoreState()
}

// NewZigZag creates an adaptive ZigZag with the default Williams 5-bar fractal.
func NewZigZag(atrPeriod int) *ZigZag {
	return &ZigZag{
		fractals: NewWilliamsFractals(),
		atr:      NewATR(atrPeriod),
	}
}

// SetDynamicFractal replaces the fractal detector with a configurable window.
func (z *ZigZag) SetDynamicFractal(leftBars, rightBars int) {
	if z == nil {
		return
	}
	z.fractals = NewDynamicFractal(leftBars, rightBars)
}

// SetSensitivity sets the base ATR multiplier for peak confirmation.
func (z *ZigZag) SetSensitivity(baseMultiplier float64) {
	z.baseMultiplier = baseMultiplier
}

func (z *ZigZag) adaptiveMultiplier(rsiVal float64) float64 {
	mult := z.baseMultiplier
	if rsiVal > 70 || rsiVal < 30 {
		return mult * 0.7
	}
	if rsiVal >= 40 && rsiVal <= 60 {
		return mult
	}
	return mult
}

// UpdateCandle ingests OHLC + RSI and returns direction with the last confirmed node.
func (z *ZigZag) UpdateCandle(high, low, close, rsiVal float64) ZigZagUpdate {
	currentATR := z.atr.UpdateCandle(high, low, close)
	mult := z.adaptiveMultiplier(rsiVal)

	fractal := z.fractals.UpdateCandle(high, low)
	if fractal.UpFractal {
		z.tryConfirmHigh(fractal.CenterHigh, currentATR, mult)
	}
	if fractal.DownFractal {
		z.tryConfirmLow(fractal.CenterLow, currentATR, mult)
	}

	return ZigZagUpdate{
		Direction: z.direction,
		Node:      z.lastNode,
	}
}

// Value returns the price of the last confirmed swing node (0 if none).
func (z *ZigZag) Value() float64 {
	if !z.hasLastNode {
		return 0
	}
	return z.lastNode.Price
}

func (z *ZigZag) tryConfirmHigh(price, atr, mult float64) {
	if z.hasLastNode {
		if !z.passesThreshold(price, atr, mult) {
			return
		}
		if z.lastNode.IsHigh && price <= z.lastNode.Price {
			return
		}
		if !z.lastNode.IsHigh {
			z.direction = ZigZagUp
		}
	}
	z.lastNode = ZigZagNode{Price: price, IsHigh: true, Confirmed: true}
	z.hasLastNode = true
}

func (z *ZigZag) tryConfirmLow(price, atr, mult float64) {
	if z.hasLastNode {
		if !z.passesThreshold(price, atr, mult) {
			return
		}
		if !z.lastNode.IsHigh && price >= z.lastNode.Price {
			return
		}
		if z.lastNode.IsHigh {
			z.direction = ZigZagDown
		}
	}
	z.lastNode = ZigZagNode{Price: price, IsHigh: false, Confirmed: true}
	z.hasLastNode = true
}

func (z *ZigZag) passesThreshold(price, atr, mult float64) bool {
	if !z.hasLastNode {
		return true
	}
	return math.Abs(price-z.lastNode.Price) > atr*mult
}

func (z *ZigZag) SaveState() {
	if z.fractals != nil {
		z.fractals.SaveState()
	}
	if z.atr != nil {
		z.atr.SaveState()
	}
	z.snapDirection = z.direction
	z.snapLastNode = z.lastNode
	z.snapHasLastNode = z.hasLastNode
}

func (z *ZigZag) RestoreState() {
	if z.fractals != nil {
		z.fractals.RestoreState()
	}
	if z.atr != nil {
		z.atr.RestoreState()
	}
	z.direction = z.snapDirection
	z.lastNode = z.snapLastNode
	z.hasLastNode = z.snapHasLastNode
}
