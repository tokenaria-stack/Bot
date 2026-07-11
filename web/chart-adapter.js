/**
 * ChartAdapter — Lightweight Charts facade.
 * app.js must not call LWC API directly; use window.ChartAdapter only.
 */
const CHART_PRICE_SCALE_MIN_WIDTH = 70;
const LOGICAL_RANGE_EPS = 0.01;

function _theme() {
  return (typeof ChartTheme !== 'undefined') ? ChartTheme : null;
}

function _themeColor(token, tvFallback) {
  const T = _theme();
  if (T && T[token]) return T[token];
  return tvFallback;
}

let _live = {};
let _backtest = {};
let _equityChart;
let _equitySeries;
let _equityResizeObserver;
let _chartType = 'candles';
let _liveUpdating = false;
let _fibPriceLines = [];
let _wozduxPriceLines = [];
let _rsxPriceLines = [];
let _btNavLines = [];
let _btNavMarkers = [];
let _liveNavLines = [];
let _liveNavMarkers = [];
let _tradeMarkers = [];
let _spikeMarkers = [];
let _cachedPriceMarkers = [];
let _cachedWozduhMarkers = [];
const _SPIKE_MARKER_TEXTS = new Set(['▲', '▼']);
let _hooks = { live: {}, backtest: {} };
let _chartInitialized = false;
/** Phase 6: live oscillators painted by DDRFactory instead of legacy applyOscillatorToChart. */
let _ddrOscCutover = false;

function ddrOscCutoverActive(context = 'live') {
  return _ddrOscCutover && context === 'live';
}

function _ctxData(context) {
  return context === 'backtest' ? _backtest : _live;
}

function _chartContext(chartData) {
  if (!chartData) return 'live';
  if (chartData._context) return chartData._context;
  return chartData === _backtest ? 'backtest' : 'live';
}

function _storeTf(context = 'live') {
  if (context === 'backtest') {
    return BacktestController?.getFormValues?.().interval || '15m';
  }
  return TimeframeController?.getActiveTf?.() || '1m';
}

function _liveStore() {
  if (typeof window !== 'undefined' && window.liveColumnarStore) {
    return window.liveColumnarStore;
  }
  return (typeof liveColumnarStore !== 'undefined' && liveColumnarStore) ? liveColumnarStore : null;
}

function _liveCandles() {
  const store = _liveStore();
  if (!store?.barCount?.()) return [];
  return store.getForLightweightCharts().candles;
}

function _storeForChart(chartData) {
  return _chartContext(chartData) === 'backtest' ? backtestStore : _liveStore();
}

function _storeDataForChart(chartData) {
  return _storeForChart(chartData).getForLightweightCharts();
}

function _isSpikeMarker(marker) {
  return marker && _SPIKE_MARKER_TEXTS.has(marker.text);
}

function _rebuildPriceMarkerCache(annotationMap) {
  const showSpike = typeof ToolbarController !== 'undefined' && ToolbarController.isSpikeEnabled();
  const spikePart = showSpike ? buildSpikeMarkersFromGrid(annotationMap) : [];
  _spikeMarkers = spikePart;
  _cachedPriceMarkers = [..._tradeMarkers, ...spikePart].sort((a, b) => a.time - b.time);
}

function _rebuildWozduhMarkerCache(annotationMap) {
  _cachedWozduhMarkers = buildWozduhMarkersFromGrid(annotationMap);
}

function _patchSpikeMarkersAtMs(ms, annotation) {
  const timeSec = ChartDataStore.msToChartSec(ms);
  const showSpike = typeof ToolbarController !== 'undefined' && ToolbarController.isSpikeEnabled();

  _spikeMarkers = _spikeMarkers.filter((m) => !(m.time === timeSec && _isSpikeMarker(m)));
  _cachedPriceMarkers = _cachedPriceMarkers.filter((m) => !(m.time === timeSec && _isSpikeMarker(m)));

  if (showSpike && annotation) {
    const added = [];
    if (annotation.spikeUp) {
      added.push({
        time: timeSec,
        position: 'belowBar',
        color: _themeColor('spikeUp', TV.green),
        shape: 'circle',
        text: '▲',
      });
    }
    if (annotation.spikeDown) {
      added.push({
        time: timeSec,
        position: 'aboveBar',
        color: _themeColor('spikeDown', TV.red),
        shape: 'circle',
        text: '▼',
      });
    }
    _spikeMarkers.push(...added);
    _cachedPriceMarkers.push(...added);
    _cachedPriceMarkers.sort((a, b) => a.time - b.time);
  }
}

function _patchWozduhMarkerAtMs(ms, annotation) {
  const timeSec = ChartDataStore.msToChartSec(ms);
  _cachedWozduhMarkers = _cachedWozduhMarkers.filter((m) => m.time !== timeSec);
  if (annotation?.volCross) {
    _cachedWozduhMarkers.push({
      time: timeSec,
      position: 'inBar',
      color: annotation.volCross,
      shape: 'circle',
      size: 1,
    });
    _cachedWozduhMarkers.sort((a, b) => a.time - b.time);
  }
}

function _flushPriceMarkerCache() {
  _live.candleSeries?.setMarkers(_cachedPriceMarkers);
}

function _flushWozduhMarkerCache(chartData = _live) {
  const markerKey = INDICATOR_CONFIG.wozduh.markerSeriesKey;
  chartData.wozduxSeries?.[markerKey]?.setMarkers(_cachedWozduhMarkers);
}

function _storeForContext(context) {
  return context === 'backtest' ? backtestStore : _liveStore();
}

function _shouldPaint(context) {
  const fn = _hooks[context]?.shouldPaint;
  return typeof fn === 'function' ? fn() : true;
}

function createMTFOverlaySeries(chart, options = {}) {
  if (!chart?.addLineSeries) return null;
  const {
    color = 'transparent',
    lineWidth = 2,
    lineStyle = 0,
    ...rest
  } = options;
  return chart.addLineSeries({
    color,
    lineWidth,
    lineStyle,
    priceLineVisible: false,
    lastValueVisible: false,
    crosshairMarkerVisible: false,
    autoscaleInfoProvider: () => null,
    ...rest,
  });
}

function ensureMtfNavigatorLayers(chartData) {
  if (!chartData.mtfNavigatorLayers) {
    chartData.mtfNavigatorLayers = { price: {}, rsx: {}, wozduh: {} };
  }
  return chartData.mtfNavigatorLayers;
}

function getNavigatorPriceChart(chartData, pane) {
  if (chartData?.charts) {
    if (pane === 'rsx') return chartData.charts.rsx || null;
    if (pane === 'wozduh' || pane === 'osc') return chartData.charts.wozduh || null;
    return chartData.charts.price || chartData.chart || null;
  }
  return chartData?.chart || null;
}

function getMtfNavigatorPlugin(chartData, pane, interval, chartTf) {
  const key = normalizeMtfPeriod(interval);
  const chartKey = normalizeMtfPeriod(chartTf);
  if (!key || mtfPeriodsEqual(key, chartKey)) {
    return getNavigatorPluginForPane(chartData, pane);
  }
  return ensureMtfNavigatorLayer(chartData, pane, key)?.plugin || null;
}

function ensureMtfNavigatorLayer(chartData, pane, tf) {
  const key = normalizeMtfPeriod(tf);
  if (!key) return null;
  const layers = ensureMtfNavigatorLayers(chartData);
  const paneMap = layers[pane] || (layers[pane] = {});
  if (paneMap[key]) return paneMap[key];

  const hostChart = getNavigatorPriceChart(chartData, pane);
  if (!hostChart) return null;

  const anchorSeries = createMTFOverlaySeries(hostChart, {
    color: getMtfPeriodColor(key),
    lineWidth: 0,
    visible: false,
  });
  let plugin = null;
  if (anchorSeries && typeof TrendlinePrimitive !== 'undefined') {
    plugin = new TrendlinePrimitive();
    anchorSeries.attachPrimitive(plugin);
  }
  paneMap[key] = { anchorSeries, plugin, tf: key };
  return paneMap[key];
}

function syncMtfNavigatorLayerRegistry(chartData, pane, activePeriods, chartTf) {
  const layers = ensureMtfNavigatorLayers(chartData);
  const paneMap = layers[pane] || (layers[pane] = {});
  const wanted = new Set(
    (activePeriods || []).map((p) => normalizeMtfPeriod(p)).filter(Boolean),
  );
  wanted.forEach((tf) => {
    if (!mtfPeriodsEqual(tf, chartTf)) {
      ensureMtfNavigatorLayer(chartData, pane, tf);
    }
  });
  Object.keys(paneMap).forEach((key) => {
    if (wanted.has(key)) return;
    paneMap[key]?.plugin?.setData([]);
    paneMap[key]?.plugin?.setBackgroundZones([]);
  });
}

function withSeriesDefaults(style) {
  ensureChartLibraryStyles();
  return { ...CHART_STYLES.seriesDefaults, ...style };
}

function wozduxLineSeriesOptions(def) {
  return withSeriesDefaults({
    color: def.color,
    lineWidth: def.lineWidth,
    lineStyle: def.lineStyle ?? LightweightCharts.LineStyle.Solid,
    title: '',
  });
}

function unixChartTimeToDate(time) {
  if (typeof time === 'object' && time !== null && 'year' in time) {
    return new Date(Date.UTC(time.year, time.month - 1, time.day));
  }
  return new Date(time * 1000);
}

