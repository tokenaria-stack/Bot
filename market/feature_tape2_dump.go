package market

import (
	"fmt"
	"os"

	"trading_bot/data"
	"trading_bot/exchange"
	"trading_bot/forecast"
)

func errTape2(msg string) error { return fmt.Errorf("market: feature-tape-2: %s", msg) }

func klineToCanonical(k exchange.Kline) forecast.CanonicalClosedBar {
	return forecast.CanonicalClosedBar{
		OpenTime: k.OpenTime, Open: k.Open, High: k.High, Low: k.Low, Close: k.Close, Volume: k.Volume,
	}
}

func filterFromStart(key forecast.MarketKey, bars []exchange.Kline) []exchange.Kline {
	start := ResearchSourceStartMs(key)
	out := bars
	i := 0
	for i < len(out) && out[i].OpenTime < start {
		i++
	}
	if i == 0 {
		return out
	}
	return out[i:]
}

func validateNativeStream(key forecast.MarketKey, bars []exchange.Kline) error {
	if err := key.Validate(); err != nil {
		return err
	}
	if len(bars) == 0 {
		return errTape2("empty source " + key.String())
	}
	start := ResearchSourceStartMs(key)
	if bars[0].OpenTime < start {
		return errTape2("source before ResearchSourceStartMs " + key.String())
	}
	for i, k := range bars {
		if k.OpenTime <= 0 {
			return forecast.Spec2UnexpectedNativeGap(key, "nonpositive OpenTime")
		}
		if i == 0 {
			continue
		}
		if k.OpenTime <= bars[i-1].OpenTime {
			return forecast.Spec2UnexpectedNativeGap(key, "order")
		}
		next, err := data.NextBarOpen(bars[i-1].OpenTime, key.Timeframe)
		if err != nil {
			return err
		}
		if k.OpenTime != next {
			return forecast.Spec2UnexpectedNativeGap(key, fmt.Sprintf("expected %d got %d", next, k.OpenTime))
		}
	}
	return nil
}

func trimHTFToPrimaryKnowledge(htf []exchange.Kline, tf string, lastPrimaryClose int64) ([]exchange.Kline, error) {
	cut := -1
	for i, k := range htf {
		ct, err := data.BarCloseTimeMs(k.OpenTime, tf)
		if err != nil {
			return nil, err
		}
		if ct <= lastPrimaryClose {
			cut = i
		}
	}
	if cut < 0 {
		return nil, errTape2("no HTF bar closed by last primary knowledge time")
	}
	return htf[:cut+1], nil
}

func sourceDigest(key forecast.MarketKey, bars []exchange.Kline) forecast.Digest {
	h := forecast.NewSourceRangeHasher(key)
	for _, k := range bars {
		h.Add(klineToCanonical(k))
	}
	return h.Sum()
}

func tape2Header(spec forecast.FeatureSpec2, d15, d1h, d4h forecast.Digest) (forecast.Tape2Header, error) {
	sid, err := spec.Identity()
	if err != nil {
		return forecast.Tape2Header{}, err
	}
	pid, err := spec.Plan.Identity()
	if err != nil {
		return forecast.Tape2Header{}, err
	}
	fid, err := spec.Features.Identity()
	if err != nil {
		return forecast.Tape2Header{}, err
	}
	aid, err := spec.Analysis.Identity()
	if err != nil {
		return forecast.Tape2Header{}, err
	}
	tid, err := spec.Target.Identity()
	if err != nil {
		return forecast.Tape2Header{}, err
	}
	return forecast.Tape2Header{
		FormatVersion:  forecast.FeatureTapeFormatV2,
		SpecDigest:     sid.Digest,
		PlanDigest:     pid.Digest,
		FeaturesDigest: fid.Digest,
		AnalysisDigest: aid.Digest,
		TargetDigest:   tid.Digest,
		Primary:        spec.Primary,
		HTF1h:          spec.HTF1h,
		HTF4h:          spec.HTF4h,
		FeatureIDs:     forecast.FeatureSpec2IDs(),
		VectorLen:      forecast.Spec2FeatureWidth,
		Q:              spec.Q,
		Demand:         spec.Demand,
		PrimarySource:  d15,
		HTF1hSource:    d1h,
		HTF4hSource:    d4h,
	}, nil
}

