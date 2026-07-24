/**
 * ScaleController — ADR-020 HostID-based Y-scale owner.
 * Prefs per hostId (shared across live/backtest UI). Bindings per context+hostId.
 * scaleGroup is a dormant socket (default = hostId); no group apply yet.
 * Log only when allowLog=true (price). Visibility must never reset prefs.
 */
(function (global) {
  'use strict';

  const STORAGE_KEY = 'chart_scale_prefs_v3';
  const STORAGE_KEY_LEGACY = 'chart_scale_prefs_v2';
  const VERSION = 3;
  const DEFAULT_PANE = Object.freeze({ isAuto: true, isLog: false });

  /**
   * @typedef {{ isAuto: boolean, isLog: boolean, manualRange?: { min: number, max: number } }} ScalePanePrefs
   * @type {Map<string, ScalePanePrefs>} hostId → prefs
   */
  let prefsByHost = new Map();

  /**
   * @typedef {{ context: string, hostId: string, chart: object, host?: HTMLElement|null, allowLog: boolean, scaleGroup: string }} ScaleBinding
   * @type {Map<string, ScaleBinding>}
   */
  const bindings = new Map();

  let buttonsBound = false;
  let storage = null;

  function bindingKey(context, hostId) {
    return `${String(context || '')}\0${String(hostId || '')}`;
  }

  function resolveStorage(explicit) {
    if (explicit) return explicit;
    try {
      if (typeof global.localStorage !== 'undefined' && global.localStorage) {
        return global.localStorage;
      }
    } catch {
      /* */
    }
    const map = new Map();
    return {
      getItem(k) { return map.has(k) ? map.get(k) : null; },
      setItem(k, v) { map.set(k, String(v)); },
      removeItem(k) { map.delete(k); },
    };
  }

  function defaultPaneState() {
    return { isAuto: true, isLog: false };
  }

  /**
   * Shallow-clone pane prefs. Preserves unknown forward-compat fields.
   * Normalizes only isAuto / isLog booleans.
   */
  function clonePane(p) {
    if (!p || typeof p !== 'object') return defaultPaneState();
    const out = { ...p };
    out.isAuto = p.isAuto !== false;
    out.isLog = p.isLog === true;
    if (p.manualRange && typeof p.manualRange === 'object') {
      out.manualRange = {
        min: p.manualRange.min,
        max: p.manualRange.max,
      };
    }
    return out;
  }

  /**
   * Future Manual contract socket: Auto OFF is only self-sufficient with a valid Y window.
   * @param {ScalePanePrefs|object|null|undefined} pane
   */
  function hasValidManualRange(pane) {
    const r = pane && pane.manualRange;
    if (!r || typeof r !== 'object') return false;
    return Number.isFinite(r.min) && Number.isFinite(r.max) && r.min < r.max;
  }

  /**
   * Invariant:
   * Persisted scale state must be self-sufficient.
   *
   * Valid:
   *   Auto ON
   *   Auto OFF + manualRange { min, max } (finite, min < max)
   *
   * Invalid:
   *   Auto OFF without valid manualRange
   *
   * Invalid states are repaired to Auto ON while preserving Log and other fields.
   * Pure: no I/O, no DOM, no chart access.
   *
   * @param {{ version?: number, panes?: Record<string, object> }|null|undefined} input
   * @returns {{ prefs: { version: number, panes: Record<string, object> }, dirty: boolean }}
   */
  function repairScalePrefs(input) {
    const panesIn = input && input.panes && typeof input.panes === 'object' ? input.panes : {};
    const panes = {};
    let dirty = false;
    for (const [hostId, pane] of Object.entries(panesIn)) {
      if (!hostId) continue;
      const next = clonePane(pane);
      if (next.isAuto === false && !hasValidManualRange(next)) {
        next.isAuto = true;
        dirty = true;
      }
      panes[hostId] = next;
    }
    const version = Number.isFinite(input?.version) ? Number(input.version) : VERSION;
    return {
      prefs: { version: version === VERSION ? VERSION : version, panes },
      dirty,
    };
  }

  function prefsMapToObject(map) {
    const panes = {};
    for (const [hostId, pane] of map.entries()) {
      panes[hostId] = clonePane(pane);
    }
    return { version: VERSION, panes };
  }

  function prefsObjectToMap(prefs) {
    const out = new Map();
    const panes = prefs?.panes && typeof prefs.panes === 'object' ? prefs.panes : {};
    for (const [hostId, pane] of Object.entries(panes)) {
      if (!hostId) continue;
      out.set(hostId, clonePane(pane));
    }
    return out;
  }

  /** Load → migrate v2 → Map (repair+persist happens in init). */
  function loadPrefsMap(store) {
    const out = new Map();
    try {
      const raw = store.getItem(STORAGE_KEY);
      if (raw) {
        const parsed = JSON.parse(raw);
        if (parsed && parsed.version === VERSION && parsed.panes && typeof parsed.panes === 'object') {
          for (const [hostId, pane] of Object.entries(parsed.panes)) {
            if (!hostId) continue;
            out.set(hostId, clonePane(pane));
          }
          return out;
        }
      }
      const legacy = store.getItem(STORAGE_KEY_LEGACY);
      if (legacy) {
        const parsed = JSON.parse(legacy);
        out.set('price', clonePane({
          isAuto: parsed?.isAuto !== false,
          isLog: parsed?.isLog === true,
        }));
      }
    } catch {
      /* */
    }
    return out;
  }

  function persist() {
    try {
      const panes = {};
      for (const [hostId, pane] of prefsByHost.entries()) {
        panes[hostId] = clonePane(pane);
      }
      storage.setItem(STORAGE_KEY, JSON.stringify({ version: VERSION, panes }));
    } catch {
      /* */
    }
  }

  /** load → repair → optional one write. Shared by init / _resetForTests. */
  function hydratePrefsFromStorage(store) {
    const loaded = loadPrefsMap(store);
    const { prefs, dirty } = repairScalePrefs(prefsMapToObject(loaded));
    prefsByHost = prefsObjectToMap(prefs);
    if (dirty) persist();
    return dirty;
  }

  function ensurePrefs(hostId) {
    const id = String(hostId || '').trim();
    if (!id) return defaultPaneState();
    if (!prefsByHost.has(id)) {
      prefsByHost.set(id, defaultPaneState());
    }
    return prefsByHost.get(id);
  }

  function resolveHostId(context, hostId) {
    if (hostId != null && String(hostId).trim() !== '') return String(hostId).trim();
    if (context === 'live' || context === 'backtest') return 'price';
    if (context != null && String(context).trim() !== '') return String(context).trim();
    return 'price';
  }

  /**
   * @param {string} [_context] reserved for API symmetry; prefs are per hostId
   * @param {string} [hostId]
   */
  function getState(_context, hostId) {
    return clonePane(ensurePrefs(resolveHostId(_context, hostId)));
  }

  function logModeFor(pane, allowLog) {
    if (typeof LightweightCharts === 'undefined') return 0;
    const useLog = allowLog && pane.isLog;
    return useLog
      ? LightweightCharts.PriceScaleMode.Logarithmic
      : LightweightCharts.PriceScaleMode.Normal;
  }

  function applyScaleModeToChart(chart, pane, allowLog) {
    if (!chart?.priceScale) return;
    const p = clonePane(pane);
    try {
      chart.priceScale('right').applyOptions({
        autoScale: !!p.isAuto,
        mode: logModeFor(p, !!allowLog),
      });
    } catch (err) {
      try {
        console.warn('[ScaleController] applyScaleMode failed:', err);
      } catch {
        /* */
      }
    }
  }

  function applyBinding(binding) {
    if (!binding?.chart) return;
    applyScaleModeToChart(binding.chart, ensurePrefs(binding.hostId), binding.allowLog);
  }

  function applyAll() {
    bindings.forEach((b) => applyBinding(b));
    syncUI();
  }

  function syncUI() {
    if (typeof document === 'undefined') return;
    document.querySelectorAll('.scale-controls').forEach((controls) => {
      const hostId = String(controls.getAttribute('data-scale-pane') || 'price').trim();
      const pane = ensurePrefs(hostId);
      const autoBtn = controls.querySelector('[data-action="auto"]');
      const logBtn = controls.querySelector('[data-action="log"]');
      if (autoBtn) autoBtn.classList.toggle('active', !!pane.isAuto);
      if (logBtn) {
        logBtn.classList.toggle('active', !!pane.isLog);
        const allow = controls.getAttribute('data-allow-log') !== 'false';
        logBtn.hidden = !allow;
      }
    });
  }

  function setPanePrefs(hostId, patch, { apply = true } = {}) {
    const id = String(hostId || '').trim();
    if (!id) return false;
    const cur = ensurePrefs(id);
    const next = clonePane(cur);
    if (patch && typeof patch === 'object') {
      if (Object.prototype.hasOwnProperty.call(patch, 'isAuto')) {
        next.isAuto = !!patch.isAuto;
      }
      if (Object.prototype.hasOwnProperty.call(patch, 'isLog')) {
        next.isLog = !!patch.isLog;
      }
    }
    if (next.isAuto === cur.isAuto && next.isLog === cur.isLog) {
      if (apply) {
        bindings.forEach((b) => {
          if (b.hostId === id) applyBinding(b);
        });
        syncUI();
      }
      return false;
    }
    prefsByHost.set(id, next);
    persist();
    if (apply) {
      bindings.forEach((b) => {
        if (b.hostId === id) applyBinding(b);
      });
      syncUI();
    }
    return true;
  }

  function toggleAuto(context, hostId) {
    const resolved = resolveHostId(context, hostId);
    const pane = ensurePrefs(resolved);
    return setPanePrefs(resolved, { isAuto: !pane.isAuto });
  }

  /** allowLog from binding; else DOM data-allow-log; else false (no HostID hardcodes). */
  function allowLogFor(hostId) {
    let seen = false;
    let allow = false;
    bindings.forEach((b) => {
      if (b.hostId !== hostId) return;
      seen = true;
      allow = !!b.allowLog;
    });
    if (seen) return allow;
    if (typeof document !== 'undefined') {
      const el = document.querySelector(`.scale-controls[data-scale-pane="${hostId}"]`);
      if (el) return el.getAttribute('data-allow-log') !== 'false';
    }
    return false;
  }

  function toggleLog(context, hostId) {
    const resolved = resolveHostId(context, hostId);
    if (!allowLogFor(resolved)) return false;
    const pane = ensurePrefs(resolved);
    return setPanePrefs(resolved, { isLog: !pane.isLog });
  }

  /** @deprecated Prefer setPanePrefs / toggle*; kept for paint paths that patched global state. */
  function setState(patch, opts) {
    return setPanePrefs('price', patch, opts);
  }

  function readChartAutoScale(chart, fallbackAuto) {
    try {
      return chart.priceScale('right').options().autoScale !== false;
    } catch {
      return !!fallbackAuto;
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
    if (!host) return false;
    const rect = host.getBoundingClientRect();
    const scaleW = chart ? priceScaleWidth(chart) : 70;
    return clientX >= rect.right - scaleW;
  }

  function attachManualScaleWatch(binding) {
    const { host, chart, hostId, context } = binding;
    if (!host || !chart) return;
    if (host._scaleWatchBound) {
      try { host._scaleWatchDispose?.(); } catch { /* */ }
    }
    host._scaleWatchBound = true;

    let draggingScale = false;

    const syncAutoFromChart = () => {
      const key = bindingKey(context, hostId);
      const active = bindings.get(key)?.chart || chart;
      if (!active) return;
      const pane = ensurePrefs(hostId);
      const autoOn = readChartAutoScale(active, pane.isAuto);
      if (pane.isAuto === autoOn) return;
      setPanePrefs(hostId, { isAuto: autoOn });
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
      setPanePrefs(hostId, { isAuto: true });
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
    if (typeof document === 'undefined') return;
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
        const context = String(controls.getAttribute('data-scale-context') || 'live');
        const hostId = String(controls.getAttribute('data-scale-pane') || 'price');
        const action = btn.getAttribute('data-action');
        if (action === 'auto') toggleAuto(context, hostId);
        else if (action === 'log') toggleLog(context, hostId);
      });
    });
    syncUI();
  }

  /**
   * @param {object} args
   * @param {string} args.context
   * @param {string} args.hostId
   * @param {object} args.chart
   * @param {HTMLElement} [args.host]
   * @param {boolean} [args.allowLog]
   * @param {string} [args.scaleGroup] dormant socket; default hostId
   */
  function registerNew(args) {
    const context = String(args.context || 'live');
    const hostId = String(args.hostId || '').trim();
    const chart = args.chart;
    if (!hostId || !chart) return;
    const allowLog = args.allowLog === true;
    const scaleGroup = String(args.scaleGroup || hostId).trim() || hostId;
    const host = args.host || null;
    const key = bindingKey(context, hostId);

    const prev = bindings.get(key);
    if (prev?.host?._scaleWatchDispose) {
      try { prev.host._scaleWatchDispose(); } catch { /* */ }
    }

    ensurePrefs(hostId);
    // Log disallowed panes never keep isLog true in applied mode (prefs may still store false).
    if (!allowLog) {
      const p = ensurePrefs(hostId);
      if (p.isLog) {
        p.isLog = false;
        prefsByHost.set(hostId, clonePane(p));
        persist();
      }
    }

    const binding = { context, hostId, chart, host, allowLog, scaleGroup };
    bindings.set(key, binding);
    applyBinding(binding);
    if (host) attachManualScaleWatch(binding);
    syncUI();
  }

  /**
   * Dual API:
   * - register({ context, hostId, chart, host, allowLog, scaleGroup })
   * - register(context, chart, host) legacy → hostId 'price', allowLog true
   */
  function register(arg1, arg2, arg3) {
    if (arg1 && typeof arg1 === 'object' && arg1.chart && arg1.hostId) {
      registerNew(arg1);
      return;
    }
    registerNew({
      context: arg1 || 'live',
      hostId: 'price',
      chart: arg2,
      host: arg3,
      allowLog: true,
    });
  }

  function unregister(context, hostId) {
    const key = hostId != null
      ? bindingKey(context, hostId)
      : bindingKey(context, 'price');
    const prev = bindings.get(key);
    if (prev?.host?._scaleWatchDispose) {
      try { prev.host._scaleWatchDispose(); } catch { /* */ }
    }
    bindings.delete(key);
  }

  function init(options = {}) {
    storage = resolveStorage(options.storage);
    hydratePrefsFromStorage(storage);
    bindButtons();
    syncUI();
  }

  /** @private tests */
  function _resetForTests(options = {}) {
    bindings.forEach((b) => {
      if (b?.host?._scaleWatchDispose) {
        try { b.host._scaleWatchDispose(); } catch { /* */ }
      }
    });
    bindings.clear();
    buttonsBound = false;
    storage = resolveStorage(options.storage);
    hydratePrefsFromStorage(storage);
  }

  const ScaleController = {
    init,
    register,
    unregister,
    applyScaleMode: (chart) => {
      applyScaleModeToChart(chart, ensurePrefs('price'), true);
    },
    applyAll,
    getState,
    setState,
    setPanePrefs,
    toggleAuto,
    toggleLog,
    syncUI,
    isPointerOnPriceScale,
    priceScaleWidth,
    repairScalePrefs,
    hasValidManualRange,
    VERSION,
    STORAGE_KEY,
    STORAGE_KEY_LEGACY,
    _resetForTests,
    _ensurePrefs: ensurePrefs,
  };

  global.ScaleController = ScaleController;
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = ScaleController;
  }
})(typeof window !== 'undefined' ? window : globalThis);