function chartTimeFormatBundle() {
  const locale = navigator.language || 'en-US';
  const timeZone = Intl.DateTimeFormat().resolvedOptions().timeZone;
  const dtfTime = new Intl.DateTimeFormat(locale, {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
    timeZone,
  });
  const dtfDate = new Intl.DateTimeFormat(locale, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    timeZone,
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

  return {
    locale,
    timeFormatter: formatTime,
    dateFormatter: formatDate,
    tickMarkFormatter,
  };
}

function chartLocalizationOptions() {
  const { locale, timeFormatter, dateFormatter } = chartTimeFormatBundle();
  return { locale, timeFormatter, dateFormatter };
}

function chartTimeScaleOptions() {
  const { tickMarkFormatter } = chartTimeFormatBundle();
  return { ...SHARED_TIME_SCALE, tickMarkFormatter };
}


function sharedRightPriceScaleOptions(extra = {}) {
  return {
    borderColor: TV.border,
    autoScale: true,
    minimumWidth: CHART_PRICE_SCALE_MIN_WIDTH,
    alignLabels: true,
    borderVisible: true,
    ...extra,
  };
}

function sharedChartLayout() {
  return {
    background: { color: TV.bg },
    textColor: TV.text,
    fontSize: 11,
    attributionLogo: false,
  };
}

function createPriceChartOptions(width, height) {
  return {
    autoSize: false,
    layout: sharedChartLayout(),
    localization: chartLocalizationOptions(),
    grid: {
      vertLines: { color: TV.grid, style: LightweightCharts.LineStyle.Dotted },
      horzLines: { color: TV.grid, style: LightweightCharts.LineStyle.Dotted },
    },
    crosshair: { ...SHARED_CROSSHAIR },
    timeScale: chartTimeScaleOptions(),
    width,
    height,
    rightPriceScale: sharedRightPriceScaleOptions({
      mode: LightweightCharts.PriceScaleMode.Logarithmic,
    }),
  };
}

function createWozduxChartOptions(width, height) {
  return {
    autoSize: false,
    layout: sharedChartLayout(),
    localization: chartLocalizationOptions(),
    grid: {
      vertLines: { color: TV.grid, style: LightweightCharts.LineStyle.Dotted },
      horzLines: { color: TV.grid, style: LightweightCharts.LineStyle.Dotted },
    },
    crosshair: { ...SHARED_CROSSHAIR },
    timeScale: { ...chartTimeScaleOptions(), visible: false },
    width,
    height,
    rightPriceScale: sharedRightPriceScaleOptions({
      mode: LightweightCharts.PriceScaleMode.Normal,
      scaleMargins: { top: 0.05, bottom: 0.05 },
    }),
  };
}

function createRSXChartOptions(width, height) {
  return {
    autoSize: false,
    layout: sharedChartLayout(),
    localization: chartLocalizationOptions(),
    grid: {
      vertLines: { color: TV.grid, style: LightweightCharts.LineStyle.Dotted },
      horzLines: { color: TV.grid, style: LightweightCharts.LineStyle.Dotted },
    },
    crosshair: { ...SHARED_CROSSHAIR },
    timeScale: chartTimeScaleOptions(),
    width,
    height,
    rightPriceScale: sharedRightPriceScaleOptions({
      mode: LightweightCharts.PriceScaleMode.Normal,
      scaleMargins: { top: 0.05, bottom: 0.05 },
    }),
  };
}


function applyOscPointDelta(oscPt, chartData = _live) {
  if (!oscPt || !chartData) return;
  const time = chartTime(oscPt.time);
  if (time == null) return;

  WOZDUX_LINE_KEYS.forEach((key) => {
    const value = Number(oscPt[key]);
    if (!Number.isFinite(value) || !chartData.wozduxSeries?.[key]) return;
    chartData.wozduxSeries[key].update({ time, value });
  });

  const rsxVal = parseFloat(oscPt.rsx ?? oscPt.jurik);
  if (Number.isFinite(rsxVal) && chartData.rsxSeries) {
    const ptColor = oscPt.color || RSX_DEFAULT_COLOR;
    chartData.rsxSeries.update({ time, value: rsxVal, color: ptColor });
    const signalVal = parseFloat(oscPt.rsx_signal ?? oscPt.rsxSignal);
    if (chartData.rsxSignalSeries && Number.isFinite(signalVal)) {
      chartData.rsxSignalSeries.update({ time, value: signalVal });
    }
  }
}


function defaultPriceScaleState() {
  return { logEnabled: true, autoScale: true };
}

function normalizePriceScaleView(saved) {
  if (!saved) return defaultPriceScaleState();
  if (saved.logEnabled != null || saved.autoScale != null) {
    return {
      logEnabled: saved.logEnabled !== false,
      autoScale: saved.autoScale !== false,
    };
  }
  return {
    logEnabled: saved.priceScaleMode !== 'auto',
    autoScale: saved.priceScaleMode === 'auto',
  };
}

function readPriceAutoScaleFromChart(chartData) {
  try {
    return chartData.chart.priceScale('right').options().autoScale !== false;
  } catch {
    return true;
  }
}

function syncPriceScaleControlsUI(chartData) {
  const controls = chartData?.scaleControlsEl;
  if (!controls) return;
  const state = chartData.priceScaleState || defaultPriceScaleState();
  const autoBtn = controls.querySelector('[data-action="auto"]');
  const logBtn = controls.querySelector('[data-action="log"]');
  if (autoBtn) autoBtn.classList.toggle('active', !!state.autoScale);
  if (logBtn) logBtn.classList.toggle('active', !!state.logEnabled);
}

function applyPriceScaleState(chartData, saved) {
  if (!chartData?.chart) return;
  const state = normalizePriceScaleView(saved);
  chartData.priceScaleState = state;
  chartData.priceScaleMode = state.logEnabled ? 'log' : 'normal';
  chartData.chart.priceScale('right').applyOptions({
    mode: state.logEnabled
      ? LightweightCharts.PriceScaleMode.Logarithmic
      : LightweightCharts.PriceScaleMode.Normal,
    autoScale: state.autoScale,
  });
  syncPriceScaleControlsUI(chartData);
}

function capturePriceScaleState(chartData) {
  const state = chartData?.priceScaleState || defaultPriceScaleState();
  return {
    logEnabled: state.logEnabled,
    autoScale: readPriceAutoScaleFromChart(chartData),
  };
}

function togglePriceLogScale(chartData) {
  if (!chartData?.chart) return;
  const state = { ...(chartData.priceScaleState || defaultPriceScaleState()) };
  state.logEnabled = !state.logEnabled;
  chartData.priceScaleState = state;
  chartData.priceScaleMode = state.logEnabled ? 'log' : 'normal';
  chartData.chart.priceScale('right').applyOptions({
    mode: state.logEnabled
      ? LightweightCharts.PriceScaleMode.Logarithmic
      : LightweightCharts.PriceScaleMode.Normal,
  });
  syncPriceScaleControlsUI(chartData);
}

function enablePriceAutoScale(chartData) {
  if (!chartData?.chart) return;
  const state = { ...(chartData.priceScaleState || defaultPriceScaleState()) };
  state.autoScale = true;
  chartData.priceScaleState = state;
  chartData.chart.priceScale('right').applyOptions({ autoScale: true });
  syncPriceScaleControlsUI(chartData);
}

function isPointerOnPriceScale(chartData, clientX) {
  const container = chartData?.elements?.chartContainer;
  if (!container || !chartData?.chart) return false;
  const rect = container.getBoundingClientRect();
  const scaleW = chartData.chart.priceScale('right').width() || CHART_PRICE_SCALE_MIN_WIDTH;
  return clientX >= rect.right - scaleW;
}

function attachPriceScaleInteractionWatch(chartData) {
  const container = chartData?.elements?.chartContainer;
  if (!container) return;
  container._priceScaleChartData = chartData;

  const syncAutoFromChart = () => {
    const active = container._priceScaleChartData;
    if (!active?.chart) return;
    const autoOn = readPriceAutoScaleFromChart(active);
    if (!active.priceScaleState) active.priceScaleState = defaultPriceScaleState();
    if (active.priceScaleState.autoScale !== autoOn) {
      active.priceScaleState.autoScale = autoOn;
      syncPriceScaleControlsUI(active);
    }
  };

  if (container._priceScaleWatchBound) return;
  container._priceScaleWatchBound = true;

  let draggingPriceScale = false;

  container.addEventListener('mousedown', (e) => {
    const active = container._priceScaleChartData;
    if (active && isPointerOnPriceScale(active, e.clientX)) draggingPriceScale = true;
  });
  container.addEventListener('wheel', (e) => {
    const active = container._priceScaleChartData;
    if (active && isPointerOnPriceScale(active, e.clientX)) {
      requestAnimationFrame(syncAutoFromChart);
    }
  }, { passive: true });
  container.addEventListener('dblclick', (e) => {
    const active = container._priceScaleChartData;
    if (!active || !isPointerOnPriceScale(active, e.clientX)) return;
    e.stopPropagation();
    enablePriceAutoScale(active);
  });

  const onPointerEnd = () => {
    if (!draggingPriceScale) return;
    draggingPriceScale = false;
    requestAnimationFrame(syncAutoFromChart);
  };
  container.addEventListener('mouseup', onPointerEnd);
  document.addEventListener('mouseup', onPointerEnd);
  if (chartData._disposers) {
    chartData._disposers.push(() => {
      container.removeEventListener('mouseup', onPointerEnd);
      document.removeEventListener('mouseup', onPointerEnd);
      container._priceScaleWatchBound = false;
    });
  }
}

function initPriceScaleControls(chartData) {
  const wrap = chartData?.elements?.priceWrap;
  if (!wrap || !chartData?.chart) return;

  let controls = wrap.querySelector('.scale-controls');
  if (!controls) {
    controls = document.createElement('div');
    controls.className = 'scale-controls';
    controls.setAttribute('aria-label', 'Price scale controls');
    controls.innerHTML = `
      <button type="button" class="scale-btn scale-btn--auto active" data-action="auto" title="Auto scale (Y)">Auto</button>
      <button type="button" class="scale-btn scale-btn--log active" data-action="log" title="Logarithmic scale">Log</button>
    `;
    wrap.appendChild(controls);
  }
  chartData.scaleControlsEl = controls;

  if (!chartData.priceScaleState) {
    chartData.priceScaleState = defaultPriceScaleState();
  }

  if (!controls._scaleControlsBound) {
    controls._scaleControlsBound = true;
    controls.querySelector('[data-action="log"]')?.addEventListener('click', (e) => {
      e.stopPropagation();
      togglePriceLogScale(chartData);
    });
    controls.querySelector('[data-action="auto"]')?.addEventListener('click', (e) => {
      e.stopPropagation();
      enablePriceAutoScale(chartData);
    });
  }

  attachPriceScaleInteractionWatch(chartData);
  syncPriceScaleControlsUI(chartData);
}

function destroyChartInstance(chartData) {
  if (chartData?._disposers && Array.isArray(chartData._disposers)) {
    chartData._disposers.forEach((fn) => {
      if (typeof fn === 'function') fn();
    });
    chartData._disposers = [];
  }
  const charts = chartData?.charts
    ? [chartData.charts.price, chartData.charts.wozduh, chartData.charts.rsx]
    : [chartData?.chart];
  charts.forEach((chart) => {
    if (!chart) return;
    try { chart.remove(); } catch { /* noop */ }
  });
}

function exitFullscreenPane() {
  document.querySelectorAll('.fullscreen-pane').forEach((el) => {
    el.classList.remove('fullscreen-pane');
  });
  handleResize();
}

function applyIndicatorConfigStyles(chartData) {
  if (!chartData) return;
  if (!ensureChartLibraryStyles()) return;

  const priceCfg = INDICATOR_CONFIG.price;
  chartData.candleSeries?.applyOptions({
    ...priceCfg.candle,
    crosshairMarkerRadius: CHART_STYLES.seriesDefaults.crosshairMarkerRadius,
    crosshairMarkerBorderWidth: CHART_STYLES.seriesDefaults.crosshairMarkerBorderWidth,
    crosshairMarkerBorderColor: CHART_STYLES.seriesDefaults.crosshairMarkerBorderColor,
  });
  chartData.barSeries?.applyOptions({
    ...priceCfg.bar,
    crosshairMarkerRadius: CHART_STYLES.seriesDefaults.crosshairMarkerRadius,
    crosshairMarkerBorderWidth: CHART_STYLES.seriesDefaults.crosshairMarkerBorderWidth,
    crosshairMarkerBorderColor: CHART_STYLES.seriesDefaults.crosshairMarkerBorderColor,
  });
  chartData.lineSeries?.applyOptions(priceCfg.line);
  chartData.volumeSeries?.applyOptions(priceCfg.volume);

  Object.entries(INDICATOR_CONFIG.wozduh.lines).forEach(([, lineCfg]) => {
    const series = chartData.wozduxSeries?.[lineCfg.dataKey];
    if (series) series.applyOptions(wozduxLineSeriesOptions(lineCfg.style));
  });

  (INDICATOR_CONFIG.wozduh.areas || []).forEach((areaCfg) => {
    const entry = chartData.wozduhAreas?.[areaCfg.id];
    if (!entry?.series) return;
    entry.series.applyOptions({
      topColor: areaCfg.topColor,
      bottomColor: areaCfg.bottomColor,
      lineColor: areaCfg.lineColor || 'transparent',
      lineWidth: areaCfg.lineWidth ?? 0,
      priceLineVisible: false,
      lastValueVisible: false,
      crosshairMarkerVisible: false,
    });
    if (areaCfg.bottom != null && areaCfg.topLineKey == null) {
      entry.series.applyOptions({
        baseValue: { type: 'price', price: areaCfg.bottom },
      });
    }
  });

  const rsxMain = INDICATOR_CONFIG.rsx.lines.rsx_main;
  chartData.rsxSeries?.applyOptions(withSeriesDefaults(rsxMain.style));
  const rsxSignalLine = INDICATOR_CONFIG.rsx.lines.rsx_signal;
  chartData.rsxSignalSeries?.applyOptions(withSeriesDefaults(rsxSignalLine.style));

  (INDICATOR_CONFIG.rsx.areas || []).forEach((areaCfg) => {
    const entry = chartData.rsxAreas?.[areaCfg.id];
    if (!entry?.series) return;
    entry.series.applyOptions({
      topColor: areaCfg.topColor,
      bottomColor: areaCfg.bottomColor,
      lineColor: areaCfg.lineColor || 'transparent',
      lineWidth: areaCfg.lineWidth ?? 0,
      priceLineVisible: false,
      lastValueVisible: false,
      crosshairMarkerVisible: false,
    });
    if (areaCfg.bottom != null) {
      entry.series.applyOptions({
        baseValue: { type: 'price', price: areaCfg.bottom },
      });
    }
  });
}

function reinitBacktestChart() {
  destroyChartInstance(_backtest);
  _backtest = initMultiPaneCharts('backtest', 'backtest-chart-container', {
    selectors: BACKTEST_CHART_SELECTORS,
    overlayTrades: true,
    navigatorPlugin: true,
    tradeMarkers: true,
  }) || {};
  applyWozduhVisibilityToChart(_backtest, 'backtest');
  attachChartHooks(_backtest, _hooks.backtest, 'backtest');
  attachRulerToChart(_backtest);
  return _backtest;
}

function ensureBacktestChart() {
  if (_backtest?.chart) return true;

  _hooks.backtest = {
    crosshairPriceSeries: () => _ctxData('backtest').candleSeries,
    onVisibleRangeChange: () => {
      if (typeof getRulerChartData !== 'undefined' && getRulerChartData() === _ctxData('backtest')) {
        updateRulerOverlay(_ctxData('backtest'));
      }
    },
    onScrollHistory: (range) => {
      if (typeof maybeLoadBacktestHistory === 'function') maybeLoadBacktestHistory(range);
    },
    onBacktestInit: () => {
      attachRulerToChart(_backtest);
      if (typeof NavigatorController !== 'undefined') NavigatorController.renderChartLegends('backtest');
    },
  };

  _backtest = _initChartPair('backtest-chart-container', 'backtest', {
    selectors: BACKTEST_CHART_SELECTORS,
    overlayTrades: true,
    navigatorPlugin: true,
    tradeMarkers: true,
  }) || {};
  if (!_backtest.chart) return false;
  applyWozduhVisibilityToChart(_backtest, 'backtest');
  if (typeof _hooks.backtest.onBacktestInit === 'function') _hooks.backtest.onBacktestInit(_backtest);
  return true;
}

function normalizePriceLineLevel(level) {
  const lineColor = level.color || TV.text;
  const { labelBackgroundColor, labelTextColor, ...rest } = level;
  return {
    ...rest,
    title: level.title ?? '',
    axisLabelVisible: level.axisLabelVisible !== false,
    axisLabelColor: level.axisLabelColor ?? TV.bg,
    axisLabelTextColor: level.axisLabelTextColor ?? lineColor,
  };
}

function addLevelLines(series, levels) {
  return (levels || []).map((level) => series.createPriceLine(normalizePriceLineLevel(level)));
}

function createAreaSeries(chart, areaCfg, priceScaleId = 'right') {
  const series = chart.addAreaSeries({
    topColor: areaCfg.topColor,
    bottomColor: areaCfg.bottomColor,
    lineColor: areaCfg.lineColor || 'transparent',
    lineWidth: areaCfg.lineWidth ?? 0,
    priceLineVisible: false,
    lastValueVisible: false,
    crosshairMarkerVisible: false,
    priceScaleId,
  });
  if (areaCfg.bottom != null && areaCfg.topLineKey == null) {
    series.applyOptions({
      baseValue: { type: 'price', price: areaCfg.bottom },
    });
  }
  return series;
}

function createStaticAreaData(times, top) {
  return (times || [])
    .filter((time) => time != null)
    .map((time) => ({ time: Number(time), value: top }));
}

function buildDynamicWtBandData(osc, topKey, bottomKey) {
  const map = new Map();
  (osc || []).forEach((p) => {
    const time = chartTime(p?.time ?? p?.Time);
    const top = Number(p[topKey]);
    const bottom = Number(p[bottomKey]);
    if (time == null || isWarmupOscValue(top) || isWarmupOscValue(bottom)) return;
    map.set(Number(time), { time: Number(time), value: Math.max(top, bottom) });
  });
  return Array.from(map.values()).sort((a, b) => a.time - b.time);
}

function applyStaticAreas(chartData, paneKey, times, osc) {
  const paneCfg = INDICATOR_CONFIG[paneKey];
  const areaMap = chartData[`${paneKey}Areas`];
  if (!paneCfg?.areas || !areaMap) return;

  paneCfg.areas.forEach((areaCfg) => {
    const entry = areaMap[areaCfg.id];
    if (!entry?.series) return;

    if (areaCfg.topLineKey && areaCfg.bottomLineKey) {
      entry.series.setData(buildDynamicWtBandData(osc, areaCfg.topLineKey, areaCfg.bottomLineKey));
      return;
    }

    if (areaCfg.top != null) {
      entry.series.setData(createStaticAreaData(times, areaCfg.top));
    }
  });
}

function _hostSize(el, fallbackW = 800, fallbackH = 200) {
  const w = Math.max(el?.clientWidth || 0, fallbackW);
  const h = Math.max(el?.clientHeight || 0, fallbackH);
  return { width: w, height: h };
}

function _paneCharts(chartData) {
  if (chartData?.charts) {
    return [chartData.charts.price, chartData.charts.wozduh, chartData.charts.rsx].filter(Boolean);
  }
  return chartData?.chart ? [chartData.chart] : [];
}

function _primarySeriesForChart(chartData, chart) {
  if (!chartData || !chart) return null;
  if (chart === chartData.charts?.price || chart === chartData.chart) {
    return chartData.candleSeries || chartData.barSeries || chartData.lineSeries;
  }
  if (chart === chartData.charts?.wozduh) {
    return chartData.wozduxSeries?.rsiPrice
      || chartData.wozduxSeries?.rsiVolSlow
      || chartData.wozduhDownSeries
      || Object.values(chartData.wozduxSeries || {})[0]
      || null;
  }
  if (chart === chartData.charts?.rsx) {
    return chartData.rsxSeries || chartData.rsxSignalSeries || null;
  }
  return chartData.candleSeries || null;
}

function _isFiniteLogicalRange(range) {
  return range
    && Number.isFinite(range.from)
    && Number.isFinite(range.to)
    && range.to > range.from;
}

/** Live slave panes without plotted data must not broadcast time (prevents price camera reset). */
function _paneCanBroadcastTimeScale(chartData, chart) {
  const master = chartData.charts?.price || chartData.chart;
  if (chart === master) return true;
  if (chartData._context !== 'live') return true;

  if (chart === chartData.charts?.wozduh) {
    if (typeof window !== 'undefined' && window.DDRFactory?.cutoverActive) {
      for (const id of window.DDRFactory.seriesMap.keys()) {
        if (String(id).startsWith('woz_') || String(id).startsWith('score_')) return true;
      }
      return false;
    }
    return Object.keys(chartData.wozduxSeries || {}).length > 0;
  }

  if (chart === chartData.charts?.rsx) {
    if (typeof window !== 'undefined' && window.DDRFactory?.cutoverActive) {
      for (const id of window.DDRFactory.seriesMap.keys()) {
        if (String(id).startsWith('line_rsx')) return true;
      }
      return false;
    }
    return !!(chartData.rsxSeries || chartData.rsxSignalSeries);
  }

  return false;
}

/**
 * Master (price) → slaves always when range is valid.
 * Slaves → others only when pane has plotted data (Data Guard).
 */
function syncTimeScale(chartData) {
  const charts = _paneCharts(chartData);
  if (charts.length < 2) return;

  charts.forEach((source) => {
    source.timeScale().subscribeVisibleLogicalRangeChange((range) => {
      if (!range || chartData._syncingTimeScale) return;
      if (chartData._context === 'live' && _liveUpdating) return;
      if (!_isFiniteLogicalRange(range)) return;
      if (!_paneCanBroadcastTimeScale(chartData, source)) return;

      chartData._syncingTimeScale = true;
      charts.forEach((target) => {
        if (target === source) return;
        syncVisibleLogicalRange(target, range, { animate: false });
      });
      chartData._syncingTimeScale = false;
    });
  });
}

/**
 * Bidirectional Crosshair sync with reentrancy guard — breaks A→B→A event loops.
 */
function syncCrosshair(chartData) {
  const charts = _paneCharts(chartData);
  if (charts.length < 2) return;

  charts.forEach((source) => {
    source.subscribeCrosshairMove((param) => {
      if (chartData._syncingCrosshair) return;
      chartData._syncingCrosshair = true;
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
          const series = _primarySeriesForChart(chartData, target);
          if (!series || typeof target.setCrosshairPosition !== 'function') return;
          let price = 0;
          const seriesData = param.seriesData?.get?.(series);
          if (seriesData && Number.isFinite(seriesData.value)) price = seriesData.value;
          else if (seriesData && Number.isFinite(seriesData.close)) price = seriesData.close;
          else {
            const sourceSeries = _primarySeriesForChart(chartData, source);
            const srcData = sourceSeries ? param.seriesData?.get?.(sourceSeries) : null;
            if (srcData && Number.isFinite(srcData.value)) price = srcData.value;
            else if (srcData && Number.isFinite(srcData.close)) price = srcData.close;
          }
          target.setCrosshairPosition(price, param.time, series);
        });
      } finally {
        chartData._syncingCrosshair = false;
      }
    });
  });
}

