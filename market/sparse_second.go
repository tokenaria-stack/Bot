package market

import (
	"log"

	"trading_bot/data"
	"trading_bot/exchange"
)

// HydrateSparseSecondFrames folds the 1s Frame into activated second children (5s–45s).
// 1s forming is the last parent in the series — not wall-clock CurrentBarOpen.
func HydrateSparseSecondFrames(frames map[string]*Frame, symbol string, chaos ChaosConfig) {
	if frames == nil {
		return
	}
	parent := frames[exchange.SecondTF]
	var parents []exchange.Kline
	if parent != nil {
		parents = parent.GetKlines()
	}
	lookBehind := sparseSecondLookBehind(symbol, parents)
	for _, e := range exchange.SparseSecondChildren() {
		if frames[e.Name] != nil {
			continue
		}
		res, err := exchange.FoldSparseSecondParents(parents, e.Name, lookBehind)
		if err != nil {
			log.Printf("[Init] sparse-second %s fold: %v", e.Name, err)
			res = exchange.SparseSecondFoldResult{}
		}
		f := NewFrame(res.Closed, e.Name, chaos)
		if res.Forming != nil {
			f.UpdateKlineTick(*res.Forming, false)
		}
		f.UpdateIndicators()
		frames[e.Name] = f
		nForm := 0
		if res.Forming != nil {
			nForm = 1
		}
		log.Printf("[Init] Frame [%s] sparse from 1s (%d closed, %d forming)", e.Name, len(res.Closed), nForm)
	}
}

func sparseSecondLookBehind(symbol string, parents []exchange.Kline) *exchange.Kline {
	if len(parents) == 0 {
		return nil
	}
	end := parents[0].OpenTime
	if end <= 0 {
		return nil
	}
	rows, err := data.LoadMicroKlinesBeforeEnd(symbol, exchange.SecondTF, end-1, 1)
	if err != nil || len(rows) == 0 {
		return nil
	}
	ks := exchange.KlinesFromDataCandles(rows)
	if len(ks) == 0 {
		return nil
	}
	k := ks[len(ks)-1]
	return &k
}

func (m *Runtime) initSparseSecondReducers() {
	if m == nil {
		return
	}
	m.sparseSec = make(map[string]*exchange.SparseSecondReducer)
	var parents []exchange.Kline
	if p := m.frames[exchange.SecondTF]; p != nil {
		parents = p.GetKlines()
	}
	lookBehind := sparseSecondLookBehind(m.symbol, parents)
	for _, e := range exchange.SparseSecondChildren() {
		acc, err := exchange.NewSparseSecondReducer(e.Name)
		if err != nil {
			log.Printf("[Master] sparse-second reducer %s: %v", e.Name, err)
			continue
		}
		res, ferr := exchange.FoldSparseSecondParents(parents, e.Name, lookBehind)
		if ferr != nil {
			log.Printf("[Master] sparse-second fold %s: %v", e.Name, ferr)
		} else if res.Forming != nil {
			acc.ResetFromParents(parents)
		}
		m.sparseSec[e.Name] = acc
	}
}

func (m *Runtime) fanoutSparseSeconds(parent exchange.Kline) {
	if m == nil {
		return
	}
	for _, e := range exchange.SparseSecondChildren() {
		m.mu.RLock()
		acc := m.sparseSec[e.Name]
		frame := m.frames[e.Name]
		cb := m.onKlineBar
		m.mu.RUnlock()
		if acc == nil || frame == nil {
			continue
		}
		closed, forming, didClose, ok := acc.OnParent(parent)
		if didClose {
			frame.UpdateKlineTick(closed, true)
			if cb != nil {
				cb(e.Name, closed, true)
			}
			frame.UpdateIndicators()
		}
		if ok {
			frame.UpdateKlineTick(forming, false)
			if cb != nil {
				cb(e.Name, forming, false)
			}
		}
	}
}
