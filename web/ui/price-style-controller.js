/**
 * PRICE-SERIES-STYLE-1.1 — price presentation picker (TF-like favorites + line paint).
 * Presentation only: same OHLC store. Does not own hydration or TF.
 */
const PriceStyleController = (() => {
  const CATALOG = (typeof PRICE_STYLE_CATALOG !== 'undefined') ? PRICE_STYLE_CATALOG : [
    { id: 'candles', label: 'Candles', defaultFavorite: true },
    { id: 'bars', label: 'Bars', defaultFavorite: true },
    { id: 'line', label: 'Line', defaultFavorite: true },
  ];
  const PREFS_KEY = (typeof LS_PRICE_STYLE_KEY !== 'undefined')
    ? LS_PRICE_STYLE_KEY
    : 'dashboard_price_style_v1';
  const DEFAULT_STYLE = (typeof PRICE_STYLE_DEFAULT !== 'undefined') ? PRICE_STYLE_DEFAULT : 'candles';

  const ICONS = {
    candles: '<svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true"><rect x="3" y="5" width="3" height="6" fill="currentColor"/><rect x="4.15" y="2" width="0.7" height="12" fill="currentColor"/><rect x="10" y="4" width="3" height="7" fill="currentColor"/><rect x="11.15" y="2.5" width="0.7" height="11" fill="currentColor"/></svg>',
    bars: '<svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true"><rect x="3.6" y="2" width="0.8" height="12" fill="currentColor"/><rect x="2" y="5" width="2.4" height="0.8" fill="currentColor"/><rect x="3.6" y="10" width="2.4" height="0.8" fill="currentColor"/><rect x="11.6" y="2" width="0.8" height="12" fill="currentColor"/><rect x="10" y="6" width="2.4" height="0.8" fill="currentColor"/><rect x="11.6" y="11" width="2.4" height="0.8" fill="currentColor"/></svg>',
    line: '<svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true"><polyline points="1.5,12 5,6.5 9,9 14.5,3.5" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round" stroke-linecap="round"/></svg>',
  };

  let currentStyle = DEFAULT_STYLE;
  let favorites = defaultFavorites();
  let linePaint = factoryLinePaint();
  let lineGearOpen = false;

  function defaultFavorites() {
    return CATALOG.filter((e) => e.defaultFavorite).map((e) => e.id);
  }

  function factoryLinePaint() {
    const f = (typeof PRICE_LINE_FACTORY !== 'undefined' && PRICE_LINE_FACTORY)
      ? PRICE_LINE_FACTORY
      : { color: '#089981', lineWidth: 2 };
    return { color: normalizeHex(f.color, '#089981'), lineWidth: clampWidth(f.lineWidth) };
  }

  function clampWidth(n) {
    const w = Math.round(Number(n));
    if (!Number.isFinite(w)) return 2;
    return Math.max(1, Math.min(4, w));
  }

  function normalizeHex(hex, fallback) {
    const s = String(hex || '').trim();
    if (/^#[0-9a-fA-F]{6}$/.test(s)) return s.toUpperCase();
    if (/^#[0-9a-fA-F]{3}$/.test(s)) {
      return `#${s[1]}${s[1]}${s[2]}${s[2]}${s[3]}${s[3]}`.toUpperCase();
    }
    return String(fallback || '#089981').toUpperCase();
  }

  function catalogEntry(id) {
    return CATALOG.find((e) => e.id === id) || null;
  }

  function normalizeStyle(id) {
    return catalogEntry(id) ? id : DEFAULT_STYLE;
  }

  function projectPricePoint(style, candle) {
    if (!candle) return null;
    if (normalizeStyle(style) === 'line') {
      const close = Number(candle.close);
      if (!Number.isFinite(close)) return null;
      return { time: candle.time, value: close };
    }
    return candle;
  }

  function projectPricePoints(style, candles) {
    if (!Array.isArray(candles)) return [];
    if (normalizeStyle(style) !== 'line') return candles;
    const out = [];
    for (let i = 0; i < candles.length; i++) {
      const pt = projectPricePoint(style, candles[i]);
      if (pt) out.push(pt);
    }
    return out;
  }

  function loadPrefs() {
    let parsed = {};
    try {
      const raw = (typeof localStorage !== 'undefined') ? localStorage.getItem(PREFS_KEY) : null;
      if (raw) {
        const next = JSON.parse(raw);
        if (next && typeof next === 'object' && !Array.isArray(next)) parsed = next;
      }
    } catch {
      parsed = {};
    }
    currentStyle = typeof parsed.currentStyle === 'string'
      ? normalizeStyle(parsed.currentStyle)
      : DEFAULT_STYLE;
    if (Array.isArray(parsed.favorites)) {
      favorites = parsed.favorites.map(normalizeStyle).filter((id, i, arr) => catalogEntry(id) && arr.indexOf(id) === i);
      if (!favorites.length) favorites = defaultFavorites();
    } else {
      favorites = defaultFavorites();
    }
    const factory = factoryLinePaint();
    linePaint = { ...factory };
    if (parsed.line && typeof parsed.line === 'object') {
      if (parsed.line.color != null) linePaint.color = normalizeHex(parsed.line.color, factory.color);
      if (parsed.line.lineWidth != null) linePaint.lineWidth = clampWidth(parsed.line.lineWidth);
    }
  }

  function sameFavs(a, b) {
    if (a.length !== b.length) return false;
    const aa = [...a].sort();
    const bb = [...b].sort();
    return aa.every((v, i) => v === bb[i]);
  }

  function sameLinePaint(a, b) {
    return normalizeHex(a.color) === normalizeHex(b.color) && clampWidth(a.lineWidth) === clampWidth(b.lineWidth);
  }

  function savePrefs() {
    if (typeof localStorage === 'undefined') return;
    const sparse = {};
    if (currentStyle !== DEFAULT_STYLE) sparse.currentStyle = currentStyle;
    if (!sameFavs(favorites, defaultFavorites())) sparse.favorites = [...favorites];
    const factory = factoryLinePaint();
    if (!sameLinePaint(linePaint, factory)) {
      sparse.line = { color: linePaint.color, lineWidth: linePaint.lineWidth };
    }
    try {
      if (!Object.keys(sparse).length) localStorage.removeItem(PREFS_KEY);
      else localStorage.setItem(PREFS_KEY, JSON.stringify(sparse));
    } catch {
      /* quota / private mode */
    }
  }

  function isFavorite(id) {
    return favorites.includes(id);
  }

  function getCurrent() {
    return currentStyle;
  }

  function getFavorites() {
    return [...favorites];
  }

  function resolveLinePaint() {
    return { color: linePaint.color, lineWidth: linePaint.lineWidth };
  }

  function setDropdownOpen(open) {
    const dd = document.getElementById('price-style-dropdown');
    if (!dd) return;
    dd.hidden = !open;
    dd.classList.toggle('open', open);
  }

  function glyphHtml(id) {
    return ICONS[id] || ICONS.candles;
  }

  function syncCurrentButton() {
    const btn = document.getElementById('price-style-current');
    const glyph = btn?.querySelector?.('.price-style-glyph');
    const entry = catalogEntry(currentStyle) || catalogEntry(DEFAULT_STYLE);
    if (glyph) glyph.innerHTML = glyphHtml(entry.id);
    if (btn) {
      const title = entry.label;
      btn.title = title;
      btn.setAttribute?.('aria-label', `Chart style: ${title}`);
    }
  }

  function renderFavs() {
    const el = document.getElementById('price-style-favorites');
    if (!el) return;
    el.innerHTML = '';
    favorites.forEach((id) => {
      const entry = catalogEntry(id);
      if (!entry) return;
      const btn = document.createElement('button');
      btn.type = 'button';
      btn.className = 'price-style-fav' + (id === currentStyle ? ' active' : '');
      btn.dataset.style = id;
      btn.title = entry.label;
      btn.setAttribute?.('aria-label', entry.label);
      btn.innerHTML = glyphHtml(id);
      el.appendChild(btn);
    });
  }

  function renderMenu() {
    const body = document.getElementById('price-style-menu');
    if (!body) return;
    body.innerHTML = '';
    CATALOG.forEach((entry) => {
      const wrap = document.createElement('div');
      const row = document.createElement('div');
      row.className = 'price-style-item' + (entry.id === currentStyle ? ' selected' : '');
      const name = document.createElement('button');
      name.type = 'button';
      name.className = 'price-style-name';
      name.dataset.style = entry.id;
      name.title = entry.label;
      name.setAttribute?.('aria-label', entry.label);
      name.innerHTML = `${glyphHtml(entry.id)}<span>${entry.label}</span>`;
      row.appendChild(name);
      if (entry.id === 'line') {
        const gear = document.createElement('button');
        gear.type = 'button';
        gear.className = 'price-style-gear' + (lineGearOpen ? ' is-open' : '');
        gear.textContent = '⚙';
        gear.title = 'Line paint';
        gear.setAttribute?.('aria-label', 'Line paint');
        row.appendChild(gear);
      }
      const star = document.createElement('button');
      star.type = 'button';
      star.className = 'tf-star' + (isFavorite(entry.id) ? ' fav' : '');
      star.textContent = '★';
      star.title = 'Favorite';
      star.dataset.style = entry.id;
      row.appendChild(star);
      wrap.appendChild(row);
      if (entry.id === 'line') {
        const panel = document.createElement('div');
        panel.className = 'price-style-line-panel';
        panel.hidden = !lineGearOpen;
        const color = document.createElement('input');
        color.type = 'color';
        color.className = 'price-style-line-color';
        color.value = linePaint.color.toLowerCase();
        color.title = 'Line color';
        color.setAttribute?.('aria-label', 'Line color');
        color.addEventListener('input', () => {
          setLinePaint({ color: color.value });
        });
        const width = document.createElement('input');
        width.type = 'number';
        width.className = 'price-style-line-width';
        width.min = '1';
        width.max = '4';
        width.step = '1';
        width.value = String(linePaint.lineWidth);
        width.title = 'Line width';
        width.setAttribute?.('aria-label', 'Line width');
        width.addEventListener('change', () => {
          setLinePaint({ lineWidth: width.value });
        });
        panel.appendChild(color);
        panel.appendChild(width);
        wrap.appendChild(panel);
      }
      body.appendChild(wrap);
    });
  }

  function render() {
    syncCurrentButton();
    renderFavs();
    renderMenu();
  }

  function applyStyle(id) {
    const next = normalizeStyle(id);
    currentStyle = next;
    savePrefs();
    render();
    if (typeof ChartAdapter !== 'undefined' && typeof ChartAdapter.setPriceStyle === 'function') {
      ChartAdapter.setPriceStyle(next);
    }
    setDropdownOpen(false);
  }

  function toggleFavorite(id) {
    const style = normalizeStyle(id);
    if (isFavorite(style)) {
      if (favorites.length <= 1) return;
      favorites = favorites.filter((f) => f !== style);
    } else {
      favorites = [...favorites, style];
    }
    savePrefs();
    render();
  }

  function toggleLineGear() {
    lineGearOpen = !lineGearOpen;
    renderMenu();
  }

  function setLinePaint(patch) {
    const factory = factoryLinePaint();
    const next = { ...linePaint };
    if (patch && patch.color != null) next.color = normalizeHex(patch.color, factory.color);
    if (patch && patch.lineWidth != null) next.lineWidth = clampWidth(patch.lineWidth);
    linePaint = next;
    savePrefs();
    if (typeof ChartAdapter !== 'undefined' && typeof ChartAdapter.applyPriceLinePaint === 'function') {
      ChartAdapter.applyPriceLinePaint({ color: linePaint.color, lineWidth: linePaint.lineWidth });
    }
  }

  function resetLinePaint() {
    linePaint = factoryLinePaint();
    savePrefs();
    if (typeof ChartAdapter !== 'undefined' && typeof ChartAdapter.applyPriceLinePaint === 'function') {
      ChartAdapter.applyPriceLinePaint({ color: linePaint.color, lineWidth: linePaint.lineWidth });
    }
    renderMenu();
  }

  function bind() {
    const bar = document.getElementById('price-style-bar') || document.getElementById('price-style-picker');
    const currentBtn = document.getElementById('price-style-current');
    if (!bar || !currentBtn || bar.dataset.bound === '1') return;
    bar.dataset.bound = '1';
    bar.addEventListener('click', (e) => {
      if (e.target.closest('#price-style-current')) {
        e.preventDefault();
        e.stopPropagation();
        const dd = document.getElementById('price-style-dropdown');
        setDropdownOpen(!!dd?.hidden);
        return;
      }
      if (e.target.closest('.price-style-gear')) {
        e.preventDefault();
        e.stopPropagation();
        toggleLineGear();
        return;
      }
      const fav = e.target.closest('.price-style-fav');
      if (fav?.dataset?.style) {
        e.preventDefault();
        e.stopPropagation();
        applyStyle(fav.dataset.style);
        return;
      }
      const name = e.target.closest('.price-style-name');
      if (name?.dataset?.style) {
        e.preventDefault();
        e.stopPropagation();
        applyStyle(name.dataset.style);
        return;
      }
      const star = e.target.closest('.tf-star');
      if (star?.dataset?.style) {
        e.preventDefault();
        e.stopPropagation();
        toggleFavorite(star.dataset.style);
      }
    }, true);
    document.addEventListener('click', (e) => {
      if (e.target.closest('#price-style-bar') || e.target.closest('#price-style-picker')) return;
      setDropdownOpen(false);
    });
  }

  function init() {
    loadPrefs();
    bind();
    render();
  }

  return {
    init,
    getCurrent,
    getFavorites,
    applyStyle,
    toggleFavorite,
    isFavorite,
    toggleLineGear,
    isLineGearOpen: () => lineGearOpen,
    setLinePaint,
    resetLinePaint,
    resolveLinePaint,
    factoryLinePaint,
    normalizeStyle,
    projectPricePoint,
    projectPricePoints,
    catalog: CATALOG,
    PREFS_KEY,
    DEFAULT_STYLE,
  };
})();

if (typeof window !== 'undefined') {
  window.PriceStyleController = PriceStyleController;
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = { PriceStyleController };
}
