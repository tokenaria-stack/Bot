/**
 * Phase 19.5 — Navigator trendlines UI (popups, legends, MTF sync, settings).
 * Chart overlay helpers exposed as window globals for chart-adapter.js.
 */
const NavigatorController = (() => {
  const chartLegendState = {
    live: { price: {}, wozduh: {}, rsx: {} },
    backtest: { price: {}, wozduh: {}, rsx: {} },
  };

  let openNavigatorPopupEl = null;
  let settingsChangedCallback = null;
  let settingsChangedTimer = null;

  const _dirtyState = {
    live: false,
    backtest: false,
  };

  function getContext() {
    return TabsController.isBacktestTabActive() ? 'backtest' : 'live';
  }

  function navigatorSettingsStorageKey(pane, context = 'backtest') {
    const prefix = context === 'live' ? LS_NAV_SETTINGS_LIVE_PREFIX : LS_NAV_SETTINGS_PREFIX;
    return `${prefix}${pane}`;
  }

  function loadNavigatorPaneSettings(pane, context = 'backtest') {
    try {
      const raw = localStorage.getItem(navigatorSettingsStorageKey(pane, context));
      if (raw) return { ...defaultNavigatorPaneSettings(pane), ...JSON.parse(raw) };
    } catch {
      /* defaults */
    }
    return defaultNavigatorPaneSettings(pane);
  }

  function saveNavigatorPaneSettings(pane, settings, context = 'backtest') {
    localStorage.setItem(navigatorSettingsStorageKey(pane, context), JSON.stringify(settings));
  }

  function loadNavigatorDefaults(pane) {
    try {
      const raw = localStorage.getItem(`${LS_NAV_DEFAULTS_PREFIX}${pane}`);
      if (raw) return { ...defaultNavigatorPaneSettings(pane), ...JSON.parse(raw) };
    } catch {
      /* defaults */
    }
    return defaultNavigatorPaneSettings(pane);
  }

  function saveNavigatorDefaults(pane, settings) {
    localStorage.setItem(`${LS_NAV_DEFAULTS_PREFIX}${pane}`, JSON.stringify(settings));
  }

  function getNavigatorPluginForPane(chartData, pane) {
    if (!chartData) return null;
    if (pane === 'price') return chartData.priceNavigatorPlugin;
    if (pane === 'rsx') return chartData.rsxNavigatorPlugin;
    if (pane === 'wozduh') return chartData.wozduhNavigatorPlugin;
    return null;
  }

  function normalizeMtfPeriod(tf) {
    if (!tf) return '';
    const s = String(tf).trim();
    if (s === '1M') return '1M';
    return s.toLowerCase();
  }

  function mtfPeriodsEqual(a, b) {
    return normalizeMtfPeriod(a) === normalizeMtfPeriod(b);
  }

  function getMtfPeriodColor(tf) {
    if (typeof ChartTheme !== 'undefined' && ChartTheme.mtfPeriodColor) {
      return ChartTheme.mtfPeriodColor(tf);
    }
    const key = normalizeMtfPeriod(tf);
    return MTF_PERIOD_COLORS[key] || ChartTheme?.text || '#787b86';
  }

  function coerceNavigatorPeriods(settings) {
    const s = settings || {};
    if (Array.isArray(s.periods)) {
      return s.periods.filter((p) => typeof p === 'string' && p.trim() && p.trim() !== '1M').map((p) => p.trim());
    }
    if (typeof s.period === 'string' && s.period.trim()) {
      return [s.period.trim()];
    }
    return [];
  }

  function normalizeNavigatorPaneSettings(settings, pane = 'price') {
    const s = settings || {};
    const defaults = defaultNavigatorPaneSettings(pane);
    return {
      ...defaults,
      ...s,
      periods: coerceNavigatorPeriods(s),
      targetPrice: s.targetPrice !== false,
      targetRSX: !!s.targetRSX,
      targetWozduh: !!s.targetWozduh,
      useLong: s.useLong !== false,
      useMedium: s.useMedium !== false,
      useShort: s.useShort !== false,
      momentumEnabled: !!s.momentumEnabled,
      timeHoldEnabled: !!s.timeHoldEnabled,
      barColor: !!s.barColor,
      backgroundColor: !!s.backgroundColor,
    };
  }

  function navigatorSettingsToAPI(settings, source, enabled = true) {
    const s = normalizeNavigatorPaneSettings(settings);
    const resolvedSource = source || 'Price';
    return {
      enabled: !!enabled,
      source: resolvedSource,
      trendType: s.trendType || 'Wicks',
      term: s.term || 'Long',
      useLong: s.useLong !== false,
      useMedium: s.useMedium !== false,
      useShort: s.useShort !== false,
      longLen: Number(s.longLen) || 60,
      mediumLen: Number(s.mediumLen) || 30,
      shortLen: Number(s.shortLen) || 10,
      momentumEnabled: !!s.momentumEnabled,
      momentumBars: Number(s.momentumBars) || 14,
      momentumPercent: Number(s.momentumPercent) || 100,
      timeHoldEnabled: !!s.timeHoldEnabled,
      timeHoldBars: Number(s.timeHoldBars) || 2,
      barColor: !!s.barColor,
      backgroundColor: !!s.backgroundColor,
      periods: coerceNavigatorPeriods(s),
    };
  }

  function getActiveMtfPeriods(settings, chartTf) {
    const chartKey = normalizeMtfPeriod(chartTf);
    const periods = coerceNavigatorPeriods(settings);
    return periods.filter((p) => p && !mtfPeriodsEqual(p, chartKey));
  }

  function groupNavigatorLinesByInterval(lines, chartTf) {
    const chartKey = normalizeMtfPeriod(chartTf);
    const groups = new Map();
    (lines || []).forEach((line) => {
      const key = normalizeMtfPeriod(line.interval) || chartKey;
      if (!groups.has(key)) groups.set(key, []);
      groups.get(key).push(line);
    });
    return groups;
  }

  function resolveChartTfForNavigator(context) {
    if (context === 'backtest') {
      return normalizeTf(
        BacktestController.getFormValues().interval || '15m',
      );
    }
    return normalizeTf(currentTf || TimeframeController.getActiveTfFromToolbar() || '15m');
  }

  function loadNavigatorPopupPos(pane) {
    try {
      const raw = localStorage.getItem(`${LS_NAV_POPUP_POS_PREFIX}${pane}`);
      if (raw) return JSON.parse(raw);
    } catch {
      /* noop */
    }
    return null;
  }

  function saveNavigatorPopupPos(pane, x, y) {
    localStorage.setItem(`${LS_NAV_POPUP_POS_PREFIX}${pane}`, JSON.stringify({ x, y }));
  }

  function navPopupField(popup, field) {
    if (!popup || !field) return null;
    return popup.querySelector(`[data-nav-field="${field}"]`);
  }

  function setNavPopupField(popup, field, value, type = 'value') {
    const el = navPopupField(popup, field);
    if (!el) return;
    if (type === 'checkbox') {
      el.checked = !!value;
      return;
    }
    el.value = value ?? '';
  }

  function getNavPopupField(popup, field, type = 'value') {
    const el = navPopupField(popup, field);
    if (!el) return null;
    if (type === 'checkbox') return el.checked;
    return el.value;
  }

  function getNavigatorPopup(pane) {
    return document.getElementById(`popup-${pane}`)
      || document.querySelector(`.navigator-popup[data-pane="${pane}"]`);
  }

  function getNavigatorPaneSettingsFromUI(pane, context = getContext()) {
    const safePane = pane || 'price';
    try {
      const popup = getNavigatorPopup(safePane);
      if (popup && !popup.hidden) {
        return normalizeNavigatorPaneSettings(readNavigatorSettingsFromPopup(popup, safePane), safePane);
      }
      return normalizeNavigatorPaneSettings(loadNavigatorPaneSettings(safePane, context), safePane);
    } catch (err) {
      console.warn(`getNavigatorPaneSettingsFromUI(${safePane}) failed:`, err);
      return defaultNavigatorPaneSettings(safePane);
    }
  }

  function navigatorPaneEnabledFlag(pane, priceUI) {
    if (pane === 'price') return priceUI.targetPrice !== false;
    if (pane === 'rsx') return !!priceUI.targetRSX;
    if (pane === 'wozduh') return !!priceUI.targetWozduh;
    return false;
  }

  function getNavigatorSettingsFromUI(pane, context = getContext()) {
    const safePane = pane || 'price';
    const source = NAVIGATOR_SOURCE_MAP[safePane] || 'Price';
    try {
      const ui = getNavigatorPaneSettingsFromUI(safePane, context);
      const priceUI = safePane === 'price' ? ui : getNavigatorPaneSettingsFromUI('price', context);
      return navigatorSettingsToAPI(
        ui,
        source,
        navigatorPaneEnabledFlag(safePane, priceUI),
      );
    } catch (err) {
      console.warn(`getNavigatorSettingsFromUI(${safePane}) failed:`, err);
      return navigatorSettingsToAPI(
        defaultNavigatorPaneSettings(safePane),
        source,
        safePane === 'price',
      );
    }
  }

  function getNavigatorPayload(context = getContext()) {
    return {
      price: getNavigatorSettingsFromUI('price', context),
      rsx: getNavigatorSettingsFromUI('rsx', context),
      wozduh: getNavigatorSettingsFromUI('wozduh', context),
    };
  }

  function getMtfSyncSettings() {
    const mtfSettings = {};
    document.querySelectorAll('.mtf-sync-chk').forEach((chk) => {
      const tf = chk.dataset.tf;
      if (tf) mtfSettings[tf] = !!chk.checked;
    });
    return mtfSettings;
  }

  function syncMtfCheckboxesFromPeriods(periods) {
    const selected = new Set(Array.isArray(periods) ? periods : []);
    document.querySelectorAll('.mtf-sync-chk').forEach((chk) => {
      const tf = chk.dataset.tf;
      if (tf) chk.checked = selected.has(tf);
    });
  }

  function syncPeriodCheckboxFromMtf(tf, checked) {
    const popup = getNavigatorPopup('price');
    const container = popup?.querySelector('[data-nav-field="periods"]');
    if (!container) return;
    const periodInput = container.querySelector(`input[type="checkbox"][value="${tf}"]`);
    if (periodInput) periodInput.checked = !!checked;
  }

  function initMtfSyncCheckboxGroup() {
    const group = document.getElementById('trendlines-sync-group');
    if (!group || group.dataset.mtfBuilt === '1') return;
    group.dataset.mtfBuilt = '1';
    const body = group.querySelector('.mtf-sync-body') || group;
    const labels = MTF_SYNC_QUICK_PERIODS.map((tf) => {
      const label = tf === '1d' ? '1D' : tf.toUpperCase();
      return `<label><input type="checkbox" id="chk-sync-${tf}" class="mtf-sync-chk" data-tf="${tf}" /> Синхронизировать ${label} Тренд</label>`;
    }).join('');
    const heading = body.querySelector('h4');
    if (heading) {
      heading.insertAdjacentHTML('afterend', labels);
    } else {
      body.innerHTML = `<h4>Синхронизация ТФ (MTF)</h4>${labels}`;
    }
  }

  function initMtfSyncCheckboxes() {
    initMtfSyncCheckboxGroup();
    const pricePopup = getNavigatorPopup('price');
    if (pricePopup) {
      syncMtfCheckboxesFromPeriods(readNavigatorPeriodsFromPopup(pricePopup));
    }

    document.querySelectorAll('.mtf-sync-chk').forEach((chk) => {
      if (chk.dataset.mtfSyncBound === '1') return;
      chk.dataset.mtfSyncBound = '1';
      chk.addEventListener('change', () => {
        const tf = chk.dataset.tf;
        console.log(`MTF Sync toggled for ${tf}: ${chk.checked}`);
        syncPeriodCheckboxFromMtf(tf, chk.checked);
        const context = getContext();
        if (context === 'live') {
          flushSettingsChanged().catch((err) => {
            console.error('[UI] MTF sync live update failed:', err);
          });
          return;
        }
        notifySettingsChanged();
      });
    });

    const periodsContainer = pricePopup?.querySelector('[data-nav-field="periods"]');
    if (periodsContainer && periodsContainer.dataset.mtfPeriodSyncBound !== '1') {
      periodsContainer.dataset.mtfPeriodSyncBound = '1';
      periodsContainer.querySelectorAll('input[type="checkbox"]').forEach((el) => {
        el.addEventListener('change', () => {
          syncMtfCheckboxesFromPeriods(readNavigatorPeriodsFromPopup(pricePopup));
        });
      });
    }
  }

  function onSettingsChanged(callback) {
    settingsChangedCallback = callback;
  }

  function notifySettingsChanged() {
    clearTimeout(settingsChangedTimer);
    settingsChangedTimer = setTimeout(() => {
      settingsChangedTimer = null;
      if (settingsChangedCallback) {
        Promise.resolve(settingsChangedCallback()).catch((err) => {
          console.error('[UI] Navigator auto-update failed:', err);
        });
      }
    }, 500);
  }

  function flushSettingsChanged() {
    clearTimeout(settingsChangedTimer);
    settingsChangedTimer = null;
    if (settingsChangedCallback) {
      return Promise.resolve(settingsChangedCallback());
    }
    return Promise.resolve();
  }

  function isNavigatorPaneEnabled(pane, context = getContext()) {
    const nav = getNavigatorSettingsFromUI(pane, context);
    return nav.enabled === true;
  }

  function commitPaneSettings(pane) {
    const safePane = pane || 'price';
    const context = getContext();
    const popup = getNavigatorPopup(safePane);
    if (popup) {
      const uiSettings = normalizeNavigatorPaneSettings(
        readNavigatorSettingsFromPopup(popup, safePane),
        safePane,
      );
      saveNavigatorPaneSettings(safePane, uiSettings, context);
      if (context === 'backtest') {
        _dirtyState.backtest = true;
      } else {
        _dirtyState.live = true;
      }
    }

    if (context === 'live') {
      return getNavigatorPayload('live');
    }

    if (typeof buildFinalBacktestPayload === 'function') {
      const finalPayload = buildFinalBacktestPayload();
      console.log(`[UI] Ok clicked for ${safePane}. Payload injected:`, finalPayload.settings.navigators[safePane]);
      console.log('[UI] Full navigators:', finalPayload.settings.navigators);
      console.log('[UI] Matrix:', finalPayload.settings.matrix);
      return finalPayload;
    }

    return null;
  }

  function initPopupOkHandlers() {
    if (window.__navigatorOkBound) return;
    window.__navigatorOkBound = true;

    document.addEventListener('click', (event) => {
      const btn = event.target.closest('.btn-ok');
      if (!btn) return;

      const popup = btn.closest('.navigator-popup, .popup-container');
      if (!popup) return;

      const pane = popup.id?.replace('popup-', '') || popup.dataset.pane || '';
      if (!pane) return;

      event.preventDefault();
      event.stopPropagation();

      try {
        commitPaneSettings(pane);

        renderChartLegends('backtest');
        renderChartLegends('live');

        popup.hidden = true;
        if (openNavigatorPopupEl === popup) openNavigatorPopupEl = null;

        if (getContext() === 'live') {
          flushSettingsChanged().catch((err) => {
            console.error('[UI] Navigator Ok pipeline failed:', err);
          });
        }
      } catch (err) {
        console.error('[UI] Navigator popup Ok failed:', err);
      }
    }, true);
  }

  function getWrapForPane(context, pane) {
    const chartData = ChartAdapter.getChartHandle(context);
    if (!chartData?.elements) return null;
    if (pane === 'price') return chartData.elements.priceWrap;
    if (pane === 'wozduh') return chartData.elements.oscWrap;
    if (pane === 'rsx') return chartData.elements.rsxWrap;
    return null;
  }

  function ensureLegendVisibilityState(context, pane, legendId) {
    if (!chartLegendState[context][pane][legendId]) {
      chartLegendState[context][pane][legendId] = { visible: true };
    }
    return chartLegendState[context][pane][legendId];
  }

  function toggleLegendVisibility(context, pane, legendId) {
    const state = ensureLegendVisibilityState(context, pane, legendId);
    state.visible = !state.visible;
    ChartAdapter.setLegendVisibility(context, pane, legendId, state.visible, ChartAdapter.getChartType());
    if (legendId === 'trendlines') {
      const paneSettings = loadNavigatorPaneSettings(pane, context);
      paneSettings.linesVisible = state.visible;
      saveNavigatorPaneSettings(pane, paneSettings, context);
    }
    renderChartLegends(context);
  }

  function hideAllPopups() {
    document.querySelectorAll('.navigator-popup').forEach((el) => {
      el.hidden = true;
    });
    openNavigatorPopupEl = null;
  }

  function readNavigatorPeriodsFromPopup(popup) {
    const container = popup?.querySelector('[data-nav-field="periods"]');
    if (!container) return [];
    return Array.from(container.querySelectorAll('input[type="checkbox"]:checked'))
      .map((el) => el.value)
      .filter(Boolean);
  }

  function applyNavigatorPeriodsToPopup(popup, periods) {
    const container = popup?.querySelector('[data-nav-field="periods"]');
    if (!container) return;
    const selected = new Set(Array.isArray(periods) ? periods : []);
    container.querySelectorAll('input[type="checkbox"]').forEach((el) => {
      el.checked = selected.has(el.value);
    });
    if (popup?.dataset?.pane === 'price' || popup?.id === 'popup-price') {
      syncMtfCheckboxesFromPeriods(periods);
    }
  }

  function buildNavigatorMTFPeriodsHTML() {
    return `
      <div class="setting-row">
        <label>Periods (MTF)</label>
        <div class="period-checkboxes" data-nav-field="periods">
          <label><input type="checkbox" value="1m" /> 1m</label>
          <label><input type="checkbox" value="3m" /> 3m</label>
          <label><input type="checkbox" value="5m" /> 5m</label>
          <label><input type="checkbox" value="15m" /> 15m</label>
          <label><input type="checkbox" value="30m" /> 30m</label>
          <label><input type="checkbox" value="1h" /> 1h</label>
          <label><input type="checkbox" value="4h" /> 4h</label>
          <label><input type="checkbox" value="1d" /> 1d</label>
          <label><input type="checkbox" value="1w" /> 1w</label>
        </div>
      </div>`;
  }

  function buildNavigatorPopupHTML(pane) {
    const showTargets = pane === 'price';
    const p = pane;
    return `
    <div class="navigator-popup__header">
      <span>Trendlines — ${pane.toUpperCase()}</span>
      <button type="button" class="navigator-popup__close" aria-label="Close">×</button>
    </div>
    <div class="navigator-popup__body" data-nav-pane="${p}">
      ${buildNavigatorMTFPeriodsHTML()}
      ${showTargets ? `
      <div class="setting-row">
        <label class="menu-row"><input type="checkbox" data-nav-field="targetPrice" /> Price</label>
        <label class="menu-row"><input type="checkbox" data-nav-field="targetRSX" /> RSX</label>
        <label class="menu-row"><input type="checkbox" data-nav-field="targetWozduh" /> Wozduh</label>
      </div>` : ''}
      <div class="setting-row">
        <label>Trend Type</label>
        <select data-nav-field="trendType"><option value="Wicks">Wicks</option><option value="Body">Body</option></select>
      </div>
      <div class="setting-row">
        <label>Periods</label>
        <div class="term-row"><label><input type="checkbox" data-nav-field="useLong" /> Long</label><input type="number" data-nav-field="longLen" min="3" max="500" /></div>
        <div class="term-row"><label><input type="checkbox" data-nav-field="useMedium" /> Medium</label><input type="number" data-nav-field="mediumLen" min="3" max="500" /></div>
        <div class="term-row"><label><input type="checkbox" data-nav-field="useShort" /> Short</label><input type="number" data-nav-field="shortLen" min="3" max="500" /></div>
      </div>
      <div class="setting-row">
        <label>Term</label>
        <select data-nav-field="term"><option value="Long">Long</option><option value="Medium">Medium</option><option value="Short">Short</option></select>
      </div>
      <div class="setting-row">
        <label>HH/LL</label>
        <select data-nav-field="hhll">
          <option value="None">None</option>
          <option value="Only">Only HH/LL</option>
          <option value="Previous">HH/LL &amp; previous H/L</option>
        </select>
      </div>
      <div class="setting-row">
        <label class="menu-row"><input type="checkbox" data-nav-field="momentumEnabled" /> Momentum Break</label>
        <div class="inline-pair">
          <input type="number" data-nav-field="momentumPercent" min="0" step="1" title="% of ATR" placeholder="% ATR" />
          <input type="number" data-nav-field="momentumBars" min="1" step="1" title="ATR Bars" placeholder="ATR bars" />
        </div>
      </div>
      <div class="setting-row">
        <label class="menu-row"><input type="checkbox" data-nav-field="timeHoldEnabled" /> Time Hold Filter</label>
        <input type="number" data-nav-field="timeHoldBars" min="1" step="1" title="Hold bars" />
      </div>
      <div class="setting-row">
        <label class="menu-row"><input type="checkbox" data-nav-field="backgroundColor" /> Background Color</label>
        <label class="menu-row"><input type="checkbox" data-nav-field="barColor" /> Bar Color</label>
      </div>
    </div>
    <div class="navigator-popup__footer">
      <button type="button" class="btn-reset">Reset</button>
      <button type="button" class="btn-save-default">Save as Default</button>
      <button type="button" class="btn-ok">Ok</button>
    </div>`;
  }

  function applyNavigatorSettingsToPopup(popup, settings) {
    if (!popup) return;
    applyNavigatorPeriodsToPopup(popup, settings.periods);
    setNavPopupField(popup, 'targetPrice', settings.targetPrice, 'checkbox');
    setNavPopupField(popup, 'targetRSX', settings.targetRSX, 'checkbox');
    setNavPopupField(popup, 'targetWozduh', settings.targetWozduh, 'checkbox');
    setNavPopupField(popup, 'trendType', settings.trendType || 'Wicks');
    setNavPopupField(popup, 'useLong', settings.useLong, 'checkbox');
    setNavPopupField(popup, 'longLen', settings.longLen ?? 60);
    setNavPopupField(popup, 'useMedium', settings.useMedium, 'checkbox');
    setNavPopupField(popup, 'mediumLen', settings.mediumLen ?? 30);
    setNavPopupField(popup, 'useShort', settings.useShort, 'checkbox');
    setNavPopupField(popup, 'shortLen', settings.shortLen ?? 10);
    setNavPopupField(popup, 'term', settings.term || 'Long');
    setNavPopupField(popup, 'hhll', settings.hhll || 'None');
    setNavPopupField(popup, 'momentumEnabled', settings.momentumEnabled, 'checkbox');
    setNavPopupField(popup, 'momentumPercent', settings.momentumPercent ?? 100);
    setNavPopupField(popup, 'momentumBars', settings.momentumBars ?? 14);
    setNavPopupField(popup, 'timeHoldEnabled', settings.timeHoldEnabled, 'checkbox');
    setNavPopupField(popup, 'timeHoldBars', settings.timeHoldBars ?? 2);
    setNavPopupField(popup, 'backgroundColor', settings.backgroundColor, 'checkbox');
    setNavPopupField(popup, 'barColor', settings.barColor, 'checkbox');
  }

  function readNavigatorSettingsFromPopup(popup, pane) {
    if (!popup) return defaultNavigatorPaneSettings(pane);
    const base = loadNavigatorPaneSettings(pane, getContext());
    const chk = (field, fallback = false) => {
      const val = getNavPopupField(popup, field, 'checkbox');
      return val == null ? fallback : val;
    };
    const num = (field, fallback) => {
      const raw = getNavPopupField(popup, field);
      const n = parseInt(raw, 10);
      return Number.isFinite(n) ? n : fallback;
    };
    const flt = (field, fallback) => {
      const raw = getNavPopupField(popup, field);
      const n = parseFloat(raw);
      return Number.isFinite(n) ? n : fallback;
    };

    return {
      ...base,
      periods: readNavigatorPeriodsFromPopup(popup),
      targetPrice: chk('targetPrice', base.targetPrice !== false),
      targetRSX: chk('targetRSX', !!base.targetRSX),
      targetWozduh: chk('targetWozduh', !!base.targetWozduh),
      trendType: getNavPopupField(popup, 'trendType') || 'Wicks',
      useLong: chk('useLong', base.useLong !== false),
      longLen: num('longLen', base.longLen ?? 60),
      useMedium: chk('useMedium', base.useMedium !== false),
      mediumLen: num('mediumLen', base.mediumLen ?? 30),
      useShort: chk('useShort', base.useShort !== false),
      shortLen: num('shortLen', base.shortLen ?? 10),
      term: getNavPopupField(popup, 'term') || 'Long',
      hhll: getNavPopupField(popup, 'hhll') || 'None',
      momentumEnabled: chk('momentumEnabled', !!base.momentumEnabled),
      momentumPercent: flt('momentumPercent', base.momentumPercent ?? 100),
      momentumBars: num('momentumBars', base.momentumBars ?? 14),
      timeHoldEnabled: chk('timeHoldEnabled', !!base.timeHoldEnabled),
      timeHoldBars: num('timeHoldBars', base.timeHoldBars ?? 2),
      backgroundColor: chk('backgroundColor', !!base.backgroundColor),
      barColor: chk('barColor', !!base.barColor),
    };
  }

  function bindNavigatorPopupChrome(popup, pane) {
    if (!popup) return;
    popup.classList.add('navigator-popup', 'popup-container');
    if (!popup.dataset.pane) popup.dataset.pane = pane;
    if (!popup.id) popup.id = `popup-${pane}`;

    if (popup.dataset.chromeBound !== '1') {
      popup.dataset.chromeBound = '1';
      const header = popup.querySelector('.navigator-popup__header');
      if (header) initNavigatorPopupDrag(popup, header, pane);
      popup.querySelector('.navigator-popup__close')?.addEventListener('click', () => {
        popup.hidden = true;
        if (openNavigatorPopupEl === popup) openNavigatorPopupEl = null;
      });
      popup.addEventListener('mousedown', (e) => e.stopPropagation());
    }
    bindNavigatorPopupActions(popup, pane);
  }

  function bindNavigatorPopupActions(popup, pane) {
    if (!popup || popup.dataset.actionsBound === '1') return;
    popup.dataset.actionsBound = '1';

    popup.querySelector('.btn-save-default')?.addEventListener('click', () => {
      const settings = readNavigatorSettingsFromPopup(popup, pane);
      saveNavigatorDefaults(pane, settings);
    });

    popup.querySelector('.btn-reset')?.addEventListener('click', () => {
      applyNavigatorSettingsToPopup(popup, loadNavigatorDefaults(pane));
    });
  }

  function ensureNavigatorPopup(pane) {
    let popup = document.getElementById(`popup-${pane}`)
      || document.querySelector(`.navigator-popup[data-pane="${pane}"]`);

    if (popup) {
      bindNavigatorPopupChrome(popup, pane);
      return popup;
    }

    popup = document.createElement('div');
    popup.className = 'popup-container navigator-popup';
    popup.id = `popup-${pane}`;
    popup.dataset.pane = pane;
    popup.hidden = true;
    popup.innerHTML = buildNavigatorPopupHTML(pane);
    document.body.appendChild(popup);
    bindNavigatorPopupChrome(popup, pane);

    return popup;
  }

  function initNavigatorPopupDrag(popup, handle, pane) {
    let dragging = false;
    let startX = 0;
    let startY = 0;
    let originX = 0;
    let originY = 0;

    handle.addEventListener('mousedown', (e) => {
      if (e.target.closest('.navigator-popup__close')) return;
      dragging = true;
      startX = e.clientX;
      startY = e.clientY;
      originX = popup.offsetLeft;
      originY = popup.offsetTop;
      e.preventDefault();
    });

    document.addEventListener('mousemove', (e) => {
      if (!dragging) return;
      const x = originX + (e.clientX - startX);
      const y = originY + (e.clientY - startY);
      popup.style.left = `${x}px`;
      popup.style.top = `${y}px`;
    });

    document.addEventListener('mouseup', () => {
      if (!dragging) return;
      dragging = false;
      saveNavigatorPopupPos(pane, popup.offsetLeft, popup.offsetTop);
    });
  }

  function openNavigatorPopup(pane, anchorEl) {
    const popup = ensureNavigatorPopup(pane);
    if (openNavigatorPopupEl === popup && popup && !popup.hidden) {
      hideAllPopups();
      return;
    }

    hideAllPopups();
    if (typeof hideRsxSettingsMenus === 'function') hideRsxSettingsMenus();
    if (typeof WozduhController !== 'undefined') WozduhController.hideMenus();
    if (typeof RiskController !== 'undefined') RiskController.hideMenu();

    if (!popup.querySelector('[data-nav-field="trendType"]')) {
      popup.dataset.actionsBound = '';
      popup.dataset.chromeBound = '';
      popup.innerHTML = buildNavigatorPopupHTML(pane);
      bindNavigatorPopupChrome(popup, pane);
    }
    applyNavigatorSettingsToPopup(popup, loadNavigatorPaneSettings(pane, getContext()));

    const saved = loadNavigatorPopupPos(pane);
    if (saved?.x != null && saved?.y != null) {
      popup.style.left = `${saved.x}px`;
      popup.style.top = `${saved.y}px`;
    } else if (anchorEl) {
      const rect = anchorEl.getBoundingClientRect();
      popup.style.left = `${rect.right + 6}px`;
      popup.style.top = `${rect.top}px`;
    } else {
      popup.style.left = '120px';
      popup.style.top = '80px';
    }

    popup.hidden = false;
    openNavigatorPopupEl = popup;
  }

  function renderLegendItem(context, pane, def, isChild = false) {
    const state = ensureLegendVisibilityState(context, pane, def.id);
    const hiddenClass = state.visible ? '' : ' legend-item--hidden';
    const eyeClass = state.visible ? '' : ' legend-btn--off';
    const hasGear = def.kind === 'wozduh' || def.kind === 'rsx' || def.id === 'trendlines';

    const item = document.createElement('div');
    item.className = `legend-item${isChild ? ' legend-item--child' : ''}${hiddenClass}`;
    item.dataset.legend = def.id;
    item.innerHTML = `
    <span class="legend-name">${def.label}</span>
    <button type="button" class="legend-btn legend-eye${eyeClass}" title="Toggle visibility">👁️</button>
    ${hasGear ? '<button type="button" class="legend-btn legend-gear" title="Settings">⚙️</button>' : ''}`;

    item.querySelector('.legend-eye')?.addEventListener('click', (e) => {
      e.stopPropagation();
      toggleLegendVisibility(context, pane, def.id);
    });

    item.querySelector('.legend-gear')?.addEventListener('click', (e) => {
      e.stopPropagation();
      if (def.id === 'trendlines') {
        openNavigatorPopup(pane, e.currentTarget);
        return;
      }
      const wrap = getWrapForPane(context, pane);
      if (!wrap) return;
      if (def.kind === 'wozduh') {
        if (typeof hideRsxSettingsMenus === 'function') hideRsxSettingsMenus();
        if (typeof RiskController !== 'undefined') RiskController.hideMenu();
        const menu = WozduhController.getSettingsMenu(wrap);
        if (!menu) return;
        const willOpen = menu.hidden;
        WozduhController.hideMenus();
        if (willOpen && typeof openFloatingMenu === 'function') openFloatingMenu(menu, e.currentTarget);
      } else if (def.kind === 'rsx') {
        if (typeof WozduhController !== 'undefined') WozduhController.hideMenus();
        if (typeof RiskController !== 'undefined') RiskController.hideMenu();
        const menu = typeof getRsxSettingsMenu === 'function' ? getRsxSettingsMenu(wrap) : null;
        if (!menu) return;
        const willOpen = menu.hidden;
        if (typeof hideRsxSettingsMenus === 'function') hideRsxSettingsMenus();
        if (willOpen && typeof openFloatingMenu === 'function') openFloatingMenu(menu, e.currentTarget);
      }
    });

    return item;
  }

  function renderChartLegends(context) {
    document.querySelectorAll(`.chart-legend[data-context="${context}"]`).forEach((legendEl) => {
      const pane = legendEl.dataset.pane;
      legendEl.innerHTML = '';
      const defs = CHART_LEGEND_DEFS[pane] || [];
      defs.forEach((def) => {
        legendEl.appendChild(renderLegendItem(context, pane, def, false));
      });

      if (NAVIGATOR_PANES.includes(pane)) {
        const tlDef = { id: 'trendlines', label: 'TLine', kind: 'trendlines' };
        const tlState = loadNavigatorPaneSettings(pane, context);
        chartLegendState[context][pane].trendlines = { visible: tlState.linesVisible !== false };
        legendEl.appendChild(renderLegendItem(context, pane, tlDef, false));
      }

      if (context === 'backtest' && pane === 'price') {
        const tradeDef = { id: 'trades', label: 'Trades', kind: 'trades' };
        legendEl.appendChild(renderLegendItem(context, pane, tradeDef, true));
      }
    });
  }

  function initLegends() {
    ['price', 'rsx', 'wozduh'].forEach((pane) => {
      const popup = ensureNavigatorPopup(pane);
      applyNavigatorSettingsToPopup(popup, loadNavigatorPaneSettings(pane, 'backtest'));
    });
    renderChartLegends('live');
    renderChartLegends('backtest');
  }

  function init() {
    initPopupOkHandlers();
    initMtfSyncCheckboxes();
  }

  function filterNavigatorLinesByTerm(lines, pane) {
    const settings = loadNavigatorPaneSettings(pane, getContext());
    const allowed = [];
    if (settings.useLong !== false) allowed.push('solid');
    if (settings.useMedium !== false) allowed.push('dashed');
    if (settings.useShort !== false) allowed.push('dotted');
    if (allowed.length === 0) return lines || [];
    return (lines || []).filter((line) => {
      const style = String(line.style || 'solid').toLowerCase();
      return allowed.includes(style);
    });
  }

  function filterNavigatorMarkersByHHLL(markers, pane) {
    const mode = loadNavigatorPaneSettings(pane, getContext()).hhll || 'None';
    if (mode === 'None') return [];
    if (mode === 'Only') {
      return (markers || []).filter((m) => {
        const type = String(m.type || m.text || '').trim();
        return type === 'HH' || type === 'LL';
      });
    }
    return markers || [];
  }

  function buildNavigatorMarkers(navigatorData, candles, pane = 'price') {
    const filtered = filterNavigatorMarkersByHHLL(navigatorData?.markers, pane);
    const markers = [];
    filtered.forEach((m) => {
      const time = chartTime(m.time) ?? navigatorBarIndexToTime(m.index, candles);
      if (time == null) return;
      const type = String(m.type || m.text || '').trim();
      if (type === 'HH') {
        markers.push({
          time,
          position: 'aboveBar',
          shape: 'circle',
          text: 'HH',
          color: (typeof ChartTheme !== 'undefined')
            ? ChartTheme.navMarkerColor('HH', m.color)
            : (m.color || '#089981'),
        });
      } else if (type === 'LL') {
        markers.push({
          time,
          position: 'belowBar',
          shape: 'circle',
          text: 'LL',
          color: (typeof ChartTheme !== 'undefined')
            ? ChartTheme.navMarkerColor('LL', m.color)
            : (m.color || '#f23645'),
        });
      } else if (type === 'WickBreak') {
        markers.push({
          time,
          position: 'inBar',
          shape: 'circle',
          color: (typeof ChartTheme !== 'undefined')
            ? ChartTheme.navMarkerColor('WickBreak', m.color)
            : (m.color || '#ff5d00'),
          size: 1,
        });
      }
    });
    return markers.sort((a, b) => a.time - b.time);
  }

  function navigatorHasBarColors(barColors) {
    if (Array.isArray(barColors)) return barColors.length > 0;
    if (barColors && typeof barColors === 'object') return Object.keys(barColors).length > 0;
    return false;
  }

  function navigatorResultHasContent(dto) {
    if (!dto) return false;
    return (dto.lines?.length > 0)
      || (dto.markers?.length > 0)
      || navigatorHasBarColors(dto.barColors)
      || (dto.backgroundZones?.length > 0);
  }

  function resolveNavigatorResults(result) {
    if (!result) return {};

    if (result.navigators && typeof result.navigators === 'object') {
      const keys = Object.keys(result.navigators);
      if (keys.length > 0) {
        return result.navigators;
      }
    }

    const legacy = result.navigatorData || result.navigatorPrice;
    if (navigatorResultHasContent(legacy)) {
      return { price: legacy };
    }

    return {};
  }

  if (typeof window !== 'undefined') {
    window.getNavigatorPluginForPane = getNavigatorPluginForPane;
    window.normalizeMtfPeriod = normalizeMtfPeriod;
    window.mtfPeriodsEqual = mtfPeriodsEqual;
    window.getMtfPeriodColor = getMtfPeriodColor;
    window.resolveChartTfForNavigator = resolveChartTfForNavigator;
    window.getActiveMtfPeriods = getActiveMtfPeriods;
    window.groupNavigatorLinesByInterval = groupNavigatorLinesByInterval;
    window.getNavigatorPaneSettingsFromUI = getNavigatorPaneSettingsFromUI;
    window.isNavigatorPaneEnabled = isNavigatorPaneEnabled;
    window.filterNavigatorLinesByTerm = filterNavigatorLinesByTerm;
    window.filterNavigatorMarkersByHHLL = filterNavigatorMarkersByHHLL;
    window.buildNavigatorMarkers = buildNavigatorMarkers;
    window.navigatorHasBarColors = navigatorHasBarColors;
    window.resolveNavigatorResults = resolveNavigatorResults;
  }

  return {
    init,
    initLegends,
    getNavigatorPayload,
    getMtfSyncSettings,
    getContext,
    onSettingsChanged,
    notifySettingsChanged,
    flushSettingsChanged,
    renderChartLegends,
    hideAllPopups,
    openNavigatorPopup,
    consumeDirtyState: (context = 'backtest') => {
      const key = context === 'live' ? 'live' : 'backtest';
      const isDirty = _dirtyState[key];
      _dirtyState[key] = false;
      return isDirty;
    },
  };
})();

if (typeof window !== 'undefined') {
  window.NavigatorController = NavigatorController;
}
