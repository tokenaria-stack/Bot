/**
 * Shared viewport capture/restore for Live and Backtest charts (Lightweight Charts).
 */
(function initViewport(global) {
  const DEFAULT_VISIBLE_BARS = 1000;
  const RIGHT_EDGE_MARGIN = 5;
  const MIN_ZOOM = 50;

  function normalizeTime(t) {
    if (typeof t === 'object' && t !== null && typeof t.year === 'number') {
      return new Date(Date.UTC(t.year, t.month - 1, t.day)).getTime() / 1000;
    }
    return Number(t);
  }

  function resolveWindowSize(visibleBars, total) {
    const maxZoom = Math.max(MIN_ZOOM, total * 2);
    let windowSize = visibleBars || DEFAULT_VISIBLE_BARS;
    return Math.max(MIN_ZOOM, Math.min(windowSize, maxZoom));
  }

  function captureViewport(chart, series) {
    if (!chart?.timeScale || !series?.data) return null;

    let range;
    let timeRange;
    try {
      range = chart.timeScale().getVisibleLogicalRange();
      timeRange = chart.timeScale().getVisibleRange();
    } catch {
      return null;
    }
    if (!range || !timeRange || timeRange.from == null || timeRange.to == null) return null;

    const timeFrom = normalizeTime(timeRange.from);
    const timeTo = normalizeTime(timeRange.to);
    if (!Number.isFinite(timeFrom) || !Number.isFinite(timeTo)) return null;

    const totalBars = series.data().length;
    const lastIndex = totalBars > 0 ? totalBars - 1 : 0;
    const isAtRight = range.to >= lastIndex - RIGHT_EDGE_MARGIN;
    const anchor = {
      visibleBars: Math.min(range.to - range.from, totalBars),
      centerTime: (timeFrom + timeTo) / 2,
      targetTime: (timeFrom + timeTo) / 2,
      type: isAtRight ? 'right' : 'center',
      isAtRight,
      rightOffset: range.to - lastIndex,
    };
    return anchor;
  }

  function barTimeAt(data, index, chartTimeFn) {
    const raw = data[index]?.time;
    if (raw == null) return null;
    return chartTimeFn ? chartTimeFn(raw) : raw;
  }

  function computeLogicalRange(data, anchor, chartTimeFn) {
    const total = data?.length ?? 0;
    if (total === 0) return null;

    if (!anchor) {
      const windowSize = resolveWindowSize(null, total);
      const targetTo = total - 1;
      return { from: targetTo - windowSize, to: targetTo };
    }

    const windowSize = resolveWindowSize(anchor.visibleBars, total);

    if (anchor.isAtRight) {
      const rightOffset = Math.min(anchor.rightOffset || 0, Math.floor(windowSize / 2));
      const targetTo = (total - 1) + rightOffset;
      const targetFrom = targetTo - windowSize;
      return { from: targetFrom, to: targetTo };
    }

    let targetIndex = total - 1;
    if (anchor.centerTime != null && Number.isFinite(anchor.centerTime)) {
      for (let i = 0; i < total; i++) {
        const t = barTimeAt(data, i, chartTimeFn);
        if (t != null && t >= anchor.centerTime) {
          targetIndex = i;
          break;
        }
      }
    }
    const half = windowSize / 2;
    return {
      from: targetIndex - half,
      to: Math.min(total - 1 + Math.floor(windowSize / 4), targetIndex + half),
    };
  }

  function restoreViewport(chart, series, anchor, options = {}) {
    if (!chart?.timeScale || !series?.data) return;
    const data = options.candles || series.data();
    const range = computeLogicalRange(data, anchor, options.chartTime);
    if (!range) return;
    const rangeOpts = options.animate === false ? { animate: false } : undefined;
    chart.timeScale().setVisibleLogicalRange(range, rangeOpts);
  }

  function captureLogicalRange(chart) {
    if (!chart?.timeScale) return null;
    try {
      return chart.timeScale().getVisibleLogicalRange();
    } catch {
      return null;
    }
  }

  function lockPriceAutoScaleDuring(chart, fn) {
    if (typeof fn !== 'function') return;
    if (!chart?.priceScale) {
      fn();
      return;
    }
    let scale;
    try {
      scale = chart.priceScale('right');
    } catch {
      fn();
      return;
    }
    let prevAuto = true;
    try {
      prevAuto = scale.options?.().autoScale ?? true;
    } catch {
      /* noop */
    }
    try {
      scale.applyOptions({ autoScale: false });
    } catch {
      /* noop */
    }
    try {
      fn();
    } finally {
      setTimeout(() => {
        try {
          scale.applyOptions({ autoScale: prevAuto });
        } catch {
          /* noop */
        }
      }, 0);
    }
  }

  function restoreLogicalRangeToCharts(charts, range, options = {}) {
    if (!range || !charts?.length) return;
    const rangeOpts = options.animate === false ? { animate: false } : undefined;
    const lockAutoScale = options.lockAutoScale !== false;
    const priceChart = options.priceChart;
    charts.forEach((chart) => {
      if (!chart?.timeScale) return;
      const applyRange = () => {
        try {
          chart.timeScale().setVisibleLogicalRange(range, rangeOpts);
        } catch {
          /* noop */
        }
      };
      if (lockAutoScale && priceChart && chart === priceChart) {
        lockPriceAutoScaleDuring(chart, applyRange);
      } else {
        applyRange();
      }
    });
  }

  function restoreViewportToCharts(chartData, series, anchor, options = {}) {
    if (!chartData || !series) return;
    const data = options.candles || series.data();
    if ((data?.length ?? 0) === 0) return;

    const range = options.logicalRange || computeLogicalRange(data, anchor, options.chartTime);
    if (!range) return;

    const charts = chartData.allCharts?.length
      ? chartData.allCharts
      : [chartData.chart || chartData.priceChart].filter(Boolean);

    restoreLogicalRangeToCharts(charts, range, {
      animate: options.animate,
      lockAutoScale: options.lockAutoScale,
      priceChart: chartData.priceChart || chartData.chart,
    });
  }

  global.Viewport = {
    DEFAULT_VISIBLE_BARS,
    RIGHT_EDGE_MARGIN,
    MIN_ZOOM,
    captureViewport,
    captureLogicalRange,
    computeLogicalRange,
    restoreViewport,
    restoreLogicalRangeToCharts,
    restoreViewportToCharts,
    lockPriceAutoScaleDuring,
  };
})(typeof window !== 'undefined' ? window : globalThis);