function setVisibleLogicalRangeAll(chartData, range, options = {}) {
  if (!chartData || !range) return;
  const charts = _paneCharts(chartData);
  const prev = chartData._syncingTimeScale;
  chartData._syncingTimeScale = true;
  try {
    charts.forEach((chart) => syncVisibleLogicalRange(chart, range, options));
  } finally {
    chartData._syncingTimeScale = prev;
  }
}

function bindFlexSplitters(chartData) {
  const stack = chartData?.root?.querySelector?.('.charts-stack');
  if (!stack || stack._flexSplittersBound) return;
  stack._flexSplittersBound = true;

  const readFlex = (el, fallback) => {
    const grow = Number.parseFloat(el.style.flexGrow);
    if (Number.isFinite(grow) && grow > 0) return grow;
    const fromShorthand = Number.parseFloat(el.style.flex);
    if (Number.isFinite(fromShorthand) && fromShorthand > 0) return fromShorthand;
    return fallback;
  };

  stack.querySelectorAll('.pane-splitter').forEach((splitter) => {
    splitter.addEventListener('pointerdown', (e) => {
      e.preventDefault();
      splitter.setPointerCapture(e.pointerId);
      const boundary = splitter.dataset.boundary;
      const priceWrap = chartData.elements?.priceWrap;
      const oscWrap = chartData.elements?.oscWrap;
      const rsxWrap = chartData.elements?.rsxWrap;
      const stackRect = stack.getBoundingClientRect();

      const onMove = (moveEvent) => {
        const y = ((moveEvent.clientY - stackRect.top) / stackRect.height) * 100;
        if (boundary === 'price-wozduh' && priceWrap && oscWrap) {
          const rsxFlex = readFlex(rsxWrap, 23);
          const available = 100 - rsxFlex;
          let priceW = Math.max(15, Math.min(available - 10, y));
          let oscW = available - priceW;
          priceWrap.style.flex = `${priceW} 1 0`;
          oscWrap.style.flex = `${oscW} 1 0`;
        } else if (boundary === 'wozduh-rsx' && oscWrap && rsxWrap && priceWrap) {
          const priceFlex = readFlex(priceWrap, 55);
          const available = 100 - priceFlex;
          let oscW = Math.max(10, Math.min(available - 10, y - priceFlex));
          let rsxW = available - oscW;
          oscWrap.style.flex = `${oscW} 1 0`;
          rsxWrap.style.flex = `${rsxW} 1 0`;
        }
        resizeChartInstance(chartData);
      };

      const onUp = (upEvent) => {
        splitter.releasePointerCapture(upEvent.pointerId);
        splitter.removeEventListener('pointermove', onMove);
        splitter.removeEventListener('pointerup', onUp);
        splitter.removeEventListener('pointercancel', onUp);
      };

      splitter.addEventListener('pointermove', onMove);
      splitter.addEventListener('pointerup', onUp);
      splitter.addEventListener('pointercancel', onUp);
    });
  });
}

