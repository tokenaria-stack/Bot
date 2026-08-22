package exchange

import (
	"fmt"
	"sort"

	"trading_bot/data"
)

// RequiredParentCount is childDuration / parentDuration (2m/1m=2, 45m/15m=3, …).
func RequiredParentCount(childTF string) (int, error) {
	e, ok := TimeframeByName(childTF)
	if !ok || e.Class != TFClassDerived || e.Parent == "" {
		return 0, fmt.Errorf("RequiredParentCount: %q is not derived", childTF)
	}
	childMs, err := data.IntervalDurationMs(childTF)
	if err != nil {
		return 0, err
	}
	parentMs, err := data.IntervalDurationMs(e.Parent)
	if err != nil {
		return 0, err
	}
	if parentMs <= 0 || childMs%parentMs != 0 {
		return 0, fmt.Errorf("RequiredParentCount: %s/%s not an integer ratio", childTF, e.Parent)
	}
	return int(childMs / parentMs), nil
}

// ParentBarsNeeded is how many parent bars to fetch for childBars complete children
// plus one child-bucket alignment (partial left edge).
func ParentBarsNeeded(childBars int, childTF string) (int, error) {
	ratio, err := RequiredParentCount(childTF)
	if err != nil {
		return 0, err
	}
	if childBars < 1 {
		childBars = 1
	}
	return childBars*ratio + ratio, nil
}

type parentSlot struct {
	k      Kline
	closed bool
}

// AssembleChildOHLCV is the shared candle law (one or more parents, time-ordered).
func AssembleChildOHLCV(childOpen int64, childTF string, parents []Kline) (Kline, error) {
	if childOpen < 0 || len(parents) == 0 {
		return Kline{}, fmt.Errorf("AssembleChildOHLCV: empty")
	}
	ct, err := data.BarCloseTimeMs(childOpen, childTF)
	if err != nil {
		return Kline{}, err
	}
	out := Kline{
		OpenTime:  childOpen,
		CloseTime: ct,
		Open:      parents[0].Open,
		High:      parents[0].High,
		Low:       parents[0].Low,
		Close:     parents[len(parents)-1].Close,
		Volume:    0,
	}
	for _, p := range parents {
		if p.High > out.High {
			out.High = p.High
		}
		if p.Low < out.Low {
			out.Low = p.Low
		}
		out.Volume += p.Volume
	}
	return out, nil
}

func requiredParentOpens(childOpen int64, childTF string, ratio int) ([]int64, error) {
	e, _ := TimeframeByName(childTF)
	opens := make([]int64, 0, ratio)
	cur := childOpen
	for i := 0; i < ratio; i++ {
		opens = append(opens, cur)
		next, err := data.NextBarOpen(cur, e.Parent)
		if err != nil {
			return nil, err
		}
		cur = next
	}
	return opens, nil
}

func parentsForRequiredOpens(byOpen map[int64]Kline, required []int64) ([]Kline, bool) {
	out := make([]Kline, 0, len(required))
	for _, ot := range required {
		p, ok := byOpen[ot]
		if !ok {
			return nil, false
		}
		out = append(out, p)
	}
	return out, true
}

