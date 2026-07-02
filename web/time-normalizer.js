/**
 * Phase 20.1 — Universal TimeGrid (snap + merge) before ChartDataStore ingress.
 */
const TimeNormalizer = (() => {
  const MS_MINUTE = 60 * 1000;
  const MS_HOUR = 60 * MS_MINUTE;
  const MS_DAY = 24 * MS_HOUR;
  const MS_WEEK = 7 * MS_DAY;
  const MS_MONTH = 30 * MS_DAY;

  function getIntervalMs(tf) {
    const raw = String(tf || '1m').trim();
    if (!raw) return MS_MINUTE;

    const tickMatch = /^(\d+)ticks?$/i.exec(raw);
    if (tickMatch) {
      const ticks = Number(tickMatch[1]);
      return Number.isFinite(ticks) && ticks > 0 ? ticks : 1;
    }

    const m = /^(\d+)([smhdwM])$/.exec(raw);
    if (!m) return MS_MINUTE;

    const val = Number(m[1]);
    const unit = m[2];
    if (!Number.isFinite(val) || val <= 0) return MS_MINUTE;

    switch (unit) {
      case 's': return val * 1000;
      case 'm': return val * MS_MINUTE;
      case 'h': return val * MS_HOUR;
      case 'd': return val * MS_DAY;
      case 'w': return val * MS_WEEK;
      case 'M': return val * MS_MONTH;
      default: return MS_MINUTE;
    }
  }

  function snapToGrid(timeMs, tf) {
    const n = Number(timeMs);
    if (!Number.isFinite(n) || n <= 0) return null;
    const intervalMs = getIntervalMs(tf);
    if (!Number.isFinite(intervalMs) || intervalMs <= 0) return Math.floor(n);
    return Math.floor(n / intervalMs) * intervalMs;
  }

  function mergeCandles(existing, incoming) {
    if (!existing) return { ...incoming };
    if (!incoming) return { ...existing };

    const timeMs = existing.timeMs ?? incoming.timeMs;
    const chartSec = typeof ChartDataStore !== 'undefined' && ChartDataStore.msToChartSec
      ? ChartDataStore.msToChartSec(timeMs)
      : Math.floor(timeMs / 1000);

    const existingVol = Number(existing.volume);
    const incomingVol = Number(incoming.volume);
    const volume = (Number.isFinite(existingVol) ? existingVol : 0)
      + (Number.isFinite(incomingVol) ? incomingVol : 0);

    return {
      ...existing,
      ...incoming,
      timeMs,
      time: chartSec,
      open: existing.open,
      high: Math.max(existing.high, incoming.high),
      low: Math.min(existing.low, incoming.low),
      close: incoming.close,
      volume,
    };
  }

  return {
    getIntervalMs,
    snapToGrid,
    mergeCandles,
  };
})();

window.TimeNormalizer = TimeNormalizer;
