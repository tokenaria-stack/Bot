/**
 * Phase 19.5 — Risk management settings panel.
 */
const RiskController = (() => {
  function hideMenu() {
    const riskMenu = document.getElementById('risk-settings-menu');
    if (riskMenu) riskMenu.hidden = true;
  }

  function readFromForm() {
    return {
      risk_per_trade: parseFloat(document.getElementById('risk-per-trade')?.value) || 1,
      max_drawdown: parseFloat(document.getElementById('risk-max-drawdown')?.value) || 5,
      leverage: parseInt(document.getElementById('risk-leverage')?.value, 10) || 10,
      stop_loss_type: document.getElementById('risk-stop-loss-type')?.value || 'fractal_atr',
      atr_multiplier: parseFloat(document.getElementById('risk-atr-multiplier')?.value) || 1.5,
    };
  }

  function applyToForm(settings) {
    if (!settings) return;
    const map = [
      ['risk-per-trade', settings.risk_per_trade],
      ['risk-max-drawdown', settings.max_drawdown],
      ['risk-leverage', settings.leverage],
      ['risk-atr-multiplier', settings.atr_multiplier],
    ];
    map.forEach(([id, val]) => {
      const el = document.getElementById(id);
      if (el && val != null) el.value = String(val);
    });
    const typeEl = document.getElementById('risk-stop-loss-type');
    if (typeEl && settings.stop_loss_type) typeEl.value = settings.stop_loss_type;
  }

  async function fetchSettings() {
    try {
      const settings = await API.fetchRiskSettings();
      applyToForm(settings);
      localStorage.setItem(LS_RISK_SETTINGS_KEY, JSON.stringify(settings));
    } catch (err) {
      console.warn('Failed to load risk settings:', err);
      try {
        const raw = localStorage.getItem(LS_RISK_SETTINGS_KEY);
        if (raw) applyToForm(JSON.parse(raw));
      } catch { /* noop */ }
    }
  }

  async function saveSettings() {
    const menu = document.getElementById('risk-settings-menu');
    const active = document.activeElement;
    if (menu && active && menu.contains(active) && typeof active.blur === 'function') {
      active.blur();
    }
    const payload = readFromForm();
    try {
      const applied = await API.postRiskSettings(payload);
      applyToForm(applied);
      localStorage.setItem(LS_RISK_SETTINGS_KEY, JSON.stringify(applied));
      hideMenu();
      if (TabsController.isBacktestTabActive() && backtestStore.candleCount() > 0) {
        buildFinalBacktestPayload();
      }
    } catch (err) {
      console.warn('Failed to save risk settings:', err);
    }
  }

  function getSettingsFromUI() {
    try {
      return readFromForm();
    } catch (err) {
      console.warn('getRiskSettingsFromUI failed:', err);
      try {
        const raw = localStorage.getItem(LS_RISK_SETTINGS_KEY);
        if (raw) return JSON.parse(raw);
      } catch {
        /* noop */
      }
      return {};
    }
  }

  function init() {
    const btn = document.getElementById('btn-risk-menu');
    const menu = document.getElementById('risk-settings-menu');
    if (!btn || !menu) return;

    fetchSettings();

    btn.addEventListener('click', (e) => {
      e.stopPropagation();
      if (typeof hideRsxSettingsMenus === 'function') hideRsxSettingsMenus();
      if (typeof WozduhController !== 'undefined') WozduhController.hideMenus();
      const willOpen = menu.hidden;
      if (willOpen && typeof openFloatingMenu === 'function') openFloatingMenu(menu, btn);
      else menu.hidden = true;
    });

    menu.addEventListener('mousedown', (e) => e.stopPropagation());
    menu.addEventListener('click', (e) => e.stopPropagation());
    document.getElementById('risk-save-btn')?.addEventListener('click', () => { saveSettings(); });
  }

  return {
    init,
    hideMenu,
    readFromForm,
    applyToForm,
    fetchSettings,
    saveSettings,
    getSettingsFromUI,
  };
})();

if (typeof window !== 'undefined') {
  window.RiskController = RiskController;
}
