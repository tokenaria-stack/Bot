/**
 * RulerMetrics — ADR-025 pure measurement numbers (not DOM, not LWC).
 * Bars ALWAYS from logical indexes — never Δtime / timeframe.
 */
(function (global) {
  'use strict';

  /**
   * @typedef {{ logical: number, price: number, time?: *|null }} RulerAnchor
   */

  /**
   * @param {RulerAnchor} a
   * @param {RulerAnchor} b
   * @param {{ intervalMs?: number, minMove?: number }} [opts]
   * @returns {{
   *   deltaPrice: number,
   *   deltaPercent: number,
   *   bars: number,
   *   durationMs: number|null,
   *   durationEstimated: boolean,
   *   ticks: number|null,
   * }}
   */
  function compute(a, b, opts) {
    const optsSafe = opts && typeof opts === 'object' ? opts : {};
    const logicalA = Number(a?.logical);
    const logicalB = Number(b?.logical);
    const priceA = Number(a?.price);
    const priceB = Number(b?.price);

    const bars = (Number.isFinite(logicalA) && Number.isFinite(logicalB))
      ? Math.abs(logicalB - logicalA)
      : 0;

    const deltaPrice = (Number.isFinite(priceA) && Number.isFinite(priceB))
      ? (priceB - priceA)
      : 0;

    const deltaPercent = (Number.isFinite(priceA) && priceA !== 0 && Number.isFinite(deltaPrice))
      ? (deltaPrice / priceA) * 100
      : 0;

    const timeA = toUnixSec(a?.time);
    const timeB = toUnixSec(b?.time);
    let durationMs = null;
    let durationEstimated = false;
    if (timeA != null && timeB != null) {
      durationMs = Math.abs(timeB - timeA) * 1000;
      durationEstimated = false;
    } else {
      const intervalMs = Number(optsSafe.intervalMs);
      if (Number.isFinite(intervalMs) && intervalMs > 0 && bars > 0) {
        durationMs = bars * intervalMs;
        durationEstimated = true;
      }
    }

    const minMove = Number(optsSafe.minMove);
    let ticks = null;
    if (Number.isFinite(minMove) && minMove > 0 && Number.isFinite(deltaPrice)) {
      ticks = Math.round(Math.abs(deltaPrice) / minMove);
    }

    return {
      deltaPrice,
      deltaPercent,
      bars,
      durationMs,
      durationEstimated,
      ticks,
    };
  }

  /** @param {*} t @returns {number|null} unix seconds */
  function toUnixSec(t) {
    if (t == null) return null;
    if (typeof t === 'number' && Number.isFinite(t)) {
      return t > 1e12 ? Math.floor(t / 1000) : Math.floor(t);
    }
    if (typeof t === 'object' && t.timestamp != null) {
      return toUnixSec(t.timestamp);
    }
    // BusinessDay { year, month, day } — not usable as duration without calendar; skip.
    return null;
  }

  /**
   * Compact duration string like TV: 45m, 1h 30m, 2d.
   * @param {number|null} ms
   * @returns {string}
   */
  function formatDuration(ms) {
    if (!Number.isFinite(ms) || ms < 0) return '—';
    const totalSec = Math.round(ms / 1000);
    if (totalSec < 60) return `${totalSec}s`;
    const totalMin = Math.round(totalSec / 60);
    if (totalMin < 60) return `${totalMin}m`;
    const h = Math.floor(totalMin / 60);
    const m = totalMin % 60;
    if (h < 48) return m ? `${h}h ${m}m` : `${h}h`;
    const d = Math.floor(h / 24);
    const rh = h % 24;
    return rh ? `${d}d ${rh}h` : `${d}d`;
  }

  /**
   * Price delta display (compact).
   * @param {number} v
   */
  function formatPriceDelta(v) {
    if (!Number.isFinite(v)) return '—';
    const abs = Math.abs(v);
    let body;
    if (abs >= 1000) body = abs.toLocaleString(undefined, { maximumFractionDigits: 1 });
    else if (abs >= 1) body = abs.toLocaleString(undefined, { minimumFractionDigits: 1, maximumFractionDigits: 2 });
    else body = abs.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 6 });
    if (v > 0) return `+${body}`;
    if (v < 0) return `-${body}`;
    return body;
  }

  /**
   * Tooltip lines (no Volume). ChartAdapter owns DOM.
   * @param {ReturnType<typeof compute>} metrics
   * @returns {{ line1: string, line2: string }}
   */
  function tooltipLines(metrics) {
    const m = metrics || {};
    const sign = Number(m.deltaPrice) >= 0 ? '+' : '';
    const pct = Number.isFinite(m.deltaPercent)
      ? `${sign}${m.deltaPercent.toFixed(2)}%`
      : '—';
    const price = formatPriceDelta(m.deltaPrice);
    const ticksPart = (m.ticks != null && Number.isFinite(m.ticks))
      ? ` ${m.ticks}`
      : '';
    const line1 = `${price} (${pct})${ticksPart}`;

    const bars = Number.isFinite(m.bars) ? Math.round(m.bars) : 0;
    const barsLabel = bars === 1 ? '1 bar' : `${bars} bars`;
    const dur = formatDuration(m.durationMs);
    const line2 = `${barsLabel}, ${dur}`;

    return { line1, line2 };
  }

  const RulerMetrics = {
    compute,
    formatDuration,
    formatPriceDelta,
    tooltipLines,
    toUnixSec,
  };

  global.RulerMetrics = RulerMetrics;
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = RulerMetrics;
  }
})(typeof window !== 'undefined' ? window : globalThis);
