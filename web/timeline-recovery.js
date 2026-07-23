/**
 * TimelineRecovery — ADR-018 FE owner of timeline recovery UI lifecycle.
 * States: LIVE | HEALING. Watchdog is diagnostic only (does not reset on duplicate enter).
 *
 * @typedef {object} TimelineRecoveryOptions
 * @property {() => void} [onEnter] called once when entering HEALING (e.g. start tick buffer)
 * @property {() => void} [onRecovered] called on publishable / watchdog Retry (e.g. loadDashboard)
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
    const onRecovered = typeof options.onRecovered === 'function' ? options.onRecovered : null;
    const watchdogMs = Number.isFinite(options.watchdogMs) && options.watchdogMs > 0
      ? Math.floor(options.watchdogMs)
      : DEFAULT_WATCHDOG_MS;

    let state = STATE_LIVE;
    let enteredAt = 0;
    let watchdogTimer = null;
    let lastReason = '';

    function clearWatchdog() {
      if (watchdogTimer != null) {
        clearTimeout(watchdogTimer);
        watchdogTimer = null;
      }
    }

    function exitToLive(logLabel) {
      const elapsedMs = enteredAt ? Date.now() - enteredAt : 0;
      clearWatchdog();
      state = STATE_LIVE;
      enteredAt = 0;
      setBadge(false);
      if (logLabel) {
        logInfo(`[Timeline] ${logLabel}`, { elapsedSec: (elapsedMs / 1000).toFixed(1) });
      }
      logInfo('[Timeline] EXIT healing');
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
            // Exit HEALING then recover (avoid sticky state if reload fails).
            exitToLive('WATCHDOG retry');
            try { onRecovered?.(); } catch (err) {
              logError('[Timeline] onRecovered failed:', err);
            }
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

    function enter(reason) {
      const why = String(reason || 'unknown');
      if (state === STATE_HEALING) {
        logInfo('[Timeline] duplicate enter ignored', { reason: why });
        return false;
      }
      state = STATE_HEALING;
      enteredAt = Date.now();
      lastReason = why;
      logInfo('[Timeline] ENTER healing', { reason: why });
      setBadge(true, 'Synchronizing live data…', false);
      armWatchdog();
      try { onEnter?.(); } catch (err) {
        logError('[Timeline] onEnter failed:', err);
      }
      return true;
    }

    function publishable() {
      if (state !== STATE_HEALING) {
        logWarn('[Timeline] publishable ignored (not healing)');
        return false;
      }
      const elapsedMs = Date.now() - enteredAt;
      logInfo('[Timeline] PUBLISHABLE', { afterSec: (elapsedMs / 1000).toFixed(1), reason: lastReason });
      exitToLive(null);
      try { onRecovered?.(); } catch (err) {
        logError('[Timeline] onRecovered failed:', err);
      }
      return true;
    }

    function isHealing() {
      return state === STATE_HEALING;
    }

    /** Test / diagnostics */
    function _debugState() {
      return { state, enteredAt, lastReason, watchdogArmed: watchdogTimer != null };
    }

    return {
      enter,
      publishable,
      isHealing,
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
