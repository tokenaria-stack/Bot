/**
 * DisplayTimeline — ADR-027 pure display-only future whitespace times.
 *
 * Generates expected bar-open timestamps for LWC whitespace (time scale chrome).
 * Never writes to ColumnarStore / DDR / engine. ChartAdapter is the only injector.
 *
 * Bar boundaries mirror data.NextBarOpen (fixed step; UTC week/month calendars).
 */
(function (global) {
  'use strict';

  const MS_MINUTE = 60 * 1000;
  const MS_HOUR = 60 * MS_MINUTE;
  const MS_DAY = 24 * MS_HOUR;
  const DEFAULT_MAX_BARS = 500;
  const DEFAULT_MIN_BUFFER = 8;

  /**
   * @param {string} tf
   * @param {(tf: string) => number} [getIntervalMs]
   * @returns {number}
   */
  function intervalMsFor(tf, getIntervalMs) {
    if (typeof getIntervalMs === 'function') {
      const ms = Number(getIntervalMs(tf));
      if (Number.isFinite(ms) && ms > 0) return ms;
    }
    if (typeof TimeNormalizer !== 'undefined' && TimeNormalizer.getIntervalMs) {
      const ms = Number(TimeNormalizer.getIntervalMs(tf));
      if (Number.isFinite(ms) && ms > 0) return ms;
    }
    return MS_MINUTE;
  }

  /**
   * @param {string} tf
   * @returns {'fixed'|'week'|'month'}
   */
  function boundaryKind(tf) {
    const raw = String(tf || '1m').trim();
    if (/^1w$/i.test(raw) || /^7d$/i.test(raw)) return 'week';
    if (/^1M$/.test(raw)) return 'month';
    return 'fixed';
  }

  /** Monday 00:00 UTC containing or before ms. */
  function mondayOpenUTC(ms) {
    const d = new Date(ms);
    const day = d.getUTCDay(); // 0=Sun … 1=Mon
    const diff = day === 0 ? -6 : 1 - day;
    return Date.UTC(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate() + diff);
  }

  function monthOpenUTC(ms) {
    const d = new Date(ms);
    return Date.UTC(d.getUTCFullYear(), d.getUTCMonth(), 1);
  }

  /**
   * Floor to current bar open (unix ms).
   * @param {number} ms
   * @param {string} tf
   * @param {(tf: string) => number} [getIntervalMs]
   */
  function currentBarOpenMs(ms, tf, getIntervalMs) {
    const t = Number(ms);
    if (!Number.isFinite(t) || t < 0) return null;
    const kind = boundaryKind(tf);
    if (kind === 'week') return mondayOpenUTC(t);
    if (kind === 'month') return monthOpenUTC(t);
    const step = intervalMsFor(tf, getIntervalMs);
    return Math.floor(t / step) * step;
  }

  /**
   * Next bar open after openMs (unix ms). Floors first.
   * @param {number} openMs
   * @param {string} tf
   * @param {(tf: string) => number} [getIntervalMs]
   */
  function nextBarOpenMs(openMs, tf, getIntervalMs) {
    const cur = currentBarOpenMs(openMs, tf, getIntervalMs);
    if (cur == null) return null;
    const kind = boundaryKind(tf);
    if (kind === 'week') {
      return cur + 7 * MS_DAY;
    }
    if (kind === 'month') {
      const d = new Date(cur);
      return Date.UTC(d.getUTCFullYear(), d.getUTCMonth() + 1, 1);
    }
    return cur + intervalMsFor(tf, getIntervalMs);
  }

  /**
   * How many future whitespace bars to project for the visible camera.
   * @param {{
   *   lastLogical: number,
   *   visibleTo?: number|null,
   *   rightOffset?: number|null,
   *   minBuffer?: number,
   *   maxBars?: number,
   * }} opts
   */
  function countFutureBars(opts) {
    const lastLogical = Number(opts?.lastLogical);
    if (!Number.isFinite(lastLogical)) return 0;
    const minBuffer = Number.isFinite(opts?.minBuffer) ? Math.max(0, opts.minBuffer) : DEFAULT_MIN_BUFFER;
    const maxBars = Number.isFinite(opts?.maxBars) ? Math.max(0, opts.maxBars) : DEFAULT_MAX_BARS;

    let need = minBuffer;
    const visibleTo = opts?.visibleTo;
    if (Number.isFinite(visibleTo)) {
      need = Math.max(need, Math.ceil(visibleTo - lastLogical));
    }
    const rightOffset = opts?.rightOffset;
    if (Number.isFinite(rightOffset) && rightOffset > 0) {
      need = Math.max(need, Math.ceil(rightOffset));
    }
    need = Math.max(0, need);
    return Math.min(need, maxBars);
  }

  /**
   * Future bar-open times as unix **seconds** (LWC chart time).
   * @param {{
   *   lastTimeSec: number,
   *   count: number,
   *   tf?: string,
   *   getIntervalMs?: (tf: string) => number,
   * }} opts
   * @returns {number[]}
   */
  function buildFutureTimes(opts) {
    const lastSec = Number(opts?.lastTimeSec);
    const count = Math.floor(Number(opts?.count) || 0);
    if (!Number.isFinite(lastSec) || count <= 0) return [];
    const tf = opts?.tf || '1m';
    const getIntervalMs = opts?.getIntervalMs;
    let ms = currentBarOpenMs(lastSec * 1000, tf, getIntervalMs);
    if (ms == null) return [];
    const out = [];
    for (let i = 0; i < count; i++) {
      ms = nextBarOpenMs(ms, tf, getIntervalMs);
      if (ms == null) break;
      out.push(Math.floor(ms / 1000));
    }
    return out;
  }

  /**
   * LWC whitespace points `{ time }` only (no OHLC).
   * @param {{
   *   lastTimeSec: number,
   *   lastLogical: number,
   *   visibleTo?: number|null,
   *   rightOffset?: number|null,
   *   tf?: string,
   *   getIntervalMs?: (tf: string) => number,
   *   minBuffer?: number,
   *   maxBars?: number,
   * }} opts
   * @returns {{ time: number }[]}
   */
  function buildWhitespaceBars(opts) {
    const count = countFutureBars({
      lastLogical: opts?.lastLogical,
      visibleTo: opts?.visibleTo,
      rightOffset: opts?.rightOffset,
      minBuffer: opts?.minBuffer,
      maxBars: opts?.maxBars,
    });
    const times = buildFutureTimes({
      lastTimeSec: opts?.lastTimeSec,
      count,
      tf: opts?.tf,
      getIntervalMs: opts?.getIntervalMs,
    });
    return times.map((time) => ({ time }));
  }

  const DisplayTimeline = {
    DEFAULT_MAX_BARS,
    DEFAULT_MIN_BUFFER,
    boundaryKind,
    currentBarOpenMs,
    nextBarOpenMs,
    countFutureBars,
    buildFutureTimes,
    buildWhitespaceBars,
  };

  global.DisplayTimeline = DisplayTimeline;
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = DisplayTimeline;
  }
})(typeof window !== 'undefined' ? window : globalThis);
