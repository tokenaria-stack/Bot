package data

import (
	"fmt"
	"strings"
	"time"
)

// Bar-boundary model (ADR-011 / Time Model Rule):
//
//   Fixed intervals (1m…1d): open = floor(ms / step) * step; next/prev = ± step.
//   Calendar intervals: Binance week = Monday 00:00 UTC; month = 1st 00:00 UTC.
//
// IntervalDurationMs must NOT be used for Cap, REST align, next tip, or month gaps.
// Prefer CurrentBarOpen / PreviousBarOpen / NextBarOpen / BarCloseTimeMs.

// CurrentBarOpen returns the open time (Unix ms) of the bar that contains ms.
func CurrentBarOpen(ms int64, interval string) (int64, error) {
	if ms < 0 {
		return 0, fmt.Errorf("CurrentBarOpen: negative time")
	}
	switch kind, stepMs, err := boundaryKind(interval); {
	case err != nil:
		return 0, err
	case kind == boundaryFixed:
		return (ms / stepMs) * stepMs, nil
	case kind == boundaryWeek:
		return mondayOpenUTC(ms), nil
	case kind == boundaryMonth:
		return monthOpenUTC(ms), nil
	default:
		return 0, fmt.Errorf("CurrentBarOpen: unsupported interval %q", interval)
	}
}

// PreviousBarOpen returns the open of the bar before openMs.
// openMs is floored via CurrentBarOpen first (safe for mid-bar inputs).
func PreviousBarOpen(openMs int64, interval string) (int64, error) {
	cur, err := CurrentBarOpen(openMs, interval)
	if err != nil {
		return 0, err
	}
	switch kind, stepMs, err := boundaryKind(interval); {
	case err != nil:
		return 0, err
	case kind == boundaryFixed:
		prev := cur - stepMs
		if prev < 0 {
			return 0, nil
		}
		return prev, nil
	case kind == boundaryWeek:
		return time.UnixMilli(cur).UTC().AddDate(0, 0, -7).UnixMilli(), nil
	case kind == boundaryMonth:
		t := time.UnixMilli(cur).UTC()
		return time.Date(t.Year(), t.Month()-1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(), nil
	default:
		return 0, fmt.Errorf("PreviousBarOpen: unsupported interval %q", interval)
	}
}

// NextBarOpen returns the open of the bar after openMs.
// openMs is floored via CurrentBarOpen first (safe for mid-bar inputs).
func NextBarOpen(openMs int64, interval string) (int64, error) {
	cur, err := CurrentBarOpen(openMs, interval)
	if err != nil {
		return 0, err
	}
	switch kind, stepMs, err := boundaryKind(interval); {
	case err != nil:
		return 0, err
	case kind == boundaryFixed:
		return cur + stepMs, nil
	case kind == boundaryWeek:
		return time.UnixMilli(cur).UTC().AddDate(0, 0, 7).UnixMilli(), nil
	case kind == boundaryMonth:
		t := time.UnixMilli(cur).UTC()
		return time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(), nil
	default:
		return 0, fmt.Errorf("NextBarOpen: unsupported interval %q", interval)
	}
}

// BarCloseTimeMs returns the inclusive close time for a bar that opens at openMs
// (Binance-style: nextOpen - 1). openMs is floored first.
func BarCloseTimeMs(openMs int64, interval string) (int64, error) {
	cur, err := CurrentBarOpen(openMs, interval)
	if err != nil {
		return 0, err
	}
	next, err := NextBarOpen(cur, interval)
	if err != nil {
		return 0, err
	}
	return next - 1, nil
}

type boundaryKindCode int

const (
	boundaryFixed boundaryKindCode = iota
	boundaryWeek
	boundaryMonth
)

func boundaryKind(interval string) (boundaryKindCode, int64, error) {
	iv := strings.TrimSpace(interval)
	switch iv {
	case "1w":
		return boundaryWeek, 0, nil
	case "1M":
		return boundaryMonth, 0, nil
	default:
		step, err := intervalDurationMs(iv)
		if err != nil {
			return 0, 0, err
		}
		return boundaryFixed, step, nil
	}
}

// mondayOpenUTC returns Monday 00:00 UTC of the ISO-style week containing ms
// (Binance 1w: Monday open).
func mondayOpenUTC(ms int64) int64 {
	t := time.UnixMilli(ms).UTC()
	// Monday=0 … Sunday=6
	offset := (int(t.Weekday()) + 6) % 7
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return day.AddDate(0, 0, -offset).UnixMilli()
}

func monthOpenUTC(ms int64) int64 {
	t := time.UnixMilli(ms).UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC).UnixMilli()
}
