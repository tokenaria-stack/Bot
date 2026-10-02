/**
 * Population/Lens workstation. Semantics come from /api/research/star-lens/*.
 * This file does not compute R state, missingness, or self-exclusion.
 */
const StarLens = (() => {
  const Metric = typeof StarMetricGroup !== 'undefined'
    ? StarMetricGroup
    : require('./star-metric-group.js');
  const Swatches = typeof StarInspectionSwatches !== 'undefined'
    ? StarInspectionSwatches
    : require('./star-inspection-swatches.js');
  const FIELDS = [
    { field: 'mfeAtr', label: 'MFE ATR' },
    { field: 'maeAtr', label: 'MAE ATR' },
    { field: 'mfePercent', label: 'MFE %' },
    { field: 'maePercent', label: 'MAE %' },
  ];
  const EVENTS = [
    { kind: 'r', level: 1, name: '1R / 0.15' },
    { kind: 'r', level: 2, name: '2R / 0.15' },
    { kind: 'r', level: 3, name: '3R / 0.15' },
    { kind: 'stop', name: 'Stop / 0.15' },
  ];

  const state = {
    source: 'all',
    clauses: [],
    last: null,
    showStars: true,
    viewFrom: 0,
    viewTo: 0,
    seq: 0,
    timer: 0,
    selectedField: '',
    catalog: [],
    catalogOpen: {},
    catHeight: 280,
  };

  const CAT_HEIGHT_KEY = 'star-lens-cat-height';

  let fetchSources = defaultSources;
  let fetchEval = defaultEval;
  let fetchSave = defaultSave;
  let fetchCatalog = defaultCatalog;
  let visibleRangeMs = defaultVisibleRangeMs;

  function defaultVisibleRangeMs() {
    try {
      if (typeof ChartAdapter === 'undefined' || typeof ChartAdapter.getChart !== 'function') {
        return { from: 0, to: 0 };
      }
      const chart = ChartAdapter.getChart('live', 'price');
      const ts = chart && typeof chart.timeScale === 'function' ? chart.timeScale() : null;
      const range = ts && typeof ts.getVisibleRange === 'function' ? ts.getVisibleRange() : null;
      if (!range || !Number.isFinite(Number(range.from)) || !Number.isFinite(Number(range.to))) {
        return { from: 0, to: 0 };
      }
      const from = Number(range.from) < 1e11 ? Math.round(Number(range.from) * 1000) : Math.round(Number(range.from));
      const to = Number(range.to) < 1e11 ? Math.round(Number(range.to) * 1000) : Math.round(Number(range.to));
      return { from: from, to: to };
    } catch {
      return { from: 0, to: 0 };
    }
  }

  function defaultSources() {
    return fetch('/api/research/star-lens/sources').then((res) => (res.ok ? res.json() : null)).catch(() => null);
  }
  function defaultEval(body) {
    return fetch('/api/research/star-lens/evaluate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }).then((res) => (res.ok ? res.json() : null)).catch(() => null);
  }
  function defaultSave(body) {
    return fetch('/api/research/star-lens/save', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }).then((res) => (res.ok ? res.json() : null)).catch(() => null);
  }
  function defaultCatalog() {
    return fetch('/api/research/star-lens/catalog').then((res) => (res.ok ? res.json() : null)).catch(() => null);
  }

  function clausesFromForm(root) {
    const out = [];
    if (!root) return out;
    FIELDS.forEach((item) => {
      const on = root.querySelector('[data-num="' + item.field + '"]');
      const cmp = root.querySelector('[data-cmp="' + item.field + '"]');
      const val = root.querySelector('[data-bound="' + item.field + '"]');
      if (!on || !on.checked) return;
      const n = Number(val && val.value);
      if (!Number.isFinite(n)) return;
      out.push({ kind: 'continuous', field: item.field, cmp: cmp ? cmp.value : '>=', bound: n });
    });
    EVENTS.forEach((item) => {
      const sel = root.querySelector('[data-event="' + item.name + '"]');
      const v = sel ? sel.value : '';
      if (!v) return;
      if (item.kind === 'r') out.push({ kind: 'r', level: item.level, state: v });
      else out.push({ kind: 'stop', state: v });
    });
    const side = root.querySelector('[data-side]');
    if (side && side.value) out.push({ kind: 'side', side: side.value });
    const status = root.querySelector('[data-status]');
    if (status && status.value) out.push({ kind: 'status', status: status.value });
    const cid = root.querySelector('[data-coord-id]');
    const con = root.querySelector('[data-coord-num]');
    const ccmp = root.querySelector('[data-coord-cmp]');
    const cval = root.querySelector('[data-coord-bound]');
    if (cid && cid.value && con && con.checked) {
      const n = Number(cval && cval.value);
      if (Number.isFinite(n)) {
        out.push({ kind: 'continuous', field: cid.value, cmp: ccmp ? ccmp.value : '>=', bound: n });
      }
    }
    return out;
  }

  function catalogNeedle(entry) {
    return [entry.id, entry.label, entry.group, entry.section, entry.timeframe, entry.family, entry.unit]
      .join(' ').toLowerCase();
  }

  function filterCatalog(entries, query) {
    const q = String(query || '').trim().toLowerCase();
    if (!q) return entries || [];
    return (entries || []).filter((e) => catalogNeedle(e).indexOf(q) >= 0);
  }

  function escapeText(s) {
    return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/"/g, '&quot;');
  }

  function pickCoordinate(root, id) {
    const entry = state.catalog.find((e) => e.id === id);
    if (!entry) return;
    state.selectedField = id;
    const panel = root.querySelector('[data-coord-panel]');
    const hid = root.querySelector('[data-coord-id]');
    const lab = root.querySelector('[data-coord-label]');
    const shown = root.querySelector('[data-coord-shown]');
    const meta = root.querySelector('[data-coord-meta]');
    if (panel) panel.hidden = false;
    if (hid) hid.value = id;
    if (lab) lab.textContent = entry.label;
    if (shown) shown.textContent = id;
    if (meta) meta.textContent = entry.group + ' · ' + entry.timeframe + ' · ' + entry.unit;
    renderCatalog(root, state.catalog);
    evaluate();
  }

  function catalogGroup(key) {
    let found = null;
    Metric.bundleCatalog(state.catalog).forEach((sec) => {
      (sec.groups || []).forEach((g) => { if (g.key === key) found = g; });
    });
    return found;
  }

  function groupHit(g, q) {
    if (!q) return true;
    const rows = [g.level, g.slope, g.accel].concat((g.extras || []).map((x) => x.reading || x));
    return rows.some((e) => e && catalogNeedle(e).indexOf(q) >= 0);
  }

  function catalogChecked(g, id, expanded) {
    if (!id) return false;
    if (expanded) return state.selectedField === id;
    return Metric.memberIds(g).indexOf(state.selectedField) >= 0;
  }

  function catalogSwatch(entry) {
    if (!entry) return '<span></span>';
    const factory = typeof window !== 'undefined' ? window.DDRFactory : null;
    const marks = Swatches.specs(entry.id).map((spec) => {
      let hex = null;
      if (factory && typeof factory.swatchHex === 'function') hex = factory.swatchHex(spec.seriesId, spec.field);
      if (!hex) return '<i class="star-swatch star-swatch--absent"></i>';
      return '<i class="star-swatch" style="background:' + escapeText(hex) + '"></i>';
    }).join('');
    return '<span class="star-swatches">' + marks + '</span>';
  }

  function clampCatHeight(n) {
    const vh = typeof window !== 'undefined' && window.innerHeight ? window.innerHeight * 0.7 : 700;
    const v = Number(n);
    if (!Number.isFinite(v)) return 280;
    return Math.min(vh, Math.max(120, Math.round(v)));
  }

  function readCatHeight() {
    try {
      if (typeof localStorage === 'undefined') return 280;
      const raw = localStorage.getItem(CAT_HEIGHT_KEY);
      if (raw == null || raw === '') return 280;
      return clampCatHeight(raw);
    } catch {
      return 280;
    }
  }

  function writeCatHeight(n) {
    state.catHeight = clampCatHeight(n);
    try {
      if (typeof localStorage !== 'undefined') localStorage.setItem(CAT_HEIGHT_KEY, String(state.catHeight));
    } catch { /* preference only */ }
    return state.catHeight;
  }

  function applyCatHeight(root) {
    const box = root && root.querySelector('[data-coord-list]');
    if (!box) return;
    box.style.height = state.catHeight + 'px';
  }

  function catalogRow(g, role, entry, expanded) {
    if (!entry) return '';
    const pick = role === 'father' ? (g.level && g.level.id) : entry.id;
    const kids = Metric.canExpand(g);
    const child = role !== 'father';
    const on = pick === state.selectedField || (!expanded && !child && catalogChecked(g, g.level && g.level.id, false));
    const name = child ? Metric.childLabel(role) : (g.label || entry.label);
    const tri = (!child && kids)
      ? '<button type="button" class="star-metric-tri" data-catalog-tri="' + Metric.escapeText(g.key) + '">' +
        (expanded ? '▼' : '▶') + '</button>'
      : '<span></span>';
    const swatch = catalogSwatch(entry);
    const checkId = child ? entry.id : (g.level && g.level.id);
    const checked = catalogChecked(g, checkId, child || expanded);
    const cls = 'star-metric-row' + (child ? ' star-metric-row--child' : ' star-metric-row--father') +
      (on ? ' star-metric-row--on' : '');
    return '<div class="' + cls + '">' +
      (child ? '<span></span>' : swatch) +
      (child ? swatch : tri) +
      '<input type="checkbox" class="star-metric-check" data-coord-check="' + Metric.escapeText(checkId) + '"' +
        (child ? '' : ' data-role="father"') +
        (checked ? ' checked' : '') + '>' +
      '<button type="button" class="star-metric-name" data-coord-pick="' + Metric.escapeText(pick) + '">' +
        Metric.escapeText(name) + '</button>' +
      '</div>';
  }

  function renderCatalog(root, entries) {
    const box = root && root.querySelector('[data-coord-list]');
    if (!box) return;
    const q = String((root.querySelector('[data-coord-search]') || {}).value || '').trim().toLowerCase();
    const sections = Metric.bundleCatalog(entries);
    let html = '';
    sections.forEach((sec) => {
      const groups = (sec.groups || []).filter((g) => groupHit(g, q));
      if (!groups.length) return;
      html += '<div class="star-lens-cat-group">' + escapeText(sec.title) + '</div>';
      groups.forEach((g) => {
        const expanded = !!state.catalogOpen[g.key];
        html += '<div class="star-metric star-metric--catalog' + (expanded ? ' is-open' : '') +
          '" data-catalog-key="' + escapeText(g.key) + '">';
        html += catalogRow(g, 'father', g.level, expanded);
        if (expanded && g.accel) html += catalogRow(g, 'accel', g.accel, true);
        if (expanded && g.slope) html += catalogRow(g, 'slope', g.slope, true);
        html += '</div>';
      });
    });
    box.innerHTML = html || '<div class="star-lens-empty">no matches</div>';
    applyCatHeight(root);
  }

  function chartRange() {
    return visibleRangeMs();
  }

  function viewCount(pass, from, to) {
    if (!Array.isArray(pass) || !(to > from)) return 0;
    let n = 0;
    for (let i = 0; i < pass.length; i++) {
      const t = Number(pass[i].decisionAt);
      if (t >= from && t < to) n += 1;
    }
    return n;
  }

  function marksInView(pass, from, to) {
    if (!Array.isArray(pass)) return [];
    if (!(to > from)) return [];
    const out = [];
    for (let i = 0; i < pass.length; i++) {
      const t = Number(pass[i].decisionAt);
      if (t >= from && t < to) out.push(pass[i]);
    }
    return out;
  }

  function binThreshold(from, to) {
    const a = Number(from);
    const b = Number(to);
    if (!Number.isFinite(a) || !Number.isFinite(b)) return null;
    if (a === b) return a;
    return (a + b) / 2;
  }

  function boundInBin(bound, bin) {
    const v = Number(bound);
    if (!Number.isFinite(v) || !bin) return false;
    const from = Number(bin.from);
    const to = Number(bin.to);
    if (!Number.isFinite(from) || !Number.isFinite(to)) return false;
    if (from === to) return v === from;
    if (bin.openHigh && !bin.openLow) return v >= from;
    if (bin.openLow && !bin.openHigh) return v < to;
    return v >= from && v < to;
  }

  function histogramSvg(pic, field, bound, active) {
    const rows = pic && pic.hist && pic.hist.length ? pic.hist : null;
    if (!pic || !pic.rangeOk || !rows) {
      return '<div class="star-lens-empty">no observations</div>';
    }
    let peak = 1;
    for (let i = 0; i < rows.length; i++) {
      if (rows[i].count > peak) peak = rows[i].count;
    }
    const w = 248;
    const h = 52;
    const gap = 0.4;
    const bw = (w - gap * rows.length) / rows.length;
    const threshold = Number(bound);
    let bars = '';
    for (let i = 0; i < rows.length; i++) {
      const bin = rows[i];
      const bh = (bin.count / peak) * (h - 12);
      const on = active && boundInBin(threshold, bin);
      bars += '<rect class="star-lens-hist-bar" data-hist-bar="1" data-hist-field="' + escapeText(field) +
        '" data-hist-from="' + bin.from + '" data-hist-to="' + bin.to +
        '" x="' + (i * (bw + gap)).toFixed(2) + '" y="' + (h - 12 - bh).toFixed(2) +
        '" width="' + bw.toFixed(2) + '" height="' + Math.max(0, bh).toFixed(2) +
        '" fill="' + (on ? '#6b7c8e' : '#5d6b7a') + '"></rect>';
    }
    let handle = '';
    if (active && Number.isFinite(threshold) && pic.max > pic.min) {
      let x = ((threshold - pic.min) / (pic.max - pic.min)) * w;
      if (x < 0) x = 0;
      if (x > w) x = w;
      handle = '<line x1="' + x.toFixed(2) + '" x2="' + x.toFixed(2) + '" y1="0" y2="' + (h - 12) +
        '" stroke="#f0b429" stroke-width="1.5"></line>';
    }
    return '<svg class="star-lens-hist" viewBox="0 0 ' + w + ' ' + h + '" width="' + w + '" height="' + h + '">' +
      bars + handle +
      '<text x="0" y="' + h + '" fill="#787b86" font-size="9">' + formatBound(pic.min) + '</text>' +
      '<text x="' + w + '" y="' + h + '" fill="#787b86" font-size="9" text-anchor="end">' + formatBound(pic.max) + '</text>' +
      '</svg>';
  }

  function formatBound(n) {
    const v = Number(n);
    if (!Number.isFinite(v)) return '—';
    if (Math.abs(v) >= 10) return v.toFixed(2);
    if (Math.abs(v) >= 1) return v.toFixed(3);
    return v.toFixed(4);
  }

  function eventCopy(ev) {
    if (!ev) return '';
    const pct = ev.rateOk ? (100 * ev.rate).toFixed(1) + '%' : '—';
    const none = ev.name && ev.name.indexOf('Stop') === 0 ? 'No valid stop' : 'No R defined';
    return '<div>Reached <b>' + ev.reached + '</b> / ' + ev.base + ' · ' + pct + '</div>' +
      '<div>Valid, not reached <b>' + ev.notReached + '</b></div>' +
      '<div>' + none + ' <b>' + ev.undefined + '</b></div>';
  }

  function paint(root) {
    if (!root || !state.last) return;
    const last = state.last;
    const pop = root.querySelector('[data-count-pop]');
    const lens = root.querySelector('[data-count-lens]');
    const view = root.querySelector('[data-count-view]');
    const mix = root.querySelector('[data-count-mix]');
    const show = root.querySelector('[data-show]');
    if (pop) pop.textContent = String(last.population);
    if (lens) lens.textContent = String(last.lensPass);
    if (view) view.textContent = state.showStars ? String(last.chartView) : 'hidden';
    if (mix) mix.textContent = 'discovery ' + last.discovery + ' · holdout ' + last.holdout;
    if (show) show.checked = state.showStars;
    const nums = last.numbers || [];
    FIELDS.forEach((item) => {
      const box = root.querySelector('[data-hist="' + item.field + '"]');
      const on = root.querySelector('[data-num="' + item.field + '"]');
      const boundEl = root.querySelector('[data-bound="' + item.field + '"]');
      const slide = root.querySelector('[data-slide="' + item.field + '"]');
      const miss = root.querySelector('[data-miss="' + item.field + '"]');
      const pic = nums.find((n) => n.field === item.field);
      const active = !!(on && on.checked);
      if (box) box.innerHTML = histogramSvg(pic, item.field, boundEl ? boundEl.value : null, active);
      if (miss && pic) miss.textContent = 'missing ' + pic.missing + (active ? '' : ' · off');
      if (pic && pic.rangeOk && slide && boundEl && document.activeElement !== slide && document.activeElement !== boundEl) {
        slide.min = String(pic.min);
        slide.max = String(pic.max);
        slide.step = String((pic.max - pic.min) / 200 || 0.01);
        if (!active) {
          slide.value = String(pic.min);
          boundEl.value = formatBound(pic.min);
        }
      }
      if (slide) slide.disabled = !active;
      if (boundEl) boundEl.disabled = !active;
    });
    const cid = root.querySelector('[data-coord-id]');
    const field = cid ? cid.value : '';
    if (field) {
      const box = root.querySelector('[data-hist="coord"]');
      const on = root.querySelector('[data-coord-num]');
      const boundEl = root.querySelector('[data-coord-bound]');
      const slide = root.querySelector('[data-slide="coord"]');
      const miss = root.querySelector('[data-miss="coord"]');
      const pic = nums.find((n) => n.field === field);
      const active = !!(on && on.checked);
      if (box) box.innerHTML = histogramSvg(pic, field, boundEl ? boundEl.value : null, active);
      if (miss && pic) miss.textContent = 'missing ' + pic.missing + (active ? '' : ' · off');
      if (pic && pic.rangeOk && slide && boundEl && document.activeElement !== slide && document.activeElement !== boundEl) {
        slide.min = String(pic.min);
        slide.max = String(pic.max);
        slide.step = String((pic.max - pic.min) / 200 || 0.01);
        if (!active) {
          slide.value = String(pic.min);
          boundEl.value = formatBound(pic.min);
        }
      }
      if (slide) slide.disabled = !active;
      if (boundEl) boundEl.disabled = !active;
    }
    (last.events || []).forEach((ev) => {
      const box = root.querySelector('[data-rate="' + ev.name + '"]');
      if (box) box.innerHTML = eventCopy(ev);
    });
  }

  async function evaluate() {
    const root = document.getElementById('star-lens');
    state.clauses = clausesFromForm(root);
    const range = chartRange();
    state.viewFrom = range.from;
    state.viewTo = range.to;
    const seq = ++state.seq;
    const body = await fetchEval({
      source: state.source,
      clauses: state.clauses,
      selectedField: state.selectedField,
      viewFrom: state.viewFrom,
      viewTo: state.viewTo,
    });
    if (seq !== state.seq || !body) return null;
    body.chartView = viewCount(body.pass, state.viewFrom, state.viewTo);
    state.last = body;
    if (typeof StarResearch !== 'undefined' && typeof StarResearch.setWalk === 'function') {
      StarResearch.setWalk((body.pass || []).map((m) => m.index));
    }
    paint(root);
    if (typeof StarResearchOverlay !== 'undefined' && typeof StarResearchOverlay.refresh === 'function') {
      StarResearchOverlay.refresh();
    }
    return body;
  }

  function scheduleEvaluate() {
    if (state.timer) clearTimeout(state.timer);
    state.timer = setTimeout(() => { state.timer = 0; evaluate(); }, 40);
  }

  function noteViewport() {
    if (!state.last) return;
    const range = chartRange();
    state.viewFrom = range.from;
    state.viewTo = range.to;
    state.last.chartView = viewCount(state.last.pass, range.from, range.to);
    const root = document.getElementById('star-lens');
    paint(root);
  }

  function getMarks() {
    if (!state.showStars || !state.last || !Array.isArray(state.last.pass)) return [];
    return state.last.pass;
  }

  function mount() {
    if (typeof document === 'undefined') return;
    if (document.getElementById('star-lens')) return;
    const host = document.getElementById('star-inspection');
    const root = document.createElement('div');
    root.id = 'star-lens';
    let fieldHtml = '';
    FIELDS.forEach((item) => {
      fieldHtml += '<div class="star-lens-field">' +
        '<label class="star-lens-row"><input type="checkbox" data-num="' + item.field + '"> ' +
        item.label + ' <span class="star-lens-off">off until checked</span></label>' +
        '<div class="star-lens-row star-lens-row--controls">' +
        '<select data-cmp="' + item.field + '"><option value=">=">&ge;</option><option value=">">&gt;</option>' +
        '<option value="<=">&le;</option><option value="<">&lt;</option></select>' +
        '<input data-bound="' + item.field + '" type="number" step="any" disabled>' +
        '</div>' +
        '<input data-slide="' + item.field + '" type="range" disabled>' +
        '<div data-hist="' + item.field + '"></div>' +
        '<div class="star-lens-meta" data-miss="' + item.field + '"></div></div>';
    });
    let eventHtml = '';
    EVENTS.forEach((item) => {
      const not = item.kind === 'stop' ? 'not_touched' : 'not_reached';
      eventHtml += '<div class="star-lens-field"><div class="star-lens-row"><b>' + item.name + '</b>' +
        ' <select data-event="' + item.name + '"><option value="">off</option>' +
        '<option value="' + (item.kind === 'stop' ? 'touched' : 'reached') + '">reached</option>' +
        '<option value="' + not + '">not on valid</option></select></div>' +
        '<div class="star-lens-rate" data-rate="' + item.name + '"></div></div>';
    });
    root.innerHTML =
      '<details open><summary>Population / Lens</summary>' +
      '<p class="star-lens-help">Lens is a temporary cut. Save keeps a named set. Show paints the current lens on the 15m chart.</p>' +
      '<div class="star-lens-counts">' +
      '<div>Population <b data-count-pop>—</b></div>' +
      '<div>Lens <b data-count-lens>—</b></div>' +
      '<div>Chart view <b data-count-view>—</b></div>' +
      '<div data-count-mix></div></div>' +
      '<label class="star-lens-row"><input type="checkbox" data-show checked> Show stars on chart</label>' +
      '<select data-source></select>' +
      '<div class="star-lens-catalog">' +
      '<p class="star-lens-help">Research coordinates. Labels are not saved. The Field ID is.</p>' +
      '<input data-coord-search type="search" placeholder="Search coordinates">' +
      '<div class="star-lens-cat-wrap">' +
      '<div class="star-lens-cat-list" data-coord-list></div>' +
      '<div class="star-lens-cat-resize" data-coord-resize></div>' +
      '</div>' +
      '<div class="star-lens-field" data-coord-panel hidden>' +
      '<div class="star-lens-row"><b data-coord-label></b> <code data-coord-shown></code></div>' +
      '<div class="star-lens-meta" data-coord-meta></div>' +
      '<label class="star-lens-row"><input type="checkbox" data-coord-num> in lens</label>' +
      '<input type="hidden" data-coord-id value="">' +
      '<div class="star-lens-row star-lens-row--controls">' +
      '<select data-coord-cmp><option value=">=">&ge;</option><option value=">">&gt;</option>' +
      '<option value="<=">&le;</option><option value="<">&lt;</option></select>' +
      '<input data-coord-bound type="number" step="any" disabled>' +
      '</div>' +
      '<input data-slide="coord" type="range" disabled>' +
      '<div data-hist="coord"></div>' +
      '<div class="star-lens-meta" data-miss="coord"></div></div></div>' +
      fieldHtml + eventHtml +
      '<label class="star-lens-row">Side <select data-side><option value="">off</option>' +
      '<option value="up">up</option><option value="down">down</option></select></label>' +
      '<label class="star-lens-row">Status <select data-status><option value="">off</option>' +
      '<option value="survived">survived</option><option value="stop first">stop first</option>' +
      '<option value="unordered">unordered</option><option value="no reading">no reading</option>' +
      '<option value="SURVIVED_WINDOW">SURVIVED_WINDOW</option>' +
      '<option value="STOP_FIRST">STOP_FIRST</option>' +
      '<option value="UNORDERED_BAR">UNORDERED_BAR</option>' +
      '<option value="INCOMPLETE_WINDOW">INCOMPLETE_WINDOW</option></select></label>' +
      '<button type="button" data-save>Save population</button>' +
      '<p class="star-lens-help">Save does not paint. It stores this lens as a child you can open later.</p>' +
      '</details>';
    const scroll = host ? host.querySelector('[data-star-scroll]') : null;
    if (scroll) scroll.insertBefore(root, scroll.firstChild);
    else if (host) host.appendChild(root);
    else document.body.appendChild(root);
    state.catHeight = readCatHeight();
    applyCatHeight(root);
    const resize = root.querySelector('[data-coord-resize]');
    if (resize) {
      resize.addEventListener('pointerdown', (ev) => {
        ev.preventDefault();
        const startY = ev.clientY;
        const startH = state.catHeight;
        function move(e) {
          writeCatHeight(startH + (e.clientY - startY));
          applyCatHeight(root);
        }
        function up() {
          window.removeEventListener('pointermove', move);
          window.removeEventListener('pointerup', up);
        }
        window.addEventListener('pointermove', move);
        window.addEventListener('pointerup', up);
      });
    }
    fetchSources().then((doc) => {
      const sel = root.querySelector('[data-source]');
      if (!sel || !doc || !doc.populations) return;
      sel.innerHTML = doc.populations.map((p) =>
        '<option value="' + p.id + '">' + p.name + ' (' + p.count + ')</option>').join('');
      state.source = sel.value || 'all';
      evaluate();
    });
    fetchCatalog().then((doc) => {
      state.catalog = (doc && doc.coordinates) || [];
      renderCatalog(root, state.catalog);
    });
    root.addEventListener('click', (ev) => {
      const bar = ev.target && ev.target.closest ? ev.target.closest('[data-hist-bar]') : null;
      if (bar) {
        const field = bar.getAttribute('data-hist-field');
        const n = binThreshold(bar.getAttribute('data-hist-from'), bar.getAttribute('data-hist-to'));
        if (!Number.isFinite(n) || !field) return;
        const outcome = root.querySelector('[data-num="' + field + '"]');
        const outcomeBound = root.querySelector('[data-bound="' + field + '"]');
        const outcomeSlide = root.querySelector('[data-slide="' + field + '"]');
        if (outcome) outcome.checked = true;
        if (outcomeBound) outcomeBound.value = String(n);
        if (outcomeSlide) {
          outcomeSlide.disabled = false;
          outcomeSlide.value = String(n);
        }
        const cid = root.querySelector('[data-coord-id]');
        if (cid && cid.value === field) {
          const coordOn = root.querySelector('[data-coord-num]');
          const coordBound = root.querySelector('[data-coord-bound]');
          const coordSlide = root.querySelector('[data-slide="coord"]');
          if (coordOn) coordOn.checked = true;
          if (coordBound) coordBound.value = String(n);
          if (coordSlide) {
            coordSlide.disabled = false;
            coordSlide.value = String(n);
          }
        }
        paint(root);
        evaluate();
        return;
      }
      const tri = ev.target && ev.target.closest ? ev.target.closest('[data-catalog-tri]') : null;
      if (tri) {
        ev.preventDefault();
        const key = tri.getAttribute('data-catalog-tri');
        state.catalogOpen[key] = !state.catalogOpen[key];
        renderCatalog(root, state.catalog);
        return;
      }
      const btn = ev.target && ev.target.closest ? ev.target.closest('[data-coord-pick]') : null;
      if (!btn) return;
      pickCoordinate(root, btn.getAttribute('data-coord-pick'));
    });
    const search = root.querySelector('[data-coord-search]');
    if (search) {
      search.addEventListener('input', () => {
        renderCatalog(root, state.catalog);
      });
    }
    root.addEventListener('change', (ev) => {
      const chk = ev.target && ev.target.getAttribute && ev.target.getAttribute('data-coord-check');
      if (chk) {
        const wrap = ev.target.closest('[data-catalog-key]');
        const g = wrap ? catalogGroup(wrap.getAttribute('data-catalog-key')) : null;
        const expanded = !!(wrap && wrap.classList.contains('is-open'));
        if (g && ev.target.getAttribute('data-role') === 'father' && !expanded) {
          const ids = Metric.memberIds(g);
          if (ev.target.checked) {
            if (ids.indexOf(state.selectedField) < 0) pickCoordinate(root, chk);
            else renderCatalog(root, state.catalog);
          } else if (ids.indexOf(state.selectedField) >= 0) {
            state.selectedField = '';
            const hid = root.querySelector('[data-coord-id]');
            if (hid) hid.value = '';
            renderCatalog(root, state.catalog);
            evaluate();
          }
          return;
        }
        pickCoordinate(root, chk);
        return;
      }
      const sel = root.querySelector('[data-source]');
      if (sel) state.source = sel.value;
      const show = root.querySelector('[data-show]');
      if (show) state.showStars = !!show.checked;
      if (ev.target && ev.target.getAttribute && ev.target.getAttribute('data-show') != null) {
        paint(root);
        if (typeof StarResearchOverlay !== 'undefined') StarResearchOverlay.refresh();
        return;
      }
      evaluate();
    });
    root.addEventListener('input', (ev) => {
      if (ev.target && ev.target.getAttribute && ev.target.getAttribute('data-coord-search') != null) return;
      const slide = ev.target && ev.target.getAttribute && ev.target.getAttribute('data-slide');
      if (!slide) return;
      if (slide === 'coord') {
        const bound = root.querySelector('[data-coord-bound]');
        if (bound) bound.value = ev.target.value;
        paint(root);
        scheduleEvaluate();
        return;
      }
      const bound = root.querySelector('[data-bound="' + slide + '"]');
      if (bound) bound.value = ev.target.value;
      paint(root);
      scheduleEvaluate();
    });
    const save = root.querySelector('[data-save]');
    if (save) {
      save.addEventListener('click', async () => {
        await fetchSave({ source: state.source, clauses: state.clauses });
        const doc = await fetchSources();
        const sel = root.querySelector('[data-source]');
        if (sel && doc && doc.populations) {
          sel.innerHTML = doc.populations.map((p) =>
            '<option value="' + p.id + '">' + p.name + ' (' + p.count + ')</option>').join('');
        }
      });
    }
  }

  function init(opts) {
    if (opts && typeof opts.fetchSources === 'function') fetchSources = opts.fetchSources;
    if (opts && typeof opts.fetchEval === 'function') fetchEval = opts.fetchEval;
    if (opts && typeof opts.fetchSave === 'function') fetchSave = opts.fetchSave;
    if (opts && typeof opts.fetchCatalog === 'function') fetchCatalog = opts.fetchCatalog;
    if (opts && typeof opts.visibleRangeMs === 'function') visibleRangeMs = opts.visibleRangeMs;
    if (typeof document !== 'undefined') mount();
  }

  function _resetForTests() {
    state.source = 'all';
    state.clauses = [];
    state.last = null;
    state.showStars = true;
    state.seq = 0;
    state.selectedField = '';
    state.catalog = [];
    state.catalogOpen = {};
    state.catHeight = 280;
  }

  return {
    init,
    evaluate,
    noteViewport,
    getMarks,
    chartRange,
    clausesFromForm,
    binThreshold,
    boundInBin,
    viewCount,
    histogramSvg,
    filterCatalog,
    setShowStars(on) { state.showStars = !!on; },
    _resetForTests,
  };
})();

if (typeof document !== 'undefined') {
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => StarLens.init());
  } else {
    StarLens.init();
  }
}
if (typeof module !== 'undefined' && module.exports) {
  module.exports = StarLens;
}
