/**
 * ScaleController — declarative price-scale modes (Core 3.0 Shot 10A / 11D).
 * SSOT: { isAuto, isLog } → localStorage → applyScaleMode(chart) → UI .active
 * Boot invariant (11D): autoScale defaults ON. Off only after user Y-gesture on price scale.
 */
const ScaleController = (() => {
  'use strict';

  // Shot 11D: bump key so stale {isAuto:false} from the old default cannot blank candles.
  const STORAGE_KEY = 'chart_scale_prefs_v2';
  const DEFAULT_STATE = Object.freeze({ isAuto: true, isLog: false });

  /** @type {{ isAuto: boolean, isLog: boolean }} */
  let state = { ...DEFAULT_STATE };

  /** @type {Map<string, { chart: object, host: HTMLElement }>} */
  const bindings = new Map();

  let buttonsBound = false;

  function loadState() {
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      if (!raw) return { ...DEFAULT_STATE };
      const parsed = JSON.parse(raw);
      return {
        // Default ON; only an explicit false (user gesture / toggle) disables Auto.
        isAuto: parsed?.isAuto !== false,
        isLog: parsed?.isLog === true,
      };
    } catch {
      return { ...DEFAULT_STATE };
    }
  }

  function persist() {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify({
        isAuto: !!state.isAuto,
        isLog: !!state.isLog,
      }));
    } catch {
      /* quota / private mode */
    }
  }

  function getState() {
    return { isAuto: !!state.isAuto, isLog: !!state.isLog };
  }

  function logMode() {
    if (typeof LightweightCharts === 'undefined') return 0;
    return state.isLog
      ? LightweightCharts.PriceScaleMode.Logarithmic
      : LightweightCharts.PriceScaleMode.Normal;
  }

  /**
   * Sole chart binder: reads SSOT and applies LWC right price scale options.
   * @param {object} chart LWC IChartApi
   */
  function applyScaleMode(chart) {
    if (!chart?.priceScale) return;
    try {
      chart.priceScale('right').applyOptions({
        autoScale: !!state.isAuto,
        mode: logMode(),
      });
    } catch (err) {
      console.warn('[ScaleController] applyScaleMode failed:', err);
    }
  }

  function applyAll() {
    bindings.forEach(({ chart }) => applyScaleMode(chart));
    syncUI();
  }

  function syncUI() {
    document.querySelectorAll('.scale-controls').forEach((controls) => {
      const autoBtn = controls.querySelector('[data-action="auto"]');
      const logBtn = controls.querySelector('[data-action="log"]');
      if (autoBtn) autoBtn.classList.toggle('active', !!state.isAuto);
      if (logBtn) logBtn.classList.toggle('active', !!state.isLog);
    });
  }

  function setState(patch, { apply = true } = {}) {
    if (patch && typeof patch === 'object') {
      if (Object.prototype.hasOwnProperty.call(patch, 'isAuto')) {
        state.isAuto = !!patch.isAuto;
      }
      if (Object.prototype.hasOwnProperty.call(patch, 'isLog')) {
        state.isLog = !!patch.isLog;
      }
    }
    persist();
    if (apply) applyAll();
    else syncUI();
  }

  function toggleAuto() {
    setState({ isAuto: !state.isAuto });
  }

  function toggleLog() {
    setState({ isLog: !state.isLog });
  }

  function readChartAutoScale(chart) {
    try {
      return chart.priceScale('right').options().autoScale !== false;
    } catch {
      return !!state.isAuto;
    }
  }

  function priceScaleWidth(chart) {
    try {
      const w = chart.priceScale('right').width();
      return Number.isFinite(w) && w > 0 ? w : 70;
    } catch {
      return 70;
    }
  }

  function isPointerOnPriceScale(host, chart, clientX) {
    if (!host || !chart) return false;
    const rect = host.getBoundingClientRect();
    const scaleW = priceScaleWidth(chart);
    return clientX >= rect.right - scaleW;
  }

  /**
   * When the user drags/wheels the right price axis, LWC turns autoScale off.
   * Sync SSOT + button highlight on pointerup / wheel (no setInterval).
   */
  function attachManualScaleWatch(context, chart, host) {
    if (!host || !chart || host._scaleWatchBound) return;
    host._scaleWatchBound = true;

    let draggingScale = false;

    const syncAutoFromChart = () => {
      const binding = bindings.get(context);
      const active = binding?.chart || chart;
      if (!active) return;
      const autoOn = readChartAutoScale(active);
      if (state.isAuto === autoOn) return;
      state.isAuto = autoOn;
      persist();
      syncUI();
    };

    const onMouseDown = (e) => {
      if (isPointerOnPriceScale(host, chart, e.clientX)) draggingScale = true;
    };
    const onWheel = (e) => {
      if (!isPointerOnPriceScale(host, chart, e.clientX)) return;
      requestAnimationFrame(syncAutoFromChart);
    };
    const onDblClick = (e) => {
      if (!isPointerOnPriceScale(host, chart, e.clientX)) return;
      e.stopPropagation();
      setState({ isAuto: true });
    };
    const onPointerEnd = () => {
      if (!draggingScale) return;
      draggingScale = false;
      requestAnimationFrame(syncAutoFromChart);
    };

    host.addEventListener('mousedown', onMouseDown);
    host.addEventListener('wheel', onWheel, { passive: true });
    host.addEventListener('dblclick', onDblClick);
    host.addEventListener('mouseup', onPointerEnd);
    document.addEventListener('mouseup', onPointerEnd);

    host._scaleWatchDispose = () => {
      host.removeEventListener('mousedown', onMouseDown);
      host.removeEventListener('wheel', onWheel);
      host.removeEventListener('dblclick', onDblClick);
      host.removeEventListener('mouseup', onPointerEnd);
      document.removeEventListener('mouseup', onPointerEnd);
      host._scaleWatchBound = false;
    };
  }

  function bindButtons() {
    if (buttonsBound) {
      syncUI();
      return;
    }
    buttonsBound = true;
    document.querySelectorAll('.scale-controls').forEach((controls) => {
      controls.addEventListener('click', (e) => {
        const btn = e.target.closest('[data-action]');
        if (!btn || !controls.contains(btn)) return;
        e.stopPropagation();
        const action = btn.getAttribute('data-action');
        if (action === 'auto') toggleAuto();
        else if (action === 'log') toggleLog();
      });
    });
    syncUI();
  }

  /**
   * Register a price chart for scale binding (call after createChart).
   * @param {string} context 'live' | 'backtest'
   * @param {object} chart
   * @param {HTMLElement} host LWC host element (price pane)
   */
  function register(context, chart, host) {
    if (!context || !chart) return;
    const prev = bindings.get(context);
    if (prev?.host?._scaleWatchDispose) {
      try { prev.host._scaleWatchDispose(); } catch { /* noop */ }
    }
    bindings.set(context, { chart, host });
    applyScaleMode(chart);
    if (host) attachManualScaleWatch(context, chart, host);
    syncUI();
  }

  function unregister(context) {
    const prev = bindings.get(context);
    if (prev?.host?._scaleWatchDispose) {
      try { prev.host._scaleWatchDispose(); } catch { /* noop */ }
    }
    bindings.delete(context);
  }

  function init() {
    state = loadState();
    bindButtons();
  }

  return {
    init,
    register,
    unregister,
    applyScaleMode,
    applyAll,
    getState,
    setState,
    toggleAuto,
    toggleLog,
    syncUI,
    isPointerOnPriceScale,
    priceScaleWidth,
  };
})();

window.ScaleController = ScaleController;
