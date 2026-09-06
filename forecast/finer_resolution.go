package forecast

import (
	"crypto/sha256"
	"fmt"
	"hash"

	"trading_bot/data"
)

const finerTileSafetyCap = 100_000

// finerTilesPrimary reports whether finerTF exactly tiles one primary bar
// via NextBarOpen hops (strictly finer: at least two hops).
func finerTilesPrimary(primaryTF, finerTF string, sampleOpen int64) error {
	if primaryTF == "" || finerTF == "" {
		return fmt.Errorf("forecast: tiling requires primary and finer timeframes")
	}
	if primaryTF == finerTF {
		return fmt.Errorf("forecast: FinerTimeframe %q is not strictly finer than %q", finerTF, primaryTF)
	}
	p, err := data.CurrentBarOpen(sampleOpen, primaryTF)
	if err != nil {
		return fmt.Errorf("forecast: primary timeframe %q: %w", primaryTF, err)
	}
	q, err := data.NextBarOpen(p, primaryTF)
	if err != nil {
		return err
	}
	if _, err := data.NextBarOpen(p, finerTF); err != nil {
		return fmt.Errorf("forecast: finer timeframe %q: %w", finerTF, err)
	}
	x := p
	hops := 0
	for x < q {
		next, err := data.NextBarOpen(x, finerTF)
		if err != nil {
			return err
		}
		if next <= x {
			return fmt.Errorf("forecast: NextBarOpen stalled tiling %s into %s", finerTF, primaryTF)
		}
		if next > q {
			return fmt.Errorf("forecast: finer %s does not tile primary %s", finerTF, primaryTF)
		}
		x = next
		hops++
		if hops > finerTileSafetyCap {
			return fmt.Errorf("forecast: finer tiling exceeded safety cap")
		}
	}
	if x != q {
		return fmt.Errorf("forecast: finer %s does not land on primary boundary of %s", finerTF, primaryTF)
	}
	if hops < 2 {
		return fmt.Errorf("forecast: FinerTimeframe %q is not strictly finer than %q", finerTF, primaryTF)
	}
	return nil
}

type finerSourceHasher struct {
	h hash.Hash
}

func newFinerSourceHasher(market MarketKey) *finerSourceHasher {
	s := &finerSourceHasher{h: sha256.New()}
	hashPutString(s.h, "LS1F")
	hashPutMarket(s.h, market)
	return s
}

func (s *finerSourceHasher) window(candidateAt, primaryDualHitAt int64, consulted []CanonicalClosedBar) {
	hashPutI64(s.h, candidateAt)
	hashPutI64(s.h, primaryDualHitAt)
	hashPutU32(s.h, uint32(len(consulted)))
	for _, b := range consulted {
		hashPutBar(s.h, b)
	}
}

func (s *finerSourceHasher) sum() Digest {
	var d Digest
	copy(d[:], s.h.Sum(nil))
	return d
}

func emptyFinerSourceDigest(market MarketKey) Digest {
	return newFinerSourceHasher(market).sum()
}

func lookupFinerBar(bars []CanonicalClosedBar, open int64) (CanonicalClosedBar, bool) {
	lo, hi := 0, len(bars)
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if bars[mid].OpenTime < open {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo < len(bars) && bars[lo].OpenTime == open {
		return bars[lo], true
	}
	return CanonicalClosedBar{}, false
}

type finerResolve struct {
	market    MarketKey
	primaryTF string
	finerTF   string
	bars      []CanonicalClosedBar
	h         *finerSourceHasher
	windows   int
}

func newFinerResolve(market MarketKey, primaryTF string, bars []CanonicalClosedBar) *finerResolve {
	return &finerResolve{
		market:    market,
		primaryTF: primaryTF,
		finerTF:   market.Timeframe,
		bars:      bars,
		h:         newFinerSourceHasher(market),
	}
}

func (f *finerResolve) resolve(candidateAt, parentAt int64, upper, lower float64) (TargetOutcome, LabelReason, error) {
	f.windows++
	q, err := data.NextBarOpen(parentAt, f.primaryTF)
	if err != nil {
		return "", "", err
	}
	var consulted []CanonicalClosedBar
	expected := parentAt
	for expected < q {
		next, err := data.NextBarOpen(expected, f.finerTF)
		if err != nil {
			return "", "", err
		}
		if next > q {
			return "", "", fmt.Errorf("forecast: finer bar at %d would cross primary seam %d", expected, q)
		}
		b, ok := lookupFinerBar(f.bars, expected)
		if !ok {
			f.h.window(candidateAt, parentAt, consulted)
			if expected == parentAt {
				return OutcomeAmbiguous, ReasonFinerMissing, nil
			}
			return OutcomeAmbiguous, ReasonFinerGap, nil
		}
		consulted = append(consulted, b)
		up := b.High >= upper
		down := b.Low <= lower
		if up && down {
			f.h.window(candidateAt, parentAt, consulted)
			return OutcomeAmbiguous, ReasonFinerDualHit, nil
		}
		if up {
			f.h.window(candidateAt, parentAt, consulted)
			return OutcomeUpFirst, ReasonNone, nil
		}
		if down {
			f.h.window(candidateAt, parentAt, consulted)
			return OutcomeDownFirst, ReasonNone, nil
		}
		expected = next
	}
	f.h.window(candidateAt, parentAt, consulted)
	return OutcomeAmbiguous, ReasonFinerInconsistent, nil
}
