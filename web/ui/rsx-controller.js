/**
 * Phase 19.5.5 — RSX indicator settings menus (live + backtest).
 */
const RsxController = (() => {
  let liveSettings = defaultRsxSettings();
  let backtestSettings = defaultRsxSettings();
  let settingsChangedCallbacks = [];

  function contextFromWrap(wrap) {
    return wrap?.id === 'bt-rsx-wrap' ? 'backtest' : 'live';
  }

  function getWrap(context = 'live') {
    return document.getElementById(context === 'backtest' ? 'bt-rsx-wrap' : 'rsx-wrap');
  }

  function getSettingsMenu(wrap) {
    if (!wrap) return null;
    return wrap.querySelector('.indicator-settings-menu');
  }

  function getSettingsState(context = 'live') {
    return context === 'backtest' ? backtestSettings : liveSettings;
  }

  function storageKey(context) {
    return context === 'backtest' ? LS_RSX_SETTINGS_BACKTEST_KEY : LS_RSX_SETTINGS_LIVE_KEY;
  }

  function setSettings(context, settings) {
    const normalized = normalizeRsxSettingsFromAPI(settings, getSettingsState(context));
    if (context === 'backtest') backtestSettings = normalized;
    else liveSettings = normalized;
    return normalized;
  }

  function persist(context, settings) {
    localStorage.setItem(storageKey(context), JSON.stringify(settings));
  }

  function loadFromStorage(context) {
    try {
      const raw = localStorage.getItem(storageKey(context));
      if (raw) {
        const parsed = JSON.parse(raw);
        if (parsed && typeof parsed === 'object') {
          return setSettings(context, parsed);
        }
      }
    } catch {
      /* use defaults */
    }

    if (context === 'live') {
      const migrated = defaultRsxSettings();
      try {
        const lb = parseInt(localStorage.getItem(LS_RSX_LOOKBACK_KEY), 10);
        const sl = parseInt(localStorage.getItem(LS_RSX_SIGNAL_LENGTH_KEY), 10);
        const len = parseInt(localStorage.getItem(LS_RSX_LENGTH_KEY), 10);
        if (Number.isFinite(lb)) migrated.div_lookback = clampRsxDivLookback(lb);
        if (Number.isFinite(sl)) migrated.signal_length = clampRsxSignalLength(sl);
        if (Number.isFinite(len)) migrated.length = clampRsxLength(len);
      } catch {
        /* noop */
      }
      return setSettings('live', migrated);
    }

    return setSettings(context, defaultRsxSettings());
  }

  function readFloatDeltaInput(el, fallback = 0) {
    if (!el) return fallback;
    const n = Number(el.value);
    return Number.isFinite(n) ? n : fallback;
  }

  function readSettingsFromMenu(contextOrWrap, context = 'live') {
    const wrap = typeof contextOrWrap === 'string'
      ? getWrap(contextOrWrap)
      : (contextOrWrap?.id === 'rsx-wrap' || contextOrWrap?.id === 'bt-rsx-wrap'
        ? contextOrWrap
        : contextOrWrap?.closest?.('.rsx-wrap') || contextOrWrap);
    const ctx = typeof contextOrWrap === 'string' ? contextOrWrap : (context || contextFromWrap(wrap));
    const defaults = getSettingsState(ctx);
    if (!wrap) return coerceRsxSettingsForAPI({ ...defaults }, defaults);
    const source = wrap.querySelector('.rsx-source-select')?.value || defaults.source;
    const divMethod = wrap.querySelector('.rsx-div-method-select')?.value || defaults.div_method;
    const showPivotsEl = wrap.querySelector('.rsx-show-pivots-chk');
    return coerceRsxSettingsForAPI({
      length: clampRsxLength(Number(wrap.querySelector('.rsx-length-input')?.value)),
      div_lookback: clampRsxDivLookback(Number(wrap.querySelector('.rsx-div-lookback-input')?.value)),
      signal_length: clampRsxSignalLength(Number(wrap.querySelector('.rsx-signal-length-input')?.value)),
      source: source === 'hlc3' ? 'hlc3' : 'close',
      pivot_radius: clampRsxPivotRadius(Number(wrap.querySelector('.rsx-pivot-radius-input')?.value)),
      div_method: divMethod === 'fractal' ? 'fractal' : 'tv',
      min_price_delta_ratio: readFloatDeltaInput(
        wrap.querySelector('.rsx-min-price-delta-input'),
        defaults.min_price_delta_ratio ?? 0,
      ),
      min_osc_delta: readFloatDeltaInput(
        wrap.querySelector('.rsx-min-osc-delta-input'),
        defaults.min_osc_delta ?? 0,
      ),
      show_pivots: showPivotsEl ? showPivotsEl.checked : rsxShowPivotsFrom(defaults, true),
    }, defaults);
  }

  function applyToMenu(context, settings, defaults = defaultRsxSettings()) {
    const wrap = getWrap(context);
    if (!wrap || !settings) return;
    const s = normalizeRsxSettingsFromAPI(settings, defaults);
    const lengthEl = wrap.querySelector('.rsx-length-input');
    const lookbackEl = wrap.querySelector('.rsx-div-lookback-input');
    const signalEl = wrap.querySelector('.rsx-signal-length-input');
    const sourceEl = wrap.querySelector('.rsx-source-select');
    const pivotEl = wrap.querySelector('.rsx-pivot-radius-input');
    const methodEl = wrap.querySelector('.rsx-div-method-select');
    const showPivotsEl = wrap.querySelector('.rsx-show-pivots-chk');
    const minPriceDeltaEl = wrap.querySelector('.rsx-min-price-delta-input');
    const minOscDeltaEl = wrap.querySelector('.rsx-min-osc-delta-input');
    if (lengthEl && Number.isFinite(s.length)) lengthEl.value = String(s.length);
    if (lookbackEl && Number.isFinite(s.div_lookback)) lookbackEl.value = String(s.div_lookback);
    if (signalEl && Number.isFinite(s.signal_length)) signalEl.value = String(s.signal_length);
    if (sourceEl && s.source) sourceEl.value = s.source;
    if (pivotEl && Number.isFinite(s.pivot_radius)) pivotEl.value = String(s.pivot_radius);
    if (methodEl && s.div_method) methodEl.value = s.div_method;
    if (minPriceDeltaEl && Number.isFinite(s.min_price_delta_ratio)) {
      minPriceDeltaEl.value = String(s.min_price_delta_ratio);
    }
    if (minOscDeltaEl && Number.isFinite(s.min_osc_delta)) {
      minOscDeltaEl.value = String(s.min_osc_delta);
    }
    if (showPivotsEl && typeof s.show_pivots === 'boolean') showPivotsEl.checked = s.show_pivots;
    syncPivotRadiusFieldState(context);
  }

  function syncPivotRadiusFieldState(contextOrWrap) {
    const wrap = typeof contextOrWrap === 'string'
      ? getWrap(contextOrWrap)
      : (contextOrWrap?.id === 'rsx-wrap' || contextOrWrap?.id === 'bt-rsx-wrap'
        ? contextOrWrap
        : contextOrWrap?.closest?.('.rsx-wrap'));
    if (!wrap) return;
    const method = wrap.querySelector('.rsx-div-method-select')?.value || 'tv';
    const pivotEl = wrap.querySelector('.rsx-pivot-radius-input');
    const row = pivotEl?.closest('.setting-row');
    const isTV = method !== 'fractal';
    if (pivotEl) pivotEl.disabled = isTV;
    if (row) row.classList.toggle('setting-row--disabled', isTV);
  }

  function syncFromMenu(context) {
    const settings = readSettingsFromMenu(context);
    const applied = setSettings(context, settings);
    persist(context, applied);
    applyToMenu(context, applied);
    return applied;
  }

  function getSettings(context = 'live') {
    return { ...getSettingsState(context) };
  }

  function notifySettingsChanged(context) {
    if (context === 'backtest') return;
    const settings = getSettings(context);
    settingsChangedCallbacks.forEach((cb) => {
      try {
        cb(settings, context);
      } catch (err) {
        console.warn('[RsxController] settingsChanged callback failed:', err);
      }
    });
  }

  function onSettingsChanged(callback) {
    if (typeof callback === 'function') settingsChangedCallbacks.push(callback);
  }

  function hideMenus() {
    document.querySelectorAll('.rsx-wrap .indicator-settings-menu').forEach((menu) => {
      menu.hidden = true;
    });
  }

  function refreshPivotsOnChart(context) {
    const settings = readSettingsFromMenu(context);
    const applied = setSettings(context, settings);
    persist(context, applied);
    if (context === 'live' && typeof liveRenderScheduler !== 'undefined' && liveRenderScheduler) {
      liveRenderScheduler.markDirty({ mode: 'full' });
      return;
    }
    const chartKey = context === 'backtest' ? 'backtest' : 'live';
    const chartData = ChartAdapter.getChartHandle(chartKey);
    const store = chartKey === 'backtest' ? backtestStore : null;
    if (!store) return;
    const storeData = store.getForLightweightCharts();
    const osc = storeData.osc;
    const anns = storeData.annotations;
    if (chartData?.rsxSeries && osc?.length) {
      ChartAdapter.applyRsxData(chartKey, osc, anns);
    }
  }

  async function saveSettingsFromMenu(menu, context) {
    if (!menu) return;
    const active = document.activeElement;
    if (active && menu.contains(active) && typeof active.blur === 'function') {
      active.blur();
    }
    try {
      if (context === 'backtest') {
        syncFromMenu('backtest');
      } else if (typeof syncRsxIndicatorSettings === 'function') {
        await syncRsxIndicatorSettings('live');
      } else {
        syncFromMenu('live');
      }
    } finally {
      menu.hidden = true;
    }
  }

  function init() {
    loadFromStorage('live');
    loadFromStorage('backtest');
    applyToMenu('live', liveSettings);
    applyToMenu('backtest', backtestSettings);

    const rsxFieldSelector = '.rsx-length-input, .rsx-div-lookback-input, .rsx-signal-length-input, .rsx-source-select, .rsx-pivot-radius-input, .rsx-div-method-select, .rsx-min-price-delta-input, .rsx-min-osc-delta-input, .rsx-show-pivots-chk';
    document.querySelectorAll('.rsx-wrap').forEach((wrap) => {
      const toggle = wrap.querySelector('.rsx-settings-toggle');
      const menu = getSettingsMenu(wrap);
      if (!menu) return;

      const context = contextFromWrap(wrap);
      if (typeof initFloatingMenuDrag === 'function') initFloatingMenuDrag(menu);
      syncPivotRadiusFieldState(context);

      toggle?.addEventListener('click', (e) => {
        e.stopPropagation();
        if (typeof WozduhController !== 'undefined') WozduhController.hideMenus();
        if (typeof RiskController !== 'undefined') RiskController.hideMenu();
        const willOpen = menu.hidden;
        hideMenus();
        if (willOpen && typeof openFloatingMenu === 'function') openFloatingMenu(menu, toggle);
        else menu.hidden = true;
      });

      menu.querySelectorAll(rsxFieldSelector).forEach((el) => {
        if (el.classList.contains('rsx-div-method-select')) {
          el.addEventListener('change', () => syncPivotRadiusFieldState(context));
        }
        if (el.classList.contains('rsx-show-pivots-chk')) {
          el.addEventListener('change', () => refreshPivotsOnChart(context));
          return;
        }
        if (context !== 'backtest') {
          el.addEventListener('input', () => notifySettingsChanged(context));
          el.addEventListener('change', () => notifySettingsChanged(context));
        }
      });

      menu.addEventListener('mousedown', (e) => e.stopPropagation());
      menu.addEventListener('click', (e) => e.stopPropagation());

      menu.querySelector('.rsx-save-btn')?.addEventListener('click', async () => {
        await saveSettingsFromMenu(menu, context);
      });
    });
  }

  return {
    init,
    getSettings,
    onSettingsChanged,
    hideMenus,
    setSettings,
    applyToMenu,
    syncFromMenu,
    persist,
    loadFromStorage,
  };
})();

window.RsxController = RsxController;
window.hideRsxSettingsMenus = () => RsxController.hideMenus();
