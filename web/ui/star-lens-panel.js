/**
 * Population/Lens workstation. Semantics come from /api/research/star-lens/*.
 * This file does not compute R state, missingness, or self-exclusion.
 */
const StarLens = (() => {
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
  };

  let fetchSources = defaultSources;
  let fetchEval = defaultEval;
  let fetchSave = defaultSave;
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

  function paintApi() {
    return typeof StarPopulationDisplay !== 'undefined' ? StarPopulationDisplay : null;
  }

  function syncPaintForm(root) {
    const api = paintApi();
    if (!root || !api || typeof api.get !== 'function') return;
    const prefs = api.get();
    const shape = root.querySelector('[data-paint-shape]');
    const up = root.querySelector('[data-paint-up]');
    const down = root.querySelector('[data-paint-down]');
    if (shape) shape.value = prefs.shape;
    if (up) up.value = prefs.upColor;
    if (down) down.value = prefs.downColor;
  }

  function applyPaintForm(root) {
    const api = paintApi();
    if (!root || !api || typeof api.set !== 'function') return;
    const shape = root.querySelector('[data-paint-shape]');
    const up = root.querySelector('[data-paint-up]');
    const down = root.querySelector('[data-paint-down]');
    api.set({
      shape: shape ? shape.value : undefined,
      upColor: up ? up.value : undefined,
      downColor: down ? down.value : undefined,
    });
    if (typeof StarResearchOverlay !== 'undefined' && typeof StarResearchOverlay.refresh === 'function') {
      StarResearchOverlay.refresh();
    }
  }

  function isPaintControl(el) {
    if (!el || typeof el.getAttribute !== 'function') return false;
    return el.getAttribute('data-paint-shape') != null
      || el.getAttribute('data-paint-up') != null
      || el.getAttribute('data-paint-down') != null;
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
    return out;
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

  function histogramSvg(pic, bound, cmp, active) {
    if (!pic || !pic.rangeOk || !pic.bins || !pic.bins.length) {
      return '<div class="star-lens-empty">no observations</div>';
    }
    const max = Math.max.apply(null, pic.bins.concat([1]));
    const w = 248;
    const h = 52;
    const gap = 1;
    const bw = (w - gap * pic.bins.length) / pic.bins.length;
    let bars = '';
    for (let i = 0; i < pic.bins.length; i++) {
      const bh = (pic.bins[i] / max) * (h - 12);
      bars += '<rect x="' + (i * (bw + gap)).toFixed(2) + '" y="' + (h - 12 - bh).toFixed(2) +
        '" width="' + bw.toFixed(2) + '" height="' + bh.toFixed(2) + '" fill="#5d6b7a"></rect>';
    }
    let handle = '';
    if (active && Number.isFinite(Number(bound)) && pic.max > pic.min) {
      let x = ((Number(bound) - pic.min) / (pic.max - pic.min)) * w;
      if (x < 0) x = 0;
      if (x > w) x = w;
      handle = '<line x1="' + x.toFixed(2) + '" x2="' + x.toFixed(2) + '" y1="0" y2="' + (h - 12) +
        '" stroke="#d1d4dc" stroke-width="1.5"></line>';
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
    syncPaintForm(root);
    const nums = last.numbers || [];
    FIELDS.forEach((item) => {
      const box = root.querySelector('[data-hist="' + item.field + '"]');
      const on = root.querySelector('[data-num="' + item.field + '"]');
      const boundEl = root.querySelector('[data-bound="' + item.field + '"]');
      const slide = root.querySelector('[data-slide="' + item.field + '"]');
      const miss = root.querySelector('[data-miss="' + item.field + '"]');
      const pic = nums.find((n) => n.field === item.field);
      const active = !!(on && on.checked);
      if (box) box.innerHTML = histogramSvg(pic, boundEl ? boundEl.value : null, '', active);
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
      '<div class="star-lens-paint">' +
      '<label class="star-lens-row">Shape <select data-paint-shape>' +
      '<option value="star4">Star</option>' +
      '<option value="arrow">Arrow</option>' +
      '<option value="circle">Circle</option>' +
      '<option value="triangle">Triangle</option>' +
      '<option value="square">Square</option>' +
      '<option value="diamond">Diamond</option>' +
      '</select></label>' +
      '<label class="star-lens-row">Up <input type="color" data-paint-up></label>' +
      '<label class="star-lens-row">Down <input type="color" data-paint-down></label>' +
      '</div>' +
      '<select data-source></select>' +
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
    fetchSources().then((doc) => {
      const sel = root.querySelector('[data-source]');
      if (!sel || !doc || !doc.populations) return;
      sel.innerHTML = doc.populations.map((p) =>
        '<option value="' + p.id + '">' + p.name + ' (' + p.count + ')</option>').join('');
      state.source = sel.value || 'all';
      evaluate();
    });
    syncPaintForm(root);
    root.addEventListener('change', (ev) => {
      if (isPaintControl(ev.target)) {
        applyPaintForm(root);
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
      if (isPaintControl(ev.target)) {
        applyPaintForm(root);
        return;
      }
      const slide = ev.target && ev.target.getAttribute && ev.target.getAttribute('data-slide');
      if (!slide) return;
      const bound = root.querySelector('[data-bound="' + slide + '"]');
      if (bound) bound.value = ev.target.value;
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
    if (opts && typeof opts.visibleRangeMs === 'function') visibleRangeMs = opts.visibleRangeMs;
    if (typeof document !== 'undefined') mount();
  }

  function _resetForTests() {
    state.source = 'all';
    state.clauses = [];
    state.last = null;
    state.showStars = true;
    state.seq = 0;
  }

  return {
    init,
    evaluate,
    noteViewport,
    getMarks,
    chartRange,
    clausesFromForm,
    viewCount,
    histogramSvg,
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
