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

  /** LWC TickMarkType ids (fallback when library not loaded — e.g. Node source tests). */
  function tickMarkTypes() {
    if (typeof LightweightCharts !== 'undefined' && LightweightCharts.TickMarkType) {
      return LightweightCharts.TickMarkType;
    }
    return { Year: 0, Month: 1, DayOfMonth: 2, Time: 3, TimeWithSeconds: 4 };
  }

  /**
   * Debt #91 — local-TZ formatters (display only).
   * Crosshair: detailed datetime. Axis ticks: minimal by TickMarkType (not currentTf).
   */
  function chartTimeFormatBundle() {
    if (_timeFormatBundle) return _timeFormatBundle;

    // Chart chrome is always English (TV-style), independent of browser UI locale.
    // Timezone remains the browser's local zone — only month/day names are fixed.
    const locale = 'en-US';
    const timeZone = (typeof Intl !== 'undefined' && Intl.DateTimeFormat)
      ? Intl.DateTimeFormat().resolvedOptions().timeZone
      : undefined;

    const dtfOpts = timeZone ? { timeZone } : {};
    // Crosshair label — TV-style detailed (e.g. "24 Jul 2026, 21:10").
    const dtfCrosshair = new Intl.DateTimeFormat(locale, {
      day: '2-digit',
      month: 'short',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      hour12: false,
      ...dtfOpts,
    });
    // Axis / date-only helpers (minimal).
    const dtfYear = new Intl.DateTimeFormat(locale, { year: 'numeric', ...dtfOpts });
    const dtfMonth = new Intl.DateTimeFormat(locale, {
      month: 'short',
      year: '2-digit',
      ...dtfOpts,
    });
    const dtfDay = new Intl.DateTimeFormat(locale, {
      day: '2-digit',
      month: 'short',
      ...dtfOpts,
    });
    const dtfHm = new Intl.DateTimeFormat(locale, {
      hour: '2-digit',
      minute: '2-digit',
      hour12: false,
      ...dtfOpts,
    });
    const dtfHms = new Intl.DateTimeFormat(locale, {
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

    const formatCrosshairTime = (time) => dtfCrosshair.format(unixChartTimeToDate(time));
    const formatDate = (time) => dtfDate.format(unixChartTimeToDate(time));

    const T = tickMarkTypes();
    const tickMarkFormatter = (time, tickMarkType) => {
      const d = unixChartTimeToDate(time);
      switch (tickMarkType) {
        case T.Year:
          return dtfYear.format(d);
        case T.Month:
          return dtfMonth.format(d);
        case T.DayOfMonth:
          return dtfDay.format(d);
        case T.TimeWithSeconds:
          return dtfHms.format(d);
        case T.Time:
        default:
          return dtfHm.format(d);
      }
    };

    _timeFormatBundle = {
      locale,
      timeFormatter: formatCrosshairTime,
      dateFormatter: formatDate,
      tickMarkFormatter,
    };
    return _timeFormatBundle;
  }

  function chartLocalizationOptions() {
    const { locale, timeFormatter, dateFormatter } = chartTimeFormatBundle();
    return { locale, timeFormatter, dateFormatter };
  }

  function themeChrome() {
    if (typeof ChartTheme !== 'undefined') {
      return {
        bg: ChartTheme.bg,
        grid: ChartTheme.grid,
        border: ChartTheme.border,
        text: ChartTheme.text,
        textBright: ChartTheme.textBright,
      };
    }
    const tv = typeof TV !== 'undefined' ? TV : {};
    return {
      bg: tv.bg || '#131722',
      grid: tv.grid || '#1e222d',
      border: tv.border || '#2A2E39',
      text: tv.text || '#787b86',
      textBright: '#D1D4DC',
    };
  }

  function layoutOptions() {
    const t = themeChrome();
    return {
      background: { color: t.bg },
      textColor: t.text,
      fontSize: 11,
      attributionLogo: false,
    };
  }

  function gridOptions(horzVisible = true) {
    const t = themeChrome();
    return {
      vertLines: { color: t.grid, style: LightweightCharts.LineStyle.Dotted },
      horzLines: horzVisible
        ? { color: t.grid, style: LightweightCharts.LineStyle.Dotted }
        : { visible: false },
    };
  }

  function timeScaleOptions() {
    const base = typeof SHARED_TIME_SCALE !== 'undefined'
      ? { ...SHARED_TIME_SCALE }
      : { borderColor: '#2a2e39', timeVisible: true, secondsVisible: false };
    return base;
  }

  /**
   * Shared vert-line chrome (Debt #91). Must be re-applied in applyHorzVisibility
   * so peer sync cannot wipe label contrast.
   */
  function vertLineChrome(extra = {}) {
    const t = themeChrome();
    const dashed = (typeof LightweightCharts !== 'undefined')
      ? LightweightCharts.LineStyle.Dashed
      : 2;
    return {
      width: 1,
      style: dashed,
      labelVisible: true,
      labelBackgroundColor: t.border || '#2A2E39',
      labelTextColor: t.textBright || '#D1D4DC',
      ...extra,
    };
  }

  /** Price-scale crosshair label contrast (same tokens as vert). */
  function horzLineChrome(extra = {}) {
    const t = themeChrome();
    const dashed = (typeof LightweightCharts !== 'undefined')
      ? LightweightCharts.LineStyle.Dashed
      : 2;
    return {
      width: 1,
      style: dashed,
      labelVisible: true,
      labelBackgroundColor: t.border || '#2A2E39',
      labelTextColor: t.textBright || '#D1D4DC',
      ...extra,
    };
  }

  function crosshairOptions() {
    const base = typeof SHARED_CROSSHAIR !== 'undefined' ? { ...SHARED_CROSSHAIR } : {};
    return {
      ...base,
      mode: 0, // CrosshairMode.Normal — free float with mouse (no Magnet)
      vertLine: { ...(base.vertLine || {}), ...vertLineChrome() },
      horzLine: { ...(base.horzLine || {}), ...horzLineChrome() },
    };
  }

  function priceScaleOptions(hostId, extra = {}) {
    const t = themeChrome();
    const id = hostId || 'price';
    const prefs = typeof ScaleController !== 'undefined'
      ? ScaleController.getState('live', id)
      : { isAuto: true, isLog: false };
    return {
      borderColor: t.border,
      autoScale: !!prefs.isAuto,
      minimumWidth: PRICE_SCALE_MIN,
      alignLabels: true,
      borderVisible: true,
      ...extra,
    };
  }

  /**
   * Time-scale chrome for one pane.
   * @param {boolean} showAxisLabels labels + visible strip (bottom owner only)
   */
  function unifiedTimeScaleOptions(showAxisLabels) {
    const base = timeScaleOptions();
    const { tickMarkFormatter } = chartTimeFormatBundle();
    const show = !!showAxisLabels;
    const opts = {
      ...base,
      visible: show,
      timeVisible: show,
      secondsVisible: false,
    };
    // Non-owner: hide the strip entirely (ADR-023 — no reserved blank axis height).
    // Owner: local-TZ formatter (LWC default is UTC components → 8h skew in UTC+8).
    if (!show) {
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
      // ADR-023: default no bottom axis; LayoutController → setBottomTimeAxis picks the owner.
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

  /**
   * ADR-023: mirror PaneLayout bottom-axis owner into LWC timeScale.visible.
   * ChartAdapter only applies; does not decide ownership.
   * @param {string} ownerHostId from PaneLayout.getBottomTimeAxisHostId()
   */
  function setBottomTimeAxis(ownerHostId) {
    if (!_live?.charts) return;
    const owner = String(ownerHostId || 'price').trim() || 'price';
    const panes = [
      { hostId: 'price', chart: _live.charts.price },
      { hostId: 'wozduh', chart: _live.charts.wozduh },
      { hostId: 'rsx', chart: _live.charts.rsx },
    ];
    for (const { hostId, chart } of panes) {
      if (!chart || typeof chart.timeScale !== 'function') continue;
      const show = hostId === owner;
      try {
        chart.timeScale().applyOptions(unifiedTimeScaleOptions(show));
      } catch {
        /* disposed */
      }
    }
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
    // ADR-025: keep overlay pinned to business times after camera move.
    refreshRulerOverlay();
  }

  function bindTimeCamera() {
    if (typeof TimeCamera === 'undefined') return;
    TimeCamera.bind({
      applyCommitted: applyCommittedCamera,
      shouldSkip: () => _liveUpdating,
    });
  }

  /**
   * ADR-021/024: every pane proposes via InteractionController → TimeCamera.
   * Y-scale gestures do not emit visible-logical-range changes (LWC).
   */
  function subscribePaneTimeProposals(state) {
    if (typeof InteractionController === 'undefined' || !state?.charts) return;
    const panes = [
      { hostId: 'price', chart: state.charts.price },
      { hostId: 'wozduh', chart: state.charts.wozduh },
      { hostId: 'rsx', chart: state.charts.rsx },
    ];
    panes.forEach(({ hostId, chart }) => {
      if (!chart?.timeScale) return;
      chart.timeScale().subscribeVisibleLogicalRangeChange((range) => {
        if (!range || _liveUpdating) return;
        if (!isFiniteLogicalRange(range)) return;
        let barSpacing = null;
        try {
          const s = chart.timeScale().options()?.barSpacing;
          if (Number.isFinite(s) && s > 0) barSpacing = s;
        } catch { /* */ }
        InteractionController.onRangeChanged(hostId, range, barSpacing);
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
    Object.keys(map).forEach((hostId) => {
      const chart = chartForHostId(state, hostId);
      if (!chart?.applyOptions) return;
      const visible = !!map[hostId];
      try {
        chart.applyOptions({
          crosshair: {
            // Re-assert vert chrome so peer sync cannot wipe Debt #91 label colors.
            vertLine: vertLineChrome(),
            horzLine: horzLineChrome({
              visible,
              labelVisible: visible,
            }),
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
   * DOM leave resolution stays here; InteractionController only receives the decision.
   */
  function bindPointerHoverOwnership(state, disposers) {
    if (typeof InteractionController === 'undefined' || typeof document === 'undefined') return;
    const root = document.getElementById('live-chart-container')
      || document.querySelector('.pro-chart-root');
    if (!root) return;

    const onEnter = (e) => {
      const wrap = e.currentTarget;
      const hostId = wrap?.dataset?.paneHost;
      if (!hostId) return;
      InteractionController.onPointerEnter(hostId);
    };
    const onLeave = (e) => {
      const related = e.relatedTarget;
      if (isInsidePaneWrap(related)) return;
      InteractionController.onPointerLeave();
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

    // LWC observational only: extract time → InteractionController (never setHovered here).
    const panes = [
      { hostId: 'price', chart: state.charts.price },
      { hostId: 'wozduh', chart: state.charts.wozduh },
      { hostId: 'rsx', chart: state.charts.rsx },
    ];
    panes.forEach(({ hostId, chart }) => {
      if (!chart?.subscribeCrosshairMove) return;
      chart.subscribeCrosshairMove((param) => {
        if (!param) return;
        if (param.point == null) return;
        if (Object.prototype.hasOwnProperty.call(param, 'sourceEvent') && !param.sourceEvent) {
          return;
        }
        if (typeof InteractionController === 'undefined') return;
        InteractionController.onCrosshairMove(hostId, param.time == null ? null : param.time);
      });
    });
  }

  // ─── ADR-025 Ruler (translate + project + tooltip DOM only) ───────────────

  function ensureRulerDom() {
    const wrap = document.getElementById('price-wrap');
    if (!wrap) return null;
    let shade = document.getElementById('ruler-shade');
    if (!shade) {
      shade = document.createElement('div');
      shade.id = 'ruler-shade';
      shade.className = 'ruler-shade';
      wrap.appendChild(shade);
    }
    let tip = document.getElementById('ruler-tooltip');
    if (!tip) {
      tip = document.createElement('div');
      tip.id = 'ruler-tooltip';
      tip.className = 'ruler-tooltip';
      wrap.appendChild(tip);
    }
    // Remove legacy infinite guides if present (ADR-025 finite rectangle only).
    const guides = document.getElementById('ruler-guides');
    if (guides) guides.remove();
    return { wrap, shade, tip };
  }

  function rulerIntervalMs() {
    const tf = (typeof window !== 'undefined' && window.currentTf) ? window.currentTf : '1m';
    if (typeof getIntervalMs === 'function') {
      const ms = Number(getIntervalMs(tf));
      if (Number.isFinite(ms) && ms > 0) return ms;
    }
    if (typeof TimeNormalizer !== 'undefined' && TimeNormalizer.getIntervalMs) {
      const ms = Number(TimeNormalizer.getIntervalMs(tf));
      if (Number.isFinite(ms) && ms > 0) return ms;
    }
    return 60_000;
  }

  function rulerMinMove() {
    try {
      const fmt = _live?.candleSeries?.options?.()?.priceFormat;
      const mm = Number(fmt?.minMove);
      if (Number.isFinite(mm) && mm > 0) return mm;
    } catch { /* */ }
    return 0.1;
  }

  /**
   * Project semantic anchors → finite rectangle + tooltip.
   * Uses logicalToCoordinate / priceToCoordinate every frame (pan/zoom safe).
   * @param {{ hostId: string, anchorA: object, anchorB: object, preview?: boolean }|null} geo
   */
  function renderRuler(geo) {
    const dom = ensureRulerDom();
    if (!dom) return;
    const { wrap, shade, tip } = dom;
    if (!geo || geo.hostId !== 'price' || !_live?.charts?.price || !_live?.candleSeries) {
      shade.style.display = 'none';
      tip.style.display = 'none';
      return;
    }
    const chart = _live.charts.price;
    const series = _live.candleSeries;
    const a = geo.anchorA;
    const b = geo.anchorB;
    let x1;
    let x2;
    let y1;
    let y2;
    try {
      const ts = chart.timeScale();
      x1 = typeof ts.logicalToCoordinate === 'function'
        ? ts.logicalToCoordinate(a.logical)
        : null;
      x2 = typeof ts.logicalToCoordinate === 'function'
        ? ts.logicalToCoordinate(b.logical)
        : null;
      y1 = series.priceToCoordinate(a.price);
      y2 = series.priceToCoordinate(b.price);
    } catch {
      shade.style.display = 'none';
      tip.style.display = 'none';
      return;
    }
    if (x1 == null || x2 == null || y1 == null || y2 == null) {
      shade.style.display = 'none';
      tip.style.display = 'none';
      return;
    }

    const left = Math.min(x1, x2);
    const top = Math.min(y1, y2);
    const width = Math.max(Math.abs(x2 - x1), 2);
    const height = Math.max(Math.abs(y2 - y1), 2);

    shade.style.display = 'block';
    shade.style.left = `${left}px`;
    shade.style.top = `${top}px`;
    shade.style.width = `${width}px`;
    shade.style.height = `${height}px`;

    if (typeof RulerMetrics === 'undefined') {
      tip.style.display = 'none';
      return;
    }
    const metrics = RulerMetrics.compute(a, b, {
      intervalMs: rulerIntervalMs(),
      minMove: rulerMinMove(),
    });
    const lines = RulerMetrics.tooltipLines(metrics);
    tip.innerHTML = `<div>${lines.line1}</div><div>${lines.line2}</div>`;
    tip.style.display = 'block';

    const tipW = tip.offsetWidth || 140;
    const tipH = tip.offsetHeight || 40;
    // Centered directly below the finite selection box.
    let tipLeft = left + width / 2 - tipW / 2;
    let tipTop = top + height + 8;
    tipLeft = Math.max(4, Math.min(tipLeft, wrap.clientWidth - tipW - 4));
    if (tipTop + tipH > wrap.clientHeight - 4) {
      tipTop = Math.max(4, top - tipH - 8);
    }
    tip.style.left = `${tipLeft}px`;
    tip.style.top = `${Math.max(4, tipTop)}px`;
  }

  function refreshRulerOverlay() {
    if (typeof RulerController === 'undefined') return;
    renderRuler(RulerController.getGeometry());
  }

  /**
   * Viewport coords → semantic anchor. time optional (empty/future space OK).
   * @returns {{ logical: number, price: number, time: *|null }|null}
   */
  function logicalPointFromClient(hostId, clientX, clientY) {
    if (hostId !== 'price' || !_live?.charts?.price || !_live?.candleSeries) return null;
    const host = document.getElementById('price-chart');
    if (!host) return null;
    const rect = host.getBoundingClientRect();
    const x = clientX - rect.left;
    const y = clientY - rect.top;
    // Allow slight out-of-bounds for empty future strip at the edge.
    if (x < -2 || y < -2 || x > rect.width + 2 || y > rect.height + 2) return null;
    if (typeof ScaleController !== 'undefined'
      && ScaleController.isPointerOnPriceScale
      && ScaleController.isPointerOnPriceScale(host, _live.charts.price, clientX)) {
      return null;
    }
    let logical;
    let price;
    let time = null;
    try {
      const ts = _live.charts.price.timeScale();
      logical = ts.coordinateToLogical(x);
      price = _live.candleSeries.coordinateToPrice(y);
      try {
        time = ts.coordinateToTime(x);
      } catch {
        time = null;
      }
    } catch {
      return null;
    }
    if (!Number.isFinite(logical) || price == null || !Number.isFinite(price)) return null;
    return { logical, price, time: time == null ? null : time };
  }

  function setRulerCursor(active) {
    const wrap = document.getElementById('price-wrap');
    if (wrap) wrap.classList.toggle('ruler-armed', !!active);
    const host = document.getElementById('price-chart');
    if (host) host.style.cursor = active ? 'crosshair' : '';
    if (typeof ToolbarController !== 'undefined' && ToolbarController.setRulerActive) {
      ToolbarController.setRulerActive(!!active);
    }
  }

  /**
   * Two-click routing: down places A or B; move previews; up ignored for finish.
   */
  function bindRulerPointerRouting(state, disposers) {
    if (typeof InteractionController === 'undefined' || typeof document === 'undefined') return;
    const wrap = document.getElementById('price-wrap');
    if (!wrap) return;

    const onDown = (e) => {
      if (typeof RulerController === 'undefined' || !RulerController.isActive()) return;
      if (e.button === 2) {
        InteractionController.onCancel();
        e.preventDefault();
        return;
      }
      if (e.button != null && e.button !== 0) return;
      const point = logicalPointFromClient('price', e.clientX, e.clientY);
      if (!point) return;
      const handled = InteractionController.onPointerDown('price', point);
      if (!handled) return;
      e.preventDefault();
      e.stopPropagation();
    };
    const onMove = (e) => {
      if (typeof RulerController === 'undefined') return;
      if (RulerController.getState() !== 'placing') return;
      const point = logicalPointFromClient('price', e.clientX, e.clientY);
      if (!point) return;
      InteractionController.onPointerMove('price', point);
    };
    const onContext = (e) => {
      if (typeof RulerController === 'undefined' || !RulerController.isActive()) return;
      InteractionController.onCancel();
      e.preventDefault();
    };
    const onKey = (e) => {
      if (e.key !== 'Escape') return;
      if (typeof RulerController === 'undefined' || !RulerController.isActive()) return;
      // Cancel measure but keep armed (TV-like). Full disarm is toolbar toggle.
      InteractionController.onCancel();
    };

    wrap.addEventListener('pointerdown', onDown);
    wrap.addEventListener('pointermove', onMove);
    wrap.addEventListener('contextmenu', onContext);
    document.addEventListener('keydown', onKey);
    disposers.push(() => {
      wrap.removeEventListener('pointerdown', onDown);
      wrap.removeEventListener('pointermove', onMove);
      wrap.removeEventListener('contextmenu', onContext);
      document.removeEventListener('keydown', onKey);
    });
  }

  function bindRulerController(state) {
    if (typeof RulerController === 'undefined') return;
    RulerController.bind({
      render: (geo) => renderRuler(geo),
      onActiveChange: (active) => setRulerCursor(active),
    });
    bindRulerPointerRouting(state, state._disposers);
  }

  function toggleRuler() {
    if (typeof RulerController === 'undefined') return false;
    RulerController.toggle();
    setRulerCursor(RulerController.isActive());
    return RulerController.isActive();
  }

  function resetRuler() {
    if (typeof RulerController === 'undefined') return false;
    RulerController.disarm();
    setRulerCursor(false);
    return true;
  }

  function bindResize(host, chart, disposers) {
    if (!host || !chart) return;
    const ro = new ResizeObserver((entries) => {
      const rect = entries[0]?.contentRect;
      if (!rect || rect.width <= 0 || rect.height <= 0) return;
      chart.applyOptions({ width: rect.width, height: rect.height });
      refreshRulerOverlay();
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

    // Bootstrap: price owns axis until LayoutController applies PaneLayout owner.
    priceChart.timeScale().applyOptions(unifiedTimeScaleOptions(true));
    wozduhChart.timeScale().applyOptions(unifiedTimeScaleOptions(false));
    rsxChart.timeScale().applyOptions(unifiedTimeScaleOptions(false));

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
    bindRulerController(state);
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
    refreshRulerOverlay();
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
      // ADR-023: LayoutController often attaches before charts exist — re-mirror owner now.
      if (typeof LayoutController !== 'undefined' && typeof LayoutController.apply === 'function') {
        LayoutController.apply();
      } else if (typeof paneLayout !== 'undefined' && paneLayout?.getBottomTimeAxisHostId) {
        setBottomTimeAxis(paneLayout.getBottomTimeAxisHostId());
      }
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

    /** ADR-021 P3 — hover via InteractionController (no policy here). */
    setHoveredPane(hostId) {
      if (typeof InteractionController === 'undefined') return false;
      if (hostId == null || hostId === '') {
        return InteractionController.onPointerLeave();
      }
      return InteractionController.onPointerEnter(hostId);
    },

    applyCrosshairVisibility(map) {
      if (!_live) return;
      applyHorzVisibility(_live, map);
    },

    syncCrosshairTime(sourceHostId, time) {
      if (!_live) return;
      syncPeerCrosshairTime(_live, sourceHostId, time);
    },

    /**
     * ADR-023 — apply PaneLayout bottom time-axis owner (mirror only).
     * @param {string} ownerHostId
     */
    setBottomTimeAxis(ownerHostId) {
      setBottomTimeAxis(ownerHostId);
    },

    /** ADR-025 — toolbar / Escape / tab switch. */
    toggleRuler,
    resetRuler,
    setRulerCursor,
    renderRuler,

    isInitialized(context) {
      return context === 'live' && !!_live?.charts?.price;
    },
  };

  window.ChartAdapter = ChartAdapter;
})();
