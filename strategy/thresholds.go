package strategy

import "sync"

const DefaultScoreThreshold = 70

const (
	minScoreThreshold = 10
	maxScoreThreshold = 200
)

var (
	thresholdMu           sync.RWMutex
	dynamicLongThreshold  = DefaultScoreThreshold
	dynamicShortThreshold = DefaultScoreThreshold
)

// LongScoreThreshold returns the current long entry score threshold.
func LongScoreThreshold() int {
	thresholdMu.RLock()
	defer thresholdMu.RUnlock()
	return dynamicLongThreshold
}

// ShortScoreThreshold returns the current short entry score threshold.
func ShortScoreThreshold() int {
	thresholdMu.RLock()
	defer thresholdMu.RUnlock()
	return dynamicShortThreshold
}

// SetScoreThresholds updates dynamic long/short entry thresholds (clamped to 10–200).
func SetScoreThresholds(long, short int) {
	thresholdMu.Lock()
	defer thresholdMu.Unlock()
	if long >= minScoreThreshold && long <= maxScoreThreshold {
		dynamicLongThreshold = long
	}
	if short >= minScoreThreshold && short <= maxScoreThreshold {
		dynamicShortThreshold = short
	}
}
