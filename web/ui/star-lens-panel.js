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
    marks: [],
    viewFrom: 0,
    viewTo: 0,
    seq: 0,
  };

  let fetchSources = defaultSources;
  let fetchEval = defaultEval;
  let fetchSave = defaultSave;

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
    try {
      if (typeof TimeCamera === 'undefined' || typeof TimeCamera.getCanonicalVisibleRange !== 'function') {
        return { from: 0, to: 0 };
      }
      const range = TimeCamera.getCanonicalVisibleRange();
      if (!range || !Number.isFinite(range.from) || !Number.isFinite(range.to)) return { from: 0, to: 0 };
      const from = range.from < 1e11 ? Math.round(range.from * 1000) : Math.round(range.from);
      const to = range.to < 1e11 ? Math.round(range.to * 1000) : Math.round(range.to);
      return { from: from, to: to };
    } catch {
      return { from: 0, to: 0 };
    }
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

  function histogramSvg(pic, bound, cmp) {
    if (!pic || !pic.rangeOk || !pic.bins || !pic.bins.length) {
      return '<div class="star-lens-empty">no observations</div>';
    }
    const max = Math.max.apply(null, pic.bins.concat([1]));
    const w = 220;
    const h = 36;
    const gap = 1;
    const bw = (w - gap * pic.bins.length) / pic.bins.length;
    let bars = '';
    for (let i = 0; i < pic.bins.length; i++) {
      const bh = (pic.bins[i] / max) * h;
      bars += '<rect x="' + (i * (bw + gap)).toFixed(2) + '" y="' + (h - bh).toFixed(2) +
        '" width="' + bw.toFixed(2) + '" height="' + bh.toFixed(2) + '" fill="#5d6b7a"></rect>';
    }
    let handle = '';
    if (Number.isFinite(Number(bound)) && pic.max > pic.min) {
      let x = ((Number(bound) - pic.min) / (pic.max - pic.min)) * w;
      if (x < 0) x = 0;
      if (x > w) x = w;
      handle = '<line x1="' + x.toFixed(2) + '" x2="' + x.toFixed(2) + '" y1="0" y2="' + h +
        '" stroke="#d1d4dc" stroke-width="1"></line>';
    }
    return '<svg class="star-lens-hist" viewBox="0 0 ' + w + ' ' + h + '" width="' + w + '" height="' + h + '">' +
      bars + handle + '</svg><div class="star-lens-meta">missing ' + pic.missing +
      (cmp ? ' · ' + cmp + ' ' + bound : '') + '</div>';
  }

  function paint(root) {
    if (!root || !state.last) return;
    const last = state.last;
    const pop = root.querySelector('[data-count-pop]');
    const lens = root.querySelector('[data-count-lens]');
    const view = root.querySelector('[data-count-view]');
    const mix = root.querySelector('[data-count-mix]');
    if (pop) pop.textContent = String(last.population);
    if (lens) lens.textContent = String(last.lensPass);
    if (view) view.textContent = String(last.chartView);
    if (mix) mix.textContent = 'discovery ' + last.discovery + ' · holdout ' + last.holdout;
    const nums = last.numbers || [];
    FIELDS.forEach((item) => {
      const box = root.querySelector('[data-hist="' + item.field + '"]');
      if (!box) return;
      const pic = nums.find((n) => n.field === item.field);
      const boundEl = root.querySelector('[data-bound="' + item.field + '"]');
      const cmpEl = root.querySelector('[data-cmp="' + item.field + '"]');
      const on = root.querySelector('[data-num="' + item.field + '"]');
      box.innerHTML = histogramSvg(pic, boundEl && on && on.checked ? boundEl.value : null, cmpEl ? cmpEl.value : '');
    });
    (last.events || []).forEach((ev) => {
      const box = root.querySelector('[data-rate="' + ev.name + '"]');
      if (!box) return;
      const pct = ev.rateOk ? (100 * ev.rate).toFixed(1) + '%' : '—';
      box.textContent = ev.reached + ' / ' + ev.base + '  ' + pct +
        ' · not ' + ev.notReached + ' · no R/stop ' + ev.undefined;
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
    const from = state.viewFrom;
    const to = state.viewTo;
    body.chartView = viewCount(body.pass, from, to);
    state.last = body;
    state.marks = marksInView(body.pass, from, to);
    if (typeof StarResearch !== 'undefined' && typeof StarResearch.setWalk === 'function') {
      StarResearch.setWalk((body.pass || []).map((m) => m.index));
    }
    paint(root);
    if (typeof StarResearchOverlay !== 'undefined' && typeof StarResearchOverlay.refresh === 'function') {
      StarResearchOverlay.refresh();
    }
    return body;
  }

  function noteViewport() {
    if (!state.last) return;
    const range = chartRange();
    state.viewFrom = range.from;
    state.viewTo = range.to;
    state.last.chartView = viewCount(state.last.pass, range.from, range.to);
    state.marks = marksInView(state.last.pass, range.from, range.to);
    const root = document.getElementById('star-lens');
    paint(root);
  }

  function getMarks() {
    return state.marks;
  }

  function mount() {
    if (typeof document === 'undefined') return;
    if (document.getElementById('star-lens')) return;
    const host = document.getElementById('star-inspection');
    const root = document.createElement('div');
    root.id = 'star-lens';
    let fieldHtml = '';
    FIELDS.forEach((item) => {
      fieldHtml += '<label class="star-lens-row"><input type="checkbox" data-num="' + item.field + '"> ' +
        item.label +
        ' <select data-cmp="' + item.field + '"><option value=">=">&ge;</option><option value=">">&gt;</option>' +
        '<option value="<=">&le;</option><option value="<">&lt;</option></select>' +
        ' <input data-bound="' + item.field + '" type="number" step="any" value="0"></label>' +
        '<div data-hist="' + item.field + '"></div>';
    });
    let eventHtml = '';
    EVENTS.forEach((item) => {
      const not = item.kind === 'stop' ? 'not_touched' : 'not_reached';
      eventHtml += '<label class="star-lens-row">' + item.name +
        ' <select data-event="' + item.name + '"><option value="">off</option>' +
        '<option value="' + (item.kind === 'stop' ? 'touched' : 'reached') + '">reached</option>' +
        '<option value="' + not + '">not on valid</option></select></label>' +
        '<div class="star-lens-rate" data-rate="' + item.name + '"></div>';
    });
    root.innerHTML =
      '<details open><summary>Population / Lens</summary>' +
      '<div class="star-lens-counts">' +
      '<div>Population <b data-count-pop>—</b></div>' +
      '<div>Lens <b data-count-lens>—</b></div>' +
      '<div>Chart view <b data-count-view>—</b></div>' +
      '<div data-count-mix></div></div>' +
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
      '</details>';
    const bar = host ? host.querySelector('.star-inspection-bar') : null;
    if (bar) bar.insertAdjacentElement('afterend', root);
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
    root.addEventListener('change', () => {
      const sel = root.querySelector('[data-source]');
      if (sel) state.source = sel.value;
      evaluate();
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
    if (typeof document !== 'undefined') mount();
  }

  function _resetForTests() {
    state.source = 'all';
    state.clauses = [];
    state.last = null;
    state.marks = [];
    state.seq = 0;
  }

  return {
    init,
    evaluate,
    noteViewport,
    getMarks,
    clausesFromForm,
    viewCount,
    histogramSvg,
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
