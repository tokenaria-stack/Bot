/**
 * RSX-STROKE-1 — Pine OB/OS stroke color for line_rsx only.
 * Not slope/50. Not a store column.
 */
(function (global) {
  'use strict';

  const GREEN = '#0ebb23';
  const RED = '#ff0000';
  const MID = '#512DA8';
  const OB = 70;
  const OS = 30;

  /**
   * @param {*} value RSX scalar
   * @returns {string|undefined} hex, or undefined when invalid
   */
  function rsxStrokeColor(value) {
    if (typeof value !== 'number' || !Number.isFinite(value)) return undefined;
    if (value > OB) return GREEN;
    if (value < OS) return RED;
    return MID;
  }

  const api = {
    rsxStrokeColor,
    GREEN,
    RED,
    MID,
    OB,
    OS,
  };

  global.RsxStrokeColor = api;
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = api;
  }
})(typeof window !== 'undefined' ? window : globalThis);
