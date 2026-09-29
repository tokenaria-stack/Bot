/**
 * STAR-CHART-VISUAL-DISPLAY-V1.
 * Workstation paint prefs for Population Stars. Not Population JSON. Not Wozduh prefs.
 */
const StarPopulationDisplay = (() => {
  const STORAGE_KEY = 'star-population-display-v1';
  const SHAPES = Object.freeze(['star4', 'arrow', 'circle', 'triangle', 'square', 'diamond']);
  const SIZE_MIN = 4;
  const SIZE_MAX = 18;
  const DEFAULTS = Object.freeze({
    shape: 'star4',
    size: 9,
    upColor: '#00E676',
    downColor: '#FF1744',
  });

  let memory = null;

  function hexOk(value) {
    return typeof value === 'string' && /^#[0-9A-Fa-f]{6}$/.test(value);
  }

  function shapeOk(value) {
    return SHAPES.indexOf(value) >= 0;
  }

  function sizeOk(value) {
    const n = Number(value);
    return Number.isFinite(n) && n >= SIZE_MIN && n <= SIZE_MAX;
  }

  function normalize(raw) {
    const src = raw && typeof raw === 'object' ? raw : {};
    return {
      shape: shapeOk(src.shape) ? src.shape : DEFAULTS.shape,
      size: sizeOk(src.size) ? Math.round(Number(src.size)) : DEFAULTS.size,
      upColor: hexOk(src.upColor) ? src.upColor : DEFAULTS.upColor,
      downColor: hexOk(src.downColor) ? src.downColor : DEFAULTS.downColor,
    };
  }

  function readStore() {
    try {
      if (typeof localStorage === 'undefined') return null;
      const raw = localStorage.getItem(STORAGE_KEY);
      if (!raw) return null;
      return JSON.parse(raw);
    } catch {
      return null;
    }
  }

  function writeStore(prefs) {
    try {
      if (typeof localStorage === 'undefined') return;
      localStorage.setItem(STORAGE_KEY, JSON.stringify(prefs));
    } catch { /* preference only */ }
  }

  function get() {
    if (memory) return memory;
    memory = normalize(readStore());
    return memory;
  }

  function set(partial) {
    const next = normalize(Object.assign({}, get(), partial || {}));
    memory = next;
    writeStore(next);
    return next;
  }

  function reset() {
    memory = {
      shape: DEFAULTS.shape,
      size: DEFAULTS.size,
      upColor: DEFAULTS.upColor,
      downColor: DEFAULTS.downColor,
    };
    try {
      if (typeof localStorage !== 'undefined') localStorage.removeItem(STORAGE_KEY);
    } catch { /* */ }
    return get();
  }

  function _resetForTests() {
    memory = null;
    try {
      if (typeof localStorage !== 'undefined') localStorage.removeItem(STORAGE_KEY);
    } catch { /* */ }
  }

  const api = {
    STORAGE_KEY,
    SHAPES,
    SIZE_MIN,
    SIZE_MAX,
    DEFAULTS,
    get,
    set,
    reset,
    normalize,
    _resetForTests,
  };

  if (typeof window !== 'undefined') window.StarPopulationDisplay = api;
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  return api;
})();
