/**
 * chart-core.js — sterile live ChartAdapter facade (Project Renaissance).
 * Contract: 7 public methods only. Price pane owns candles; indicators via DDRFactory.
 */
(function () {
  'use strict';

  const PRICE_SCALE_MIN = 75;
  /** @type {{ charts: object, candleSeries: object, volumeSeries: object, _syncingTimeScale: boolean, _syncingCrosshair: boolean, _disposers: (() => void)[] }|null} */
  let _live = null;
  let _liveUpdating = false;

  function hostSize(el, fw, fh) {
    return {
      width: Math.max(el?.clientWidth || 0, fw),
      height: Math.max(el?.clientHeight || 0, fh),
    };
  }

  function layoutOptions() {
    const tv = typeof TV !== 'undefined' ? TV : { bg: '#131722', grid: '#1e222d', border: '#2a2e39', text: '#787b86' };
    return {
      background: { color: tv.bg },
      textColor: tv.text,
      fontSize: 11,
      attributionLogo: false,
    };
  }

  function gridOptions() {
    const tv = typeof TV !== 'undefined' ? TV : { grid: '#1e222d' };
    return {
      vertLines: { color: tv.grid, style: LightweightCharts.LineStyle.Dotted },
      horzLines: { color: tv.grid, style: LightweightCharts.LineStyle.Dotted },
    };
  }

  function timeScaleOptions() {
    const base = typeof SHARED_TIME_SCALE !== 'undefined'
      ? { ...SHARED_TIME_SCALE }
      : { borderColor: '#2a2e39', timeVisible: true, secondsVisible: false };
    return base;
  }

  function crosshairOptions() {
    return typeof SHARED_CROSSHAIR !== 'undefined' ? { ...SHARED_CROSSHAIR } : { mode: LightweightCharts.CrosshairMode.Normal };
  }

  function priceScaleOptions(extra = {}) {
    const tv = typeof TV !== 'undefined' ? TV : { border: '#2a2e39' };
    return {
      borderColor: tv.border,
      autoScale: true,
      minimumWidth: PRICE_SCALE_MIN,
      alignLabels: true,
      borderVisible: true,
      ...extra,
    };
  }

  function unifiedTimeScaleOptions(showAxisLabels) {
    const base = timeScaleOptions();
    const opts = {
      ...base,
      timeVisible: showAxisLabels,
      secondsVisible: false,
    };
    // Keep axis/grid geometry; hide tick labels on slave panes only.
    if (!showAxisLabels) {
      opts.tickMarkFormatter = () => '';
    }
    return opts;
  }

  function createPaneChart(host, width, height, showAxisLabels) {
    return LightweightCharts.createChart(host, {
      autoSize: false,
      layout: layoutOptions(),
      grid: gridOptions(),
      crosshair: crosshairOptions(),
      timeScale: unifiedTimeScaleOptions(showAxisLabels),
      width,
      height,
      rightPriceScale: priceScaleOptions({
        mode: LightweightCharts.PriceScaleMode.Normal,
        scaleMargins: showAxisLabels
          ? undefined
          : { top: 0.05, bottom: 0.05 },
      }),
    });
  }

  function createPriceChart(host, width, height) {
    const chart = createPaneChart(host, width, height, true);
    chart.applyOptions({
      rightPriceScale: priceScaleOptions({ mode: LightweightCharts.PriceScaleMode.Logarithmic }),
    });
    return chart;
  }

  function createSlaveChart(host, width, height) {
    return LightweightCharts.createChart(host, {
      autoSize: false,
      layout: layoutOptions(),
      grid: gridOptions(),
      crosshair: crosshairOptions(),
      timeScale: unifiedTimeScaleOptions(false),
      width,
      height,
      handleScroll: false,
      handleScale: { axisPressedMouseMove: { price: true, time: false }, mouseWheel: false },
      rightPriceScale: priceScaleOptions({
        mode: LightweightCharts.PriceScaleMode.Normal,
        scaleMargins: { top: 0.05, bottom: 0.05 },
      }),
    });
  }

  function isFiniteLogicalRange(range) {
    return range
      && Number.isFinite(range.from)
      && Number.isFinite(range.to)
      && range.to > range.from;
  }

  function syncVisibleLogicalRange(targetChart, range) {
    if (!targetChart?.timeScale || !isFiniteLogicalRange(range)) return;
    targetChart.timeScale().setVisibleLogicalRange(range, { animate: false });
  }

  /** Asymmetric Data Guard: only Price subscribes; slaves never broadcast. */
  function syncTimeScaleMasterToSlaves(state) {
    const master = state.charts.price;
    const slaves = [state.charts.wozduh, state.charts.rsx].filter(Boolean);
    if (!master || !slaves.length) return;

    master.timeScale().subscribeVisibleLogicalRangeChange((range) => {
      if (!range || state._syncingTimeScale || _liveUpdating) return;
      if (!isFiniteLogicalRange(range)) return;

      state._syncingTimeScale = true;
      try {
        slaves.forEach((slave) => syncVisibleLogicalRange(slave, range));
      } finally {
        state._syncingTimeScale = false;
      }
    });
  }

  /** Canonical pane anchors — never hunt seriesMap / score / debt series. */
  function crosshairSeriesForChart(state, chart) {
    if (chart === state.charts.price) return state.candleSeries;
    const factory = (typeof window !== 'undefined') ? window.DDRFactory : null;
    if (!factory?.cutoverActive || typeof factory.getSeries !== 'function') return null;
    if (chart === state.charts.wozduh) return factory.getSeries('woz_fast');
    if (chart === state.charts.rsx) return factory.getSeries('line_rsx');
    return null;
  }

  function crosshairAnchorId(state, chart) {
    if (chart === state.charts.wozduh) return 'woz_fast';
    if (chart === state.charts.rsx) return 'line_rsx';
    return null;
  }

  /** Value at business-time from DDR hydrated buffer (cross-pane when seriesData is empty). */
  function hydratedValueAtTime(seriesId, time) {
    const factory = (typeof window !== 'undefined') ? window.DDRFactory : null;
    const points = factory?.getHydratedSeries?.(seriesId);
    if (!points?.length || time == null) return null;
    // Exact match first (LWC business-day / unix sec equality).
    for (let i = points.length - 1; i >= 0; i--) {
      const p = points[i];
      if (p?.time !== time) continue;
      const v = p.value;
      return Number.isFinite(v) ? v : null; // whitespace → null
    }
    return null;
  }

  function candleCloseAtTime(time) {
    const store = (typeof window !== 'undefined') ? window.liveColumnarStore : null;
    if (!store || time == null) return null;
    const snap = typeof store.snapshot === 'function' ? store.snapshot() : null;
    const times = snap?.times;
    const closes = snap?.candles?.close;
    if (!Array.isArray(times) || !Array.isArray(closes)) return null;
    for (let i = times.length - 1; i >= 0; i--) {
      const t = times[i];
      // Columnar times are unix-sec; LWC may use the same after normalize.
      if (t !== time && t !== time?.timestamp && Number(t) !== Number(time)) continue;
      const v = Number(closes[i]);
      return Number.isFinite(v) ? v : null;
    }
    return null;
  }

  /**
   * Y for setCrosshairPosition must be in the *target* series domain.
   * Same-chart: seriesData. Cross-pane: hydrated / store (never candle.close on osc).
   */
  function resolveCrosshairY(state, targetChart, targetSeries, param) {
    const point = param.seriesData?.get?.(targetSeries);
    const fromParam = point?.value ?? point?.close;
    if (Number.isFinite(fromParam)) return fromParam;

    const anchorId = crosshairAnchorId(state, targetChart);
    if (anchorId) return hydratedValueAtTime(anchorId, param.time);

    if (targetChart === state.charts.price) return candleCloseAtTime(param.time);
    return null;
  }

  function syncCrosshair(state) {
    const charts = [state.charts.price, state.charts.wozduh, state.charts.rsx].filter(Boolean);
    charts.forEach((source) => {
      source.subscribeCrosshairMove((param) => {
        if (state._syncingCrosshair) return;
        state._syncingCrosshair = true;
        try {
          if (param?.time == null) {
            charts.forEach((target) => {
              if (target !== source && typeof target.clearCrosshairPosition === 'function') {
                target.clearCrosshairPosition();
              }
            });
            return;
          }
          charts.forEach((target) => {
            if (target === source) return;
            if (typeof target.setCrosshairPosition !== 'function') return;
            const targetSeries = crosshairSeriesForChart(state, target);
            if (!targetSeries) {
              target.clearCrosshairPosition?.();
              return;
            }
            const yValue = resolveCrosshairY(state, target, targetSeries, param);
            if (yValue == null || !Number.isFinite(yValue)) {
              target.clearCrosshairPosition?.();
              return;
            }
            target.setCrosshairPosition(yValue, param.time, targetSeries);
          });
        } finally {
          state._syncingCrosshair = false;
        }
      });
    });
  }

  function bindResize(host, chart, disposers) {
    if (!host || !chart) return;
    const ro = new ResizeObserver((entries) => {
      const rect = entries[0]?.contentRect;
      if (!rect || rect.width <= 0 || rect.height <= 0) return;
      chart.applyOptions({ width: rect.width, height: rect.height });
    });
    ro.observe(host);
    disposers.push(() => ro.disconnect());
  }

  function defaultPricePaneConfig() {
    return {
      candle: {
        upColor: '#089981',
        downColor: '#f23645',
        wickUpColor: '#089981',
        wickDownColor: '#f23645',
        borderVisible: false,
      },
      volume: { priceFormat: { type: 'volume' }, priceScaleId: 'volume' },
      priceScale: { scaleMargins: { top: 0.05, bottom: 0.22 } },
      volumeScale: { scaleMargins: { top: 0.82, bottom: 0 } },
    };
  }

  function resolvePricePaneConfig() {
    if (typeof ensureChartLibraryStyles === 'function') {
      ensureChartLibraryStyles();
    }
    return INDICATOR_CONFIG?.price ?? defaultPricePaneConfig();
  }

  function buildLiveState(selectors) {
    const sel = selectors || (typeof LIVE_CHART_SELECTORS !== 'undefined' ? LIVE_CHART_SELECTORS : {});
    const priceHost = document.getElementById(sel.chartContainer || 'price-chart');
    const wozHost = document.getElementById(sel.oscContainer || 'wozduh-chart');
    const rsxHost = document.getElementById(sel.rsxContainer || 'rsx-chart');
    if (!priceHost || !wozHost || !rsxHost) return null;

    priceHost.innerHTML = '';
    wozHost.innerHTML = '';
    rsxHost.innerHTML = '';

    const root = document.getElementById('live-chart-container');
    const priceSize = hostSize(priceHost, root?.clientWidth || 800, 280);
    const wozSize = hostSize(wozHost, root?.clientWidth || 800, 140);
    const rsxSize = hostSize(rsxHost, root?.clientWidth || 800, 140);

    const priceChart = createPriceChart(priceHost, priceSize.width, priceSize.height);
    const wozduhChart = createSlaveChart(wozHost, wozSize.width, wozSize.height);
    const rsxChart = createSlaveChart(rsxHost, rsxSize.width, rsxSize.height);

    const sharedTs = unifiedTimeScaleOptions(false);
    wozduhChart.timeScale().applyOptions(sharedTs);
    rsxChart.timeScale().applyOptions(sharedTs);
    priceChart.timeScale().applyOptions(unifiedTimeScaleOptions(true));

    const priceCfg = resolvePricePaneConfig();
    const candleOpts = priceCfg?.candle || { upColor: '#089981', downColor: '#f23645', wickUpColor: '#089981', wickDownColor: '#f23645', borderVisible: false };
    const volumeOpts = priceCfg?.volume || { priceFormat: { type: 'volume' }, priceScaleId: 'volume' };

    const candleSeries = priceChart.addCandlestickSeries({ ...candleOpts, priceScaleId: 'right' });
    const volumeSeries = priceChart.addHistogramSeries(volumeOpts);

    const priceMargins = priceCfg?.priceScale?.scaleMargins || { top: 0.05, bottom: 0.22 };
    const volumeMargins = priceCfg?.volumeScale?.scaleMargins || { top: 0.82, bottom: 0 };
    priceChart.priceScale('right').applyOptions({
      ...priceScaleOptions({ mode: LightweightCharts.PriceScaleMode.Logarithmic }),
      scaleMargins: priceMargins,
    });
    priceChart.priceScale('volume').applyOptions({
      scaleMargins: volumeMargins,
      autoScale: true,
      visible: true,
    });

    const state = {
      charts: { price: priceChart, wozduh: wozduhChart, rsx: rsxChart },
      candleSeries,
      volumeSeries,
      _syncingTimeScale: false,
      _syncingCrosshair: false,
      _disposers: [],
    };

    bindResize(priceHost, priceChart, state._disposers);
    bindResize(wozHost, wozduhChart, state._disposers);
    bindResize(rsxHost, rsxChart, state._disposers);
    syncTimeScaleMasterToSlaves(state);
    syncCrosshair(state);
    return state;
  }

  function paintCandles(state, candles) {
    if (!state?.candleSeries || !Array.isArray(candles) || !candles.length) return;
    state.candleSeries.setData(candles);
    if (state.volumeSeries && typeof toVolumeBars === 'function') {
      state.volumeSeries.setData(toVolumeBars(candles));
    }
    if (typeof ToolbarController !== 'undefined') {
      ToolbarController.updateVolume(candles);
    }
  }

  function updateCandle(state, candle) {
    if (!state?.candleSeries || !candle) return;
    state.candleSeries.update(candle);
    if (state.volumeSeries && typeof toVolumeBars === 'function') {
      state.volumeSeries.update(toVolumeBars([candle])[0]);
    }
  }

  const ChartAdapter = {
    initLiveCharts(selectors) {
      if (typeof LightweightCharts === 'undefined') return false;
      if (_live?.charts?.price) return true;
      _live = buildLiveState(selectors);
      return !!_live;
    },

    getChart(context, pane = 'price') {
      if (context !== 'live' || !_live) return null;
      if (pane === 'wozduh' || pane === 'osc') return _live.charts.wozduh;
      if (pane === 'rsx') return _live.charts.rsx;
      return _live.charts.price;
    },

    applyFullData(context, storeData, options = {}) {
      if (context !== 'live' || !_live) return;
      paintCandles(_live, storeData?.candles || []);
    },

    applyDelta(context, delta) {
      if (context !== 'live' || !_live || !delta?.candle) return;
      const barCount = Number.isFinite(delta.barCount) ? delta.barCount : 0;
      if (barCount <= 1) {
        const candles = delta.candle ? [delta.candle] : [];
        paintCandles(_live, candles);
        return;
      }
      updateCandle(_live, delta.candle);
    },

    setLiveUpdating(flag) {
      _liveUpdating = !!flag;
    },

    getVisibleLogicalRange(context) {
      if (context !== 'live' || !_live?.charts?.price) return null;
      return _live.charts.price.timeScale().getVisibleLogicalRange();
    },

    /**
     * Shift visible logical range after history prepend (native camera hold).
     * @param {string} context
     * @param {number} addedBars bars inserted on the left
     * @param {{ from: number, to: number }|null} [baseRange] pre-setData range (preferred)
     */
    shiftCamera(context, addedBars, baseRange = null) {
      if (context !== 'live' || !_live?.charts?.price) return;
      const n = Number(addedBars);
      if (!Number.isFinite(n) || n <= 0) return;

      const timeScale = _live.charts.price.timeScale();
      const range = (baseRange && isFiniteLogicalRange(baseRange))
        ? baseRange
        : timeScale.getVisibleLogicalRange();
      if (!isFiniteLogicalRange(range)) return;

      this.setVisibleLogicalRange(context, {
        from: range.from + n,
        to: range.to + n,
      }, { animate: false });
    },

    setVisibleLogicalRange(context, range, options = {}) {
      if (context !== 'live' || !_live || !isFiniteLogicalRange(range)) return;
      _live._syncingTimeScale = true;
      try {
        const charts = [_live.charts.price, _live.charts.wozduh, _live.charts.rsx];
        charts.forEach((chart) => {
          if (!chart) return;
          chart.timeScale().setVisibleLogicalRange(range, { animate: false });
        });
      } finally {
        _live._syncingTimeScale = false;
      }
    },

    isInitialized(context) {
      return context === 'live' && !!_live?.charts?.price;
    },
  };

  window.ChartAdapter = ChartAdapter;
})();
