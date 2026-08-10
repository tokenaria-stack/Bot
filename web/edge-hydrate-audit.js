/**
 * TEMPORARY — EDGE_HYDRATE throughput probe (measure only; no camera / prefetch).
 *
 * Always logs one line per completed left/right chunk.
 * Also stores window.__EDGE_HYDRATE_LAST__ for copy/paste.
 *
 * paintMs = time from paint sync start through the NEXT requestAnimationFrame
 * (Gemini: sync setData alone misses GPU/layout cost).
 *
 * scheduleMs = markDirty → compositor flush start (RenderScheduler RAF gap).
 * Not in the original GPT template — without it, scheduler wait looks like "paint".
 */
(function initEdgeHydrateAudit(global) {
  /** @type {object|null} */
  let open = null;

  function now() {
    return (typeof performance !== 'undefined' && performance.now)
      ? performance.now()
      : Date.now();
  }

  function noteIntent(direction) {
    const dir = direction === 'LEFT' ? 'LEFT' : 'RIGHT';
    // Keep earliest intent for this open txn; refresh if previous completed.
    if (open && open._phase !== 'done' && open.direction === dir && open.intentAt != null) {
      return;
    }
    open = {
      direction: dir,
      intentAt: now(),
      fetchStart: null,
      fetchEnd: null,
      mergeStart: null,
      mergeEnd: null,
      markDirtyAt: null,
      paintStart: null,
      paintEnd: null,
      barsAdded: 0,
      tipBefore: null,
      tipAfter: null,
      storeBefore: null,
      storeAfter: null,
      tf: '',
      _phase: 'intent',
    };
  }

  function markFetchStart(direction) {
    const dir = direction === 'LEFT' ? 'LEFT' : 'RIGHT';
    if (!open || open.direction !== dir || open.fetchStart != null) {
      // Fetch can start without a prior note (force path) — synthesize intent.
      open = {
        direction: dir,
        intentAt: now(),
        fetchStart: null,
        fetchEnd: null,
        mergeStart: null,
        mergeEnd: null,
        markDirtyAt: null,
        paintStart: null,
        paintEnd: null,
        barsAdded: 0,
        tipBefore: null,
        tipAfter: null,
        storeBefore: null,
        storeAfter: null,
        tf: '',
        _phase: 'intent',
      };
    }
    open.fetchStart = now();
    open._phase = 'fetch';
  }

  function markFetchEnd() {
    if (!open || open.fetchEnd != null) return;
    open.fetchEnd = now();
  }

  function markMergeStart(meta) {
    if (!open) return;
    open.mergeStart = now();
    open._phase = 'merge';
    if (meta && typeof meta === 'object') {
      if (meta.tipBefore != null) open.tipBefore = meta.tipBefore;
      if (meta.storeBefore != null) open.storeBefore = meta.storeBefore;
      if (meta.tf != null) open.tf = String(meta.tf);
    }
  }

  function markMergeEnd(meta) {
    if (!open) return;
    open.mergeEnd = now();
    if (meta && typeof meta === 'object') {
      if (Number.isFinite(meta.barsAdded)) open.barsAdded = meta.barsAdded;
      if (meta.tipAfter != null) open.tipAfter = meta.tipAfter;
      if (meta.storeAfter != null) open.storeAfter = meta.storeAfter;
    }
  }

  function attachToPaintIntent(intent) {
    if (!open || !intent || typeof intent !== 'object') return intent;
    open.markDirtyAt = now();
    open._phase = 'scheduled';
    intent._edgeHydrate = open;
    return intent;
  }

  function markPaintStart(diag) {
    const d = diag || open;
    if (!d || d.paintStart != null) return;
    d.paintStart = now();
    d._phase = 'paint';
  }

  /**
   * After sync flush: wait one rAF, then log. Does not move the camera.
   * @param {object} [diag]
   */
  function completeAfterPaintRaf(diag) {
    const d = diag || open;
    if (!d || d._phase === 'done' || d._rafPending) return;
    if (d.paintStart == null) d.paintStart = now();
    d._rafPending = true;

    const finish = () => {
      if (d._phase === 'done') return;
      d.paintEnd = now();
      d._phase = 'done';
      logLine(d);
      if (open === d) open = null;
    };

    if (typeof requestAnimationFrame === 'function') {
      requestAnimationFrame(finish);
    } else {
      finish();
    }
  }

  function abort() {
    open = null;
  }

  function round1(n) {
    return Math.round(Number(n) * 10) / 10;
  }

  function logLine(d) {
    const orchestrationMs = (d.fetchStart != null && d.intentAt != null)
      ? d.fetchStart - d.intentAt
      : null;
    const fetchMs = (d.fetchEnd != null && d.fetchStart != null)
      ? d.fetchEnd - d.fetchStart
      : null;
    const mergeMs = (d.mergeEnd != null && d.mergeStart != null)
      ? d.mergeEnd - d.mergeStart
      : null;
    const scheduleMs = (d.paintStart != null && d.markDirtyAt != null)
      ? d.paintStart - d.markDirtyAt
      : null;
    const paintMs = (d.paintEnd != null && d.paintStart != null)
      ? d.paintEnd - d.paintStart
      : null;
    const totalMs = (d.paintEnd != null && d.intentAt != null)
      ? d.paintEnd - d.intentAt
      : null;

    const report = {
      direction: d.direction,
      orchestrationMs: orchestrationMs != null ? round1(orchestrationMs) : null,
      fetchMs: fetchMs != null ? round1(fetchMs) : null,
      mergeMs: mergeMs != null ? round1(mergeMs) : null,
      scheduleMs: scheduleMs != null ? round1(scheduleMs) : null,
      paintMs: paintMs != null ? round1(paintMs) : null,
      totalMs: totalMs != null ? round1(totalMs) : null,
      barsAdded: d.barsAdded,
      tipBefore: d.tipBefore,
      tipAfter: d.tipAfter,
      storeBefore: d.storeBefore,
      storeAfter: d.storeAfter,
      tf: d.tf || '',
    };

    global.__EDGE_HYDRATE_LAST__ = report;
    console.log(
      `EDGE_HYDRATE ${report.direction}`
      + `\norchestrationMs: ${report.orchestrationMs}`
      + `\nfetchMs: ${report.fetchMs}`
      + `\nmergeMs: ${report.mergeMs}`
      + `\nscheduleMs: ${report.scheduleMs}`
      + `\npaintMs: ${report.paintMs}`
      + `\ntotalMs: ${report.totalMs}`
      + `\nbarsAdded: ${report.barsAdded}`
      + `\ntipBefore: ${report.tipBefore}`
      + `\ntipAfter: ${report.tipAfter}`
      + `\nstoreBefore: ${report.storeBefore}`
      + `\nstoreAfter: ${report.storeAfter}`
      + `\ntf: ${report.tf}`,
    );
  }

  const EdgeHydrateAudit = {
    noteIntent,
    markFetchStart,
    markFetchEnd,
    markMergeStart,
    markMergeEnd,
    attachToPaintIntent,
    markPaintStart,
    completeAfterPaintRaf,
    abort,
  };

  global.EdgeHydrateAudit = EdgeHydrateAudit;
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = EdgeHydrateAudit;
  }
})(typeof window !== 'undefined' ? window : globalThis);