// DumpFeatureTape2 writes feature-tape-v2 from three native closed streams.
func DumpFeatureTape2(path string, spec forecast.FeatureSpec2, primary, htf1h, htf4h []exchange.Kline) error {
	if _, err := forecast.ResolveFeatureSpec2(spec.Primary, spec.Target, spec.Analysis); err != nil {
		return err
	}
	if spec.Analysis.Config.RSXSignal != forecast.Spec2RSXSignal {
		return errTape2("analysis signal must be 14")
	}
	p := filterFromStart(spec.Primary, primary)
	s1 := filterFromStart(spec.HTF1h, htf1h)
	s4 := filterFromStart(spec.HTF4h, htf4h)
	if err := validateNativeStream(spec.Primary, p); err != nil {
		return err
	}
	if err := validateNativeStream(spec.HTF1h, s1); err != nil {
		return err
	}
	if err := validateNativeStream(spec.HTF4h, s4); err != nil {
		return err
	}
	lastCT, err := data.BarCloseTimeMs(p[len(p)-1].OpenTime, spec.Primary.Timeframe)
	if err != nil {
		return err
	}
	s1, err = trimHTFToPrimaryKnowledge(s1, spec.HTF1h.Timeframe, lastCT)
	if err != nil {
		return err
	}
	s4, err = trimHTFToPrimaryKnowledge(s4, spec.HTF4h.Timeframe, lastCT)
	if err != nil {
		return err
	}
	d15, d1, d4 := sourceDigest(spec.Primary, p), sourceDigest(spec.HTF1h, s1), sourceDigest(spec.HTF4h, s4)
	if sourceDigest(spec.Primary, p) != d15 || sourceDigest(spec.HTF1h, s1) != d1 || sourceDigest(spec.HTF4h, s4) != d4 {
		return errTape2("source snapshot changed")
	}
	hdr, err := tape2Header(spec, d15, d1, d4)
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		tmp := path + ".cmp"
		_ = os.Remove(tmp)
		if err := writeTape2(tmp, spec, hdr, p, s1, s4); err != nil {
			_ = os.Remove(tmp)
			return err
		}
		_, _, oldF, errOld := forecast.ReadTape2(path)
		_, _, newF, errNew := forecast.ReadTape2(tmp)
		_ = os.Remove(tmp)
		if errOld != nil {
			return fmt.Errorf("market: refuse incomplete feature-tape-v2 %s: %w", path, errOld)
		}
		if errNew != nil {
			return errNew
		}
		if oldF.ContentDigest == newF.ContentDigest {
			return nil // MATCH
		}
		return errTape2("existing artifact identity differs (REFUSE)")
	} else if !os.IsNotExist(err) {
		return err
	}
	return writeTape2(path, spec, hdr, p, s1, s4)
}

func writeTape2(path string, spec forecast.FeatureSpec2, hdr forecast.Tape2Header, p, s1, s4 []exchange.Kline) error {
	rt, err := NewFeatureRuntime2(spec)
	if err != nil {
		return err
	}
	w, err := forecast.CreateTape2Writer(path, hdr)
	if err != nil {
		return err
	}
	i1, i4 := 0, 0
	closeOf := func(k exchange.Kline, tf string) (int64, error) {
		return data.BarCloseTimeMs(k.OpenTime, tf)
	}
	for _, bar := range p {
		pct, err := closeOf(bar, spec.Primary.Timeframe)
		if err != nil {
			w.Abort()
			return err
		}
		for i4 < len(s4) {
			ct, err := closeOf(s4[i4], spec.HTF4h.Timeframe)
			if err != nil {
				w.Abort()
				return err
			}
			if ct > pct {
				break
			}
			k := s4[i4]
			if err := rt.Update4h(k.OpenTime, k.High, k.Low, k.Close); err != nil {
				w.Abort()
				return err
			}
			i4++
		}
		for i1 < len(s1) {
			ct, err := closeOf(s1[i1], spec.HTF1h.Timeframe)
			if err != nil {
				w.Abort()
				return err
			}
			if ct > pct {
				break
			}
			k := s1[i1]
			if err := rt.Update1h(k.OpenTime, k.High, k.Low, k.Close); err != nil {
				w.Abort()
				return err
			}
			i1++
		}
		row, err := rt.Update15m(bar.OpenTime, bar.High, bar.Low, bar.Close)
		if err != nil {
			w.Abort()
			return err
		}
		var values []float64
		if row.Ready {
			values = row.Values.Slice()
		}
		if err := w.WriteRow(row.At, row.Ready, row.Reason, values); err != nil {
			return err
		}
	}
	_, err = w.Finish()
	return err
}

func ResearchFeatureTape2FileName(spec forecast.FeatureSpec2) (string, error) {
	id, err := spec.Identity()
	if err != nil {
		return "", err
	}
	return spec.Primary.Venue + "_" + spec.Primary.Instrument + "_" + spec.Primary.Contract +
		"_15m_spec2-" + id.Digest.Short() + ".featuretape2", nil
}