// FoldClosedChildren emits only complete child buckets. Partial left edge and
// holes are dropped. lastParentForming: the last parent kline is live forming
// (excluded from closed fold; used only if the caller wants a forming child).
func FoldClosedChildren(parents []Kline, childTF string, lastParentForming bool) (closed []Kline, forming *Kline, err error) {
	ratio, err := RequiredParentCount(childTF)
	if err != nil {
		return nil, nil, err
	}
	if len(parents) == 0 {
		return nil, nil, nil
	}
	work := parents
	var formingParent *Kline
	if lastParentForming {
		last := parents[len(parents)-1]
		formingParent = &last
		work = parents[:len(parents)-1]
	}
	byOpen := make(map[int64]Kline, len(work))
	childSet := map[int64]struct{}{}
	var childOpens []int64
	for _, p := range work {
		if p.OpenTime < 0 {
			continue
		}
		byOpen[p.OpenTime] = p
		co, err := data.CurrentBarOpen(p.OpenTime, childTF)
		if err != nil {
			return nil, nil, err
		}
		if _, seen := childSet[co]; !seen {
			childSet[co] = struct{}{}
			childOpens = append(childOpens, co)
		}
	}
	sort.Slice(childOpens, func(i, j int) bool { return childOpens[i] < childOpens[j] })

	for _, co := range childOpens {
		req, err := requiredParentOpens(co, childTF, ratio)
		if err != nil {
			return nil, nil, err
		}
		plist, ok := parentsForRequiredOpens(byOpen, req)
		if !ok {
			continue
		}
		bar, err := AssembleChildOHLCV(co, childTF, plist)
		if err != nil {
			return nil, nil, err
		}
		closed = append(closed, bar)
	}

	if formingParent != nil {
		co, err := data.CurrentBarOpen(formingParent.OpenTime, childTF)
		if err != nil {
			return closed, nil, err
		}
		req, err := requiredParentOpens(co, childTF, ratio)
		if err != nil {
			return closed, nil, err
		}
		slots := make(map[int64]Kline, len(req))
		for _, ot := range req {
			if p, ok := byOpen[ot]; ok {
				slots[ot] = p
			}
		}
		slots[formingParent.OpenTime] = *formingParent
		ordered := make([]Kline, 0, len(req))
		for _, ot := range req {
			if p, ok := slots[ot]; ok {
				ordered = append(ordered, p)
			}
		}
		if len(ordered) > 0 {
			bar, err := AssembleChildOHLCV(co, childTF, ordered)
			if err != nil {
				return closed, nil, err
			}
			forming = &bar
		}
	}
	return closed, forming, nil
}

// DerivedAccumulator is the live consumer of the same OHLCV / completeness law.
type DerivedAccumulator struct {
	childTF  string
	parentTF string
	ratio    int
	bucket   int64
	slots    map[int64]parentSlot
}

func NewDerivedAccumulator(childTF string) (*DerivedAccumulator, error) {
	e, ok := TimeframeByName(childTF)
	if !ok || e.Class != TFClassDerived {
		return nil, fmt.Errorf("NewDerivedAccumulator: %q", childTF)
	}
	ratio, err := RequiredParentCount(childTF)
	if err != nil {
		return nil, err
	}
	return &DerivedAccumulator{
		childTF:  childTF,
		parentTF: e.Parent,
		ratio:    ratio,
		slots:    make(map[int64]parentSlot),
	}, nil
}

// ClosedParentCount is distinct closed parent buckets in the current child bucket.
func (a *DerivedAccumulator) ClosedParentCount() int {
	if a == nil {
		return 0
	}
	n := 0
	for _, s := range a.slots {
		if s.closed {
			n++
		}
	}
	return n
}

// OnParent updates/replaces the contribution for parent.OpenTime.
func (a *DerivedAccumulator) OnParent(parent Kline, isClosed bool) (child Kline, childClosed bool, ok bool) {
	if a == nil || parent.OpenTime < 0 {
		return Kline{}, false, false
	}
	co, err := data.CurrentBarOpen(parent.OpenTime, a.childTF)
	if err != nil {
		return Kline{}, false, false
	}
	if a.bucket != 0 && co != a.bucket {
		a.slots = make(map[int64]parentSlot)
	}
	a.bucket = co
	a.slots[parent.OpenTime] = parentSlot{k: parent, closed: isClosed}

	req, err := requiredParentOpens(co, a.childTF, a.ratio)
	if err != nil {
		return Kline{}, false, false
	}
	ordered := make([]Kline, 0, len(a.slots))
	closedN := 0
	complete := true
	for _, ot := range req {
		s, have := a.slots[ot]
		if !have {
			complete = false
			continue
		}
		ordered = append(ordered, s.k)
		if s.closed {
			closedN++
		} else {
			complete = false
		}
	}
	if len(ordered) == 0 {
		return Kline{}, false, false
	}
	bar, err := AssembleChildOHLCV(co, a.childTF, ordered)
	if err != nil {
		return Kline{}, false, false
	}
	childClosed = complete && closedN == a.ratio
	return bar, childClosed, true
}

// ResetFromParents replays parent klines into the accumulator (boot / rebuild).
func (a *DerivedAccumulator) ResetFromParents(parents []Kline, lastIsForming bool) {
	if a == nil {
		return
	}
	a.bucket = 0
	a.slots = make(map[int64]parentSlot)
	for i, p := range parents {
		closed := true
		if lastIsForming && i == len(parents)-1 {
			closed = false
		}
		a.OnParent(p, closed)
	}
}
