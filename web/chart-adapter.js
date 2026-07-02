/**
 * ChartAdapter — Lightweight Charts facade.
 * app.js must not call LWC API directly; use window.ChartAdapter only.
 */
const CHART_PRICE_SCALE_MIN_WIDTH = 70;
const LOGICAL_RANGE_EPS = 0.01;

let _live = {};
let _backtest = {};
let _equityChart;
let _equitySeries;
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
let _hooks = { live: {}, backtest: {} };
let _chartInitialized = false;

function _ctxData(context) {
  return context === 'backtest' ? _backtest : _live;
}

function _chartContext(chartData) {
  if (!chartData) return 'live';
  if (chartData._context) return chartData._context;
  return chartData === _backtest ? 'backtest' : 'live';
}

function _storeForChart(chartData) {
  return _chartContext(chartData) === 'backtest' ? backtestStore : liveStore;
}

function _storeDataForChart(chartData) {
  return _storeForChart(chartData).getForLightweightCharts();
}

function _storeForContext(context) {
  return context === 'backtest' ? backtestStore : liveStore;
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
  if (!chartData) return null;
  if (pane === 'rsx') return chartData.rsxChart;
  if (pane === 'wozduh') return chartData.oscChart;
  return chartData.priceChart;
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

  const priceChart = getNavigatorPriceChart(chartData, pane);
  if (!priceChart) return null;

  const anchorSeries = createMTFOverlaySeries(priceChart, {
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

function chartLocalizationOptions() {
  const locale = navigator.language || 'en-US';
  return {
    locale,
    timeFormatter: (time) => unixChartTimeToDate(time).toLocaleTimeString(locale, {
      hour: '2-digit',
      minute: '2-digit',
      hour12: false,
    }),
    dateFormatter: (time) => unixChartTimeToDate(time).toLocaleDateString(locale, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    }),
  };
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
    autoSize: true,
    layout: sharedChartLayout(),
    localization: chartLocalizationOptions(),
    grid: {
      vertLines: { color: TV.grid, style: LightweightCharts.LineStyle.Dotted },
      horzLines: { color: TV.grid, style: LightweightCharts.LineStyle.Dotted },
    },
    crosshair: { ...SHARED_CROSSHAIR },
    timeScale: { ...SHARED_TIME_SCALE },
    width,
    height,
    rightPriceScale: sharedRightPriceScaleOptions({
      mode: LightweightCharts.PriceScaleMode.Logarithmic,
    }),
  };
}

function createWozduxChartOptions(width, height) {
  return {
    autoSize: true,
    layout: sharedChartLayout(),
    localization: chartLocalizationOptions(),
    grid: {
      vertLines: { color: TV.grid, style: LightweightCharts.LineStyle.Dotted },
      horzLines: { color: TV.grid, style: LightweightCharts.LineStyle.Dotted },
    },
    crosshair: { ...SHARED_CROSSHAIR },
    timeScale: { ...SHARED_TIME_SCALE },
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
    autoSize: true,
    layout: sharedChartLayout(),
    localization: chartLocalizationOptions(),
    grid: {
      vertLines: { color: TV.grid, style: LightweightCharts.LineStyle.Dotted },
      horzLines: { color: TV.grid, style: LightweightCharts.LineStyle.Dotted },
    },
    crosshair: { ...SHARED_CROSSHAIR },
    timeScale: { ...SHARED_TIME_SCALE },
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
  if (oscPt.volCrossMarker) {
    applyWozduxMarkers(_storeDataForChart(chartData).osc, chartData);
  }

  const rsxVal = parseFloat(oscPt.rsx ?? oscPt.jurik);
  if (Number.isFinite(rsxVal) && chartData.rsxSeries) {
    const ptColor = oscPt.color || RSX_DEFAULT_COLOR;
    chartData.rsxSeries.update({ time, value: rsxVal, color: ptColor });
    const signalVal = parseFloat(oscPt.rsx_signal ?? oscPt.rsxSignal);
    if (chartData.rsxSignalSeries && Number.isFinite(signalVal)) {
      chartData.rsxSignalSeries.update({ time, value: signalVal });
    }
    if (_chartContext(chartData) === 'live') {
      const rsxEl = document.getElementById('rsx-val');
      if (rsxEl) {
        rsxEl.textContent = rsxVal.toFixed(1);
        rsxEl.style.color = ptColor;
      }
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
    return chartData.priceChart.priceScale('right').options().autoScale !== false;
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
  if (!chartData?.priceChart) return;
  const state = normalizePriceScaleView(saved);
  chartData.priceScaleState = state;
  chartData.priceScaleMode = state.logEnabled ? 'log' : 'normal';
  chartData.priceChart.priceScale('right').applyOptions({
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
  if (!chartData?.priceChart) return;
  const state = { ...(chartData.priceScaleState || defaultPriceScaleState()) };
  state.logEnabled = !state.logEnabled;
  chartData.priceScaleState = state;
  chartData.priceScaleMode = state.logEnabled ? 'log' : 'normal';
  chartData.priceChart.priceScale('right').applyOptions({
    mode: state.logEnabled
      ? LightweightCharts.PriceScaleMode.Logarithmic
      : LightweightCharts.PriceScaleMode.Normal,
  });
  syncPriceScaleControlsUI(chartData);
}

function enablePriceAutoScale(chartData) {
  if (!chartData?.priceChart) return;
  const state = { ...(chartData.priceScaleState || defaultPriceScaleState()) };
  state.autoScale = true;
  chartData.priceScaleState = state;
  chartData.priceChart.priceScale('right').applyOptions({ autoScale: true });
  syncPriceScaleControlsUI(chartData);
}

function isPointerOnPriceScale(chartData, clientX) {
  const container = chartData?.elements?.chartContainer;
  if (!container || !chartData?.priceChart) return false;
  const rect = container.getBoundingClientRect();
  const scaleW = chartData.priceChart.priceScale('right').width() || CHART_PRICE_SCALE_MIN_WIDTH;
  return clientX >= rect.right - scaleW;
}

function attachPriceScaleInteractionWatch(chartData) {
  const container = chartData?.elements?.chartContainer;
  if (!container) return;
  container._priceScaleChartData = chartData;

  const syncAutoFromChart = () => {
    const active = container._priceScaleChartData;
    if (!active?.priceChart) return;
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
}

function initPriceScaleControls(chartData) {
  const wrap = chartData?.elements?.priceWrap;
  if (!wrap || !chartData?.priceChart) return;

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
  if (!chartData?.allCharts?.length) return;
  chartData.allCharts.forEach((chart) => {
    try {
      chart.remove();
    } catch {
      /* noop */
    }
  });
  ['oscWrap', 'rsxWrap', 'priceWrap'].forEach((key) => {
    const wrap = chartData.elements?.[key];
    if (wrap?._paneResizeObs) {
      wrap._paneResizeObs.disconnect();
      wrap._paneResizeObs = null;
    }
  });
}

function exitFullscreenPane() {
  document.querySelectorAll('.fullscreen-pane').forEach((el) => {
    el.classList.remove('fullscreen-pane');
  });
  handleResize();
}

function toggleFullscreenPane(wrapEl, chartData) {
  if (!wrapEl) return;
  const entering = !wrapEl.classList.contains('fullscreen-pane');
  exitFullscreenPane();
  if (entering) wrapEl.classList.add('fullscreen-pane');
  requestAnimationFrame(() => {
    const root = document.getElementById(chartData.containerId);
    if (root) resizeChartForContainer(chartData, root);
  });
}

function initPaneFullscreen(chartData) {
  if (!chartData?.elements) return;
  ['priceWrap', 'oscWrap', 'rsxWrap'].forEach((key) => {
    const wrap = chartData.elements[key];
    if (!wrap || wrap._fullscreenBound) return;
    wrap._fullscreenBound = true;
    wrap.addEventListener('dblclick', (e) => {
      if (e.target.closest('.indicator-settings-menu, button')) return;
      toggleFullscreenPane(wrap, chartData);
    });
  });
}

function initPaneVerticalResize(chartData) {
  if (!chartData?.elements) return;
  ['priceWrap', 'oscWrap', 'rsxWrap'].forEach((key) => {
    const wrap = chartData.elements[key];
    if (!wrap || wrap._paneResizeObs) return;
    wrap._paneResizeObs = new ResizeObserver(() => {
      const root = document.getElementById(chartData.containerId);
      if (root) resizeChartForContainer(chartData, root);
    });
    wrap._paneResizeObs.observe(wrap);
  });
}

function setupChartPaneUX(chartData) {
  initPaneVerticalResize(chartData);
  initPaneFullscreen(chartData);
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
  _backtest = initProfessionalChart('backtest-chart-container', {
    selectors: BACKTEST_CHART_SELECTORS,
    overlayTrades: true,
    navigatorPlugin: true,
    tradeMarkers: true,
  }) || {};
  applyWozduhVisibilityToChart(_backtest, 'backtest');
  attachProfessionalChartSync(_backtest, {
    crosshairPriceSeries: _hooks.backtest.crosshairPriceSeries || (() => _backtest.candleSeries),
    onVisibleRangeChange: _hooks.backtest.onVisibleRangeChange,
    onScrollHistory: _hooks.backtest.onScrollHistory,
  });
  attachRulerToChart(_backtest);
  renderChartLegends('backtest');
  return _backtest;
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

function createAreaSeries(chart, areaCfg) {
  const series = chart.addAreaSeries({
    topColor: areaCfg.topColor,
    bottomColor: areaCfg.bottomColor,
    lineColor: areaCfg.lineColor || 'transparent',
    lineWidth: areaCfg.lineWidth ?? 0,
    priceLineVisible: false,
    lastValueVisible: false,
    crosshairMarkerVisible: false,
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

function initProfessionalChart(containerId, options = {}) {
  if (typeof LightweightCharts === 'undefined') return null;

  const selectors = options.selectors || LIVE_CHART_SELECTORS;
  const root = document.getElementById(containerId);
  const chartContainer = document.getElementById(selectors.chartContainer);
  const oscContainer = document.getElementById(selectors.oscContainer);
  const rsxContainer = document.getElementById(selectors.rsxContainer);
  const priceWrap = document.getElementById(selectors.priceWrap);
  const oscWrap = document.getElementById(selectors.oscWrap);
  const rsxWrap = document.getElementById(selectors.rsxWrap);

  if (!root || !chartContainer || !oscContainer || !rsxContainer) return null;
  if (!ensureChartLibraryStyles()) return null;

  const CHART_MIN_PRICE_PANE_H = 400;
  const CHART_MIN_OSC_PANE_H = 150;
  const width = Math.max(chartContainer.clientWidth || 0, root.clientWidth || 0, 800);
  const priceH = Math.max(priceWrap?.clientHeight || 0, CHART_MIN_PRICE_PANE_H);
  const oscH = Math.max(oscWrap?.clientHeight || 0, CHART_MIN_OSC_PANE_H);
  const rsxH = Math.max(rsxWrap?.clientHeight || 0, CHART_MIN_OSC_PANE_H);

  const priceChart = LightweightCharts.createChart(
    chartContainer,
    createPriceChartOptions(width, priceH),
  );
  const oscChart = LightweightCharts.createChart(
    oscContainer,
    createWozduxChartOptions(oscContainer.clientWidth || width, oscH),
  );
  const rsxChart = LightweightCharts.createChart(
    rsxContainer,
    createRSXChartOptions(rsxContainer.clientWidth || width, rsxH),
  );

  const priceCfg = INDICATOR_CONFIG.price;
  const candleSeries = priceChart.addCandlestickSeries(priceCfg.candle);
  const barSeries = priceChart.addBarSeries(priceCfg.bar);
  const lineSeries = createMTFOverlaySeries(priceChart, {
    ...priceCfg.line,
    visible: priceCfg.line?.visible ?? false,
  });
  const volumeSeries = priceChart.addHistogramSeries(priceCfg.volume);
  priceChart.priceScale('volume').applyOptions(priceCfg.volumeScale);
  priceChart.priceScale('right').applyOptions({
    ...priceCfg.priceScale,
    minimumWidth: CHART_PRICE_SCALE_MIN_WIDTH,
  });
  oscChart.priceScale('right').applyOptions({ minimumWidth: CHART_PRICE_SCALE_MIN_WIDTH });
  rsxChart.priceScale('right').applyOptions({ minimumWidth: CHART_PRICE_SCALE_MIN_WIDTH });

  const wozduhAreas = {};
  INDICATOR_CONFIG.wozduh.areas.forEach((areaCfg) => {
    wozduhAreas[areaCfg.id] = { series: createAreaSeries(oscChart, areaCfg), cfg: areaCfg };
  });

  const wozduxSeries = {};
  Object.entries(INDICATOR_CONFIG.wozduh.lines).forEach(([seriesKey, lineCfg]) => {
    wozduxSeries[seriesKey] = oscChart.addLineSeries(
      wozduxLineSeriesOptions(lineCfg.style),
    );
    wozduxSeries[lineCfg.dataKey] = wozduxSeries[seriesKey];
  });

  const wozduhLevelAnchor = wozduxSeries.rsiPrice || wozduxSeries.wozduh_wt1;
  const wozduxPriceLinesLocal = addLevelLines(wozduhLevelAnchor, INDICATOR_CONFIG.wozduh.levels);

  const rsxAreas = {};
  INDICATOR_CONFIG.rsx.areas.forEach((areaCfg) => {
    rsxAreas[areaCfg.id] = { series: createAreaSeries(rsxChart, areaCfg), cfg: areaCfg };
  });

  const rsxLineCfg = INDICATOR_CONFIG.rsx.lines.rsx_main;
  const rsxSeries = rsxChart.addLineSeries(withSeriesDefaults(rsxLineCfg.style));
  const rsxSignalCfg = INDICATOR_CONFIG.rsx.lines.rsx_signal;
  const rsxSignalSeries = rsxChart.addLineSeries(withSeriesDefaults(rsxSignalCfg.style));
  const rsxPriceLinesLocal = addLevelLines(rsxSeries, INDICATOR_CONFIG.rsx.levels);

  let entryMarkerSeries = null;
  if (options.overlayTrades) {
    entryMarkerSeries = createMTFOverlaySeries(priceChart, {
      color: 'transparent',
      lineWidth: 0,
    });
  }

  let priceNavigatorPlugin = null;
  let rsxNavigatorPlugin = null;
  let wozduhNavigatorPlugin = null;
  let tradeMarkerPlugin = null;

  if (options.navigatorPlugin && typeof TrendlinePrimitive !== 'undefined') {
    priceNavigatorPlugin = new TrendlinePrimitive();
    candleSeries.attachPrimitive(priceNavigatorPlugin);

    rsxNavigatorPlugin = new TrendlinePrimitive();
    rsxSeries.attachPrimitive(rsxNavigatorPlugin);

    const wozduhSeries = wozduxSeries.rsiVolSlow || wozduxSeries.wozduh_wt2;
    if (wozduhSeries) {
      wozduhNavigatorPlugin = new TrendlinePrimitive();
      wozduhSeries.attachPrimitive(wozduhNavigatorPlugin);
    }
  }

  if (options.tradeMarkers && typeof TradeMarkerPrimitive !== 'undefined') {
    tradeMarkerPlugin = new TradeMarkerPrimitive();
    candleSeries.attachPrimitive(tradeMarkerPlugin);
  }

  const allCharts = [priceChart, oscChart, rsxChart];

  const result = {
    chart: priceChart,
    containerId,
    root,
    priceChart,
    oscChart,
    rsxChart,
    candleSeries,
    priceSeries: candleSeries,
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
    allCharts,
    elements: { priceWrap, oscWrap, rsxWrap, chartContainer, oscContainer, rsxContainer },
    syncingTimeScale: false,
    priceScaleState: defaultPriceScaleState(),
    priceScaleMode: 'log',
    rulerAttached: false,
  };

  setupChartPaneUX(result);
  initPriceScaleControls(result);
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

function captureLiveViewportSnapshot() {
  if (!_live?.priceChart?.timeScale) return null;
  try {
    const logicalRange = _live.priceChart.timeScale().getVisibleLogicalRange();
    if (!logicalRange) return null;
    return { logicalRange };
  } catch {
    return null;
  }
}

function restoreLiveViewportSnapshot(snapshot, options = {}) {
  if (!snapshot?.logicalRange || !_live?.allCharts?.length) return;
  const animate = options.animate === true;
  const lockAutoScale = options.lockAutoScale !== false;
  _live.allCharts.forEach((chart) => {
    const apply = () => syncVisibleLogicalRange(chart, snapshot.logicalRange, { animate });
    if (lockAutoScale && chart === _live.priceChart) {
      window.Viewport?.lockPriceAutoScaleDuring?.(chart, apply);
    } else {
      apply();
    }
  });
}

function restoreLiveViewportSnapshotTwice(snapshot) {
  restoreLiveViewportSnapshot(snapshot, { animate: false });
  requestAnimationFrame(() => {
    restoreLiveViewportSnapshot(snapshot, { animate: false });
    syncLiveChartPanesFromPrice();
  });
}

function syncVisibleTimeRange(targetChart, range) {
  if (!targetChart?.timeScale || !range) return;
  const timeScale = targetChart.timeScale();
  const currentRange = timeScale.getVisibleRange();
  if (logicalRangesEqual(currentRange, range)) return;
  timeScale.setVisibleRange(range);
}

function applyLiveViewportAfterData(chartData, klines, options = {}) {
  if (!chartData?.allCharts?.length || !klines.length) return;

  const {
    isPrepend = false,
    prevRange = null,
  } = options;

  const applyRange = (range, animate = false) => {
    chartData.allCharts.forEach((chart) => syncVisibleLogicalRange(chart, range, { animate }));
  };

  if (isPrepend) {
    return;
  }

  if (window.__pendingAnchor) {
    runWithSuppressedHistoryLoad(() => {
      window.Viewport?.restoreViewportToCharts(chartData, chartData.candleSeries, window.__pendingAnchor, {
        candles: klines,
        chartTime,
        animate: false,
      });
    });
    window.__pendingAnchor = null;
    return;
  }

  if (prevRange) {
    runWithSuppressedHistoryLoad(() => applyRange(prevRange, false));
  }
}

function syncChartGroupLogicalRange(charts, sourceChart, range) {
  if (!range || !charts?.length) return;
  charts.forEach((targetChart) => {
    if (targetChart !== sourceChart) {
      syncVisibleLogicalRange(targetChart, range);
    }
  });
}

function syncChartGroupTimeRange(charts, sourceChart, range) {
  if (!range || !charts?.length) return;
  charts.forEach((targetChart) => {
    if (targetChart !== sourceChart) {
      syncVisibleTimeRange(targetChart, range);
    }
  });
}

function syncPaneCrosshairs(chartGroups) {
  if (!chartGroups?.length) return;

  chartGroups.forEach((sourceData) => {
    const sourceChart = sourceData?.chart;
    if (!sourceChart || sourceChart._crosshairSyncBound) return;
    sourceChart._crosshairSyncBound = true;

    sourceChart.subscribeCrosshairMove((param) => {
      if (_liveUpdating) return;
      if (!param.sourceEvent) return;

      const container = sourceData.container;
      if (!container?.matches) return;
      if (!container.matches(':hover') && !container.matches(':active')) return;

      chartGroups.forEach((targetData) => {
        const targetChart = targetData?.chart;
        if (!targetChart || targetChart === sourceChart) return;

        if (!param.point || param.time === undefined) {
          if (typeof targetChart.clearCrosshairPosition === 'function') {
            targetChart.clearCrosshairPosition();
          }
          return;
        }

        const series = typeof targetData.seriesGetter === 'function'
          ? targetData.seriesGetter()
          : null;
        if (!series) return;

        const price = seriesValueAtTime(series, param.time);
        if (price != null) {
          targetChart.setCrosshairPosition(price, param.time, series);
        }
      });
    });
  });
}

function attachProfessionalChartSync(chartData, hooks = {}) {
  if (!chartData?.allCharts?.length) return;

  const priceSeriesGetter = hooks.crosshairPriceSeries || (() => chartData.candleSeries);
  const wozduhSeriesGetter = () => chartData.wozduxSeries?.rsiPrice || chartData.wozduhUpSeries;
  const rsxSeriesGetter = () => chartData.rsxSeries;
  const context = chartData._context || 'live';
  const scrollThreshold = (typeof CONFIG !== 'undefined' && CONFIG.LIVE_HISTORY_SCROLL_THRESHOLD) || 50;

  const { chartContainer, oscContainer, rsxContainer } = chartData.elements || {};

  chartData.allCharts.forEach((sourceChart) => {
    sourceChart.timeScale().subscribeVisibleLogicalRangeChange((range) => {
      const liveUpdatingBlock = context === 'live' && _liveUpdating;
      const suppressHook = context === 'live'
        && typeof liveHistorySuppressRangeHook !== 'undefined'
        && liveHistorySuppressRangeHook;
      if (liveUpdatingBlock || suppressHook || !range) return;

      syncChartGroupLogicalRange(chartData.allCharts, sourceChart, range);

      if (typeof hooks.onVisibleRangeChange === 'function') {
        hooks.onVisibleRangeChange(range);
      }
    });
  });

  if (chartData.priceChart) {
    chartData.priceChart.timeScale().subscribeVisibleLogicalRangeChange((range) => {
      const liveUpdatingBlock = context === 'live' && _liveUpdating;
      const suppressHook = context === 'live'
        && typeof liveHistorySuppressRangeHook !== 'undefined'
        && liveHistorySuppressRangeHook;
      if (liveUpdatingBlock || suppressHook || !range) return;
      if (typeof hooks.onScrollHistory === 'function' && range.from < scrollThreshold) {
        hooks.onScrollHistory(range);
      }
    });
  }

  const crosshairPeers = [
    { chart: chartData.priceChart, seriesGetter: priceSeriesGetter, container: chartContainer },
    { chart: chartData.oscChart, seriesGetter: wozduhSeriesGetter, container: oscContainer },
    { chart: chartData.rsxChart, seriesGetter: rsxSeriesGetter, container: rsxContainer },
  ].filter((entry) => entry.chart && entry.container);

  syncPaneCrosshairs(crosshairPeers);
}

function resetChartCrosshair(chart) {
  if (!chart) return;
  chart.applyOptions({ crosshair: { mode: LightweightCharts.CrosshairMode.Normal } });
  if (typeof chart.crosshair === 'function') {
    chart.crosshair().applyOptions({ mode: LightweightCharts.CrosshairMode.Normal });
  }
}

function applyChartPanelResize(chart, width, height) {
  if (!chart) return;
  const opts = { width: Math.max(1, width) };
  if (height > 0) opts.height = height;
  chart.applyOptions(opts);
  resetChartCrosshair(chart);
}

function resizeChartInstance(chartData, rootEl) {
  if (!chartData?.elements || !rootEl) return;
  const rootWidth = rootEl.clientWidth;
  const { chartContainer, oscContainer, rsxContainer, priceWrap, oscWrap, rsxWrap } = chartData.elements;

  applyChartPanelResize(
    chartData.priceChart,
    chartContainer?.clientWidth || rootWidth,
    priceWrap?.clientHeight,
  );
  applyChartPanelResize(
    chartData.oscChart,
    oscContainer?.clientWidth || rootWidth,
    oscWrap?.clientHeight,
  );
  applyChartPanelResize(
    chartData.rsxChart,
    rsxContainer?.clientWidth || rootWidth,
    rsxWrap?.clientHeight,
  );
}

function resizeChartForContainer(chartData, container) {
  if (!chartData?.chart || !container) return;
  resizeChartInstance(chartData, container);
}

function handleResize() {
  const activeTab = getActiveTabId();

  if (activeTab === 'tab-live') {
    resizeChartForContainer(
      _live,
      document.getElementById('live-chart-container'),
    );
  } else if (activeTab === 'tab-backtest') {
    resizeChartForContainer(
      _backtest,
      document.getElementById('backtest-chart-container'),
    );
  }

  if (ruler.active) {
    updateRulerOverlay(getRulerChartData());
  }
}

function fitChartInstance(chartData) {
  if (!chartData?.priceChart?.timeScale) return;
  chartData.priceChart.timeScale().fitContent();
  forceSyncChartTimeScales(chartData);
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

function forceSyncChartTimeScales(chartData) {
  if (!chartData?.priceChart?.timeScale || !chartData?.allCharts?.length) return;
  if (_liveUpdating) return;

  const applyRange = () => {
    if (_liveUpdating) return false;
    const range = chartData.priceChart.timeScale().getVisibleLogicalRange();
    if (!range || range.from == null || range.to == null) return false;

    chartData.allCharts.forEach((chart) => {
      syncVisibleLogicalRange(chart, range);
    });
    return true;
  };

  if (!applyRange()) {
    requestAnimationFrame(() => {
      applyRange();
      requestAnimationFrame(applyRange);
    });
    return;
  }
  requestAnimationFrame(applyRange);
}

function syncTimeScaleToAll(_sourceChart, range) {
  if (!range || !_live?.allCharts) return;
  if (_liveUpdating) return;

  _live.allCharts.forEach((chart) => {
    syncVisibleLogicalRange(chart, range);
  });
  requestAnimationFrame(() => forceSyncChartTimeScales(_live));
}

function seriesValueAtTime(series, time) {
  const data = series.data();
  for (let i = data.length - 1; i >= 0; i--) {
    const bar = data[i];
    if (bar.time <= time) {
      return bar.value ?? bar.close ?? null;
    }
  }
  return null;
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
  _chartType = type;
  _live.candleSeries.applyOptions({ visible: type === 'candles' });
  _live.barSeries.applyOptions({ visible: type === 'bars' });
  _live.lineSeries.applyOptions({ visible: type === 'line' });
}

function applyOrderFlowTimeScale(enabled) {
  if (!_live.priceChart) return;
  _live.priceChart.timeScale().applyOptions({
    secondsVisible: enabled,
    timeVisible: true,
  });
}

function clearFibLines() {
  _fibPriceLines.forEach((pl) => {
    try { _live.candleSeries.removePriceLine(pl); } catch (_) { /* noop */ }
  });
  _fibPriceLines = [];
}

function applyPriceToChart(chartData, candles, options = {}) {
  if (!chartData?.candleSeries) return;
  const sorted = [...(candles || [])].sort((a, b) => a.time - b.time);
  const lineData = toLineClose(sorted);
  lineData.sort((a, b) => a.time - b.time);
  chartData.candleSeries.setData(sorted);
  if (chartData.barSeries) chartData.barSeries.setData(sorted);
  if (chartData.lineSeries) chartData.lineSeries.setData(lineData);
  if (options.includeVolume !== false && chartData.volumeSeries) {
    chartData.volumeSeries.setData(toVolumeBars(sorted));
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

function wozduxMarkersFromOsc(osc) {
  const markers = [];
  (osc || []).forEach((d) => {
    if (!d.volCrossMarker) return;
    const time = chartTime(d.time);
    if (time == null) return;
    markers.push({
      time: Number(time),
      position: 'inBar',
      color: d.volCrossMarker,
      shape: 'circle',
      size: 1,
    });
  });
  return markers.sort((a, b) => a.time - b.time);
}

function applyWozduxData(osc, chartData = _live) {
  WOZDUX_LINE_KEYS.forEach((key) => {
    const series = chartData.wozduxSeries?.[key];
    if (series) series.setData(toLine(osc, key));
  });
  applyWozduxMarkers(osc, chartData);
}

function applyWozduxMarkers(osc, chartData = _live) {
  const markerKey = INDICATOR_CONFIG.wozduh.markerSeriesKey;
  const markerSeries = chartData.wozduxSeries?.[markerKey];
  if (markerSeries) {
    markerSeries.setMarkers(wozduxMarkersFromOsc(osc));
  }
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
    applyWozduxMarkers(_storeDataForChart(chartData).osc, chartData);
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

  if (_chartContext(chartData) === 'live') {
    const last = mappedRSX.length ? mappedRSX[mappedRSX.length - 1] : null;
    const rsxEl = document.getElementById('rsx-val');
    if (rsxEl && last) {
      rsxEl.textContent = Number.isFinite(last.value) ? last.value.toFixed(1) : '—';
      rsxEl.style.color = last.color || RSX_DEFAULT_COLOR;
    }
  }
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

  if (chartData === _live) {
    const rsxEl = document.getElementById('rsx-val');
    if (rsxEl) {
      rsxEl.textContent = value.toFixed(1);
      rsxEl.style.color = ptColor;
    }
  }
}

function applyAllMarkers() {
  const showSpike = document.getElementById('tog-spike').checked;
  const combined = [...tradeMarkers];
  if (showSpike) combined.push(...spikeMarkers);
  combined.sort((a, b) => a.time - b.time);
  _live.candleSeries.setMarkers(combined);
}

function renderFibZones(zones) {
  lastFibZones = zones || [];
  clearFibLines();
  if (!document.getElementById('tog-fib').checked) return;
  lastFibZones.forEach((z) => {
    if (!z.isActive || !Number.isFinite(z.price)) return;
    const color = z.ratio === 0.618 ? '#e3b341' : 'rgba(120,123,134,0.7)';
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
  if (chartData?.priceChart) {
    window.Viewport?.lockPriceAutoScaleDuring?.(chartData.priceChart, applyColoredData);
  } else {
    applyColoredData();
  }

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
    const hostChart = getNavigatorPriceChart(chartData, pane);
    if (pane === 'price' && hostChart) {
      window.Viewport?.lockPriceAutoScaleDuring?.(hostChart, applyPluginData);
    } else {
      applyPluginData();
    }
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
      colored.forEach((c) => backtestStore.upsertCandle(c));
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
  const spikeMarkers = buildSpikeMarkers(osc);
  const navMarkers = navigatorMarkers ?? _btNavMarkers;
  applyCandleMarkers(chartData, [...spikeMarkers, ...navMarkers]);
}

function applyBacktestTradeMarkerPrimitive(chartData, trades) {
  if (!chartData?.tradeMarkerPlugin) return;
  chartData.tradeMarkerPlugin.setData(buildTradeMarkerPrimitiveData(trades));
}

function applyBacktestIndicatorPatch(result) {
  if (!result || !Array.isArray(result.chartData) || result.chartData.length === 0) return;
  if (!_backtest?.candleSeries || backtestStore.candleCount() === 0) return;

  const trades = result.trades || [];
  backtestLastTrades = trades;

  const patchPayload = { oscillators: chartPointsToOsc(result.chartData) };
  if (Array.isArray(result.annotations)) {
    patchPayload.annotations = result.annotations;
  }
  backtestStore.replaceOscAndAnnotations(patchPayload);
  buildBacktestTradeMarkers(trades);

  const storeData = backtestStore.getForLightweightCharts();
  const priceChart = _backtest.priceChart;
  const prevRange = priceChart?.timeScale()?.getVisibleLogicalRange();

  isUpdatingData = true;
  try {
    applyOscillatorToChart(_backtest, storeData.osc, storeData.annotations);
    applyWozduhVisibilityToChart(_backtest, 'backtest');

    applyNavigatorOverlays(result, storeData.candles, _backtest, {
      preserveView: true,
      updateLoadedCandles: true,
    });
    applyBacktestCandleMarkers(_backtest, trades, storeData.osc, _btNavMarkers);
    applyBacktestTradeMarkerPrimitive(_backtest, trades);
  } finally {
    setTimeout(() => { isUpdatingData = false; }, 0);
  }

  if (prevRange && _backtest.allCharts?.length) {
    _backtest.allCharts.forEach((chart) => syncVisibleLogicalRange(chart, prevRange));
  }

  renderChartLegends('backtest');
}

function applyBacktestResultToChart(result, options = {}, viewportAnchor = null) {
  if (!result || !Array.isArray(result.chartData) || result.chartData.length === 0) return;

  const anchor = viewportAnchor ?? options.viewportAnchor ?? null;

  const patchIndicatorsOnly = options.patchIndicatorsOnly === true
    || (options.patchIndicatorsOnly !== false && canPatchBacktestIndicatorsOnly(options));

  const container = document.getElementById('backtest-chart-container');
  if (container) container.classList.add('visible');

  if (!_backtest?.candleSeries) {
    reinitBacktestChart();
  }
  if (!_backtest.candleSeries) return;

  if (patchIndicatorsOnly) {
    applyBacktestIndicatorPatch(result);
    applyIndicatorConfigStyles(_backtest);
    return;
  }

  backtestStore.replaceFromServer(chartPointsToStorePayload(result.chartData, result.annotations));
  backtestLastTrades = result.trades || [];
  backtestHistoryHasMore = true;
  backtestHistoryLoading = false;
  buildBacktestTradeMarkers(result.trades);

  const storeData = backtestStore.getForLightweightCharts();

  isUpdatingData = true;
  try {
    applyPriceToChart(_backtest, storeData.candles);
    applyOscillatorToChart(_backtest, storeData.osc, storeData.annotations);
    applyWozduhVisibilityToChart(_backtest, 'backtest');

    applyNavigatorOverlays(result, storeData.candles, _backtest, {
      updateLoadedCandles: true,
    });
    applyBacktestCandleMarkers(_backtest, result.trades, storeData.osc, _btNavMarkers);
    applyBacktestTradeMarkerPrimitive(_backtest, result.trades);
    renderChartLegends('backtest');

    applyIndicatorConfigStyles(_backtest);

    window.Viewport?.restoreViewportToCharts(
      _backtest,
      _backtest.candleSeries,
      anchor,
      { candles: storeData.candles, chartTime, animate: false },
    );
  } finally {
    setTimeout(() => { isUpdatingData = false; }, 0);
  }
}

function initEquityChart() {
  const container = document.getElementById('equity-chart');
  if (!container || typeof LightweightCharts === 'undefined') return;

  _equityChart = LightweightCharts.createChart(container, {
    layout: { ...sharedChartLayout(), background: { type: 'solid', color: TV.bg }, textColor: '#d1d4dc' },
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
    width: container.clientWidth,
    height: 300,
    rightPriceScale: {
      borderColor: TV.border,
      autoScale: true,
    },
  });

  _equitySeries = _equityChart.addLineSeries({
    color: TV.green,
    lineWidth: 2,
    priceLineVisible: false,
    lastValueVisible: true,
  });
}

function resizeEquityChart() {
  const container = document.getElementById('equity-chart');
  if (!_equityChart || !container) return;
  _equityChart.applyOptions({ width: container.clientWidth, height: 300 });
}

function setAllPriceData(candles) {
  const sorted = [...(candles || [])].sort((a, b) => a.time - b.time);
  const lineData = toLineClose(sorted);
  lineData.sort((a, b) => a.time - b.time);
  _live.candleSeries.setData(sorted);
  _live.barSeries.setData(sorted);
  _live.lineSeries.setData(lineData);
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
    updateVolumeLabel(liveStore.candleCount() ? liveStore.getForLightweightCharts().candles : [normalized]);
  }
  if (tradeMarkers.length > 0) {
    applyAllMarkers();
  }
}

function syncLiveChartPanesFromPrice() {
  if (!_live?.allCharts?.length || !_live?.priceChart) return;
  const mainRange = _live.priceChart.timeScale().getVisibleLogicalRange();
  if (mainRange) {
    syncChartGroupLogicalRange(_live.allCharts, _live.priceChart, mainRange);
  }
}

function rulerPriceSeries(chartData) {
  if (chartData === _live) return activePriceSeries();
  return chartData?.candleSeries;
}

function attachRulerToChart(chartData) {
  if (!chartData?.priceChart || chartData.rulerAttached) return;
  ensureRulerElements(chartData);

  const clickHandler = (param) => onRulerClick(param, chartData);
  const moveHandler = (param) => onRulerMove(param, chartData);
  chartData.priceChart.subscribeClick(clickHandler);
  chartData.priceChart.subscribeCrosshairMove(moveHandler);
  chartData._rulerClickHandler = clickHandler;
  chartData._rulerMoveHandler = moveHandler;
  chartData.rulerAttached = true;
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

  const x1 = chartData.priceChart.timeScale().timeToCoordinate(ruler.p1.time);
  const x2 = chartData.priceChart.timeScale().timeToCoordinate(ruler.p2.time);
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
  if (context === 'live' && typeof updateVolumeLabel === 'function') {
    updateVolumeLabel(storeData.candles);
  }
  applyOscillatorToChart(chartData, storeData.osc, storeData.annotations);
  const wozPrefs = options.wozduhPrefs;
  if (wozPrefs) {
    applyWozduhVisibilityFromPrefs(chartData, wozPrefs);
  } else if (typeof getWozduhPrefsForChart === 'function') {
    applyWozduhVisibilityToChart(chartData, context);
  }
  if (context === 'live') {
    _spikeMarkers = buildSpikeMarkers(storeData.osc);
    if (typeof _hooks.live.applyTradeMarkers === 'function') {
      _hooks.live.applyTradeMarkers(_spikeMarkers);
    } else {
      applyAllMarkersFromState();
    }
  }

  if (options.isPrepend && options.addedCount > 0) {
    const prependOldRange = options.prependOldRange ?? _ctxData(context)?.priceChart?.timeScale()?.getVisibleLogicalRange();
    if (prependOldRange) {
      const nextRange = {
        from: prependOldRange.from + options.addedCount,
        to: prependOldRange.to + options.addedCount,
      };
      const shiftViewport = () => {
        const handle = _ctxData(context);
        handle?.allCharts?.forEach((chart) => {
          syncVisibleLogicalRange(chart, nextRange, { animate: false });
        });
      };
      if (typeof runWithSuppressedHistoryLoad === 'function') {
        runWithSuppressedHistoryLoad(shiftViewport);
      } else {
        shiftViewport();
      }
    }
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

function applyAllMarkersFromState() {
  const showSpike = document.getElementById('tog-spike')?.checked;
  const combined = [..._tradeMarkers];
  if (showSpike) combined.push(..._spikeMarkers);
  combined.sort((a, b) => a.time - b.time);
  _live.candleSeries?.setMarkers(combined);
}

function _applyDeltaInternal(context, delta, options = {}) {
  if (!delta) return;
  if (!_shouldPaint(context)) return;
  const chartData = _ctxData(context);

  if (delta.candle && context === 'live') {
    if (liveStore.candleCount() <= 1) {
      const candles = liveStore.getForLightweightCharts().candles;
      setAllPriceData(candles);
      _live.volumeSeries?.setData(toVolumeBars(candles));
      if (typeof updateVolumeLabel === 'function') updateVolumeLabel(candles);
    } else {
      updateAllPriceSeries(delta.candle);
    }
  }

  if (delta.osc) {
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

  const after = _hooks[context]?.onAfterDelta;
  if (typeof after === 'function') after();
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
  const chartData = initProfessionalChart(containerId, options);
  if (!chartData) return null;
  chartData._context = context;
  if (context === 'live') _live = chartData;
  else _backtest = chartData;

  const hooks = _hooks[context];
  attachProfessionalChartSync(chartData, {
    crosshairPriceSeries: hooks.crosshairPriceSeries || (() => chartData.candleSeries),
    onVisibleRangeChange: hooks.onVisibleRangeChange,
    onScrollHistory: hooks.onScrollHistory,
  });
  return chartData;
}

const ChartAdapter = {
  initLiveCharts(selectors = LIVE_CHART_SELECTORS, hooks = {}) {
    if (typeof LightweightCharts === 'undefined') return false;
    if (_live.chart) return true;
    _hooks.live = hooks;
    _live = _initChartPair('live-chart-container', 'live', {
      selectors,
      navigatorPlugin: true,
    }) || {};
    if (!_live.chart) return false;
    _wozduxPriceLines = _live.wozduxPriceLines;
    _rsxPriceLines = _live.rsxPriceLines;
    if (typeof hooks.onLiveInit === 'function') hooks.onLiveInit(_live);
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

  applyFullData(context, storeData, options = {}) {
    _applyFullDataInternal(context, storeData, options);
  },

  applyDelta(context, delta, options = {}) {
    _applyDeltaInternal(context, delta, options);
  },

  setNavigatorOverlay(context, result, candles, options = {}) {
    const chartData = _ctxData(context);
    return applyNavigatorOverlays(result, candles, chartData, { ...options, context });
  },

  /** @internal — for viewport.js only */
  getChartHandle(context) {
    return _ctxData(context);
  },

  isInitialized(context) {
    return !!_ctxData(context)?.chart;
  },

  clearSeries(context) {
    _clearSeries(context);
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

  forceSyncTimeScales(context) {
    forceSyncChartTimeScales(_ctxData(context));
  },

  syncLivePanesFromPrice() { syncLiveChartPanesFromPrice(); },

  captureLiveViewport() { return captureLiveViewportSnapshot(); },

  restoreLiveViewport(snapshot, options) { restoreLiveViewportSnapshot(snapshot, options); },

  restoreLiveViewportTwice(snapshot) { restoreLiveViewportSnapshotTwice(snapshot); },

  applyLiveViewportAfterData(candles, options) {
    applyLiveViewportAfterData(_live, candles, options);
  },

  applyOrderFlowTimeScale(enabled) { applyOrderFlowTimeScale(enabled); },

  reinitBacktest(hooks = {}) {
    _hooks.backtest = { ..._hooks.backtest, ...hooks };
    return reinitBacktestChart();
  },

  applyBacktestResult(result, options = {}, viewportAnchor = null) {
    return applyBacktestResultToChart(result, options, viewportAnchor);
  },

  applyBacktestPatch(result) {
    return applyBacktestIndicatorPatch(result);
  },

  applyIndicatorStyles(context) {
    applyIndicatorConfigStyles(_ctxData(context));
  },

  renderFib(zones) { renderFibZones(zones); },

  clearFib() { clearFibLines(); },

  setTradeMarkers(markers) {
    _tradeMarkers = markers || [];
    applyAllMarkersFromState();
  },

  setSpikeMarkers(markers) {
    _spikeMarkers = markers || [];
  },

  applyAllMarkers() { applyAllMarkersFromState(); },

  initEquity() { initEquityChart(); },

  resizeEquity() { resizeEquityChart(); },

  setEquityData(points) {
    _equitySeries?.setData(points || []);
  },

  fitEquityContent() {
    _equityChart?.timeScale()?.fitContent();
  },

  captureViewport(context) {
    const h = _ctxData(context);
    return h?.chart && h?.candleSeries
      ? window.Viewport?.captureViewport(h.chart, h.candleSeries) ?? null
      : null;
  },

  restoreViewport(context, anchor, candles) {
    const h = _ctxData(context);
    window.Viewport?.restoreViewportToCharts(h, h.candleSeries, anchor, {
      candles, chartTime, animate: false,
    });
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
    } else if (legendId === 'rsx') {
      chartData.rsxSeries?.applyOptions({ visible });
      chartData.rsxSignalSeries?.applyOptions({ visible });
      if (!visible) {
        chartData.rsxSeries?.applyOptions({ visible: false });
        chartData.rsxSignalSeries?.applyOptions({ visible: false });
      }
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

  applyRsxData(context, osc, annotations) {
    applyRsxData(osc, _ctxData(context), annotations);
  },

  applySeriesDataLive(options = {}) {
    const storeData = liveStore.getForLightweightCharts();
    const candles = storeData.candles;
    if (!_shouldPaint('live')) return;
    applyPriceToChart(_live, candles);
    if (typeof updateVolumeLabel === 'function') updateVolumeLabel(candles);
    applyOscillatorToChart(_live, storeData.osc, storeData.annotations);
    applyWozduhVisibilityToChart(_live, 'live');
    _spikeMarkers = buildSpikeMarkers(storeData.osc);
    applyAllMarkersFromState();
    return { candles, storeData };
  },

  syncVisibleLogicalRange(chart, range, options = {}) {
    syncVisibleLogicalRange(chart, range, options);
  },

  applyBacktestMarkers(trades, osc) {
    applyBacktestCandleMarkers(_backtest, trades, osc, _btNavMarkers);
    applyBacktestTradeMarkerPrimitive(_backtest, trades);
  },

  getVisibleLogicalRange(context) {
    return _ctxData(context)?.priceChart?.timeScale()?.getVisibleLogicalRange() ?? null;
  },

  rulerPointFromParam(chartData, param) {
    if (!param?.point || !chartData?.priceChart) return null;
    const series = rulerPriceSeries(chartData);
    const price = series?.coordinateToPrice(param.point.y);
    const time = chartData.priceChart.timeScale().coordinateToTime(param.point.x);
    if (price == null || time == null) return null;
    return { time, price };
  },

  setToggleSeriesVisible(context, key, visible) {
    const chartData = _ctxData(context);
    const series = key === 'rsx' ? chartData?.rsxSeries : chartData?.volumeSeries;
    series?.applyOptions({ visible });
  },
};

if (typeof window !== 'undefined') {
  window.ChartAdapter = ChartAdapter;
}
