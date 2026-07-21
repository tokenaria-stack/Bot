package data

import "fmt"

const barStepsSafetyCap = 100_000

// BarStepsBetween counts how many NextBarOpen hops take fromOpen to toOpen.
// Both times are floored to bar opens. Returns 0 when equal.
// Used for tip lag and gap thresholds (not duration arithmetic).
func BarStepsBetween(fromOpen, toOpen int64, interval string) (int, error) {
	from, err := CurrentBarOpen(fromOpen, interval)
	if err != nil {
		return 0, err
	}
	to, err := CurrentBarOpen(toOpen, interval)
	if err != nil {
		return 0, err
	}
	if from > to {
		return 0, nil
	}
	if from == to {
		return 0, nil
	}
	n := 0
	cur := from
	for cur < to {
		next, err := NextBarOpen(cur, interval)
		if err != nil {
			return 0, err
		}
		if next <= cur {
			return 0, fmt.Errorf("BarStepsBetween: NextBarOpen stalled at %d (%s)", cur, interval)
		}
		cur = next
		n++
		if n > barStepsSafetyCap {
			return n, fmt.Errorf("BarStepsBetween: exceeded safety cap (%s)", interval)
		}
	}
	return n, nil
}

// AdvanceBarOpen returns the open n bars after openMs (n>=0). n=0 → CurrentBarOpen.
func AdvanceBarOpen(openMs int64, n int, interval string) (int64, error) {
	cur, err := CurrentBarOpen(openMs, interval)
	if err != nil {
		return 0, err
	}
	if n < 0 {
		return RetreatBarOpen(cur, -n, interval)
	}
	for i := 0; i < n; i++ {
		cur, err = NextBarOpen(cur, interval)
		if err != nil {
			return 0, err
		}
	}
	return cur, nil
}

// RetreatBarOpen returns the open n bars before openMs (n>=0). n=0 → CurrentBarOpen.
func RetreatBarOpen(openMs int64, n int, interval string) (int64, error) {
	cur, err := CurrentBarOpen(openMs, interval)
	if err != nil {
		return 0, err
	}
	if n < 0 {
		return AdvanceBarOpen(cur, -n, interval)
	}
	for i := 0; i < n; i++ {
		cur, err = PreviousBarOpen(cur, interval)
		if err != nil {
			return 0, err
		}
	}
	return cur, nil
}
