/**
 * ScaleContribution — ADR-022 / Debt #68.
 * Dumb translator: DDR scaleContribution → LWC autoscaleInfoProvider.
 * ScaleController owns Auto/Manual/Log only; this owns "what Auto measures".
 */
(function (global) {
  'use strict';

  /**
   * @typedef {{ type: 'dynamic' } | { type: 'bounded', min: number, max: number } | { type: 'ignore' }} ScaleContribution
   */

  /**
   * @param {ScaleContribution|object|null|undefined} contribution
   * @returns {((() => ({ priceRange: { minValue: number, maxValue: number } })) | (() => null) | undefined)}
   */
  function createAutoscaleProvider(contribution) {
    const type = contribution && contribution.type;
    if (type === 'bounded') {
      const min = Number(contribution.min);
      const max = Number(contribution.max);
      if (!Number.isFinite(min) || !Number.isFinite(max) || max <= min) {
        return undefined;
      }
      return () => ({
        priceRange: { minValue: min, maxValue: max },
      });
    }
    if (type === 'ignore') {
      return () => null;
    }
    // dynamic / omit / unknown → ordinary LWC autoscale
    return undefined;
  }

  const api = {
    createAutoscaleProvider,
  };

  global.ScaleContribution = api;
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = api;
  }
})(typeof window !== 'undefined' ? window : globalThis);
