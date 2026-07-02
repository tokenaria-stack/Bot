package strategy

import (
	"log"

	"trading_bot/exchange"
)

// markerExportSnapshot is a point-in-time copy of Marker RAM used for chart export
// without holding the read lock during serialization.
type markerExportSnapshot struct {
	window      []exchange.Kline
	chartPoints []BacktestChartPoint
	annotations []ChartAnnotation
}

// ExportChartSeriesForWindow exports chart points aligned to the requested kline window.
// Only the window slice is materialized; the full RAM buffer is not exported.
func ExportChartSeriesForWindow(m *Marker, window []exchange.Kline, settings RSXSettings) (*StreamingReplayResult, bool) {
	if m == nil || len(window) == 0 {
		return nil, false
	}
	_ = settings
	snap, ok := m.captureExportSnapshot(window)
	if !ok {
		return nil, false
	}
	return exportChartSeriesFromSnapshot(snap)
}

// ExportChartSeriesFromMarker builds chart points from the live Marker RAM state (no replay).
// Returns false when series are not aligned with klines.
func ExportChartSeriesFromMarker(m *Marker, settings RSXSettings) (*StreamingReplayResult, bool) {
	if m == nil {
		return nil, false
	}
	_ = settings
	m.mu.RLock()
	n := len(m.klines)
	if n == 0 || len(m.chartExportPoints) != n {
		if n > 0 && len(m.chartExportPoints) != n {
			log.Printf("[ChartExport] desync tf=%s klines=%d chartExportPoints=%d — export aborted",
				m.timeframe, n, len(m.chartExportPoints))
		}
		m.mu.RUnlock()
		return nil, false
	}
	snap := markerExportSnapshot{
		window:      append([]exchange.Kline(nil), m.klines...),
		chartPoints: append([]BacktestChartPoint(nil), m.chartExportPoints...),
		annotations: append([]ChartAnnotation(nil), m.Annotations...),
	}
	m.mu.RUnlock()
	return exportChartSeriesFromSnapshot(snap)
}

func (m *Marker) captureExportSnapshot(window []exchange.Kline) (markerExportSnapshot, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	n := len(m.klines)
	if !markerCoversKlines(m.klines, window) {
		return markerExportSnapshot{}, false
	}
	if n == 0 || len(m.chartExportPoints) != n {
		if n > 0 && len(m.chartExportPoints) != n {
			log.Printf("[ChartExport] desync tf=%s klines=%d chartExportPoints=%d — snapshot aborted",
				m.timeframe, n, len(m.chartExportPoints))
		}
		return markerExportSnapshot{}, false
	}
	start := n - len(window)
	end := start + len(window)
	if start < 0 || end > len(m.chartExportPoints) {
		log.Printf("[ChartExport] invalid slice tf=%s start=%d end=%d points=%d klines=%d",
			m.timeframe, start, end, len(m.chartExportPoints), n)
		return markerExportSnapshot{}, false
	}
	return markerExportSnapshot{
		window:      append([]exchange.Kline(nil), window...),
		chartPoints: append([]BacktestChartPoint(nil), m.chartExportPoints[start:end]...),
		annotations: append([]ChartAnnotation(nil), m.Annotations...),
	}, true
}

func exportChartSeriesFromSnapshot(snap markerExportSnapshot) (*StreamingReplayResult, bool) {
	window := snap.window
	chartData := append([]BacktestChartPoint(nil), snap.chartPoints...)
	if len(window) == 0 || len(chartData) == 0 {
		return nil, false
	}
	if len(chartData) != len(window) {
		log.Printf("[ChartExport] window/points length mismatch: klines=%d points=%d", len(window), len(chartData))
		return nil, false
	}

	markerByTime := make(map[int64]string, len(snap.annotations))
	for _, ann := range snap.annotations {
		if ann.Pane == normalizeAnnotationPane("rsx") && ann.Label != "" {
			if prev, ok := markerByTime[ann.Time]; !ok || rsxMarkerChartStrength(ann.Label) > rsxMarkerChartStrength(prev) {
				markerByTime[ann.Time] = ann.Label
			}
		}
	}
	for i := range chartData {
		if label, ok := markerByTime[chartData[i].Time]; ok {
			chartData[i].Marker = label
		}
	}

	// Drop trailing live bar when OHLC is not yet initialized (RAM seam vs WS).
	for len(chartData) > 0 {
		last := chartData[len(chartData)-1]
		if validReplayOHLC(last.Open, last.High, last.Low, last.Close) {
			break
		}
		chartData = chartData[:len(chartData)-1]
	}
	if len(chartData) == 0 {
		return nil, false
	}

	firstSec := exchange.ChartTimeSec(window[0].OpenTime)
	lastSec := exchange.ChartTimeSec(window[len(window)-1].OpenTime)
	rsxAnnotations := make([]ChartAnnotation, 0)
	for _, ann := range snap.annotations {
		if ann.Pane != normalizeAnnotationPane("rsx") {
			continue
		}
		if ann.Time >= firstSec && ann.Time <= lastSec {
			rsxAnnotations = append(rsxAnnotations, ann)
		}
	}

	histRSX := make([]float64, len(chartData))
	histWozduh := make([]float64, len(chartData))
	for i, pt := range chartData {
		histRSX[i] = pt.RSX
		histWozduh[i] = pt.RedLine
	}

	return &StreamingReplayResult{
		ChartPoints: chartData,
		Annotations: rsxAnnotations,
		HistKlines:  append([]exchange.Kline(nil), window...),
		HistRSX:     histRSX,
		HistWozduh:  histWozduh,
	}, true
}

// MarkerCoversKlines reports whether analyst klines end with the same open times as target.
func MarkerCoversKlines(analystKlines, target []exchange.Kline) bool {
	return markerCoversKlines(analystKlines, target)
}

func markerCoversKlines(analystKlines, target []exchange.Kline) bool {
	if len(analystKlines) < len(target) || len(target) == 0 {
		return false
	}
	tail := analystKlines[len(analystKlines)-len(target):]
	for i := range target {
		if tail[i].OpenTime != target[i].OpenTime {
			return false
		}
	}
	return true
}
