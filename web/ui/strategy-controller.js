/**
 * Phase F — Strategy matrix / L-S thresholds purged.
 * Socket kept so boot/legacy callers do not throw; no server sync.
 */
const StrategyController = (() => {
  function getStrategyState() {
    return { matrix: {}, thresholds: { long: 0, short: 0 } };
  }

  function getMatrixPayload() {
    return {};
  }

  function getThresholdsPayload() {
    return {};
  }

  function saveThresholdsFromHeaderToState() {}

  function applyThresholdsToHeader() {}

  function init() {
    const modal = document.getElementById('matrix-modal');
    if (modal) {
      modal.hidden = true;
      modal.setAttribute('aria-hidden', 'true');
      modal.classList.remove('open');
    }
    ['ui-threshold-long', 'ui-threshold-short', 'matrix-open-btn'].forEach((id) => {
      const el = document.getElementById(id);
      if (el) el.hidden = true;
    });
  }

  return {
    init,
    getStrategyState,
    getMatrixPayload,
    getThresholdsPayload,
    saveThresholdsFromHeaderToState,
    applyThresholdsToHeader,
  };
})();

if (typeof window !== 'undefined') {
  window.StrategyController = StrategyController;
}
