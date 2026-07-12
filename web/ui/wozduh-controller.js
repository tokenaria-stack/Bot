/**
 * Phase 19.5 — Wozduh visibility menu (live + backtest osc panes).
 */
const WozduhController = (() => {
  function wozduhStorageKey(context) {
    return context === 'backtest' ? WOZDUH_PREFS_BACKTEST_KEY : WOZDUH_PREFS_LIVE_KEY;
  }

  function oscContextFromWrap(wrap) {
    return wrap?.id === 'bt-osc-wrap' ? 'backtest' : 'live';
  }

  function getSettingsMenu(wrap) {
    return wrap?.querySelector('.wozduh-settings-menu');
  }

  function applyPrefsToMenu(menu, prefs) {
    if (!menu) return;
    WOZDUH_MENU_ITEMS.forEach((item) => {
      const el = menu.querySelector(`.wozduh-chk[data-pref-key="${item.prefKey}"]`);
      if (!el) return;
      el.checked = typeof prefs[item.prefKey] === 'boolean' ? prefs[item.prefKey] : item.default;
    });
  }

  function readPrefsFromMenu(menu) {
    const prefs = {};
    if (!menu) return prefs;
    menu.querySelectorAll('.wozduh-chk').forEach((el) => {
      const key = el.dataset.prefKey;
      if (key) prefs[key] = el.checked;
    });
    return prefs;
  }

  function savePrefs(context, prefs) {
    localStorage.setItem(wozduhStorageKey(context), JSON.stringify(prefs));
  }

  function loadPrefsForContext(context) {
    try {
      const raw = localStorage.getItem(wozduhStorageKey(context));
      if (raw) {
        const prefs = JSON.parse(raw);
        if (prefs && typeof prefs === 'object') return prefs;
      }
      if (context === 'live') {
        const legacy = localStorage.getItem(WOZDUH_PREFS_KEY);
        if (legacy) {
          const prefs = JSON.parse(legacy);
          if (prefs && typeof prefs === 'object') return prefs;
        }
      }
    } catch {
      /* use defaults */
    }
    return null;
  }

  function getSettingsFromUI(context = 'backtest') {
    const wrapId = context === 'backtest' ? 'bt-osc-wrap' : 'osc-wrap';
    const menu = document.getElementById(wrapId)?.querySelector('.wozduh-settings-menu');
    if (menu) return readPrefsFromMenu(menu);
    return loadPrefsForContext(context) || CONFIG.defaultWozduhPrefs();
  }

  function getPrefsForChart(context) {
    return loadPrefsForContext(context) || CONFIG.defaultWozduhPrefs();
  }

  function hideMenus() {
    document.querySelectorAll('.osc-wrap .wozduh-settings-menu').forEach((menu) => {
      menu.hidden = true;
    });
  }

  function init() {
    document.querySelectorAll('.osc-wrap').forEach((wrap) => {
      const context = oscContextFromWrap(wrap);
      const menu = getSettingsMenu(wrap);
      if (!menu) return;
      const prefs = loadPrefsForContext(context) || CONFIG.defaultWozduhPrefs();
      applyPrefsToMenu(menu, prefs);
      savePrefs(context, prefs);
      if (typeof SettingsRenderer !== 'undefined' && context === 'live' && window.DDRFactory?.cutoverActive) {
        SettingsRenderer.applyWozduhPrefs(context, prefs);
      } else {
        ChartAdapter.applyWozduhVisibility(context);
      }
    });

    document.querySelectorAll('.osc-wrap').forEach((wrap) => {
      const context = oscContextFromWrap(wrap);
      const toggle = wrap.querySelector('.wozduh-settings-toggle');
      const menu = getSettingsMenu(wrap);
      if (!menu) return;

      toggle?.addEventListener('click', (e) => {
        e.stopPropagation();
        if (typeof hideRsxSettingsMenus === 'function') hideRsxSettingsMenus();
        if (typeof RiskController !== 'undefined') RiskController.hideMenu();
        const willOpen = menu.hidden;
        hideMenus();
        if (willOpen && typeof openFloatingMenu === 'function') openFloatingMenu(menu, toggle);
        else menu.hidden = true;
      });

      menu.addEventListener('mousedown', (e) => e.stopPropagation());
      menu.addEventListener('click', (e) => e.stopPropagation());

      menu.querySelectorAll('.wozduh-chk').forEach((el) => {
        el.addEventListener('change', () => {
          const prefs = readPrefsFromMenu(menu);
          savePrefs(context, prefs);
          if (typeof SettingsRenderer !== 'undefined' && context === 'live' && window.DDRFactory?.cutoverActive) {
            SettingsRenderer.applyWozduhPrefs(context, prefs);
          } else {
            ChartAdapter.applyWozduhVisibility(context, prefs);
          }
        });
      });
    });
  }

  return {
    init,
    applyPrefsToMenu,
    readPrefsFromMenu,
    savePrefs,
    loadPrefsForContext,
    getSettingsFromUI,
    getPrefsForChart,
    hideMenus,
    getSettingsMenu,
  };
})();

if (typeof window !== 'undefined') {
  window.WozduhController = WozduhController;
  window.getWozduhPrefsForChart = (context) => WozduhController.getPrefsForChart(context);
}
