package exchange

import (
	"fmt"
	"sort"

	"trading_bot/data"
)

// SparseSecondFoldResult is event-confirmed child state. Forming is never
// inferred from wall clock.
type SparseSecondFoldResult struct {
	Closed  []Kline
	Forming *Kline
}

// SparseSecondReducer folds existing 1s parents into a sparse child bucket.
// Same OpenTime replaces. A later child-bucket parent closes the previous child once.
type SparseSecondReducer struct {
	childTF string
	bucket  int64
	slots   map[int64]Kline
}

// NewSparseSecondReducer accepts any catalog second child of 1s (including inactive 10s–45s).
func NewSparseSecondReducer(childTF string) (*SparseSecondReducer, error) {
	e, ok := TimeframeByName(childTF)
	if !ok || e.Class != TFClassSeconds || e.Parent != SecondTF {
		return nil, fmt.Errorf("NewSparseSecondReducer: %q is not a 1s-fold second TF", childTF)
	}
	return &SparseSecondReducer{
		childTF: childTF,
		slots:   make(map[int64]Kline),
	}, nil
}

// SparseSecondParentRows is the 1s fetch cap: wantChildren × childSeconds.
// Caller adds a separate look-behind query; this is not dense ParentBarsNeeded.
func SparseSecondParentRows(childBars int, childTF string) (int, error) {
	ratio, err := sparseSecondRatio(childTF)
	if err != nil {
		return 0, err
	}
	if childBars < 1 {
		childBars = 1
	}
	return childBars * ratio, nil
}

func sparseSecondRatio(childTF string) (int, error) {
	e, ok := TimeframeByName(childTF)
	if !ok || e.Class != TFClassSeconds || e.Parent != SecondTF {
		return 0, fmt.Errorf("sparseSecondRatio: %q", childTF)
	}
	childMs, err := data.IntervalDurationMs(childTF)
	if err != nil {
		return 0, err
	}
	parentMs, err := data.IntervalDurationMs(SecondTF)
	if err != nil {
		return 0, err
	}
	if parentMs <= 0 || childMs%parentMs != 0 {
		return 0, fmt.Errorf("sparseSecondRatio: %s/%s", childTF, SecondTF)
	}
	return int(childMs / parentMs), nil
}

func (a *SparseSecondReducer) assemble() (Kline, bool) {
	if a == nil || a.bucket == 0 || len(a.slots) == 0 {
		return Kline{}, false
	}
	opens := make([]int64, 0, len(a.slots))
	for ot := range a.slots {
		opens = append(opens, ot)
	}
	sort.Slice(opens, func(i, j int) bool { return opens[i] < opens[j] })
	parents := make([]Kline, 0, len(opens))
	for _, ot := range opens {
		parents = append(parents, a.slots[ot])
	}
	bar, err := AssembleChildOHLCV(a.bucket, a.childTF, parents)
	if err != nil {
		return Kline{}, false
	}
	return bar, true
}

// OnParent updates the current child from one 1s parent. didClose means the previous
// child bucket is event-confirmed. ok means a forming child exists after the update.
func (a *SparseSecondReducer) OnParent(parent Kline) (closed Kline, forming Kline, didClose, ok bool) {
	if a == nil || parent.OpenTime < 0 {
		return Kline{}, Kline{}, false, false
	}
	co, err := data.CurrentBarOpen(parent.OpenTime, a.childTF)
	if err != nil {
		return Kline{}, Kline{}, false, false
	}
	if a.bucket != 0 && co < a.bucket {
		return Kline{}, Kline{}, false, false
	}
	if a.bucket != 0 && co != a.bucket {
		prev, have := a.assemble()
		a.slots = map[int64]Kline{parent.OpenTime: parent}
		a.bucket = co
		next, nextOK := a.assemble()
		if have {
			return prev, next, true, nextOK
		}
		return Kline{}, next, false, nextOK
	}
	if a.slots == nil {
		a.slots = make(map[int64]Kline)
	}
	a.bucket = co
	a.slots[parent.OpenTime] = parent
	next, nextOK := a.assemble()
	return Kline{}, next, false, nextOK
}

// ResetFromParents replays an ordered 1s series (boot / rebuild).
func (a *SparseSecondReducer) ResetFromParents(parents []Kline) {
	if a == nil {
		return
	}
	a.bucket = 0
	a.slots = make(map[int64]Kline)
	for _, p := range parents {
		a.OnParent(p)
	}
}

func dropFirstSparseChild(res SparseSecondFoldResult, lookBehind Kline, childTF string) SparseSecondFoldResult {
	co, err := data.CurrentBarOpen(lookBehind.OpenTime, childTF)
	if err != nil || co < 0 {
		return res
	}
	firstOpen := int64(-1)
	if len(res.Closed) > 0 {
		firstOpen = res.Closed[0].OpenTime
	} else if res.Forming != nil {
		firstOpen = res.Forming.OpenTime
	}
	if firstOpen < 0 || co != firstOpen {
		return res
	}
	if len(res.Closed) > 0 {
		res.Closed = res.Closed[1:]
		return res
	}
	res.Forming = nil
	return res
}

// FoldSparseSecondParents runs the live reducer over ordered 1s parents.
// lookBehind, when non-nil, is a parent strictly older than parents[0]:
// same child bucket → left child is truncated and dropped.
func FoldSparseSecondParents(parents []Kline, childTF string, lookBehind *Kline) (SparseSecondFoldResult, error) {
	acc, err := NewSparseSecondReducer(childTF)
	if err != nil {
		return SparseSecondFoldResult{}, err
	}
	var closed []Kline
	var forming *Kline
	for _, p := range parents {
		c, f, didClose, ok := acc.OnParent(p)
		if didClose {
			closed = append(closed, c)
		}
		if ok {
			bar := f
			forming = &bar
		}
	}
	res := SparseSecondFoldResult{Closed: closed, Forming: forming}
	if lookBehind != nil && lookBehind.OpenTime >= 0 {
		res = dropFirstSparseChild(res, *lookBehind, childTF)
	}
	return res, nil
}
