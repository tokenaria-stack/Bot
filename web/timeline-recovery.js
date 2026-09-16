/**
 * TimelineRecovery — ADR-018 + TIMELINE-RECOVERY-STATE-1.
 * FSM: LIVE | HEALING only. snapshotRequired is a local distrust bit, not a third state.
 * Watchdog arms only while Master is unpublishable (HEALING).
 *
 * @typedef {object} TimelineRecoveryOptions
 * @property {() => void} [onEnter] HEALING enter (buffer ticks)
 * @property {(ctx: { generation: number, reason: string }) => void} [onReplaceSnapshot]
 * @property {() => void} [onRetry] watchdog Retry — same recovery contract, not fake LIVE
 * @property {number} [watchdogMs] default 25000
 */
(function (global) {
  'use strict';

  const STATE_LIVE = 'LIVE';
  const STATE_HEALING = 'HEALING';
  const DEFAULT_WATCHDOG_MS = 25000;

  function logInfo(...args) {
    try { console.log(...args); } catch (_) { /* node/jsc without console */ }
  }
  function logWarn(...args) {
    try { console.warn(...args); } catch (_) { /* */ }
  }
  function logError(...args) {
    try { console.error(...args); } catch (_) { /* */ }
  }

  function resolveBadgeEl() {
    return typeof document !== 'undefined'
      ? document.getElementById('timeline-sync-badge')
      : null;
  }

  function setBadge(visible, text, stalled) {
    const el = resolveBadgeEl();
    if (!el) return;
    if (!visible) {
      el.hidden = true;
      el.classList.remove('is-stalled');
      el.textContent = '';
      return;
    }
    el.hidden = false;
    el.classList.toggle('is-stalled', !!stalled);
    el.textContent = text || 'Synchronizing live data…';
  }

  /**
   * @param {TimelineRecoveryOptions} [options]
   */
  function create(options = {}) {
    const onEnter = typeof options.onEnter === 'function' ? options.onEnter : null;
    const onReplaceSnapshot = typeof options.onReplaceSnapshot === 'function'
      ? options.onReplaceSnapshot
      : null;
    const onRetry = typeof options.onRetry === 'function' ? options.onRetry : null;
    const watchdogMs = Number.isFinite(options.watchdogMs) && options.watchdogMs > 0
      ? Math.floor(options.watchdogMs)
      : DEFAULT_WATCHDOG_MS;

    let state = STATE_LIVE;
    let enteredAt = 0;
    let watchdogTimer = null;
    let lastReason = '';
    let snapshotRequired = false;
    let recoveryGeneration = 0;

    function clearWatchdog() {
      if (watchdogTimer != null) {
        clearTimeout(watchdogTimer);
        watchdogTimer = null;
      }
    }

    function clearHealingUi() {
      clearWatchdog();
      state = STATE_LIVE;
      enteredAt = 0;
      setBadge(false);
    }

    function retry() {
      snapshotRequired = true;
      lastReason = 'manual_retry';
      recoveryGeneration += 1;
      try { onRetry?.(); } catch (err) {
        logError('[Timeline] onRetry failed:', err);
      }
    }

    function armWatchdog() {
      clearWatchdog();
      watchdogTimer = setTimeout(() => {
        watchdogTimer = null;
        if (state !== STATE_HEALING) return;
        logWarn('[Timeline] WATCHDOG', { reason: lastReason, watchdogMs });
        setBadge(true, 'Reconnect stalled — Retry', true);
        const el = resolveBadgeEl();
        if (el) {
          el.setAttribute('role', 'button');
          el.tabIndex = 0;
          const retry = () => {
            el.removeEventListener('click', retry);
            el.removeEventListener('keydown', onKey);
            el.removeAttribute('role');
            el.removeAttribute('tabindex');
            retry();
          };
          const onKey = (ev) => {
            if (ev.key === 'Enter' || ev.key === ' ') {
              ev.preventDefault();
              retry();
            }
          };
          el.addEventListener('click', retry);
          el.addEventListener('keydown', onKey);
        }
      }, watchdogMs);
    }

    function markSnapshotRequired(reason) {
      const why = String(reason || lastReason || 'unknown');
      snapshotRequired = true;
      lastReason = why;
      recoveryGeneration += 1;
      return recoveryGeneration;
    }

    function enter(reason) {
      const why = String(reason || 'unknown');
      snapshotRequired = true;
      lastReason = why;
      if (state === STATE_HEALING) {
        logInfo('[Timeline] duplicate enter ignored', { reason: why });
        return false;
      }
      state = STATE_HEALING;
      enteredAt = Date.now();
      logInfo('[Timeline] ENTER healing', { reason: why });
      setBadge(true, 'Synchronizing live data…', false);
      armWatchdog();
      try { onEnter?.(); } catch (err) {
        logError('[Timeline] onEnter failed:', err);
      }
      return true;
    }

    /**
     * Current Master bool. Returns action for the dashboard owner.
     * @returns {{ action: 'observe'|'replace'|'heal', generation: number, reason: string, snapshotRequired: boolean }}
     */
    function onTimelineState(publishable) {
      const ok = publishable === true;
      if (!ok) {
        snapshotRequired = true;
        enter(lastReason || 'master_unpublishable');
        return {
          action: 'heal',
          generation: recoveryGeneration,
          reason: lastReason,
          snapshotRequired: true,
        };
      }
      if (!snapshotRequired) {
        return {
          action: 'observe',
          generation: recoveryGeneration,
          reason: lastReason,
          snapshotRequired: false,
        };
      }
      if (state === STATE_HEALING) {
        clearHealingUi();
        logInfo('[Timeline] EXIT healing (snapshot replace; Master publishable)');
      }
      return {
        action: 'replace',
        generation: recoveryGeneration,
        reason: lastReason,
        snapshotRequired: true,
      };
    }

    function snapshotCommitted(generation) {
      if (generation !== recoveryGeneration) return false;
      snapshotRequired = false;
      if (state === STATE_HEALING) {
        clearHealingUi();
        logInfo('[Timeline] EXIT healing');
      }
      return true;
    }

    function requestReplace(generation) {
      try {
        onReplaceSnapshot?.({ generation, reason: lastReason });
      } catch (err) {
        logError('[Timeline] onReplaceSnapshot failed:', err);
      }
    }

    function isHealing() {
      return state === STATE_HEALING;
    }

    function isSnapshotRequired() {
      return snapshotRequired;
    }

    function currentGeneration() {
      return recoveryGeneration;
    }

    function lastRecoveryReason() {
      return lastReason;
    }

    /** Test / diagnostics */
    function _debugState() {
      return {
        state,
        enteredAt,
        lastReason,
        snapshotRequired,
        recoveryGeneration,
        watchdogArmed: watchdogTimer != null,
      };
    }

    return {
      enter,
      markSnapshotRequired,
      onTimelineState,
      snapshotCommitted,
      requestReplace,
      isHealing,
      isSnapshotRequired,
      currentGeneration,
      lastRecoveryReason,
      retry,
      _debugState,
      STATE_LIVE,
      STATE_HEALING,
    };
  }

  const TimelineRecovery = { create, STATE_LIVE, STATE_HEALING, DEFAULT_WATCHDOG_MS };
  global.TimelineRecovery = TimelineRecovery;
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = TimelineRecovery;
  }
})(typeof window !== 'undefined' ? window : globalThis);