function initMultiPaneCharts(context, containerId, options = {}) {
  if (typeof LightweightCharts === 'undefined') return null;

  const selectors = options.selectors || (context === 'backtest' ? BACKTEST_CHART_SELECTORS : LIVE_CHART_SELECTORS);
  const root = document.getElementById(containerId);
  const priceHost = document.getElementById(selectors.chartContainer);
  const wozduhHost = document.getElementById(selectors.oscContainer);
  const rsxHost = document.getElementById(selectors.rsxContainer);
  const priceWrap = document.getElementById(selectors.priceWrap);
  const oscWrap = document.getElementById(selectors.oscWrap);
  const rsxWrap = document.getElementById(selectors.rsxWrap);

  if (!root || !priceHost || !wozduhHost || !rsxHost) return null;
  if (!ensureChartLibraryStyles()) return null;

  const priceSize = _hostSize(priceHost, root.clientWidth || 800, priceWrap?.clientHeight || 280);
  const wozSize = _hostSize(wozduhHost, root.clientWidth || 800, oscWrap?.clientHeight || 140);
  const rsxSize = _hostSize(rsxHost, root.clientWidth || 800, rsxWrap?.clientHeight || 140);

  const priceChart = LightweightCharts.createChart(priceHost, createPriceChartOptions(priceSize.width, priceSize.height));
  const wozduhChart = LightweightCharts.createChart(wozduhHost, createWozduxChartOptions(wozSize.width, wozSize.height));
  const rsxChart = LightweightCharts.createChart(rsxHost, createRSXChartOptions(rsxSize.width, rsxSize.height));

  const priceCfg = INDICATOR_CONFIG.price;
  const candleSeries = priceChart.addCandlestickSeries({ ...priceCfg.candle, priceScaleId: 'right' });
  const barSeries = priceChart.addBarSeries({ ...priceCfg.bar, priceScaleId: 'right' });
  const lineSeries = createMTFOverlaySeries(priceChart, {
    ...priceCfg.line,
    priceScaleId: 'right',
    visible: priceCfg.line?.visible ?? false,
  });
  const volumeSeries = priceChart.addHistogramSeries(priceCfg.volume);

  const priceMargins = INDICATOR_CONFIG.price.priceScale?.scaleMargins || { top: 0.05, bottom: 0.22 };
  const volumeMargins = INDICATOR_CONFIG.price.volumeScale?.scaleMargins || { top: 0.82, bottom: 0 };
  priceChart.priceScale('right').applyOptions({
    ...sharedRightPriceScaleOptions({ mode: LightweightCharts.PriceScaleMode.Logarithmic }),
    scaleMargins: priceMargins,
  });
  priceChart.priceScale('volume').applyOptions({
    scaleMargins: volumeMargins,
    autoScale: true,
    visible: true,
  });

  const isLivePane = context === 'live';
  let wozduhAreas = {};
  let wozduxSeries = {};
  let wozduxPriceLinesLocal = [];
  let rsxAreas = {};
  let rsxSeries = null;
  let rsxSignalSeries = null;
  let rsxPriceLinesLocal = [];

  if (!isLivePane) {
    INDICATOR_CONFIG.wozduh.areas.forEach((areaCfg) => {
      wozduhAreas[areaCfg.id] = { series: createAreaSeries(wozduhChart, areaCfg, 'right'), cfg: areaCfg };
    });

    Object.entries(INDICATOR_CONFIG.wozduh.lines).forEach(([seriesKey, lineCfg]) => {
      wozduxSeries[seriesKey] = wozduhChart.addLineSeries({
        ...wozduxLineSeriesOptions(lineCfg.style),
        priceScaleId: 'right',
      });
      wozduxSeries[lineCfg.dataKey] = wozduxSeries[seriesKey];
    });

    const wozduhLevelAnchor = wozduxSeries.rsiPrice || wozduxSeries.wozduh_wt1;
    wozduxPriceLinesLocal = addLevelLines(wozduhLevelAnchor, INDICATOR_CONFIG.wozduh.levels);

    INDICATOR_CONFIG.rsx.areas.forEach((areaCfg) => {
      rsxAreas[areaCfg.id] = { series: createAreaSeries(rsxChart, areaCfg, 'right'), cfg: areaCfg };
    });

    const rsxLineCfg = INDICATOR_CONFIG.rsx.lines.rsx_main;
    rsxSeries = rsxChart.addLineSeries({
      ...withSeriesDefaults(rsxLineCfg.style),
      priceScaleId: 'right',
    });
    const rsxSignalCfg = INDICATOR_CONFIG.rsx.lines.rsx_signal;
    rsxSignalSeries = rsxChart.addLineSeries({
      ...withSeriesDefaults(rsxSignalCfg.style),
      priceScaleId: 'right',
    });
    rsxPriceLinesLocal = addLevelLines(rsxSeries, INDICATOR_CONFIG.rsx.levels);
  }

  let entryMarkerSeries = null;
  if (options.overlayTrades) {
    entryMarkerSeries = createMTFOverlaySeries(priceChart, {
      color: 'transparent',
      lineWidth: 0,
      priceScaleId: 'right',
    });
  }

  let priceNavigatorPlugin = null;
  let rsxNavigatorPlugin = null;
  let wozduhNavigatorPlugin = null;
  let tradeMarkerPlugin = null;

  if (options.navigatorPlugin && typeof TrendlinePrimitive !== 'undefined') {
    priceNavigatorPlugin = new TrendlinePrimitive();
    candleSeries.attachPrimitive(priceNavigatorPlugin);

    if (!isLivePane && rsxSeries) {
      rsxNavigatorPlugin = new TrendlinePrimitive();
      rsxSeries.attachPrimitive(rsxNavigatorPlugin);
    }

    const wozduhNavHost = wozduxSeries.rsiVolSlow || wozduxSeries.wozduh_wt2;
    if (!isLivePane && wozduhNavHost) {
      wozduhNavigatorPlugin = new TrendlinePrimitive();
      wozduhNavHost.attachPrimitive(wozduhNavigatorPlugin);
    }
  }

  if (options.tradeMarkers && typeof TradeMarkerPrimitive !== 'undefined') {
    tradeMarkerPlugin = new TradeMarkerPrimitive();
    candleSeries.attachPrimitive(tradeMarkerPlugin);
  }

  const result = {
    multiPane: true,
    monolith: false,
    _context: context,
    chart: priceChart,
    charts: { price: priceChart, wozduh: wozduhChart, rsx: rsxChart },
    containerId,
    root,
    candleSeries,
    barSeries,
    lineSeries,
    volumeSeries,
    rsxSeries,
    rsxSignalSeries,
    wozduhUpSeries: wozduxSeries.rsiVolFast || wozduxSeries.wozduh_wt1,
    wozduhDownSeries: wozduxSeries.rsiVolSlow || wozduxSeries.wozduh_wt2,
    wozduxSeries,
    wozduhAreas,
    rsxAreas,
    _wozduxPriceLines: wozduxPriceLinesLocal,
    _rsxPriceLines: rsxPriceLinesLocal,
    entryMarkerSeries,
    priceNavigatorPlugin,
    rsxNavigatorPlugin,
    wozduhNavigatorPlugin,
    tradeMarkerPlugin,
    mtfNavigatorLayers: { price: {}, rsx: {}, wozduh: {} },
    elements: {
      priceWrap,
      chartHost: priceWrap,
      chartContainer: priceHost,
      oscContainer: wozduhHost,
      rsxContainer: rsxHost,
      oscWrap,
      rsxWrap,
    },
    priceScaleState: defaultPriceScaleState(),
    priceScaleMode: 'log',
    rulerAttached: false,
    _syncingTimeScale: false,
    _syncingCrosshair: false,
    _disposers: [],
  };

  initPriceScaleControls(result);
  syncTimeScale(result);
  syncCrosshair(result);
  bindFlexSplitters(result);

  const observeHost = (host, chart) => {
    if (!host || !chart) return;
    const ro = new ResizeObserver((entries) => {
      if (!entries?.length) return;
      const { width, height } = entries[0].contentRect;
      if (width <= 0 || height <= 0) return;
      chart.applyOptions({ width, height });
      if (context === 'backtest' && typeof ChartProjection !== 'undefined') {
        ChartProjection.trySync();
      }
    });
    ro.observe(host);
    result._disposers.push(() => ro.disconnect());
  };

  observeHost(priceHost, priceChart);
  observeHost(wozduhHost, wozduhChart);
  observeHost(rsxHost, rsxChart);

  return result;
}

function logicalRangesEqual(a, b, eps = LOGICAL_RANGE_EPS) {
  if (!a || !b) return false;
  return Math.abs(a.from - b.from) < eps && Math.abs(a.to - b.to) < eps;
}

function syncVisibleLogicalRange(targetChart, range, options = {}) {
  if (!targetChart?.timeScale || !range) return;
  const timeScale = targetChart.timeScale();
  const currentRange = timeScale.getVisibleLogicalRange();
  if (logicalRangesEqual(currentRange, range)) return;
  const rangeOpts = options.animate === false ? { animate: false } : undefined;
  timeScale.setVisibleLogicalRange(range, rangeOpts);
}

function attachChartHooks(chartData, hooks = {}, context = 'live') {
  const chart = chartData?.chart;
  if (!chart?.timeScale) return;

  const scrollThreshold = (typeof CONFIG !== 'undefined' && CONFIG.LIVE_HISTORY_SCROLL_THRESHOLD) || 50;

  chart.timeScale().subscribeVisibleLogicalRangeChange((range) => {
    if (!range || (context === 'live' && _liveUpdating)) return;

    if (typeof hooks.onScrollHistory === 'function' && range.from < scrollThreshold) {
      if (context === 'live' && _liveStore()?.isSealed()) return;
      hooks.onScrollHistory(range);
    }

    if (typeof hooks.onVisibleRangeChange === 'function') {
      hooks.onVisibleRangeChange(range);
    }
  });
}

