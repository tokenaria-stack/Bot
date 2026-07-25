/**
 * TimelineDecoration — ADR-027 Decoration Plane (sealed LWC owner).
 *
 * Owns: how display-only future timestamps become visible on the LWC time scale.
 * Does NOT own: future math (DisplayTimeline), candles, camera, crosshair, store/DDR.
 *
 * Public API: attach / refresh / dispose / applyCrosshairTime.
 * No series getters, no series.update, no event bus.
 * applyCrosshairTime — ChartAdapter-only seam so the bottom-axis owner can
 * paint LWC's native time label from synchronized {time} without exposing series.
 *
 * Private series title (for legend filters later): __timeline_decoration__
 */
(function (global) {
  'use strict';

  /** @private — not market data; legend/OHLC must ignore this title. */
  const SERIES_TITLE = '__timeline_decoration__';

  /**
   * @typedef {{ chart: object, series: object }} Attachment
   * @type {Attachment[]}
   */
  let attachments = [];

  function decorationSeriesOptions() {
    return {
      title: SERIES_TITLE,
      color: 'rgba(0,0,0,0)',
      lineWidth: 1,
      lineVisible: false,
      lastValueVisible: false,
      priceLineVisible: false,
      crosshairMarkerVisible: false,
      // Mandatory: contribute nothing to Y autoscale.
      autoscaleInfoProvider: () => null,
    };
  }

  /**
   * Normalize refresh payload to whitespace points `{ time }` only.
   * @param {{ times?: Array<number|{time:*}> }|null|undefined} payload
   * @returns {{ time: * }[]}
   */
  function normalizeWhitespace(payload) {
    if (!payload || typeof payload !== 'object') return [];
    const raw = payload.times;
    if (!Array.isArray(raw) || !raw.length) return [];
    const out = [];
    for (let i = 0; i < raw.length; i++) {
      const item = raw[i];
      if (item != null && typeof item === 'object' && Object.prototype.hasOwnProperty.call(item, 'time')) {
        if (item.time == null) continue;
        out.push({ time: item.time });
        continue;
      }
      if (item == null || item === '') continue;
      const n = Number(item);
      if (!Number.isFinite(n)) continue;
      out.push({ time: n });
    }
    return out;
  }

  function findAttachment(chart) {
    for (let i = 0; i < attachments.length; i++) {
      if (attachments[i].chart === chart) return attachments[i];
    }
    return null;
  }

  /**
   * Create private decoration series on a chart (idempotent per chart).
   * @param {object} chart LWC IChartApi
   * @returns {boolean}
   */
  function attach(chart) {
    if (!chart || typeof chart.addLineSeries !== 'function') return false;
    if (findAttachment(chart)) return true;
    let series;
    try {
      series = chart.addLineSeries(decorationSeriesOptions());
    } catch {
      return false;
    }
    if (!series || typeof series.setData !== 'function') return false;
    attachments.push({ chart, series });
    return true;
  }

  /**
   * Replace whitespace on all attached charts. Empty times clears the decoration.
   * @param {{ times?: Array<number|{time:*}> }} payload
   * @returns {boolean}
   */
  function refresh(payload) {
    if (!attachments.length) return false;
    const data = normalizeWhitespace(payload);
    let ok = false;
    for (let i = 0; i < attachments.length; i++) {
      const series = attachments[i].series;
      if (!series || typeof series.setData !== 'function') continue;
      try {
        series.setData(data);
        ok = true;
      } catch { /* disposed chart */ }
    }
    return ok;
  }

  /**
   * Remove private series from all attached charts.
   * @returns {boolean}
   */
  function dispose() {
    if (!attachments.length) return false;
    const prev = attachments;
    attachments = [];
    for (let i = 0; i < prev.length; i++) {
      const series = prev[i].series;
      try {
        if (series && typeof series.remove === 'function') series.remove();
      } catch { /* */ }
    }
    return true;
  }

  /**
   * Position LWC crosshair on this chart via the private decoration series.
   * Used so the bottom-axis owner can render the native time label when it is
   * not the hovered pane (series stays private — no getter).
   * @param {object} chart
   * @param {*} time LWC chart time (unix sec / BusinessDay)
   * @param {number} price local Y for setCrosshairPosition (mid-scale ok)
   * @returns {boolean}
   */
  function applyCrosshairTime(chart, time, price) {
    const att = findAttachment(chart);
    if (!att?.series || !chart || time == null) return false;
    if (typeof chart.setCrosshairPosition !== 'function') return false;
    const y = Number(price);
    if (!Number.isFinite(y)) return false;
    try {
      chart.setCrosshairPosition(y, time, att.series);
      return true;
    } catch {
      return false;
    }
  }

  /** @private tests */
  function _resetForTests() {
    attachments = [];
  }

  /** @private tests — inspect options used at create time */
  function _decorationSeriesOptionsForTests() {
    return decorationSeriesOptions();
  }

  const TimelineDecoration = {
    attach,
    refresh,
    dispose,
    applyCrosshairTime,
    // Test seams only (not for product callers).
    _resetForTests,
    _decorationSeriesOptionsForTests,
  };

  global.TimelineDecoration = TimelineDecoration;
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = TimelineDecoration;
  }
})(typeof window !== 'undefined' ? window : globalThis);
