/**
 * STAR-RESEARCH-NAV-1. Selected-star identity lives here.
 * A number selection asks the existing history island to center that time.
 * A click on the painted canonical Star selects the same row and does not seek.
 * Paint, indicators, and timeframe changes do not call seek.
 * Listeners see the row after that one seek. A listener must not seek.
 */
const StarResearch = (() => {
  const state = {
    index: null,
    count: 0,
    decisionAt: null,
    row: null,
    seq: 0,
    walk: null,
  };
  const listeners = [];

  let fetchStar = defaultFetchStar;
  let fetchAt = defaultFetchAt;
  let seek = defaultSeek;
  let visibleBars = defaultVisibleBars;
  let getTf = defaultTf;
  let canonicalOpenSec = defaultCanonicalOpenSec;
  let clickBound = false;

  function defaultFetchStar(index) {
    return fetch('/api/research/star-stop?index=' + encodeURIComponent(index))
      .then((res) => (res.ok ? res.json() : null))
      .catch(() => null);
  }

  function defaultFetchAt(openMs) {
    return fetch('/api/research/star-stop?at=' + encodeURIComponent(openMs))
      .then((res) => (res.ok ? res.json() : null))
      .catch(() => null);
  }

  function defaultTf() {
    if (typeof window === 'undefined' || window.currentTf == null) return '';
    return String(window.currentTf);
  }

  function defaultCanonicalOpenSec(point) {
    if (!point || typeof WozduhCrossovers === 'undefined' || typeof WozduhCrossovers.canonicalOpenSec !== 'function') {
      return null;
    }
    return WozduhCrossovers.canonicalOpenSec(point.x, point.y);
  }

  function defaultSeek(centerTimeMs, bars) {
    if (typeof window.seekHistoryIsland !== 'function') return false;
    window.seekHistoryIsland(centerTimeMs, bars);
    return true;
  }

  function defaultVisibleBars() {
    try {
      if (typeof TimeCamera === 'undefined' || typeof TimeCamera.getCanonicalVisibleRange !== 'function') {
        return 140;
      }
      const range = TimeCamera.getCanonicalVisibleRange();
      if (!range || !Number.isFinite(range.from) || !Number.isFinite(range.to)) return 140;
      const width = range.to - range.from;
      if (!Number.isFinite(width) || width < 10) return 140;
      return width;
    } catch {
      return 140;
    }
  }

  function walkLength() {
    if (state.walk) return state.walk.length;
    return state.count > 0 ? state.count : 0;
  }

  function ordinalOf(index) {
    if (index == null) return null;
    if (state.walk) {
      const at = state.walk.indexOf(index);
      return at < 0 ? null : at + 1;
    }
    return index + 1;
  }

  function paintCount() {
    const el = typeof document !== 'undefined' ? document.getElementById('star-research-count') : null;
    if (!el) return;
    const len = walkLength();
    const ord = ordinalOf(state.index);
    if (len <= 0) {
      el.textContent = '— / —';
      return;
    }
    el.textContent = (ord == null ? '—' : String(ord)) + ' / ' + len;
  }

  function paintNumber(force) {
    const input = typeof document !== 'undefined' ? document.getElementById('star-research-no') : null;
    if (!input) return;
    if (!force && document.activeElement === input) return;
    const ord = ordinalOf(state.index);
    input.value = ord == null ? '' : String(ord);
  }

  function paintSide() {
    const el = typeof document !== 'undefined' ? document.getElementById('star-research-side') : null;
    if (!el) return;
    const side = state.row && state.row.side === 'down' ? 'down' : (state.row && state.row.side === 'up' ? 'up' : '');
    el.className = 'star-side' + (side ? ' star-side--' + side : '');
    el.title = side || '';
  }

  function paintNav(force) {
    paintCount();
    paintNumber(force);
    paintSide();
  }

  function clear() {
    state.seq += 1;
    state.index = null;
    state.decisionAt = null;
    state.row = null;
    paintNav(true);
    emitSelected();
    return true;
  }

  function acceptBody(body, center) {
    if (!body || !body.row || !Number.isFinite(Number(body.row.decisionAt)) || Number(body.row.decisionAt) <= 0) {
      return false;
    }
    const next = Number(body.index);
    if (!Number.isFinite(next) || next < 0) return false;
    state.index = next;
    state.count = Number(body.count) > 0 ? Number(body.count) : state.count;
    state.decisionAt = Number(body.row.decisionAt);
    state.row = body.row;
    paintNav(center !== true);
    if (center) seek(state.decisionAt, visibleBars());
    emitSelected();
    return true;
  }

  async function applyIndex(index) {
    const seq = ++state.seq;
    const body = await fetchStar(index);
    if (seq !== state.seq) return false;
    return acceptBody(body, true);
  }

  async function selectOpenTime(openMs) {
    const at = Number(openMs);
    if (!Number.isFinite(at) || at <= 0) return false;
    const seq = ++state.seq;
    const body = await fetchAt(at);
    if (seq !== state.seq) return false;
    if (!body) {
      if (typeof StarResearchOverlay !== 'undefined' && typeof StarResearchOverlay.noteUnresolved === 'function') {
        StarResearchOverlay.noteUnresolved();
      }
      return false;
    }
    return acceptBody(body, false);
  }

  function onPaneClick(point) {
    if (getTf() !== '15m') return Promise.resolve(false);
    const sec = Number(canonicalOpenSec(point));
    if (!Number.isFinite(sec) || sec <= 0) return Promise.resolve(false);
    const openMs = sec < 1e11 ? Math.round(sec * 1000) : Math.round(sec);
    return selectOpenTime(openMs);
  }

  function bindChartClick() {
    if (clickBound) return true;
    if (typeof ChartAdapter === 'undefined' || typeof ChartAdapter.onWozduhClick !== 'function') return false;
    clickBound = ChartAdapter.onWozduhClick((pt) => { onPaneClick(pt); }) === true;
    return clickBound;
  }

  function emitSelected() {
    for (let i = 0; i < listeners.length; i++) {
      try { listeners[i](); } catch { /* presentation must not break the seek */ }
    }
  }

  function onSelected(fn) {
    if (typeof fn === 'function') listeners.push(fn);
  }

  function selectNumber(raw) {
    const n = Math.round(Number(raw));
    if (!Number.isFinite(n) || n < 1) return Promise.resolve(false);
    if (state.count > 0 && n > state.count) return Promise.resolve(false);
    return applyIndex(n - 1);
  }

  function selectOrdinal(raw) {
    const n = Math.round(Number(raw));
    if (!Number.isFinite(n) || n < 1) return Promise.resolve(false);
    if (state.walk) {
      if (n > state.walk.length) return Promise.resolve(false);
      return applyIndex(state.walk[n - 1]);
    }
    return selectNumber(n);
  }

  function walkAt(delta) {
    if (!state.walk) return null;
    if (!state.walk.length) return false;
    if (state.index == null) {
      return delta > 0 ? state.walk[0] : state.walk[state.walk.length - 1];
    }
    const at = state.walk.indexOf(state.index);
    if (at < 0) {
      return delta > 0 ? state.walk[0] : state.walk[state.walk.length - 1];
    }
    const next = at + delta;
    if (next < 0 || next >= state.walk.length) return false;
    return state.walk[next];
  }

  function previous() {
    const stepped = walkAt(-1);
    if (stepped === false) return Promise.resolve(false);
    if (stepped != null) return applyIndex(stepped);
    if (state.index == null) {
      if (state.count <= 0) return Promise.resolve(false);
      return applyIndex(state.count - 1);
    }
    if (state.index <= 0) return Promise.resolve(false);
    return applyIndex(state.index - 1);
  }

  function next() {
    const stepped = walkAt(1);
    if (stepped === false) return Promise.resolve(false);
    if (stepped != null) return applyIndex(stepped);
    if (state.count > 0 && state.index != null && state.index >= state.count - 1) {
      return Promise.resolve(false);
    }
    const index = state.index == null ? 0 : state.index + 1;
    return applyIndex(index);
  }

  function setWalk(list) {
    if (!Array.isArray(list)) {
      state.walk = null;
      paintNav();
      return Promise.resolve();
    }
    state.walk = list.map((n) => Number(n)).filter((n) => Number.isFinite(n) && n >= 0);
    if (state.index == null) {
      paintNav();
      return Promise.resolve();
    }
    if (state.walk.indexOf(state.index) >= 0) {
      paintNav();
      return Promise.resolve();
    }
    if (!state.walk.length) {
      clear();
      return Promise.resolve();
    }
    return applyIndex(state.walk[0]);
  }

  /** Timeframe changes keep this identity and do not move the camera. */
  function noteTimeframe() {
    return state.index;
  }

  function getState() {
    return {
      index: state.index,
      count: state.count,
      decisionAt: state.decisionAt,
      row: state.row,
      number: state.index == null ? null : state.index + 1,
      ordinal: ordinalOf(state.index),
      walkLength: walkLength(),
    };
  }

  function init(opts) {
    if (opts && typeof opts.fetchStar === 'function') fetchStar = opts.fetchStar;
    if (opts && typeof opts.fetchAt === 'function') fetchAt = opts.fetchAt;
    if (opts && typeof opts.seek === 'function') seek = opts.seek;
    if (opts && typeof opts.visibleBars === 'function') visibleBars = opts.visibleBars;
    if (opts && typeof opts.getTf === 'function') getTf = opts.getTf;
    if (opts && typeof opts.canonicalOpenSec === 'function') canonicalOpenSec = opts.canonicalOpenSec;
    if (typeof document === 'undefined') return;
    const input = document.getElementById('star-research-no');
    const prev = document.getElementById('star-research-prev');
    const nextBtn = document.getElementById('star-research-next');
    const clearBtn = document.getElementById('star-research-clear');
    if (input && !input.dataset.starResearch) {
      input.dataset.starResearch = '1';
      input.addEventListener('keydown', (ev) => {
        if (ev.key === 'ArrowUp' || ev.key === 'ArrowDown') {
          ev.preventDefault();
          return;
        }
        if (ev.key !== 'Enter') return;
        ev.preventDefault();
        selectOrdinal(input.value);
      });
      input.addEventListener('wheel', (ev) => { ev.preventDefault(); }, { passive: false });
    }
    if (prev && !prev.dataset.starResearch) {
      prev.dataset.starResearch = '1';
      prev.addEventListener('click', () => { previous(); });
    }
    if (nextBtn && !nextBtn.dataset.starResearch) {
      nextBtn.dataset.starResearch = '1';
      nextBtn.addEventListener('click', () => { next(); });
    }
    if (clearBtn && !clearBtn.dataset.starResearch) {
      clearBtn.dataset.starResearch = '1';
      clearBtn.addEventListener('click', () => { clear(); });
    }
    paintNav();
  }

  function _resetForTests() {
    state.index = null;
    state.count = 0;
    state.decisionAt = null;
    state.row = null;
    state.seq = 0;
    state.walk = null;
    listeners.length = 0;
    fetchStar = defaultFetchStar;
    fetchAt = defaultFetchAt;
    seek = defaultSeek;
    visibleBars = defaultVisibleBars;
    getTf = defaultTf;
    canonicalOpenSec = defaultCanonicalOpenSec;
    clickBound = false;
  }

  return {
    init,
    selectNumber,
    selectOrdinal,
    selectOpenTime,
    onPaneClick,
    bindChartClick,
    previous,
    next,
    clear,
    noteTimeframe,
    onSelected,
    getState,
    setWalk,
    _resetForTests,
  };
})();

if (typeof document !== 'undefined') {
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => StarResearch.init());
  } else {
    StarResearch.init();
  }
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = StarResearch;
}
