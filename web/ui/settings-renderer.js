/**
 * SettingsRenderer — Phase 14.11 stub + minimal live Wozduh visibility bridge.
 * Full prefs→series matrix returns when Falcon/Chaos layers remount into DDR.
 */
const SettingsRenderer = (() => {
  function applyWozduhPrefs(_context, prefs) {
    const factory = (typeof window !== 'undefined') ? window.DDRFactory : null;
    if (!factory?.cutoverActive || typeof factory.setSeriesVisible !== 'function') return;
    if (!prefs || typeof prefs !== 'object') return;
    if (typeof prefs.rsiVol === 'boolean') {
      factory.setSeriesVisible('woz_fast', prefs.rsiVol);
      factory.setSeriesVisible('woz_slow', prefs.rsiVol);
    }
  }

  function initToolbarToggles() { /* RSX/volume toggles: ChartAdapter / future DDR */ }

  function refreshFromManifest() {
    if (typeof window !== 'undefined' && window.WozduhController) {
      applyWozduhPrefs('live', window.WozduhController.getPrefsForChart('live'));
    }
  }

  function setToggleVisible() {
    return false;
  }

  return {
    applyWozduhPrefs,
    initToolbarToggles,
    refreshFromManifest,
    setToggleVisible,
  };
})();

if (typeof window !== 'undefined') {
  window.SettingsRenderer = SettingsRenderer;
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = { SettingsRenderer };
}
