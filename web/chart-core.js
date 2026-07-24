/**
 * chart-core.js — sterile live ChartAdapter facade (Project Renaissance).
 * Contract: 7 public methods only. Price pane owns candles; indicators via DDRFactory.
 *
 * Time axis: wire data stays UTC unix seconds. Axis/crosshair labels use the browser
 * local timezone (display only — never shift stored timestamps).
 */
(function () {
  'use strict';

  const PRICE_SCALE_MIN = 75;
  /** @type {{ charts: object, candleSeries: object, volumeSeries: object, _syncingCrosshair: boolean, _disposers: (() => void)[] }|null} */
  let _live = null;
  let _liveUpdating = false;

  /** @type {null|{ locale: string, timeFormatter: Function, dateFormatter: Function, tickMarkFormatter: Function }} */
  let _timeFormatBundle = null;

  function hostSize(el, fw, fh) {
    return {
      width: Math.max(el?.clientWidth || 0, fw),
      height: Math.max(el?.clientHeight || 0, fh),
    };
  }

  /**
   * Display-only: UTCTimestamp / BusinessDay → Date for Intl formatting.
   * Does not mutate series data; timestamps in ColumnarStore remain UTC.
   */
  function unixChartTimeToDate(time) {
    if (typeof time === 'object' && time !== null && 'year' in time) {
      return new Date(Date.UTC(time.year, time.month - 1, time.day));
    }
    const sec = Number(time);
    if (!Number.isFinite(sec)) return new Date(NaN);
    return new Date(sec * 1000);
  }

  /**
   * Ported from adapter.legacy chartTimeFormatBundle, with cached Intl instances.
   * LWC default tick marks use UTC components; we format in the browser local TZ.
   */
  function chartTimeFormatBundle() {
    if (_timeFormatBundle) return _timeFormatBundle;

    const locale = (typeof navigator !== 'undefined' && navigator.language) ? navigator.language : 'en-US';
    const timeZone = (typeof Intl !== 'undefined' && Intl.DateTimeFormat)
      ? Intl.DateTimeFormat().resolvedOptions().timeZone
      : undefined;

    const dtfOpts = timeZone ? { timeZone } : {};
    const dtfTime = new Intl.DateTimeFormat(locale, {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
      ...dtfOpts,
    });
    const dtfDate = new Intl.DateTimeFormat(locale, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      ...dtfOpts,
    });

    const formatTime = (time) => dtfTime.format(unixChartTimeToDate(time));
    const formatDate = (time) => dtfDate.format(unixChartTimeToDate(time));

    const tickMarkFormatter = (time, tickMarkType) => {
      if (typeof LightweightCharts !== 'undefined'
        && tickMarkType === LightweightCharts.TickMarkType.Time) {
        return formatTime(time);
      }
      return formatDate(time);
    };

    _timeFormatBundle = {
      locale,
      timeFormatter: formatTime,
      dateFormatter: formatDate,
      tickMarkFormatter,
    };
    return _timeFormatBundle;
  }

  function chartLocalizationOptions() {
    const { locale, timeFormatter, dateFormatter } = chartTimeFormatBundle();
    return { locale, timeFormatter, dateFormatter };
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

  function gridOptions(horzVisible = true) {
    const tv = typeof TV !== 'undefined' ? TV : { grid: '#1e222d' };
    return {
      vertLines: { color: tv.grid, style: LightweightCharts.LineStyle.Dotted },
      horzLines: horzVisible
        ? { color: tv.grid, style: LightweightCharts.LineStyle.Dotted }
        : { visible: false },
    };
  }

  function timeScaleOptions() {
    const base = typeof SHARED_TIME_SCALE !== 'undefined'
      ? { ...SHARED_TIME_SCALE }
      : { borderColor: '#2a2e39', timeVisible: true, secondsVisible: false };
    return base;
  }

  function crosshairOptions() {
    const base = typeof SHARED_CROSSHAIR !== 'undefined' ? { ...SHARED_CROSSHAIR } : {};
    // Short dashes (LineStyle.Dashed) — not Dotted circles. Same V+H on every pane.
    const dashed = (typeof LightweightCharts !== 'undefined')
      ? LightweightCharts.LineStyle.Dashed
      : 2;
    const line = {
      width: 1,
      style: dashed,
    };
    return {
      ...base,
      mode: 0, // CrosshairMode.Normal — free float with mouse (no Magnet)
      vertLine: { ...(base.vertLine || {}), ...line },
      horzLine: { ...(base.horzLine || {}), ...line },
    };
  }

  function priceScaleOptions(hostId, extra = {}) {
    const tv = typeof TV !== 'undefined' ? TV : { border: '#2a2e39' };
    const id = hostId || 'price';
    const prefs = typeof ScaleController !== 'undefined'
      ? ScaleController.getState('live', id)
      : { isAuto: true, isLog: false };
    return {
      borderColor: tv.border,
      autoScale: !!prefs.isAuto,
      minimumWidth: PRICE_SCALE_MIN,
      alignLabels: true,
      borderVisible: true,
      ...extra,
    };
  }

  function unifiedTimeScaleOptions(showAxisLabels) {
    const base = timeScaleOptions();
    const { tickMarkFormatter } = chartTimeFormatBundle();
    const opts = {
      ...base,
      timeVisible: showAxisLabels,
      secondsVisible: false,
    };
    // Keep axis/grid geometry; hide tick labels on slave panes only.
    // Price pane: local-TZ formatter (LWC default is UTC components → 8h skew in UTC+8).
    if (!showAxisLabels) {
      opts.tickMarkFormatter = () => '';
    } else {
      opts.tickMarkFormatter = tickMarkFormatter;
    }
    return opts;
  }

  function createPaneChart(host, width, height, showAxisLabels, hostId = 'price') {
    return LightweightCharts.createChart(host, {
      autoSize: false,
      layout: layoutOptions(),
      localization: chartLocalizationOptions(),
      grid: gridOptions(),
      crosshair: crosshairOptions(),
      timeScale: unifiedTimeScaleOptions(showAxisLabels),
      width,
      height,
      rightPriceScale: priceScaleOptions(hostId, {
        mode: LightweightCharts.PriceScaleMode.Normal,
        scaleMargins: showAxisLabels
          ? undefined
          : { top: 0.05, bottom: 0.05 },
      }),
    });
  }

  function createPriceChart(host, width, height) {
    const prefs = typeof ScaleController !== 'undefined'
      ? ScaleController.getState('live', 'price')
      : { isAuto: true, isLog: false };
    const mode = (typeof LightweightCharts !== 'undefined' && prefs.isLog)
      ? LightweightCharts.PriceScaleMode.Logarithmic
      : LightweightCharts.PriceScaleMode.Normal;
    const chart = createPaneChart(host, width, height, true, 'price');
    chart.applyOptions({
      rightPriceScale: priceScaleOptions('price', { mode }),
    });
    return chart;
  }

  function createSlaveChart(host, width, height, hostId) {
    const id = hostId || 'wozduh';
    return LightweightCharts.createChart(host, {
      autoSize: false,
      layout: layoutOptions(),
      localization: chartLocalizationOptions(),
      grid: gridOptions(false),
      crosshair: {
        ...crosshairOptions(),
        // Default peer state; CrosshairController enables horz only while hovered.
        horzLine: { visible: false, labelVisible: false },
      },
      timeScale: unifiedTimeScaleOptions(false),
      width,
      height,
      // ADR-021 P1: footer is a full time-input surface; TimeCamera owns canonical sync.
      handleScroll: {
        mouseWheel: true,
        pressedMouseMove: true,
        horzTouchDrag: true,
        vertTouchDrag: false,
      },
      handleScale: {
        mouseWheel: true,
        axisPressedMouseMove: { price: true, time: false },
        axisDoubleClickReset: { price: true, time: false },
      },
      rightPriceScale: priceScaleOptions(id, {
        mode: LightweightCharts.PriceScaleMode.Normal,
        scaleMargins: { top: 0.05, bottom: 0.05 },
      }),
    });
  }

  function isFiniteLogicalRange(range) {
    if (typeof TimeCamera !== 'undefined' && TimeCamera.isFiniteLogicalRange) {
      return TimeCamera.isFiniteLogicalRange(range);
    }
    return range
      && Number.isFinite(range.from)
      && Number.isFinite(range.to)
      && range.to > range.from;
  }

  /** ChartAdapter-only LWC apply of a TimeCamera-committed snapshot. */
  function applyCommittedCamera(state) {
    if (!_live?.charts || !state) return;
    const charts = [_live.charts.price, _live.charts.wozduh, _live.charts.rsx].filter(Boolean);
    const tsOpts = {};
    if (Number.isFinite(state.barSpacing) && state.barSpacing > 0) {
      tsOpts.barSpacing = state.barSpacing;
    }
    if (Number.isFinite(state.rightOffset)) {
      tsOpts.rightOffset = state.rightOffset;
    }
    if (Object.keys(tsOpts).length) {
      charts.forEach((chart) => {
        try { chart.timeScale().applyOptions(tsOpts); } catch { /* */ }
      });
    }
    if (isFiniteLogicalRange(state.visibleRange)) {
      const range = { from: state.visibleRange.from, to: state.visibleRange.to };
      charts.forEach((chart) => {
        try {
          chart.timeScale().setVisibleLogicalRange(range, { animate: false });
        } catch { /* */ }
      });
    }
  }

  function bindTimeCamera() {
    if (typeof TimeCamera === 'undefined') return;
    TimeCamera.bind({
      applyCommitted: applyCommittedCamera,
      shouldSkip: () => _liveUpdating,
    });
  }

  /**
   * ADR-021: every pane proposes; TimeCamera commits; apply under echo lock.
   * Y-scale gestures do not emit visible-logical-range changes (LWC).
   */
  function subscribePaneTimeProposals(state) {
    if (typeof TimeCamera === 'undefined' || !state?.charts) return;
    const panes = [
      { hostId: 'price', chart: state.charts.price },
      { hostId: 'wozduh', chart: state.charts.wozduh },
      { hostId: 'rsx', chart: state.charts.rsx },
    ];
    panes.forEach(({ hostId, chart }) => {
      if (!chart?.timeScale) return;
      chart.timeScale().subscribeVisibleLogicalRangeChange((range) => {
        if (!range || TimeCamera.isSyncing() || _liveUpdating) return;
        if (!isFiniteLogicalRange(range)) return;
        let barSpacing = null;
        try {
          const s = chart.timeScale().options()?.barSpacing;
          if (Number.isFinite(s) && s > 0) barSpacing = s;
        } catch { /* */ }
        TimeCamera.proposeFromPane(hostId, range, barSpacing);
      });
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
   * Local Y at business time for a target pane (never source/foreign Y).
   */
  function resolveLocalYAtTime(state, targetChart, targetSeries, time) {
    if (time == null || !targetChart || !targetSeries) return null;
    const anchorId = crosshairAnchorId(state, targetChart);
    if (anchorId) return hydratedValueAtTime(anchorId, time);
    if (targetChart === state.charts.price) return candleCloseAtTime(time);
    return null;
  }

  function chartForHostId(state, hostId) {
    if (!state?.charts) return null;
    if (hostId === 'price') return state.charts.price;
    if (hostId === 'wozduh') return state.charts.wozduh;
    if (hostId === 'rsx') return state.charts.rsx;
    return null;
  }

  function applyHorzVisibility(state, map) {
    if (!state?.charts || !map) return;
    const dashed = (typeof LightweightCharts !== 'undefined')
      ? LightweightCharts.LineStyle.Dashed
      : 2;
    Object.keys(map).forEach((hostId) => {
      const chart = chartForHostId(state, hostId);
      if (!chart?.applyOptions) return;
      const visible = !!map[hostId];
      try {
        chart.applyOptions({
          crosshair: {
            horzLine: {
              visible,
              labelVisible: visible,
              width: 1,
              style: dashed,
            },
            vertLine: {
              width: 1,
              style: dashed,
            },
          },
        });
      } catch { /* */ }
    });
  }

  /**
   * Sync vertical crosshair time to peers. Each peer uses its own local Y — never source Y.
   * Does not touch the hovered/source pane (native LWC crosshair).
   */
  function syncPeerCrosshairTime(state, sourceHostId, time) {
    if (!state?.charts || time == null) return;
    const panes = [
      { hostId: 'price', chart: state.charts.price },
      { hostId: 'wozduh', chart: state.charts.wozduh },
      { hostId: 'rsx', chart: state.charts.rsx },
    ];
    panes.forEach(({ hostId, chart }) => {
      if (!chart || hostId === sourceHostId) return;
      if (typeof chart.setCrosshairPosition !== 'function') return;
      const targetSeries = crosshairSeriesForChart(state, chart);
      if (!targetSeries) {
        chart.clearCrosshairPosition?.();
        return;
      }
      const yValue = resolveLocalYAtTime(state, chart, targetSeries, time);
      if (yValue == null || !Number.isFinite(yValue)) {
        chart.clearCrosshairPosition?.();
        return;
      }
      try {
        chart.setCrosshairPosition(yValue, time, targetSeries);
      } catch { /* */ }
    });
  }

  function clearPeerCrosshairs(state, sourceHostId) {
    if (!state?.charts) return;
    ['price', 'wozduh', 'rsx'].forEach((hostId) => {
      if (sourceHostId != null && hostId === sourceHostId) return;
      const chart = chartForHostId(state, hostId);
      try { chart?.clearCrosshairPosition?.(); } catch { /* */ }
    });
  }

  function isInsidePaneWrap(node) {
    if (!node || typeof node.closest !== 'function') return false;
    return !!node.closest('.chart-wrap[data-pane-host]');
  }

  /**
   * Authoritative hover: PaneLayout wrappers only (never .lwc-host internals).
   */
  function bindPointerHoverOwnership(state, disposers) {
    if (typeof CrosshairController === 'undefined' || typeof document === 'undefined') return;
    const root = document.getElementById('live-chart-container')
      || document.querySelector('.pro-chart-root');
    if (!root) return;

    const onEnter = (e) => {
      const wrap = e.currentTarget;
      const hostId = wrap?.dataset?.paneHost;
      if (!hostId) return;
      CrosshairController.setHovered(hostId);
    };
    const onLeave = (e) => {
      const related = e.relatedTarget;
      if (isInsidePaneWrap(related)) return;
      CrosshairController.setHovered(null);
    };

    root.querySelectorAll('.chart-wrap[data-pane-host]').forEach((wrap) => {
      wrap.addEventListener('pointerenter', onEnter);
      wrap.addEventListener('pointerleave', onLeave);
      disposers.push(() => {
        wrap.removeEventListener('pointerenter', onEnter);
        wrap.removeEventListener('pointerleave', onLeave);
      });
    });
  }

  function bindCrosshairController(state) {
    if (typeof CrosshairController === 'undefined' || !state?.charts) return;

    CrosshairController.bind({
      applyHorzVisibility: (map) => applyHorzVisibility(state, map),
      syncPeerTime: (sourceHostId, time) => {
        syncPeerCrosshairTime(state, sourceHostId, time);
      },
      clearPeerCrosshairs: (sourceHostId) => clearPeerCrosshairs(state, sourceHostId),
      shouldIgnoreTimeSync: () => {
        if (_liveUpdating) return true;
        if (typeof TimeCamera !== 'undefined') {
          if (TimeCamera.isGesturing?.() || TimeCamera.isSyncing?.()) return true;
        }
        return false;
      },
    });

    bindPointerHoverOwnership(state, state._disposers);

    // LWC observational only: time signal → never setHovered.
    const panes = [
      { hostId: 'price', chart: state.charts.price },
      { hostId: 'wozduh', chart: state.charts.wozduh },
      { hostId: 'rsx', chart: state.charts.rsx },
    ];
    panes.forEach(({ hostId, chart }) => {
      if (!chart?.subscribeCrosshairMove) return;
      chart.subscribeCrosshairMove((param) => {
        if (!param) return;
        // Optional: ignore synthetic / non-pointer moves (not ownership — sync filter only).
        if (param.point == null) return;
        if (Object.prototype.hasOwnProperty.call(param, 'sourceEvent') && !param.sourceEvent) {
          return;
        }
        const hovered = CrosshairController.getHovered();
        if (!hovered || hostId !== hovered) return;
        if (param.time == null) {
          CrosshairController.syncTime({ sourceHostId: hostId, time: null });
          return;
        }
        CrosshairController.syncTime({ sourceHostId: hostId, time: param.time });
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
    const wozduhChart = createSlaveChart(wozHost, wozSize.width, wozSize.height, 'wozduh');
    const rsxChart = createSlaveChart(rsxHost, rsxSize.width, rsxSize.height, 'rsx');

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
    const prefs = typeof ScaleController !== 'undefined'
      ? ScaleController.getState('live', 'price')
      : { isAuto: true, isLog: false };
    const priceMode = (typeof LightweightCharts !== 'undefined' && prefs.isLog)
      ? LightweightCharts.PriceScaleMode.Logarithmic
      : LightweightCharts.PriceScaleMode.Normal;
    priceChart.priceScale('right').applyOptions({
      ...priceScaleOptions('price', { mode: priceMode }),
      scaleMargins: priceMargins,
    });
    priceChart.priceScale('volume').applyOptions({
      scaleMargins: volumeMargins,
      autoScale: true,
      visible: true,
    });

    if (typeof ScaleController !== 'undefined') {
      ScaleController.register({
        context: 'live',
        hostId: 'price',
        chart: priceChart,
        host: priceHost,
        allowLog: true,
      });
      ScaleController.register({
        context: 'live',
        hostId: 'wozduh',
        chart: wozduhChart,
        host: wozHost,
        allowLog: false,
      });
      ScaleController.register({
        context: 'live',
        hostId: 'rsx',
        chart: rsxChart,
        host: rsxHost,
        allowLog: false,
      });
    }
    const state = {
      charts: { price: priceChart, wozduh: wozduhChart, rsx: rsxChart },
      candleSeries,
      volumeSeries,
      _disposers: [],
    };

    bindTimeCamera();
    bindResize(priceHost, priceChart, state._disposers);
    bindResize(wozHost, wozduhChart, state._disposers);
    bindResize(rsxHost, rsxChart, state._disposers);
    subscribePaneTimeProposals(state);
    bindCrosshairController(state);
    return state;
  }

  function paintCandles(state, candles) {
    if (!state?.candleSeries || !Array.isArray(candles) || !candles.length) return;
    state.candleSeries.setData(candles);
    if (state.volumeSeries && typeof toVolumeBars === 'function') {
      state.volumeSeries.setData(toVolumeBars(candles));
    }
    // Shot 11D-HOTFIX: re-arm Auto after setData so LWC recomputes Y on real bars, not empty canvas.
    if (typeof ScaleController !== 'undefined' && typeof ScaleController.applyAll === 'function') {
      ScaleController.applyAll();
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
      if (context !== 'live' || !_live || !delta) return;
      // Shot 11E: compositor may pass a boundary chain (close tip → open new bar).
      if (Array.isArray(delta)) {
        for (let i = 0; i < delta.length; i++) {
          ChartAdapter.applyDelta(context, delta[i]);
        }
        return;
      }
      if (!delta.candle) return;
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

    setVisibleLogicalRange(context, range, options = {}) {
      if (context !== 'live' || !_live || !isFiniteLogicalRange(range)) return;
      // Debt #80: 0×0 host → LWC NaN scale (blank chart). Caller must use fresh camera.
      const host = typeof document !== 'undefined'
        ? document.getElementById('price-chart')
        : null;
      if (host && (host.clientWidth <= 0 || host.clientHeight <= 0)) return;
      if (typeof TimeCamera === 'undefined') {
        applyCommittedCamera({ visibleRange: range, barSpacing: null, rightOffset: null });
        return;
      }
      TimeCamera.commit({
        visibleRange: range,
        sourceHostId: 'system',
      });
    },

    /** System / compositor path: spacing + rightOffset without inventing a range. */
    commitTimeCamera(patch) {
      if (typeof TimeCamera === 'undefined') {
        applyCommittedCamera({
          visibleRange: patch?.visibleRange || null,
          barSpacing: patch?.barSpacing ?? null,
          rightOffset: patch?.rightOffset ?? null,
        });
        return false;
      }
      return TimeCamera.commit({
        ...patch,
        sourceHostId: patch?.sourceHostId || 'system',
      });
    },

    /** ADR-021 P2 — thin CrosshairController surface (no policy here). */
    setHoveredPane(hostId) {
      if (typeof CrosshairController === 'undefined') return false;
      return CrosshairController.setHovered(hostId);
    },

    applyCrosshairVisibility(map) {
      if (!_live) return;
      applyHorzVisibility(_live, map);
    },

    syncCrosshairTime(sourceHostId, time) {
      if (!_live) return;
      syncPeerCrosshairTime(_live, sourceHostId, time);
    },

    isInitialized(context) {
      return context === 'live' && !!_live?.charts?.price;
    },
  };

  window.ChartAdapter = ChartAdapter;
})();
