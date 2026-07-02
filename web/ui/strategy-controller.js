/**
 * Phase 19.5.2 — Strategy matrix and L/S thresholds (live + backtest contexts).
 */
const StrategyController = (() => {
  let liveStrategyState = null;
  let backtestStrategyState = null;
  let matrixModalContext = 'live';

  function defaultStrategyState() {
    return {
      matrix: { ...SCORING_MATRIX_DEFAULTS },
      thresholds: { ...DEFAULT_STRATEGY_THRESHOLDS },
    };
  }

  function getStrategyState(context = 'live') {
    return context === 'backtest' ? backtestStrategyState : liveStrategyState;
  }

  function strategyStorageKey(context) {
    return context === 'backtest' ? LS_BT_STRATEGY_KEY : LS_LIVE_STRATEGY_KEY;
  }

  function persistStrategyState(context) {
    const state = getStrategyState(context);
    if (!state) return;
    try {
      localStorage.setItem(strategyStorageKey(context), JSON.stringify(state));
    } catch {
      /* noop */
    }
  }

  function loadLegacyThresholdsFromStorage() {
    try {
      const raw = localStorage.getItem(THRESHOLDS_KEY);
      if (!raw) return null;
      return normalizeStrategyThresholds(JSON.parse(raw));
    } catch {
      return null;
    }
  }

  function loadLegacyScoringMatrixFromStorage() {
    try {
      const raw = localStorage.getItem(SCORING_MATRIX_KEY);
      if (!raw) return null;
      return normalizeStrategyMatrix(JSON.parse(raw));
    } catch {
      return null;
    }
  }

  function loadStrategyState(context) {
    let state = defaultStrategyState();
    try {
      const raw = localStorage.getItem(strategyStorageKey(context));
      if (raw) {
        const parsed = JSON.parse(raw);
        if (parsed && typeof parsed === 'object') {
          state = {
            matrix: normalizeStrategyMatrix(parsed.matrix),
            thresholds: normalizeStrategyThresholds(parsed.thresholds),
          };
        }
      } else if (context === 'live') {
        const legacyMatrix = loadLegacyScoringMatrixFromStorage();
        const legacyThresholds = loadLegacyThresholdsFromStorage();
        if (legacyMatrix) state.matrix = legacyMatrix;
        if (legacyThresholds) state.thresholds = legacyThresholds;
      } else if (context === 'backtest' && liveStrategyState) {
        state = {
          matrix: { ...liveStrategyState.matrix },
          thresholds: { ...liveStrategyState.thresholds },
        };
      }
    } catch {
      /* defaults */
    }
    if (context === 'backtest') backtestStrategyState = state;
    else liveStrategyState = state;
    persistStrategyState(context);
    return state;
  }

  function readThresholdsFromHeader() {
    const longEl = document.getElementById('ui-threshold-long');
    const shortEl = document.getElementById('ui-threshold-short');
    return normalizeStrategyThresholds({
      long: longEl ? Number(longEl.value) : NaN,
      short: shortEl ? Number(shortEl.value) : NaN,
    });
  }

  function applyThresholdsToHeader(thresholds) {
    const longEl = document.getElementById('ui-threshold-long');
    const shortEl = document.getElementById('ui-threshold-short');
    const t = normalizeStrategyThresholds(thresholds);
    if (longEl) longEl.value = String(t.long);
    if (shortEl) shortEl.value = String(t.short);
  }

  function saveThresholdsFromHeaderToState(context = TabsController.getActiveStrategyContext()) {
    const state = getStrategyState(context);
    if (!state) return;
    state.thresholds = readThresholdsFromHeader();
    persistStrategyState(context);
  }

  function getThresholds(context = 'backtest') {
    const t = normalizeStrategyThresholds(getStrategyState(context)?.thresholds);
    return { long: t.long, short: t.short };
  }

  function getThresholdsPayload(context = 'backtest') {
    const t = getThresholds(context);
    return { longThreshold: t.long, shortThreshold: t.short };
  }

  function getMatrixPayload(context = 'backtest') {
    return normalizeStrategyMatrix(getStrategyState(context)?.matrix);
  }

  function matrixHasEntrySources(matrix) {
    if (!matrix) return false;
    return !!(matrix.useRSX || matrix.useWozduhCross || matrix.useTrendlines);
  }

  function readMatrixCheckbox(key) {
    const el = document.getElementById(`matrix-${key}`)
      || document.querySelector(`input[type="checkbox"][data-matrix-key="${key}"]`);
    if (!el) return null;
    return !!el.checked;
  }

  function collectMatrixFromUI() {
    const matrix = { ...SCORING_MATRIX_DEFAULTS };
    SCORING_MATRIX_LABELS.forEach(({ key }) => {
      const checked = readMatrixCheckbox(key);
      if (checked !== null) {
        matrix[key] = checked;
      }
    });
    return matrix;
  }

  function applyMatrixToUI(matrix) {
    const merged = { ...SCORING_MATRIX_DEFAULTS, ...matrix };
    SCORING_MATRIX_LABELS.forEach(({ key }) => {
      const el = document.getElementById(`matrix-${key}`)
        || document.querySelector(`input[type="checkbox"][data-matrix-key="${key}"]`);
      if (el) el.checked = !!merged[key];
    });
  }

  async function postThresholdsToServer(thresholds) {
    const t = normalizeStrategyThresholds(thresholds);
    try {
      await API.postThresholds({ long: t.long, short: t.short });
    } catch (err) {
      console.warn('Failed to sync thresholds:', err);
    }
  }

  async function postMatrixToServer(matrix) {
    const payload = normalizeStrategyMatrix(matrix);
    try {
      await API.postMatrix(payload);
    } catch (err) {
      console.warn('Failed to sync scoring matrix:', err);
    }
  }

  async function syncThresholds() {
    const context = TabsController.getActiveStrategyContext();
    const thresholds = readThresholdsFromHeader();
    const state = getStrategyState(context);
    if (!state) return;
    state.thresholds = thresholds;
    persistStrategyState(context);

    if (context === 'live') {
      await postThresholdsToServer(thresholds);
    }
  }

  function loadThresholdsFromStorage() {
    applyThresholdsToHeader(liveStrategyState?.thresholds || DEFAULT_STRATEGY_THRESHOLDS);
  }

  function initThresholdInputs() {
    loadThresholdsFromStorage();
    if (TabsController.isLiveTabActive()) {
      postThresholdsToServer(liveStrategyState.thresholds);
    }

    ['ui-threshold-long', 'ui-threshold-short'].forEach((id) => {
      const el = document.getElementById(id);
      if (!el) return;
      el.addEventListener('change', () => syncThresholds());
    });
  }

  async function saveMatrixForContext(context = matrixModalContext) {
    const matrix = collectMatrixFromUI();
    const state = getStrategyState(context);
    if (!state) return;
    state.matrix = normalizeStrategyMatrix(matrix);
    persistStrategyState(context);

    if (context === 'live') {
      await postMatrixToServer(state.matrix);
    }
  }

  function openMatrixModal() {
    matrixModalContext = TabsController.getActiveStrategyContext();
    applyMatrixToUI(getStrategyState(matrixModalContext).matrix);
    const title = document.getElementById('matrix-modal-title');
    if (title) {
      title.textContent = matrixModalContext === 'backtest'
        ? 'Signal Matrix (Backtest)'
        : 'Signal Matrix (Live)';
    }
    const modal = document.getElementById('matrix-modal');
    if (!modal) return;
    modal.classList.add('open');
    modal.setAttribute('aria-hidden', 'false');
  }

  function closeMatrixModal() {
    const modal = document.getElementById('matrix-modal');
    if (!modal) return;
    modal.classList.remove('open');
    modal.setAttribute('aria-hidden', 'true');
  }

  function initScoringMatrix() {
    const body = document.getElementById('matrix-modal-body');
    if (!body) return;

    if (!body.querySelector('[data-matrix-key]')) {
      body.innerHTML = SCORING_MATRIX_LABELS.map(({ key, label }) => `
        <label class="matrix-toggle">
          <input type="checkbox" id="matrix-${key}" data-matrix-key="${key}" checked />
          ${label}
        </label>
      `).join('');
    }

    applyMatrixToUI(liveStrategyState?.matrix || SCORING_MATRIX_DEFAULTS);

    document.getElementById('matrix-open-btn')?.addEventListener('click', openMatrixModal);
    document.getElementById('matrix-modal-close')?.addEventListener('click', closeMatrixModal);
    document.getElementById('matrix-modal-backdrop')?.addEventListener('click', closeMatrixModal);
    document.getElementById('matrix-save-btn')?.addEventListener('click', async () => {
      try {
        await saveMatrixForContext(matrixModalContext);
        closeMatrixModal();
      } catch (err) {
        console.warn('Failed to save scoring matrix:', err);
      }
    });
    document.addEventListener('keydown', (e) => {
      if (e.key === 'Escape') closeMatrixModal();
    });
  }

  function init() {
    loadStrategyState('live');
    loadStrategyState('backtest');
    initThresholdInputs();
    initScoringMatrix();
  }

  return {
    init,
    getMatrixPayload,
    getThresholds,
    getThresholdsPayload,
    getStrategyState,
    saveThresholdsFromHeaderToState,
    applyThresholdsToHeader,
    matrixHasEntrySources,
    openMatrixModal,
    closeMatrixModal,
  };
})();

if (typeof window !== 'undefined') {
  window.StrategyController = StrategyController;
}
