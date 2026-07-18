/**
 * Phase F — Risk settings UI purged with /api/settings/risk.
 * Socket kept so boot/legacy callers do not throw.
 */
const RiskController = (() => {
  function hideMenu() {
    const menu = document.getElementById('risk-settings-menu');
    if (menu) menu.hidden = true;
  }

  function getSettingsFromUI() {
    return null;
  }

  function init() {
    hideMenu();
    const btn = document.getElementById('risk-save-btn');
    if (btn) btn.hidden = true;
    document.querySelectorAll('[data-open-risk], #risk-open-btn, .risk-settings-trigger').forEach((el) => {
      el.hidden = true;
    });
  }

  return {
    init,
    hideMenu,
    getSettingsFromUI,
  };
})();

if (typeof window !== 'undefined') {
  window.RiskController = RiskController;
}