function resizeChartInstance(chartData) {
  if (!chartData) return;
  const resizeOne = (host, chart) => {
    if (!host || !chart) return;
    const rect = host.getBoundingClientRect();
    const w = rect.width || host.clientWidth || 800;
    const h = rect.height || host.clientHeight || 200;
    if (w > 0 && h > 0) chart.applyOptions({ width: w, height: h });
  };
  if (chartData.charts) {
    resizeOne(chartData.elements?.chartContainer, chartData.charts.price);
    resizeOne(chartData.elements?.oscContainer, chartData.charts.wozduh);
    resizeOne(chartData.elements?.rsxContainer, chartData.charts.rsx);
    return;
  }
  if (!chartData.chart || !chartData.elements?.chartContainer) return;
  resizeOne(chartData.elements.chartContainer, chartData.chart);
}

function resizeChartForContainer(chartData, container) {
  if (!chartData?.chart) return;
  resizeChartInstance(chartData);
}

function handleResize() {
  const activeTab = getActiveTabId();
  if (activeTab === 'tab-live') {
    resizeChartInstance(_live);
  } else if (activeTab === 'tab-backtest') {
    resizeChartInstance(_backtest);
  } else if (activeTab === 'tab-stats') {
    resizeEquityChart();
  }
  if (typeof ruler !== 'undefined' && ruler.active) {
    updateRulerOverlay(getRulerChartData());
  }
}

function fitChartInstance(chartData) {
  _paneCharts(chartData).forEach((chart) => {
    chart?.timeScale()?.fitContent();
  });
}

function fitProfessionalChart(chartData) {
  if (!chartData?.chart) return;

  const containerId = chartData.containerId;
  const container = containerId
    ? document.getElementById(containerId)
    : null;
  if (container) {
    resizeChartForContainer(chartData, container);
  }
  fitChartInstance(chartData);
}

function activePriceSeries() {
  if (_chartType === 'bars') return _live.barSeries;
  if (_chartType === 'line') return _live.lineSeries;
  return _live.candleSeries;
}


function resizeAllCharts() {
  handleResize();
}

function applyWozduhVisibilityToChart(chartData, context = 'live') {
  if (!chartData?.wozduxSeries) return;
  const prefs = (typeof window.WozduhController !== 'undefined')
    ? window.WozduhController.getSettingsFromUI(context)
    : (typeof CONFIG !== 'undefined' ? CONFIG.defaultWozduhPrefs() : {});
  applyWozduhVisibilityFromPrefs(chartData, prefs);
}

function setChartType(type) {
  if (_chartType === type) return;
  _chartType = type;

  if (_live?.chart && _liveCandles().length > 0) {
    applyPriceToChart(_live, _liveCandles());
  }

  _live.candleSeries?.applyOptions({ visible: type === 'candles' });
  _live.barSeries?.applyOptions({ visible: type === 'bars' });
  _live.lineSeries?.applyOptions({ visible: type === 'line' });
}

function applyOrderFlowTimeScale(enabled) {
  _paneCharts(_live).forEach((chart) => {
    chart.timeScale().applyOptions({ secondsVisible: enabled, timeVisible: true });
  });
}

function clearFibLines() {
  if (!_live?.chart || !_live?.candleSeries) {
    _fibPriceLines = [];
    return;
  }
  _fibPriceLines.forEach((pl) => {
    try { _live.candleSeries.removePriceLine(pl); } catch (_) { /* noop */ }
  });
  _fibPriceLines = [];
}

function applyPriceToChart(chartData, candles, options = {}) {
  if (!chartData?.candleSeries || !candles?.length) return;

  const isLive = chartData === _live || chartData._context === 'live';
  const activeType = isLive ? (_chartType || 'candles') : 'candles';

  if (activeType === 'candles') {
    chartData.candleSeries.setData(candles);
  } else if (activeType === 'bars' && chartData.barSeries) {
    chartData.barSeries.setData(candles);
  } else if (activeType === 'line' && chartData.lineSeries) {
    chartData.lineSeries.setData(toLineClose(candles));
  }

  if (options.includeVolume !== false && chartData.volumeSeries) {
    chartData.volumeSeries.setData(toVolumeBars(candles));
  }
}

function applyOscillatorToChart(chartData, osc, annotations = null) {
  applyWozduxData(osc, chartData);
  applyRsxData(osc, chartData, annotations);
  const times = (osc || []).map((p) => chartTime(p?.time)).filter((t) => t != null);
  applyStaticAreas(chartData, 'rsx', times, osc);
  applyStaticAreas(chartData, 'wozduh', times, osc);
}

function applyCandleMarkers(chartData, markers) {
  if (!chartData?.candleSeries) return;
  chartData.candleSeries.setMarkers(
    (markers || []).slice().sort((a, b) => a.time - b.time),
  );
}

function wozduxMarkersFromGrid(chartData = _live) {
  return buildWozduhMarkersFromGrid(_storeForChart(chartData).getAnnotationsMap());
}

function applyWozduxMarkers(osc, chartData = _live) {
  const markerKey = INDICATOR_CONFIG.wozduh.markerSeriesKey;
  const markerSeries = chartData.wozduxSeries?.[markerKey];
  if (!markerSeries) return;
  if (_chartContext(chartData) === 'live') {
    markerSeries.setMarkers(_cachedWozduhMarkers);
    return;
  }
  markerSeries.setMarkers(wozduxMarkersFromGrid(chartData));
}

function applyWozduxData(osc, chartData = _live) {
  WOZDUX_LINE_KEYS.forEach((key) => {
    const series = chartData.wozduxSeries?.[key];
    if (series) series.setData(toLine(osc, key));
  });
  applyWozduxMarkers(osc, chartData);
}

function updateWozduxPoint(pt, chartData = _live) {
  if (!pt) return;
  const time = chartTime(pt?.time ?? pt?.Time);
  if (time == null) return;
  WOZDUX_LINE_KEYS.forEach((key) => {
    const value = Number(pt[key]);
    if (!Number.isFinite(value) || !chartData.wozduxSeries?.[key]) return;
    chartData.wozduxSeries[key].update({ time, value });
  });
  if (pt.volCrossMarker) {
    const rawMs = ChartDataStore.toMs(pt.time ?? pt.Time);
    const ms = TimeNormalizer.snapToGrid(rawMs, _storeTf(_chartContext(chartData)));
    const ann = ms != null ? _storeForChart(chartData).getAnnotationAt(ms) : null;
    if (_chartContext(chartData) === 'live' && ann) {
      _patchWozduhMarkerAtMs(ms, ann);
      _flushWozduhMarkerCache(chartData);
    } else {
      applyWozduxMarkers(null, chartData);
    }
  }
}

function getChartAnnotationPanes(chartData) {
  const wozduhMarkerKey = INDICATOR_CONFIG.wozduh.markerSeriesKey;
  const panes = {
    price: chartData?.candleSeries || null,
    rsx: chartData?.rsxSeries || null,
    wozduh: chartData?.wozduxSeries?.[wozduhMarkerKey] || null,
  };
  return Object.fromEntries(
    Object.entries(panes).map(([pane, series]) => [pane.toLowerCase(), series]),
  );
}

function rsxSettingsContextForChart(chartData) {
  return chartData === _backtest ? 'backtest' : 'live';
}

function applyLiveAnnotationLayer(storeData, options = {}) {
  const chartData = _live;
  if (!chartData?.candleSeries) return;

  const annotationMap = storeData?.annotationMap instanceof Map
    ? storeData.annotationMap
    : (_liveStore()?.getAnnotationsMap() || new Map());
  const wireAnnotations = storeData?.annotations || [];
  const showPivots = options.showPivots !== false;

  // Layer 1 — price pane: session trades + volume spikes (+ optional price-pane wire labels)
  _rebuildPriceMarkerCache(annotationMap);
  wireAnnotations.forEach((ann) => {
    if (normalizeAnnotationPane(ann?.pane) !== 'price') return;
    const native = annotationToNativeMarker(ann);
    if (!native) return;
    const { _rawTime, ...marker } = native;
    _cachedPriceMarkers.push(marker);
  });
  _cachedPriceMarkers.sort((a, b) => a.time - b.time);
  _flushPriceMarkerCache();

  // Layer 2 — wozduh pane: vol-cross grid markers merged with wire wozduh annotations
  _rebuildWozduhMarkerCache(annotationMap);
  const wozduhWire = wireAnnotations
    .filter((ann) => normalizeAnnotationPane(ann?.pane) === 'wozduh')
    .map((ann) => {
      const native = annotationToNativeMarker(ann);
      if (!native) return null;
      const { _rawTime, ...marker } = native;
      return marker;
    })
    .filter(Boolean);
  if (wozduhWire.length) {
    _cachedWozduhMarkers = [..._cachedWozduhMarkers, ...wozduhWire]
      .sort((a, b) => a.time - b.time);
  }
  _flushWozduhMarkerCache(chartData);

  // Layer 3 — rsx pane: pivot / signal wire annotations (single setMarkers per series)
  const panes = getChartAnnotationPanes(chartData);
  const rsxWire = wireAnnotations.filter((ann) => normalizeAnnotationPane(ann?.pane) === 'rsx');
  if (panes.rsx) {
    applyUniversalAnnotations({ rsx: panes.rsx }, rsxWire, {}, { showPivots });
  }
}

function applyLiveAnnotationLayerFromSnapshot(snapshot, annotationMap, options = {}) {
  const storeData = {
    annotations: snapshot?.annotations || [],
    annotationMap: annotationMap instanceof Map ? annotationMap : new Map(),
  };
  applyLiveAnnotationLayer(storeData, options);
}

function applyUniversalAnnotations(chartPanes, annotations, seriesTimesByPane = {}, options = {}) {
  if (!chartPanes) return;
  const showPivots = options.showPivots !== false;
  const grouped = {};
  (annotations || []).forEach((ann) => {
    const label = String(ann?.label ?? ann?.Label ?? ann?.text ?? '').toUpperCase();
    if (!showPivots && label === 'P') return;
    const pane = normalizeAnnotationPane(ann?.pane);
    const series = chartPanes[pane];
    if (!series) {
      console.warn('[Annotations] unknown pane', ann?.pane, '→', pane);
      return;
    }
    const native = annotationToNativeMarker(ann);
    if (!native) {
      console.warn('[Annotations] invalid time', pane, ann?.time ?? ann?.Time);
      return;
    }
    const { _rawTime, ...marker } = native;
    if (!grouped[pane]) grouped[pane] = [];
    grouped[pane].push(marker);
  });
  Object.entries(chartPanes).forEach(([pane, series]) => {
    if (!series) return;
    const formattedMarkers = (grouped[pane] || []).slice().sort((a, b) => a.time - b.time);
    series.setMarkers(formattedMarkers);
  });
}

