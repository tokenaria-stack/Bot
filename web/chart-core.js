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
   * Bottom axis is the time-label *renderer* only — never the sync owner (ADR-027 polish).
   * @param {string} ownerHostId from PaneLayout.getBottomTimeAxisHostId()
   */
  function setBottomTimeAxis(ownerHostId) {
    if (!_live?.charts) return;
    const owner = String(ownerHostId || 'price').trim() || 'price';
    _live._bottomTimeAxisHostId = owner;
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
    const applyRange = () => {
      if (!isFiniteLogicalRange(state.visibleRange)) return;
      const range = { from: state.visibleRange.from, to: state.visibleRange.to };
      charts.forEach((chart) => {
        try {
          chart.timeScale().setVisibleLogicalRange(range);
        } catch { /* */ }
      });
    };
    // rangeOnly: LEFT prepend hard restore — pin logical indices, skip rightOffset/decoration
    // side-effects that can fight setData's auto-shift in the same stack.
    if (state.rangeOnly === true) {
      applyRange();
      return;
    }
    applyRange();
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
    applyRange();
    refreshRulerOverlay();
    refreshPeerCrosshair(_live);
    refreshDecorationFromState(_live);
    applyRange();
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
    if (chart === state.charts.wozduh) return factory.getSeries('woz_slow');
    if (chart === state.charts.rsx) return factory.getSeries('line_rsx');
    return null;
  }

  function crosshairAnchorId(state, chart) {
    if (chart === state.charts.wozduh) return 'woz_slow';
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

  /**
   * ADR-027: when time is on display whitespace (no series value), still place
   * native crosshair so the time label shows — use visible mid price (local only).
   */
  function resolveLocalYForCrosshair(state, targetChart, targetSeries, time) {
    const y = resolveLocalYAtTime(state, targetChart, targetSeries, time);
    if (y != null && Number.isFinite(y)) return y;
    return midVisiblePrice(targetChart);
  }

  function chartForHostId(state, hostId) {
    if (!state?.charts) return null;
    if (hostId === 'price') return state.charts.price;
    if (hostId === 'wozduh') return state.charts.wozduh;
    if (hostId === 'rsx') return state.charts.rsx;
    return null;
  }

  function midVisiblePrice(chart) {
    try {
      const ps = chart?.priceScale?.('right');
      const range = ps?.getVisibleRange?.();
      if (range && Number.isFinite(range.from) && Number.isFinite(range.to)) {
        return (range.from + range.to) / 2;
      }
    } catch { /* */ }
    return null;
  }

  /**
   * Paint LWC native crosshair (incl. time label) on one pane.
   * Market series first; decoration series fallback (future DisplayTimeline times).
   */
  function paintNativeCrosshairAtTime(state, hostId, time) {
    const chart = chartForHostId(state, hostId);
    if (!chart || time == null || typeof chart.setCrosshairPosition !== 'function') return false;
    const market = crosshairSeriesForChart(state, chart);
    let y = market ? resolveLocalYForCrosshair(state, chart, market, time) : null;
    if (market && y != null && Number.isFinite(y)) {
      try {
        chart.setCrosshairPosition(y, time, market);
        return true;
      } catch { /* */ }
    }
    if (typeof TimelineDecoration === 'undefined' || !TimelineDecoration.applyCrosshairTime) {
      return false;
    }
    if (y == null || !Number.isFinite(y)) y = midVisiblePrice(chart);
    if (y == null || !Number.isFinite(y)) y = 0;
    return TimelineDecoration.applyCrosshairTime(chart, time, y);
  }

  /**
   * Private ChartAdapter crosshair step (not a public architecture feature).
   * Bottom time label = sync semantics (CrosshairController) rendered on the
   * ADR-023 axis owner. Hovered pane must never own the label.
   */
  function applyBottomAxisLabel(state) {
    if (!state?.charts) return;
    const saved = state._peerCrosshair;
    const owner = state._bottomTimeAxisHostId;
    if (!saved || !owner) return;
    // Owner hovered → native LWC already paints its own time label.
    if (saved.sourceHostId === owner) return;
    if (saved.time == null) return;
    paintNativeCrosshairAtTime(state, owner, saved.time);
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
    // Horz policy runs after peer sync and may reset LWC crosshair chrome —
    // re-assert bottom-axis time label (renderer only; sync owner stays CrosshairController).
    applyBottomAxisLabel(state);
  }

  /**
   * ADR-026 — single peer-crosshair apply entry.
   * time present → native setCrosshairPosition (local Y only).
   * time null + logical → logical guide (no fabricated timestamps).
   * Never touches the hovered/source pane (native LWC owns it).
   * @param {object} state
   * @param {string} sourceHostId
   * @param {{ logical: number, time?: *|null }} pos
   */
  function applyPeerCrosshair(state, sourceHostId, pos) {
    if (!state?.charts || !pos) return;
    const logical = Number(pos.logical);
    if (!Number.isFinite(logical)) {
      clearPeerCrosshairs(state, sourceHostId);
      return;
    }
    const time = pos.time == null ? null : pos.time;
    state._peerCrosshair = { sourceHostId, logical, time };

    const panes = [
      { hostId: 'price', chart: state.charts.price },
      { hostId: 'wozduh', chart: state.charts.wozduh },
      { hostId: 'rsx', chart: state.charts.rsx },
    ];
    panes.forEach(({ hostId, chart }) => {
      if (!chart || hostId === sourceHostId) {
        hidePeerGuide(hostId);
        return;
      }
      if (time != null) {
        hidePeerGuide(hostId);
        if (paintNativeCrosshairAtTime(state, hostId, time)) return;
        try { chart.clearCrosshairPosition?.(); } catch { /* */ }
        showPeerGuideAtLogical(hostId, chart, logical);
        return;
      }
      // Empty space without resolvable time: logical guide only (ADR-026).
      try { chart.clearCrosshairPosition?.(); } catch { /* */ }
      showPeerGuideAtLogical(hostId, chart, logical);
    });
    applyBottomAxisLabel(state);
  }

  /** Re-project stored empty-space guides after camera / resize. */
  function refreshPeerCrosshair(state) {
    const saved = state?._peerCrosshair;
    if (!saved || !Number.isFinite(saved.logical)) return;
    if (saved.time != null) return; // native LWC peers; no guide to refresh
    applyPeerCrosshair(state, saved.sourceHostId, saved);
  }

  function clearPeerCrosshairs(state, sourceHostId) {
    if (state) state._peerCrosshair = null;
    if (!state?.charts) {
      hideAllPeerGuides();
      return;
    }
    ['price', 'wozduh', 'rsx'].forEach((hostId) => {
      if (sourceHostId != null && hostId === sourceHostId) return;
      const chart = chartForHostId(state, hostId);
      try { chart?.clearCrosshairPosition?.(); } catch { /* */ }
      hidePeerGuide(hostId);
    });
  }

  const PEER_GUIDE_HOST_EL = Object.freeze({
    price: 'price-chart',
    wozduh: 'wozduh-chart',
    rsx: 'rsx-chart',
  });

  function peerGuideColor() {
    if (typeof ChartTheme !== 'undefined' && ChartTheme.crosshair) return ChartTheme.crosshair;
    return '#555555';
  }

  function ensurePeerGuideEl(hostId) {
    const elId = PEER_GUIDE_HOST_EL[hostId];
    if (!elId || typeof document === 'undefined') return null;
    const host = document.getElementById(elId);
    if (!host) return null;
    let guide = host.querySelector(':scope > .peer-crosshair-guide');
    if (!guide) {
      guide = document.createElement('div');
      guide.className = 'peer-crosshair-guide';
      guide.setAttribute('aria-hidden', 'true');
      host.appendChild(guide);
    }
    guide.style.borderLeftColor = peerGuideColor();
    return guide;
  }

  function showPeerGuideAtLogical(hostId, chart, logical) {
    const guide = ensurePeerGuideEl(hostId);
    if (!guide || !chart?.timeScale) {
      hidePeerGuide(hostId);
      return;
    }
    let x = null;
    try {
      x = chart.timeScale().logicalToCoordinate(logical);
    } catch {
      x = null;
    }
    if (x == null || !Number.isFinite(x)) {
      guide.style.display = 'none';
      return;
    }
    guide.style.display = 'block';
    guide.style.left = `${x}px`;
  }

  function hidePeerGuide(hostId) {
    if (typeof document === 'undefined') return;
    const elId = PEER_GUIDE_HOST_EL[hostId];
    if (!elId) return;
    const host = document.getElementById(elId);
    const guide = host?.querySelector?.(':scope > .peer-crosshair-guide');
    if (guide) guide.style.display = 'none';
  }

  function hideAllPeerGuides() {
    ['price', 'wozduh', 'rsx'].forEach(hidePeerGuide);
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

  /**
   * Resolve chart time at a logical index.
   * Real bars → candle times; future logical → DisplayTimeline.buildFutureTimes
   * (same math as decoration refresh — composition, not a second math owner).
   * @param {object|null} state
   * @param {number} logical
   * @returns {*|null}
   */
  function resolveDisplayTimeAtLogical(state, logical) {
    const real = state?._realCandles;
    if (!Array.isArray(real) || !real.length) return null;
    const lastLogical = real.length - 1;
    const idx = Math.round(Number(logical));
    if (!Number.isFinite(idx)) return null;
    if (idx >= 0 && idx <= lastLogical) {
      const t = real[idx]?.time;
      return t == null ? null : t;
    }
    if (typeof DisplayTimeline === 'undefined' || !DisplayTimeline.buildFutureTimes) return null;
    const offset = idx - lastLogical;
    if (offset <= 0) {
      const t = real[lastLogical]?.time;
      return t == null ? null : t;
    }
    const lastSec = Number(real[lastLogical]?.time);
    if (!Number.isFinite(lastSec)) return null;
    const tf = (typeof window !== 'undefined' && window.currentTf) ? window.currentTf : '1m';
    const times = DisplayTimeline.buildFutureTimes({
      lastTimeSec: lastSec,
      count: offset,
      tf,
    });
    return times[offset - 1] ?? null;
  }

  /**
   * ChartAdapter translate: LWC param → { logical, time? }.
   * Never invents time — may look up DisplayTimeline times already on the decoration plane.
   */
  function extractCrosshairPosition(chart, param) {
    if (!param || param.point == null) return null;
    let time = param.time == null ? null : param.time;
    let logical = param.logical;
    if (logical == null && chart?.timeScale) {
      try {
        logical = chart.timeScale().coordinateToLogical(param.point.x);
      } catch {
        logical = null;
      }
    }
    logical = Number(logical);
    if (!Number.isFinite(logical)) return null;
    if (time == null && chart?.timeScale) {
      try {
        const ct = chart.timeScale().coordinateToTime?.(param.point.x);
        if (ct != null) time = ct;
      } catch { /* */ }
    }
    if (time == null) {
      time = resolveDisplayTimeAtLogical(_live, logical);
    }
    return { logical, time };
  }

  function bindCrosshairController(state) {
    if (typeof CrosshairController === 'undefined' || !state?.charts) return;

    CrosshairController.bind({
      applyHorzVisibility: (map) => applyHorzVisibility(state, map),
      syncPeerCrosshair: (sourceHostId, pos) => {
        applyPeerCrosshair(state, sourceHostId, pos);
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

    // LWC observational only: extract {logical,time?} → InteractionController.
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
        const pos = extractCrosshairPosition(chart, param);
        if (!pos) return;
        InteractionController.onCrosshairMove(hostId, pos);
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
      _realCandles: null,
      _lastRealCandleTime: null,
      _bottomTimeAxisHostId: 'price',
      _peerCrosshair: null,
      _disposers: [],
    };

    // ADR-027: decoration plane on every live pane (TimeCamera logical alignment + ADR-023 axis owner).
    if (typeof TimelineDecoration !== 'undefined') {
      TimelineDecoration.dispose();
      TimelineDecoration.attach(priceChart);
      TimelineDecoration.attach(wozduhChart);
      TimelineDecoration.attach(rsxChart);
      state._disposers.push(() => {
        if (typeof TimelineDecoration !== 'undefined') TimelineDecoration.dispose();
      });
    }

    if (typeof WozduhExtremeBands !== 'undefined') {
      WozduhExtremeBands.dispose();
      WozduhExtremeBands.attach(wozduhChart);
      state._disposers.push(() => {
        if (typeof WozduhExtremeBands !== 'undefined') WozduhExtremeBands.dispose();
      });
    }

    bindTimeCamera();
    bindResize(priceHost, priceChart, state._disposers);
    bindResize(wozHost, wozduhChart, state._disposers);
    bindResize(rsxHost, rsxChart, state._disposers);
    subscribePaneTimeProposals(state);
    bindCrosshairController(state);
    bindRulerController(state);
    return state;
  }

  /**
   * ADR-027 composer helper — ChartAdapter only.
   * Reads real tip + camera; DisplayTimeline owns math; TimelineDecoration owns LWC setData.
   */
  function refreshDecorationFromState(state) {
    if (typeof TimelineDecoration === 'undefined') return;
    if (typeof DisplayTimeline === 'undefined') return;
    if (!state) return;
    const real = state._realCandles;
    if (!Array.isArray(real) || !real.length) {
      TimelineDecoration.refresh({ times: [] });
      return;
    }
    const lastTimeSec = Number(real[real.length - 1]?.time);
    if (!Number.isFinite(lastTimeSec)) {
      TimelineDecoration.refresh({ times: [] });
      return;
    }

    let visibleTo = null;
    let rightOffset = null;
    try {
      const range = state.charts?.price?.timeScale?.()?.getVisibleLogicalRange?.();
      if (range && Number.isFinite(range.to)) visibleTo = range.to;
    } catch { /* */ }
    if (typeof TimeCamera !== 'undefined' && TimeCamera.getCanonical) {
      const c = TimeCamera.getCanonical();
      if (c && Number.isFinite(c.rightOffset)) rightOffset = c.rightOffset;
      if (c?.visibleRange && Number.isFinite(c.visibleRange.to)) {
        visibleTo = c.visibleRange.to;
      }
    }

    const tf = (typeof window !== 'undefined' && window.currentTf) ? window.currentTf : '1m';
    const bars = DisplayTimeline.buildWhitespaceBars({
      lastTimeSec,
      lastLogical: real.length - 1,
      visibleTo,
      rightOffset,
      tf,
    });
    const times = bars.map((b) => b.time);
    TimelineDecoration.refresh({ times });
  }

  /**
   * ADR-027 — candleSeries is real OHLC only (tip update invariant).
   * Future strip via TimelineDecoration, never concatenated onto candles.
   *
   * Paint order (no mid-paint camera):
   *   setData → volume → Y-scale prefs → (optional decoration) → done
   * Camera restore belongs to ChartCompositor AFTER this returns.
   *
   * @param {object} state
   * @param {object[]} candles
   * @param {{ skipDecoration?: boolean }} [paintOpts]
   */
  function paintCandles(state, candles, paintOpts = {}) {
    if (!state?.candleSeries || !Array.isArray(candles) || !candles.length) return;
    state._realCandles = candles;
    state._lastRealCandleTime = candles[candles.length - 1]?.time ?? null;
    state.candleSeries.setData(candles);
    if (state.volumeSeries && typeof toVolumeBars === 'function') {
      state.volumeSeries.setData(toVolumeBars(candles));
    }
    // Y-scale only (autoScale/log). Does NOT write visibleLogicalRange.
    if (typeof ScaleController !== 'undefined' && typeof ScaleController.applyAll === 'function') {
      ScaleController.applyAll();
    }
    if (typeof ToolbarController !== 'undefined') {
      ToolbarController.updateVolume(candles);
    }
    // ADR-027 future strip — skipped on prepend F1 (camera settle). Band host is not that strip.
    if (paintOpts.skipDecoration !== true) {
      refreshDecorationFromState(state);
    }
    if (typeof WozduhExtremeBands !== 'undefined' && typeof WozduhExtremeBands.refresh === 'function') {
      WozduhExtremeBands.refresh(state._lastRealCandleTime);
    }
    refreshRulerOverlay();
  }

  /**
   * PAINT-ORDER-1 belt: older than painted tip is illegal for series.update.
   * Same-time is allowed. Not a tick-bar identity law.
   */
  function isOlderThanPaintedTip(state, candle) {
    if (!state || !candle) return false;
    const t = Number(candle.time);
    const tip = Number(state._lastRealCandleTime);
    return Number.isFinite(t) && Number.isFinite(tip) && t < tip;
  }

  function updateCandle(state, candle) {
    if (!state?.candleSeries || !candle) return;
    if (isOlderThanPaintedTip(state, candle)) return;
    const isNewBar = candle.time !== state._lastRealCandleTime;
    if (Array.isArray(state._realCandles) && state._realCandles.length) {
      if (state._realCandles[state._realCandles.length - 1]?.time === candle.time) {
        state._realCandles[state._realCandles.length - 1] = candle;
      } else {
        state._realCandles.push(candle);
      }
    } else {
      state._realCandles = [candle];
    }
    state.candleSeries.update(candle);
    if (state.volumeSeries && typeof toVolumeBars === 'function') {
      state.volumeSeries.update(toVolumeBars([candle])[0]);
    }
    if (isNewBar) {
      state._lastRealCandleTime = candle.time;
      refreshDecorationFromState(state);
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
      paintCandles(_live, storeData?.candles || [], {
        skipDecoration: options.skipDecoration === true,
      });
    },

    /** ADR-027 whitespace strip — call after camera settle on LEFT prepend. */
    refreshLiveDecoration() {
      if (!_live) return;
      refreshDecorationFromState(_live);
      refreshRulerOverlay();
    },

    /**
     * Hard pin visible logical range on all live panes (no rightOffset/decoration).
     * Used by LEFT prepend restore immediately after setData.
     */
    forceVisibleLogicalRange(context, range) {
      if (context !== 'live' || !_live?.charts) return false;
      if (!isFiniteLogicalRange(range)) return false;
      const expected = { from: range.from, to: range.to };
      const charts = [_live.charts.price, _live.charts.wozduh, _live.charts.rsx].filter(Boolean);
      charts.forEach((chart) => {
        try {
          chart.timeScale().setVisibleLogicalRange(expected);
        } catch { /* */ }
      });
      if (typeof TimeCamera !== 'undefined') {
        if (typeof TimeCamera.beginPreserveTransaction === 'function') {
          TimeCamera.beginPreserveTransaction();
        }
        TimeCamera.commit({
          visibleRange: expected,
          sourceHostId: 'system',
          rangeOnly: true,
        }, {
          force: true,
        });
      }
      return true;
    },

    applyDelta(context, delta) {
      if (context !== 'live' || !_live || !delta) return false;
      // Shot 11E: compositor may pass a boundary chain (close tip → open new bar).
      if (Array.isArray(delta)) {
        let any = false;
        for (let i = 0; i < delta.length; i++) {
          if (ChartAdapter.applyDelta(context, delta[i]) !== false) any = true;
        }
        return any;
      }
      if (!delta.candle) return false;
      if (isOlderThanPaintedTip(_live, delta.candle)) return false;
      const barCount = Number.isFinite(delta.barCount) ? delta.barCount : 0;
      if (barCount <= 1) {
        const candles = delta.candle ? [delta.candle] : [];
        paintCandles(_live, candles);
        return true;
      }
      updateCandle(_live, delta.candle);
      return true;
    },

    setLiveUpdating(flag) {
      _liveUpdating = !!flag;
    },

    /** Read-only: compositor/setData in flight. Same SSOT as pane range-echo skip. */
    isLiveUpdating() {
      return _liveUpdating === true;
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

    /**
     * ADR-026 — peer crosshair apply (native time path or logical guide).
     * @param {string} sourceHostId
     * @param {{ logical: number, time?: *|null }} pos
     */
    renderPeerCrosshair(sourceHostId, pos) {
      if (!_live) return;
      applyPeerCrosshair(_live, sourceHostId, pos);
    },

    /** @deprecated ADR-026 — use renderPeerCrosshair({ logical, time? }) */
    syncCrosshairTime(_sourceHostId, _time) {
      /* Legacy time-only API cannot represent empty space — no-op. */
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
