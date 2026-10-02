/**
 * Inspection shell. One selected Star, owned by StarResearch.
 * cursorTime is only the crosshair. It does not select a Star.
 */
const StarInspection = (() => {
  const Grammar = typeof StarInspectionGrammar !== 'undefined'
    ? StarInspectionGrammar
    : require('./star-inspection-grammar.js');
  const Swatches = typeof StarInspectionSwatches !== 'undefined'
    ? StarInspectionSwatches
    : require('./star-inspection-swatches.js');
  const Metric = typeof StarMetricGroup !== 'undefined'
    ? StarMetricGroup
    : require('./star-metric-group.js');
  const workspace = {
    cursorTime: null,
    selectedIndex: null,
    viewport: null,
  };
  const COLOR_KEY = 'star-inspection-dot-colors';
  let root = null;
  let width = 380;
  let palette = Grammar.paletteFromHex(null);
  let lastBody = null;
  let foldMemory = null;

  function noteCursor(time) {
    workspace.cursorTime = time == null ? null : time;
    return workspace.selectedIndex;
  }

  function noteViewport(range) {
    workspace.viewport = range || null;
  }

  function getWorkspace() {
    return {
      cursorTime: workspace.cursorTime,
      selectedIndex: workspace.selectedIndex,
      viewport: workspace.viewport,
    };
  }

  function escapeText(value) {
    return String(value == null ? '' : value)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;');
  }

  function clock(ms) {
    const n = Number(ms);
    if (!Number.isFinite(n) || n <= 0) return '—';
    return new Date(n).toISOString().replace('T', ' ').slice(0, 16) + ' UTC';
  }

  function swatchHex(spec) {
    const factory = typeof window !== 'undefined' ? window.DDRFactory : null;
    if (!factory || typeof factory.swatchHex !== 'function' || !spec) return null;
    return factory.swatchHex(spec.seriesId, spec.field);
  }

  function swatchHtml(reading) {
    const marks = Swatches.specs(reading.id).map((spec) => {
      const hex = swatchHex(spec);
      if (!hex) return '<i class="star-swatch star-swatch--absent"></i>';
      return '<i class="star-swatch" style="background:' + escapeText(hex) + '"></i>';
    }).join('');
    return '<span class="star-swatches">' + marks + '</span>';
  }

  function cellSwatch(reading) {
    if (!reading) return '<span></span>';
    return swatchHtml(reading);
  }

  function cellDot(reading) {
    if (!reading) return '<span></span>';
    const ok = !!reading.ok;
    const color = ok ? Grammar.dotColor(reading.position, palette) : null;
    return color
      ? '<i class="star-dot" style="background:' + color + '"></i>'
      : '<i class="star-dot star-dot--absent"></i>';
  }

  function cellValue(reading) {
    if (!reading) return '<span class="star-metric-val"></span>';
    return '<span class="star-metric-val">' +
      escapeText(Grammar.formatValue(reading.value, !!reading.ok, reading.family)) + '</span>';
  }

  function cellGlyph(kind, reading) {
    if (!reading) return '<span class="star-metric-g"></span>';
    return '<span class="star-metric-g">' +
      escapeText(Grammar.arrowGlyph(kind, reading.value, !!reading.ok)) + '</span>';
  }

  function metricClusterHtml(cluster) {
    const level = cluster.level;
    if (!level) return '';
    const kids = Metric.canExpand(cluster);
    const open = !!(kids && foldOpen('metric:' + cluster.key, false));
    const tri = kids
      ? '<button type="button" class="star-metric-tri" data-metric-tri>' + (open ? '▼' : '▶') + '</button>'
      : '<span></span>';
    const accelCol = open ? '<span class="star-metric-g"></span>' : cellGlyph('accel', cluster.accel);
    const slopeCol = open ? '<span class="star-metric-g"></span>' : cellGlyph('slope', cluster.slope);
    let html = '<div class="star-metric' + (open ? ' is-open' : '') + '" data-metric-key="' +
      escapeText(cluster.key) + '" data-open="' + (open ? '1' : '0') + '">';
    html += '<div class="star-metric-row star-metric-row--father">' +
      cellSwatch(level) + tri +
      '<span class="star-metric-name">' + escapeText(level.label) + '</span>' +
      accelCol + slopeCol + cellDot(level) + cellValue(level) +
      '</div>';
    if (open && cluster.accel) {
      html += '<div class="star-metric-row star-metric-row--child">' +
        '<span></span>' + cellSwatch(cluster.accel) +
        '<span class="star-metric-name">' + escapeText(Metric.childLabel('accel')) + '</span>' +
        cellGlyph('accel', cluster.accel) + '<span class="star-metric-g"></span>' +
        cellDot(cluster.accel) + cellValue(cluster.accel) +
        '</div>';
    }
    if (open && cluster.slope) {
      html += '<div class="star-metric-row star-metric-row--child">' +
        '<span></span>' + cellSwatch(cluster.slope) +
        '<span class="star-metric-name">' + escapeText(Metric.childLabel('slope')) + '</span>' +
        '<span class="star-metric-g"></span>' + cellGlyph('slope', cluster.slope) +
        cellDot(cluster.slope) + cellValue(cluster.slope) +
        '</div>';
    }
    if (open) {
      (cluster.extras || []).forEach((x) => {
        const r = x.reading;
        html += '<div class="star-metric-row star-metric-row--child">' +
          '<span></span>' + cellSwatch(r) +
          '<span class="star-metric-name">' + escapeText(Metric.childLabel(x.role)) + '</span>' +
          '<span class="star-metric-g"></span><span class="star-metric-g"></span>' +
          cellDot(r) + cellValue(r) +
          '</div>';
      });
    }
    html += '</div>';
    return html;
  }

  function groups(readings) {
    const clusters = Metric.bundleReadings(readings);
    const order = [];
    const buckets = {};
    clusters.forEach((cluster) => {
      const name = cluster.group || 'State';
      if (!buckets[name]) {
        buckets[name] = [];
        order.push(name);
      }
      buckets[name].push(cluster);
    });
    return order.map((name) => {
      const open = foldOpen(name, name === '15m' || name === '15m relations' || name === '15m to 1h');
      const rows = buckets[name].map(metricClusterHtml).join('');
      return '<details class="star-group" data-fold="' + escapeText(name) + '"' + (open ? ' open' : '') + '>' +
        '<summary>' + escapeText(name) + '</summary>' + rows + '</details>';
    });
  }

  function splitGroups(readings) {
    const state = [];
    const rel = [];
    (readings || []).forEach((reading) => {
      if (String(reading.id || '').indexOf('rel.') === 0) rel.push(reading);
      else state.push(reading);
    });
    return { state, rel };
  }

  function captureFolds() {
    if (!root) return null;
    const card = root.querySelector('[data-star-card]');
    if (!card) return null;
    const open = {};
    card.querySelectorAll('details[data-fold]').forEach((el) => {
      open[el.getAttribute('data-fold')] = el.open;
    });
    card.querySelectorAll('[data-metric-key]').forEach((el) => {
      open['metric:' + el.getAttribute('data-metric-key')] = el.getAttribute('data-open') === '1';
    });
    return open;
  }

  function foldOpen(key, fallback) {
    if (!foldMemory || !Object.prototype.hasOwnProperty.call(foldMemory, key)) return fallback;
    return !!foldMemory[key];
  }

  function fold(key, summary, body, fallbackOpen) {
    const open = foldOpen(key, fallbackOpen);
    return '<details class="star-fold" data-fold="' + escapeText(key) + '"' + (open ? ' open' : '') + '>' +
      '<summary>' + summary + '</summary>' + body + '</details>';
  }

  function paint(body) {
    if (!root) return;
    const card = root.querySelector('[data-star-card]');
    if (!card) return;
    const captured = captureFolds();
    if (captured) foldMemory = Object.assign({}, captured, foldMemory || {});
    else if (!foldMemory) foldMemory = {};
    if (!body) {
      lastBody = null;
      card.innerHTML = '<p class="star-empty">Select a Star.</p>';
      workspace.selectedIndex = null;
      return;
    }
    lastBody = body;
    workspace.selectedIndex = body.index;
    const parts = splitGroups(body.readings);
    const favorable = Grammar.formatPath(body.favorable, body.favorable != null);
    const adverse = Grammar.formatPath(body.adverse, body.adverse != null);
    const side = body.side === 'down' ? 'down' : (body.side === 'up' ? 'up' : '');
    const sideMark = side
      ? '<i class="star-side star-side--' + side + '" title="' + side + '"></i>'
      : '';
    const starSummary = '<span class="star-title"><strong>Star ' + escapeText(body.index + 1) + '</strong>' + sideMark + '</span>';
    const starBody =
      (body.holdout ? '<p class="star-id"><span class="star-holdout">Holdout</span></p>' : '') +
      '<p class="star-clock">Decision ' + escapeText(clock(body.decisionAt)) + '</p>' +
      '<p class="star-clock">Inspection ' + escapeText(clock(body.inspectionAt)) + '</p>';
    const pathBody =
      '<p class="star-path">' + escapeText(body.pathLabel || 'No reading') + '</p>' +
      '<p class="star-path">Favorable ' + escapeText(favorable) + '</p>' +
      '<p class="star-path">Adverse ' + escapeText(adverse) + '</p>';
    card.innerHTML =
      fold('star', starSummary, starBody, true) +
      fold('path', 'Path', pathBody, true) +
      '<section><h3>State</h3>' + groups(parts.state).join('') + '</section>' +
      '<section><h3>Relations</h3>' + groups(parts.rel).join('') + '</section>';
    fitValueColumns(card);
  }

  function fitValueColumns(card) {
    if (!card) return;
    card.querySelectorAll('section').forEach((section) => {
      const probe = document.createElement('span');
      probe.className = 'star-metric-val';
      probe.style.position = 'absolute';
      probe.style.visibility = 'hidden';
      probe.style.width = 'auto';
      probe.style.whiteSpace = 'nowrap';
      section.appendChild(probe);
      let max = 0;
      section.querySelectorAll('.star-metric-val').forEach((el) => {
        if (el === probe) return;
        const fold = el.closest('details');
        if (fold && !fold.open) return;
        probe.textContent = el.textContent;
        if (probe.offsetWidth > max) max = probe.offsetWidth;
      });
      probe.remove();
      if (max > 0) section.style.setProperty('--star-value-width', max + 'px');
      else section.style.removeProperty('--star-value-width');
    });
  }

  async function load(index) {
    if (!Number.isFinite(index) || index < 0) {
      paint(null);
      return;
    }
    const res = await fetch('/api/research/star-inspection?index=' + encodeURIComponent(index));
    if (!res.ok) {
      paint(null);
      return;
    }
    paint(await res.json());
  }

  function ensureDom() {
    if (typeof document === 'undefined' || root) return;
    const host = document.getElementById('workspace-main');
    if (!host) return;
    root = document.createElement('aside');
    root.id = 'star-inspection';
    root.innerHTML =
      '<div class="star-inspection-resize" data-star-resize></div>' +
      '<div class="star-inspection-bar">' +
        '<span>Star</span>' +
        '<i id="star-research-side" class="star-side" aria-hidden="true"></i>' +
        '<input id="star-research-no" type="number" min="1" step="1" inputmode="numeric" aria-label="Star ordinal">' +
        '<span id="star-research-count">— / —</span>' +
        '<button type="button" id="star-research-prev">Prev</button>' +
        '<button type="button" id="star-research-next">Next</button>' +
        '<button type="button" id="star-research-clear">Clear</button>' +
        '<span id="star-research-status"></span>' +
        '<button type="button" data-star-colors aria-label="Style" aria-expanded="false" title="Style">' +
          '<span class="star-color-icon" aria-hidden="true"></span>' +
        '</button>' +
        '<div class="star-color-pop" data-star-color-pop hidden>' +
          '<div class="star-style-kicker">Chart</div>' +
          '<label>Shape <select data-paint-shape>' +
            '<option value="star4">Star</option>' +
            '<option value="arrow">Arrow</option>' +
            '<option value="circle">Circle</option>' +
            '<option value="triangle">Triangle</option>' +
            '<option value="square">Square</option>' +
            '<option value="diamond">Diamond</option>' +
          '</select></label>' +
          '<label>Size <input type="range" data-paint-size min="4" max="18" step="1"></label>' +
          '<div class="star-style-pair">' +
            '<label>Up <input type="color" data-paint-up></label>' +
            '<label>Down <input type="color" data-paint-down></label>' +
          '</div>' +
          '<div class="star-style-kicker">Table</div>' +
          '<label>Negative / low <input type="color" data-star-color="low"></label>' +
          '<label>Center <input type="color" data-star-color="mid"></label>' +
          '<label>Positive / high <input type="color" data-star-color="high"></label>' +
          '<button type="button" data-star-color-reset>Reset</button>' +
        '</div>' +
      '</div>' +
      '<div class="star-inspection-scroll" data-star-scroll>' +
      '<div data-star-card><p class="star-empty">Select a Star.</p></div>' +
      '</div>';
    host.appendChild(root);
    const scroll = root.querySelector('[data-star-scroll]');
    if (scroll) {
      scroll.addEventListener('wheel', (ev) => { ev.stopPropagation(); }, { passive: true });
    }
    const card = root.querySelector('[data-star-card]');
    card.addEventListener('toggle', (ev) => {
      if (ev.target && ev.target.matches && ev.target.matches('details')) fitValueColumns(card);
    }, true);
    card.addEventListener('click', (ev) => {
      const tri = ev.target && ev.target.closest ? ev.target.closest('[data-metric-tri]') : null;
      if (!tri || !lastBody) return;
      ev.preventDefault();
      const wrap = tri.closest('[data-metric-key]');
      if (!wrap) return;
      const key = wrap.getAttribute('data-metric-key');
      const captured = captureFolds();
      if (captured) foldMemory = Object.assign({}, captured, foldMemory);
      if (!foldMemory) foldMemory = {};
      foldMemory['metric:' + key] = wrap.getAttribute('data-open') !== '1';
      paint(lastBody);
    });
    applyWidth();
    bindColors();
    bindChartStyle();
    if (typeof StarResearch !== 'undefined' && typeof StarResearch.init === 'function') {
      StarResearch.init();
    }
    const handle = root.querySelector('[data-star-resize]');
    handle.addEventListener('pointerdown', (ev) => {
      const startX = ev.clientX;
      const startW = width;
      function move(e) {
        width = Math.min(560, Math.max(280, startW + (startX - e.clientX)));
        applyWidth();
      }
      function up() {
        window.removeEventListener('pointermove', move);
        window.removeEventListener('pointerup', up);
      }
      window.addEventListener('pointermove', move);
      window.addEventListener('pointerup', up);
    });
    window.addEventListener('resize', applyWidth);
  }

  function readStoredPalette() {
    try {
      if (typeof localStorage === 'undefined') return Grammar.paletteFromHex(null);
      const raw = localStorage.getItem(COLOR_KEY);
      if (!raw) return Grammar.paletteFromHex(null);
      return Grammar.paletteFromHex(JSON.parse(raw));
    } catch {
      return Grammar.paletteFromHex(null);
    }
  }

  function writeStoredPalette(hexes) {
    try {
      if (typeof localStorage === 'undefined') return;
      localStorage.setItem(COLOR_KEY, JSON.stringify(hexes));
    } catch { /* preference only */ }
  }

  function clearStoredPalette() {
    try {
      if (typeof localStorage !== 'undefined') localStorage.removeItem(COLOR_KEY);
    } catch { /* preference only */ }
  }

  function currentHex() {
    return {
      low: Grammar.rgbToHex(palette.low),
      mid: Grammar.rgbToHex(palette.mid),
      high: Grammar.rgbToHex(palette.high),
    };
  }

  function syncColorInputs() {
    if (!root) return;
    const hex = currentHex();
    root.querySelectorAll('[data-star-color]').forEach((input) => {
      input.value = hex[input.getAttribute('data-star-color')] || input.value;
    });
  }

  function setPalette(next, persist) {
    palette = Grammar.paletteFromHex(next);
    if (persist) writeStoredPalette(currentHex());
    syncColorInputs();
    if (lastBody) paint(lastBody);
  }

  function bindColors() {
    palette = readStoredPalette();
    syncColorInputs();
    const button = root.querySelector('[data-star-colors]');
    const pop = root.querySelector('[data-star-color-pop]');
    button.addEventListener('click', (ev) => {
      ev.stopPropagation();
      const open = pop.hidden;
      pop.hidden = !open;
      button.setAttribute('aria-expanded', open ? 'true' : 'false');
    });
    pop.addEventListener('click', (ev) => ev.stopPropagation());
    pop.addEventListener('input', (ev) => {
      const input = ev.target;
      if (!input || !input.getAttribute) return;
      const key = input.getAttribute('data-star-color');
      if (!key) return;
      const hex = currentHex();
      hex[key] = input.value;
      setPalette(hex, true);
    });
    root.querySelector('[data-star-color-reset]').addEventListener('click', () => {
      clearStoredPalette();
      setPalette(Grammar.defaultHex(), false);
    });
    document.addEventListener('click', () => {
      pop.hidden = true;
      button.setAttribute('aria-expanded', 'false');
    });
    document.addEventListener('keydown', (ev) => {
      if (ev.key !== 'Escape') return;
      pop.hidden = true;
      button.setAttribute('aria-expanded', 'false');
    });
  }

  function bindChartStyle() {
    const pop = root.querySelector('[data-star-color-pop]');
    if (!pop) return;
    syncChartStyle(pop);
    pop.addEventListener('input', (ev) => applyChartStyle(ev.target));
    pop.addEventListener('change', (ev) => applyChartStyle(ev.target));
  }

  function chartStyleApi() {
    return typeof StarPopulationDisplay !== 'undefined' ? StarPopulationDisplay : null;
  }

  function syncChartStyle(pop) {
    const api = chartStyleApi();
    if (!pop || !api || typeof api.get !== 'function') return;
    const prefs = api.get();
    const shape = pop.querySelector('[data-paint-shape]');
    const size = pop.querySelector('[data-paint-size]');
    const up = pop.querySelector('[data-paint-up]');
    const down = pop.querySelector('[data-paint-down]');
    if (shape) shape.value = prefs.shape;
    if (size) size.value = String(prefs.size);
    if (up) up.value = prefs.upColor;
    if (down) down.value = prefs.downColor;
  }

  function applyChartStyle(el) {
    if (!el || typeof el.getAttribute !== 'function') return;
    const api = chartStyleApi();
    if (!api || typeof api.set !== 'function') return;
    const key = el.getAttribute('data-paint-shape') != null ? 'shape'
      : (el.getAttribute('data-paint-size') != null ? 'size'
        : (el.getAttribute('data-paint-up') != null ? 'upColor'
          : (el.getAttribute('data-paint-down') != null ? 'downColor' : '')));
    if (!key) return;
    const patch = {};
    patch[key] = key === 'size' ? Number(el.value) : el.value;
    api.set(patch);
    if (typeof StarResearchOverlay !== 'undefined' && typeof StarResearchOverlay.refresh === 'function') {
      StarResearchOverlay.refresh();
    }
  }

  function applyWidth() {
    if (!root || typeof document === 'undefined' || !document.body) return;
    if (root.hidden) {
      document.body.classList.remove('star-inspection-docked');
      document.body.style.setProperty('--star-inspection-width', '0px');
      return;
    }
    const dock = (typeof ToolDock !== 'undefined' && ToolDock.WIDTH) ? ToolDock.WIDTH : 40;
    const narrow = window.innerWidth < 1100 || width > window.innerWidth - 480 - dock;
    root.classList.toggle('is-overlay', narrow);
    root.style.width = width + 'px';
    document.body.classList.toggle('star-inspection-docked', !narrow);
    document.body.style.setProperty('--star-inspection-width', narrow ? '0px' : width + 'px');
  }

  function init() {
    ensureDom();
    if (typeof StarResearch !== 'undefined' && typeof StarResearch.onSelected === 'function') {
      StarResearch.onSelected(() => {
        const state = StarResearch.getState();
        if (state && state.index != null) load(state.index);
        else paint(null);
      });
    }
  }

  return { init, noteCursor, noteViewport, getWorkspace, applyWidth };
})();

if (typeof document !== 'undefined') {
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', () => StarInspection.init());
  else StarInspection.init();
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = StarInspection;
}