function applyRsxData(osc, chartData = _live, annotations = null) {
  if (!chartData?.rsxSeries) return;

  const mappedRSX = mapRSXData(osc);
  const lineData = mappedRSX.map(({ time, value, color }) => {
    if (isWarmupOscValue(value)) return { time };
    return { time, value, color };
  });
  chartData.rsxSeries.setData(lineData);
  if (chartData.rsxSignalSeries) {
    chartData.rsxSignalSeries.setData(mapRSXSignalData(osc));
  }

  const seriesTimesByPane = {
    rsx: new Set(lineData.map((d) => d.time)),
  };
  const resolved = annotations ?? _storeDataForChart(chartData).annotations;
  const showPivots = rsxShowPivotsFrom(
    RsxController.getSettings(rsxSettingsContextForChart(chartData)),
    true,
  );
  applyUniversalAnnotations(getChartAnnotationPanes(chartData), resolved, seriesTimesByPane, { showPivots });
}

function updateRsxPoint(time, value, color, marker, rsxSignal, chartData = _live) {
  if (time == null || !Number.isFinite(value)) return;
  const t = Number(time);
  const ptColor = color || RSX_DEFAULT_COLOR;
  chartData.rsxSeries.update({ time: t, value, color: ptColor });

  const signalVal = parseFloat(rsxSignal);
  if (chartData.rsxSignalSeries && Number.isFinite(signalVal)) {
    chartData.rsxSignalSeries.update({ time: t, value: signalVal });
  }

  if (marker || _storeForChart(chartData).annotationCount() > 0) {
    const storeData = _storeDataForChart(chartData);
    const mapped = mapRSXData(storeData.osc);
    const showPivots = rsxShowPivotsFrom(
    RsxController.getSettings(rsxSettingsContextForChart(chartData)),
    true,
  );
    applyUniversalAnnotations(
      getChartAnnotationPanes(chartData),
      storeData.annotations,
      { rsx: new Set(mapped.map((d) => d.time)) },
      { showPivots },
    );
  }
}

function renderFibZones(zones) {
  lastFibZones = zones || [];
  clearFibLines();
  if (!ToolbarController.isFibEnabled()) return;
  lastFibZones.forEach((z) => {
    if (!z.isActive || !Number.isFinite(z.price)) return;
    const color = z.ratio === 0.618 ? _themeColor('fibGolden', '#e3b341') : _themeColor('fibMuted', 'rgba(120,123,134,0.7)');
    _fibPriceLines.push(_live.candleSeries.createPriceLine(normalizePriceLineLevel({
      price: z.price,
      color,
      lineWidth: z.ratio === 0.618 ? 2 : 1,
      lineStyle: LightweightCharts.LineStyle.Dashed,
      axisLabelVisible: true,
      title: `${(z.ratio * 100).toFixed(1)}%`,
    })));
  });
}

function buildTradeMarkerPrimitiveData(trades) {
  const markers = [];
  (trades || []).forEach((trade) => {
    const entryT = chartTime(trade.entryTime || trade.EntryTime);
    const exitT = chartTime(trade.time || trade.Time);
    const entryP = parseFloat(trade.entryPrice ?? trade.EntryPrice);
    const exitP = parseFloat(trade.exitPrice ?? trade.ExitPrice);
    const side = String(trade.side || trade.Side || '').toUpperCase();
    const isLong = side === 'LONG' || side === 'BUY';

    if (entryT && Number.isFinite(entryP)) {
      markers.push({ time: entryT, price: entryP, kind: isLong ? 'long' : 'short' });
    }
    if (exitT && Number.isFinite(exitP)) {
      markers.push({ time: exitT, price: exitP, kind: 'exit' });
    }
  });
  return markers.sort((a, b) => a.time - b.time);
}

function applyNavigatorBarColors(chartData, barColors, candles, enabled) {
  const colorByTime = navigatorBarColorMap(barColors, candles);
  if (!enabled || !chartData?.candleSeries || !candles?.length || colorByTime.size === 0) {
    return candles;
  }

  const colored = candles.map((candle) => {
    const color = colorByTime.get(candle.time);
    if (!color) return { ...candle };
    return {
      ...candle,
      color,
      borderColor: color,
      wickColor: color,
    };
  });

  const applyColoredData = () => {
    chartData.candleSeries.setData(colored);
    if (chartData.barSeries) chartData.barSeries.setData(colored);
  };
  applyColoredData();

  return colored;
}

function reapplyCachedNavigatorPlugins(chartData = _backtest) {
  if (!chartData || !backtestStore.candleCount()) return;

  const pricePlugin = chartData.priceNavigatorPlugin;
  if (pricePlugin && _btNavLines?.length) {
    pricePlugin.setData(_btNavLines);
    pricePlugin.setLinesVisible(true);
  }

  const navMarkers = _btNavMarkers || [];
  if (navMarkers.length) {
    const storeOsc = backtestStore.getForLightweightCharts().osc;
    applyBacktestCandleMarkers(chartData, backtestLastTrades, storeOsc, navMarkers);
  }
}

function clearNavigatorOverlays(chartData = _backtest) {
  ['price', 'rsx', 'wozduh'].forEach((pane) => {
    syncMtfNavigatorLayerRegistry(chartData, pane, [], '');
    const plugin = getNavigatorPluginForPane(chartData, pane);
    plugin?.setData([]);
    plugin?.setBackgroundZones([]);
  });
  _btNavLines = [];
  _btNavMarkers = [];
}

function applyNavigatorPaneOverlay(pane, navigatorData, candles, chartData, options = {}) {
  if (!chartData) return [];

  const context = options.context || (chartData === _live ? 'live' : 'backtest');
  const chartTf = resolveChartTfForNavigator(context);
  const paneEnabled = isNavigatorPaneEnabled(pane, context);
  const settings = getNavigatorPaneSettingsFromUI(pane, context);
  const hasLines = navigatorData?.lines?.length > 0;
  const hasZones = navigatorData?.backgroundZones?.length > 0;
  const activeMtfPeriods = getActiveMtfPeriods(settings, chartTf);
  const chartKey = normalizeMtfPeriod(chartTf);

  const clearAllLayers = () => {
    syncMtfNavigatorLayerRegistry(chartData, pane, [], chartTf);
    const chartPlugin = getNavigatorPluginForPane(chartData, pane);
    chartPlugin?.setData([]);
    chartPlugin?.setBackgroundZones([]);
    chartPlugin?.setLinesVisible(false);
    chartPlugin?.setBackgroundVisible(false);
  };

  if (!paneEnabled || (!hasLines && !hasZones)) {
    clearAllLayers();
    return [];
  }

  const grouped = groupNavigatorLinesByInterval(navigatorData.lines || [], chartTf);
  syncMtfNavigatorLayerRegistry(chartData, pane, activeMtfPeriods, chartTf);

  const zones = settings.backgroundColor
    ? mapNavigatorBackgroundZones(navigatorData.backgroundZones)
    : [];

  const pushLinesToPlugin = (targetPlugin, rawLines, isChartTf) => {
    if (!targetPlugin) return [];
    const mapped = filterNavigatorLinesByTerm(
      mapNavigatorLinesForChart(rawLines, candles),
      pane,
    );
    const applyPluginData = () => {
      targetPlugin.setData(mapped);
      if (isChartTf) {
        targetPlugin.setBackgroundZones(zones);
        targetPlugin.setBackgroundVisible(
          settings.backgroundVisible !== false && settings.backgroundColor,
        );
      } else {
        targetPlugin.setBackgroundZones([]);
        targetPlugin.setBackgroundVisible(false);
      }
      targetPlugin.setLinesVisible(settings.linesVisible !== false);
    };
    applyPluginData();
    return mapped;
  };

  let allMappedLines = [];
  allMappedLines = allMappedLines.concat(
    pushLinesToPlugin(
      getMtfNavigatorPlugin(chartData, pane, chartKey, chartTf),
      grouped.get(chartKey) || [],
      true,
    ),
  );

  activeMtfPeriods.forEach((tf) => {
    const key = normalizeMtfPeriod(tf);
    allMappedLines = allMappedLines.concat(
      pushLinesToPlugin(
        getMtfNavigatorPlugin(chartData, pane, key, chartTf),
        grouped.get(key) || [],
        false,
      ),
    );
  });

  if (pane === 'price' && settings.barColor && navigatorHasBarColors(navigatorData.barColors)) {
    const colored = applyNavigatorBarColors(
      chartData,
      navigatorData.barColors,
      candles,
      true,
    );
    if (colored && options.updateLoadedCandles !== false && context === 'backtest') {
      colored.forEach((c) => backtestStore.upsertCandle(c, _storeTf('backtest')));
    }
  }

  return buildNavigatorMarkers(navigatorData, candles, pane);
}

function applyNavigatorOverlays(result, candles, chartData = _backtest, options = {}) {
  const context = options.context || (chartData === _live ? 'live' : 'backtest');
  const navigators = resolveNavigatorResults(result);
  const allNavMarkers = [];

  NAVIGATOR_PANES.forEach((pane) => {
    const navData = navigators[pane] || null;
    const markers = applyNavigatorPaneOverlay(pane, navData, candles, chartData, { ...options, context });
    if (pane === 'price') {
      const lines = (isNavigatorPaneEnabled('price', context) && navData?.lines?.length)
        ? filterNavigatorLinesByTerm(mapNavigatorLinesForChart(navData.lines, candles), pane)
        : [];
      if (context === 'live') {
        _liveNavLines = lines;
        _liveNavMarkers = markers;
      } else if (isNavigatorPaneEnabled('price', context) && navData?.lines?.length) {
        _btNavLines = lines;
        _btNavMarkers = markers;
      } else {
        _btNavLines = [];
        _btNavMarkers = [];
      }
    }
    allNavMarkers.push(...markers);
  });

  return allNavMarkers;
}

function applyNavigatorPriceOverlay(navigatorPrice, candles) {
  applyNavigatorOverlays({ navigatorData: navigatorPrice }, candles, _backtest);
}

function applyBacktestCandleMarkers(chartData, trades, osc, navigatorMarkers) {
  const spikeMarkers = buildSpikeMarkersFromGrid(backtestStore.getAnnotationsMap());
  const navMarkers = navigatorMarkers ?? _btNavMarkers;
  applyCandleMarkers(chartData, [...spikeMarkers, ...navMarkers]);
}

function applyBacktestTradeMarkerPrimitive(chartData, trades) {
  if (!chartData?.tradeMarkerPlugin) return;
  chartData.tradeMarkerPlugin.setData(buildTradeMarkerPrimitiveData(trades));
}

function applySimOverlay(context = 'backtest', payload = {}) {
  const chartData = typeof _ctxData === 'function' ? _ctxData(context) : null;
  if (!chartData?.chart) return;

  const store = context === 'backtest' ? backtestStore : _liveStore();

  const overlayData = typeof store.getSimOverlayPayload === 'function'
    ? store.getSimOverlayPayload()
    : store.getForLightweightCharts();

  applyOscillatorToChart(chartData, overlayData.osc, overlayData.annotations);
  if (typeof applyWozduhVisibilityToChart === 'function') {
    applyWozduhVisibilityToChart(chartData, context);
  }

  if (payload.navigators) {
    applyNavigatorOverlays({ navigators: payload.navigators }, null, chartData, {
      context,
      preserveView: true,
      updateLoadedCandles: false,
    });
  }

  const trades = typeof store.getTrades === 'function' ? store.getTrades() : [];
  if (typeof applyBacktestCandleMarkers === 'function') {
    applyBacktestCandleMarkers(
      chartData,
      trades,
      overlayData.osc,
      typeof _btNavMarkers !== 'undefined' ? _btNavMarkers : null,
    );
  }
  if (typeof applyBacktestTradeMarkerPrimitive === 'function') {
    applyBacktestTradeMarkerPrimitive(chartData, trades);
  }
}

function initEquityChart() {
  const container = document.getElementById('equity-chart');
  if (!container || typeof LightweightCharts === 'undefined') return;

  if (_equityResizeObserver) {
    _equityResizeObserver.disconnect();
    _equityResizeObserver = null;
  }
  if (_equityChart) {
    try { _equityChart.remove(); } catch { /* noop */ }
    _equityChart = null;
    _equitySeries = null;
  }

  const width = Math.max(container.clientWidth || 0, 800);
  const height = Math.max(container.clientHeight || 0, 300);

  _equityChart = LightweightCharts.createChart(container, {
    autoSize: false,
    width,
    height,
    layout: { ...sharedChartLayout(), background: { type: 'solid', color: TV.bg }, textColor: _themeColor('textBright', '#d1d4dc') },
    grid: {
      vertLines: { color: TV.grid },
      horzLines: { color: TV.grid },
    },
    crosshair: { mode: LightweightCharts.CrosshairMode.Normal },
    timeScale: {
      borderColor: TV.border,
      timeVisible: true,
      secondsVisible: false,
    },
    rightPriceScale: {
      borderColor: TV.border,
      autoScale: true,
    },
  });

  _equitySeries = _equityChart.addLineSeries({
    color: _themeColor('bull', TV.green),
    lineWidth: 2,
    priceLineVisible: false,
    lastValueVisible: true,
  });

  _equityResizeObserver = new ResizeObserver((entries) => {
    if (!entries?.length || !_equityChart) return;
    const { width: w, height: h } = entries[0].contentRect;
    if (w > 0 && h > 0) {
      _equityChart.applyOptions({ width: w, height: h });
    }
  });
  _equityResizeObserver.observe(container);
}

function resizeEquityChart() {
  const container = document.getElementById('equity-chart');
  if (_equityChart && container) {
    const rect = container.getBoundingClientRect();
    const w = rect.width || container.clientWidth || 800;
    const h = rect.height || container.clientHeight || 300;
    _equityChart.applyOptions({ width: w, height: h });
  }
}

function setAllPriceData(candles) {
  applyPriceToChart(_live, candles);
}

function updateAllPriceSeries(bar) {
  const normalized = normalizeCandle(bar);
  if (!normalized) return;
  _live.candleSeries.update(normalized);
  _live.barSeries.update(normalized);
  _live.lineSeries.update({ time: normalized.time, value: normalized.close });
  if (normalized.volume != null) {
    _live.volumeSeries.update({
      time: normalized.time,
      value: normalized.volume,
      color: normalized.close >= normalized.open ? CHART_STYLES.volumeBar.upColor : CHART_STYLES.volumeBar.downColor,
    });
    const liveCandles = _liveCandles();
    ToolbarController.updateVolume(liveCandles.length ? liveCandles : [normalized]);
  }
}

function rulerPriceSeries(chartData) {
  if (chartData === _live) return activePriceSeries();
  return chartData?.candleSeries;
}

function attachRulerToChart(chartData) {
  if (!chartData?.chart || chartData.rulerAttached) return;
  ensureRulerElements(chartData);

  const clickHandler = (param) => onRulerClick(param, chartData);
  const moveHandler = (param) => onRulerMove(param, chartData);
  chartData.chart.subscribeClick(clickHandler);
  chartData.chart.subscribeCrosshairMove(moveHandler);
  chartData._rulerClickHandler = clickHandler;
  chartData._rulerMoveHandler = moveHandler;
  chartData.rulerAttached = true;
}

function fmtPrice(v) {
  return typeof v === 'number' && Number.isFinite(v)
    ? v.toLocaleString(undefined, { minimumFractionDigits: 1, maximumFractionDigits: 1 })
    : '—';
}

function updateRulerOverlay(chartData) {
  chartData = chartData || getRulerChartData();
  ensureRulerElements(chartData);

  const shade = chartData.rulerShade;
  const tooltip = chartData.rulerTooltip;
  const wrap = chartData.elements?.priceWrap;
  const series = rulerPriceSeries(chartData);

  if (!shade || !tooltip || !wrap || !series) return;

  if (ruler.state === RULER_IDLE || !ruler.p1 || !ruler.p2) {
    shade.style.display = 'none';
    tooltip.style.display = 'none';
    return;
  }

  const x1 = chartData.chart.timeScale().timeToCoordinate(ruler.p1.time);
  const x2 = chartData.chart.timeScale().timeToCoordinate(ruler.p2.time);
  const y1 = series.priceToCoordinate(ruler.p1.price);
  const y2 = series.priceToCoordinate(ruler.p2.price);

  if (x1 == null || x2 == null || y1 == null || y2 == null) {
    shade.style.display = 'none';
    tooltip.style.display = 'none';
    return;
  }

  const left = Math.min(x1, x2);
  const top = Math.min(y1, y2);
  const width = Math.abs(x2 - x1);
  const height = Math.abs(y2 - y1);

  shade.style.display = 'block';
  shade.style.left = `${left}px`;
  shade.style.top = `${top}px`;
  shade.style.width = `${Math.max(width, 2)}px`;
  shade.style.height = `${Math.max(height, 2)}px`;

  const delta = ruler.p2.price - ruler.p1.price;
  const pct = ruler.p1.price !== 0 ? (delta / ruler.p1.price) * 100 : 0;
  const bars = countBarsBetween(ruler.p1.time, ruler.p2.time, chartData);
  const sign = delta >= 0 ? '+' : '';

  tooltip.style.display = 'block';
  tooltip.innerHTML = `
    <div class="pct">${sign}${fmtPrice(delta)} (${sign}${pct.toFixed(2)}%)</div>
    <div>${formatDuration(bars)}</div>
  `;

  const tipLeft = left + width / 2 - 60;
  tooltip.style.left = `${Math.max(4, Math.min(tipLeft, wrap.clientWidth - 140))}px`;
  tooltip.style.top = `${Math.max(4, top - 52)}px`;
}



function _applyFullDataInternal(context, storeData, options = {}) {
  const chartData = _ctxData(context);
  if (!chartData?.candleSeries) return;
  applyPriceToChart(chartData, storeData.candles, options);
  if (context === 'live' && typeof ToolbarController !== 'undefined') {
    ToolbarController.updateVolume(storeData.candles);
  }
  // Phase 6 DDR cutover: legacy oscillator paint disabled for live chart.
  if (!ddrOscCutoverActive(context)) {
    applyOscillatorToChart(chartData, storeData.osc, storeData.annotations);
    const wozPrefs = options.wozduhPrefs;
    if (wozPrefs) {
      applyWozduhVisibilityFromPrefs(chartData, wozPrefs);
    } else if (typeof getWozduhPrefsForChart === 'function') {
      applyWozduhVisibilityToChart(chartData, context);
    }
  }
  if (context === 'live' && !options.skipAnnotations) {
    applyLiveAnnotationLayer(storeData, options);
  }
}

function applyHistoryPrepend(context, storeData, addedBars = 0) {
  const chartData = _ctxData(context);
  if (!chartData?.chart) return;

  const ts = chartData.chart.timeScale();
  const prevRange = ts.getVisibleLogicalRange();

  _applyFullDataInternal(context, storeData);

  if (prevRange && addedBars > 0) {
    ts.setVisibleLogicalRange({
      from: prevRange.from + addedBars,
      to: prevRange.to + addedBars,
    });
  }
}

function applyWozduhVisibilityFromPrefs(chartData, prefs) {
  if (!chartData?.wozduxSeries) return;
  WOZDUH_MENU_ITEMS.forEach((item) => {
    const visible = prefs[item.prefKey] !== undefined ? !!prefs[item.prefKey] : item.default;
    item.keys.forEach((key) => {
      if (chartData.wozduxSeries[key]) {
        chartData.wozduxSeries[key].applyOptions({ visible });
      }
    });
  });
}

function refreshPriceMarkersFromStore() {
  const store = _liveStore();
  if (!store) return;
  _rebuildPriceMarkerCache(store.getAnnotationsMap());
  _flushPriceMarkerCache();
}

function applyAllMarkersFromState() {
  refreshPriceMarkersFromStore();
}

function _applyDeltaInternal(context, delta, options = {}) {
  if (!delta) return;
  if (!_shouldPaint(context)) return;
  const chartData = _ctxData(context);

  if (delta.candle && context === 'live') {
    const barCount = Number.isFinite(delta.barCount) ? delta.barCount : (_liveStore()?.barCount() || 0);
    if (barCount <= 1) {
      const candles = _liveCandles();
      setAllPriceData(candles.length ? candles : [delta.candle]);
      _live.volumeSeries?.setData(toVolumeBars(candles.length ? candles : [delta.candle]));
      if (typeof ToolbarController !== 'undefined') {
        ToolbarController.updateVolume(candles.length ? candles : [delta.candle]);
      }
    } else {
      updateAllPriceSeries(delta.candle);
    }
  }

  if (delta.osc && !ddrOscCutoverActive(context)) {
    applyOscPointDelta(delta.osc, chartData);
  }

  if (delta.fullAnnotations) {
    const showPivots = options.showPivots !== false;
    const rsxTimes = delta.osc?.time != null ? new Set([delta.osc.time]) : new Set();
    applyUniversalAnnotations(
      getChartAnnotationPanes(chartData),
      delta.fullAnnotations,
      { rsx: rsxTimes },
      { showPivots },
    );
  }

  if (context === 'live' && delta.annotationMs != null) {
    _patchSpikeMarkersAtMs(delta.annotationMs, delta.annotation);
    if (delta.annotation?.volCross !== undefined) {
      _patchWozduhMarkerAtMs(delta.annotationMs, delta.annotation);
      _flushWozduhMarkerCache(chartData);
    }
    _flushPriceMarkerCache();
  }

  const after = _hooks[context]?.onAfterDelta;
  if (typeof after === 'function') after();
}

function destroyLiveCharts() {
  if (!_live?.chart) {
    _live = {};
    return;
  }

  try {
    clearFibLines();
  } catch {
    _fibPriceLines = [];
  }
  _liveNavLines = [];
  _liveNavMarkers = [];
  _cachedPriceMarkers = [];
  _cachedWozduhMarkers = [];

  if (_live.scaleControlsEl) {
    _live.scaleControlsEl.remove();
    _live.scaleControlsEl = null;
  }
  document.querySelectorAll('.scale-controls').forEach((el) => el.remove());

  destroyChartInstance(_live);

  const selectors = (typeof CONFIG !== 'undefined' && CONFIG.LIVE_CHART_SELECTORS) || {
    chartContainer: 'price-chart',
    oscContainer: 'wozduh-chart',
    rsxContainer: 'rsx-chart',
  };
  [selectors.chartContainer, selectors.oscContainer, selectors.rsxContainer].forEach((id) => {
    const el = document.getElementById(id);
    if (el) el.innerHTML = '';
  });

  _live = {};
  _chartInitialized = false;
}

function _clearSeries(context) {
  const chartData = _ctxData(context);
  if (!chartData?.candleSeries) return;
  chartData.candleSeries.setData([]);
  chartData.barSeries?.setData([]);
  chartData.lineSeries?.setData([]);
  chartData.volumeSeries?.setData([]);
  chartData.rsxSeries?.setData([]);
  chartData.rsxSeries?.setMarkers([]);
  chartData.rsxSignalSeries?.setData([]);
  if (chartData.tradeMarkerPlugin) chartData.tradeMarkerPlugin.setData([]);
  if (chartData.entryMarkerSeries) chartData.entryMarkerSeries.setData([]);
  Object.values(chartData.wozduxSeries || {}).forEach((s) => s.setData([]));
  chartData.wozduxSeries?.rsiVolSlow?.setMarkers([]);
  chartData.candleSeries.setMarkers([]);
  clearNavigatorOverlays(chartData);
  if (context === 'live') clearFibLines();
}

function _initChartPair(containerId, context, options = {}) {
  const chartData = initMultiPaneCharts(context, containerId, options);
  if (!chartData) return null;
  chartData._context = context;
  if (context === 'live') _live = chartData;
  else _backtest = chartData;

  const hooks = _hooks[context];
  attachChartHooks(chartData, {
    crosshairPriceSeries: hooks.crosshairPriceSeries || (() => chartData.candleSeries),
    onVisibleRangeChange: hooks.onVisibleRangeChange,
    onScrollHistory: hooks.onScrollHistory,
  }, context);
  return chartData;
}

const ChartAdapter = {
  initLiveCharts(selectors = LIVE_CHART_SELECTORS, hooks = {}) {
    if (typeof LightweightCharts === 'undefined') return false;
    if (_live?.chart) return true;
    if (hooks && Object.keys(hooks).length > 0) {
      _hooks.live = { ..._hooks.live, ...hooks };
    }
    _live = _initChartPair('live-chart-container', 'live', {
      selectors,
      navigatorPlugin: true,
    }) || {};
    if (!_live.chart) return false;
    _wozduxPriceLines = _live.wozduxPriceLines;
    _rsxPriceLines = _live.rsxPriceLines;
    if (typeof _hooks.live.onLiveInit === 'function') _hooks.live.onLiveInit(_live);
    attachRulerToChart(_live);
    _chartInitialized = true;
    return true;
  },

  initBacktestCharts(selectors = BACKTEST_CHART_SELECTORS, hooks = {}) {
    _hooks.backtest = hooks;
    _backtest = _initChartPair('backtest-chart-container', 'backtest', {
      selectors,
      overlayTrades: true,
      navigatorPlugin: true,
      tradeMarkers: true,
    }) || {};
    if (!_backtest.chart) return false;
    applyWozduhVisibilityToChart(_backtest, 'backtest');
    if (typeof hooks.onBacktestInit === 'function') hooks.onBacktestInit(_backtest);
    return true;
  },

  ensureBacktestChart() {
    return ensureBacktestChart();
  },

  activateSurface(context) {
    const chartData = _ctxData(context);
    if (!chartData || !chartData.chart || !chartData.root) return false;
    const w = chartData.root.clientWidth;
    const h = chartData.root.clientHeight;
    const ready = w > 0 && h > 0;
    if (!ready) return false;
    resizeChartInstance(chartData);
    return ready;
  },

  applyFullData(context, storeData, options = {}) {
    _applyFullDataInternal(context, storeData, options);
  },

  applyLiveAnnotationLayer(storeData, options = {}) {
    applyLiveAnnotationLayer(storeData, options);
  },

  applySimOverlay(context = 'backtest', payload = {}) {
    applySimOverlay(context, payload);
  },

  applyHistoryPrepend(context, storeData, addedBars = 0) {
    applyHistoryPrepend(context, storeData, addedBars);
  },

  applyDelta(context, delta, options = {}) {
    _applyDeltaInternal(context, delta, options);
  },

  setNavigatorOverlay(context, result, candles, options = {}) {
    const chartData = _ctxData(context);
    return applyNavigatorOverlays(result, candles, chartData, { ...options, context });
  },

  /** @internal — chart handle for ViewportManager */
  getChartHandle(context) {
    return _ctxData(context);
  },

  getChart(context, pane = 'price') {
    const data = _ctxData(context);
    if (!data) return null;
    if (data.charts) {
      if (pane === 'osc' || pane === 'wozduh') return data.charts.wozduh || null;
      if (pane === 'rsx') return data.charts.rsx || null;
      return data.charts.price || data.chart || null;
    }
    return data.chart || null;
  },

  isInitialized(context) {
    return !!_ctxData(context)?.chart;
  },

  clearSeries(context) {
    _clearSeries(context);
  },

  destroyLiveCharts() {
    destroyLiveCharts();
  },

  setChartType(type) {
    setChartType(type);
  },

  applyWozduhVisibility(context, prefs) {
    const chartData = _ctxData(context);
    if (prefs) applyWozduhVisibilityFromPrefs(chartData, prefs);
    else applyWozduhVisibilityToChart(chartData, context);
  },

  resize(contextOrEvent) {
    if (contextOrEvent === 'live' || contextOrEvent === 'backtest') {
      const id = contextOrEvent === 'live' ? 'live-chart-container' : 'backtest-chart-container';
      resizeChartForContainer(_ctxData(contextOrEvent), document.getElementById(id));
      return;
    }
    handleResize();
  },

  handleResize() { handleResize(); },

  fitContent(context) {
    fitProfessionalChart(_ctxData(context));
  },

  setPaneVisibility(context, paneKey, isVisible) {
    const chartData = _ctxData(context);
    if (!chartData?.elements) return;
    const wrap = paneKey === 'wozduh' || paneKey === 'osc'
      ? chartData.elements.oscWrap
      : paneKey === 'rsx'
        ? chartData.elements.rsxWrap
        : chartData.elements.priceWrap;
    if (wrap) wrap.style.display = isVisible ? '' : 'none';
    resizeChartInstance(chartData);
  },

  applyOrderFlowTimeScale(enabled) { applyOrderFlowTimeScale(enabled); },

  reinitBacktest(hooks = {}) {
    _hooks.backtest = { ..._hooks.backtest, ...hooks };
    return reinitBacktestChart();
  },

  applyIndicatorStyles(context) {
    applyIndicatorConfigStyles(_ctxData(context));
  },

  renderFib(zones) { renderFibZones(zones); },

  clearFib() { clearFibLines(); },

  setTradeMarkers(markers) {
    _tradeMarkers = markers || [];
    _cachedPriceMarkers = [..._tradeMarkers, ..._spikeMarkers].sort((a, b) => a.time - b.time);
  },

  setSpikeMarkers(markers) {
    _spikeMarkers = markers || [];
    _cachedPriceMarkers = [..._tradeMarkers, ..._spikeMarkers].sort((a, b) => a.time - b.time);
  },

  applyAllMarkers() { refreshPriceMarkersFromStore(); },

  refreshPriceMarkers() { refreshPriceMarkersFromStore(); },

  initEquity() { initEquityChart(); },

  resizeEquity() { resizeEquityChart(); },

  setEquityData(points) {
    _equitySeries?.setData(points || []);
  },

  fitEquityContent() {
    _equityChart?.timeScale()?.fitContent();
  },

  setLegendVisibility(context, pane, legendId, visible, chartTypeMode) {
    const chartData = _ctxData(context);
    if (legendId === 'price') {
      const mode = chartTypeMode || _chartType;
      const series = mode === 'bars' ? chartData.barSeries
        : mode === 'line' ? chartData.lineSeries
          : chartData.candleSeries;
      series?.applyOptions({ visible });
      chartData.candleSeries?.applyOptions({ visible: mode === 'candles' ? visible : false });
      chartData.barSeries?.applyOptions({ visible: mode === 'bars' ? visible : false });
      chartData.lineSeries?.applyOptions({ visible: mode === 'line' ? visible : false });
      chartData.volumeSeries?.applyOptions({ visible });
    } else if (legendId === 'wozduh') {
      if (visible) applyWozduhVisibilityToChart(chartData, context);
      else Object.values(chartData.wozduxSeries || {}).forEach((s) => s?.applyOptions({ visible: false }));
      ChartAdapter.setPaneVisibility(context, 'wozduh', visible);
    } else if (legendId === 'rsx') {
      chartData.rsxSeries?.applyOptions({ visible });
      chartData.rsxSignalSeries?.applyOptions({ visible });
      if (!visible) {
        chartData.rsxSeries?.applyOptions({ visible: false });
        chartData.rsxSignalSeries?.applyOptions({ visible: false });
      }
      ChartAdapter.setPaneVisibility(context, 'rsx', visible);
    } else if (legendId === 'trendlines') {
      const plugin = getNavigatorPluginForPane(chartData, pane);
      plugin?.setLinesVisible(visible);
    } else if (legendId === 'trades') {
      chartData.tradeMarkerPlugin?.setVisible(visible);
    }
  },

  attachRuler(chartData) { attachRulerToChart(chartData || _live); },

  initRuler() {
    attachRulerToChart(_live);
    attachRulerToChart(_backtest);
  },

  updateRulerOverlay(chartData) { updateRulerOverlay(chartData); },

  rulerPriceSeries(chartData) { return rulerPriceSeries(chartData); },

  setLiveUpdating(v) { _liveUpdating = !!v; },

  isLiveUpdating() { return _liveUpdating; },

  getChartType() { return _chartType; },

  reapplyCachedNavigator(context) {
    reapplyCachedNavigatorPlugins(_ctxData(context));
  },

  getNavigatorPlugin(chartData, pane) {
    return getNavigatorPluginForPane(chartData, pane);
  },

  chartInitialized() { return _chartInitialized; },

  setChartInitialized(v) { _chartInitialized = !!v; },

  enableDDROscCutover() {
    _ddrOscCutover = true;
  },

  isDDROscCutover() {
    return _ddrOscCutover;
  },

  hideLegacyOscillatorSeries(context = 'live') {
    const chartData = _ctxData(context);
    if (!chartData) return;
    chartData.rsxSeries?.applyOptions({ visible: false });
    chartData.rsxSignalSeries?.applyOptions({ visible: false });
    Object.values(chartData.wozduxSeries || {}).forEach((series) => {
      series?.applyOptions({ visible: false });
    });
  },

  applyRsxData(context, osc, annotations) {
    applyRsxData(osc, _ctxData(context), annotations);
  },

  syncVisibleLogicalRange(chart, range, options = {}) {
    syncVisibleLogicalRange(chart, range, options);
  },

  setVisibleLogicalRange(context, range, options = {}) {
    setVisibleLogicalRangeAll(_ctxData(context), range, options);
  },

  applyBacktestMarkers(trades, osc) {
    applyBacktestCandleMarkers(_backtest, trades, osc, _btNavMarkers);
    applyBacktestTradeMarkerPrimitive(_backtest, trades);
  },

  getVisibleLogicalRange(context) {
    return _ctxData(context)?.chart?.timeScale()?.getVisibleLogicalRange() ?? null;
  },

  rulerPointFromParam(chartData, param) {
    if (!param?.point || !chartData?.chart) return null;
    const series = rulerPriceSeries(chartData);
    const price = series?.coordinateToPrice(param.point.y);
    const time = chartData.chart.timeScale().coordinateToTime(param.point.x);
    if (price == null || time == null) return null;
    return { time, price };
  },

  setToggleSeriesVisible(context, key, visible) {
    if (typeof SettingsRenderer !== 'undefined' && SettingsRenderer.setToggleVisible(context, key, visible)) {
      return;
    }
    const chartData = _ctxData(context);
    const series = key === 'rsx' ? chartData?.rsxSeries : chartData?.volumeSeries;
    series?.applyOptions({ visible });
  },
};

if (typeof window !== 'undefined') {
  window.ChartAdapter = ChartAdapter;
}
