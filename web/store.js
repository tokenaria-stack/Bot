/**
 * ChartDataStore — ms-keyed candle/osc/annotation store.
 * Runtime dependency: global normalizeCandle() and normalizeOscPoint() from mappers.js.
 */
class ChartDataStore {
  constructor(context) {
    this.context = context;
    this.candles = new Map(); // Map<ms, Candle>
    this.osc = new Map(); // Map<ms, OscPoint>
    // Map<ms, Map<text, Annotation>>
    this.annotations = new Map();
    this._dirtyMs = null;
    this._dirtyIsNewBar = false;
    this._dirtyAnnotations = false;
  }

  static toMs(t) {
    const n = Number(t);
    if (!Number.isFinite(n) || n <= 0) return null;
    return n < 1e12 ? Math.floor(n * 1000) : Math.floor(n);
  }

  static msToChartSec(ms) {
    return Math.floor(ms / 1000);
  }

  clear() {
    this.candles.clear();
    this.osc.clear();
    this.annotations.clear();
    this._resetDirtyState();
  }

  _resetDirtyState() {
    this._dirtyMs = null;
    this._dirtyIsNewBar = false;
    this._dirtyAnnotations = false;
  }

  _markDirtyMs(ms, isNewBar = false) {
    if (this._dirtyMs == null) {
      this._dirtyMs = ms;
      this._dirtyIsNewBar = isNewBar;
    } else if (this._dirtyMs === ms) {
      if (isNewBar) this._dirtyIsNewBar = true;
    } else {
      this._dirtyMs = ms;
      this._dirtyIsNewBar = isNewBar;
    }
  }

  _ingestAnnotationKey(ms, text, extra = {}) {
    const key = String(text ?? '').trim().substring(0, 2);
    if (!key) return;
    if (!this.annotations.has(ms)) this.annotations.set(ms, new Map());
    const textMap = this.annotations.get(ms);
    const isNew = !textMap.has(key);
    textMap.set(key, { ...extra, timeMs: ms, text: key });
    if (isNew) this._dirtyAnnotations = true;
  }

  candleCount() {
    return this.candles.size;
  }

  sortedCandleTimesMs() {
    return Array.from(this.candles.keys()).sort((a, b) => a - b);
  }

  firstCandleTimeSec() {
    const times = this.sortedCandleTimesMs();
    return times.length ? ChartDataStore.msToChartSec(times[0]) : null;
  }

  lastCandleTimeSec() {
    const times = this.sortedCandleTimesMs();
    return times.length ? ChartDataStore.msToChartSec(times[times.length - 1]) : null;
  }

  lastCandleChartSec() {
    const times = this.sortedCandleTimesMs();
    if (!times.length) return null;
    const ms = times[times.length - 1];
    const bar = this.candles.get(ms);
    if (!bar) return null;
    return { ...bar, time: ChartDataStore.msToChartSec(ms) };
  }

  annotationCount() {
    let count = 0;
    this.annotations.forEach((textMap) => { count += textMap.size; });
    return count;
  }

  _ingestAnnotations(annArray, anchorMs = null) {
    if (!Array.isArray(annArray)) return;
    annArray.forEach((a) => {
      const ms = ChartDataStore.toMs(a.time ?? a.Time);
      if (!ms) return;
      if (anchorMs != null && ms >= anchorMs) return;
      const text = (a.text ?? a.label ?? a.Label ?? '').trim().substring(0, 2);
      if (!text) return;

      if (!this.annotations.has(ms)) this.annotations.set(ms, new Map());
      const textMap = this.annotations.get(ms);
      const isNew = !textMap.has(text);
      textMap.set(text, { ...a, timeMs: ms, text });
      if (isNew) this._dirtyAnnotations = true;
    });
  }

  _ingestCandle(c, { allowOverwrite = true, anchorMs = null } = {}) {
    const bar = normalizeCandle(c);
    if (!bar) return;
    const ms = ChartDataStore.toMs(bar.time);
    if (!ms) return;
    if (anchorMs != null && ms >= anchorMs) return;
    if (!allowOverwrite && this.candles.has(ms)) return;
    this.candles.set(ms, { ...bar, timeMs: ms });
  }

  _ingestOsc(o, { allowOverwrite = true, anchorMs = null } = {}) {
    const norm = normalizeOscPoint(o);
    if (!norm) return;
    const ms = ChartDataStore.toMs(norm.time);
    if (!ms) return;
    if (anchorMs != null && ms >= anchorMs) return;
    if (!allowOverwrite && this.osc.has(ms)) return;
    this.osc.set(ms, { ...norm, timeMs: ms });
  }

  replaceFromServer(payload) {
    this.candles.clear();
    this.osc.clear();
    this.annotations.clear();

    (payload.candles || []).forEach((c) => {
      this._ingestCandle(c, { allowOverwrite: true });
    });
    (payload.oscillators || []).forEach((o) => {
      this._ingestOsc(o, { allowOverwrite: true });
    });
    this._ingestAnnotations(payload.annotations);
    this._resetDirtyState();
  }

  prependHistory(payload) {
    let added = 0;
    let rejected = 0;
    const anchorMs = this.sortedCandleTimesMs()[0] ?? null;

    (payload.candles || []).forEach((c) => {
      const bar = normalizeCandle(c);
      if (!bar) {
        rejected += 1;
        return;
      }
      const ms = ChartDataStore.toMs(bar.time);
      if (!ms) {
        rejected += 1;
        return;
      }
      if (anchorMs != null && ms >= anchorMs) {
        rejected += 1;
      } else if (!this.candles.has(ms)) {
        this.candles.set(ms, { ...bar, timeMs: ms });
        added += 1;
      } else {
        rejected += 1;
      }
    });

    (payload.oscillators || []).forEach((o) => {
      const norm = normalizeOscPoint(o);
      if (!norm) {
        rejected += 1;
        return;
      }
      const ms = ChartDataStore.toMs(norm.time);
      if (!ms) {
        rejected += 1;
        return;
      }
      if (anchorMs != null && ms >= anchorMs) {
        rejected += 1;
      } else if (!this.osc.has(ms)) {
        this.osc.set(ms, { ...norm, timeMs: ms });
      } else {
        rejected += 1;
      }
    });

    this._ingestAnnotations(payload.annotations, anchorMs);
    this._resetDirtyState();

    return { added, rejected, anchorMs };
  }

  upsertCandle(c) {
    const bar = normalizeCandle(c);
    if (!bar) return null;
    const ms = ChartDataStore.toMs(bar.time);
    if (!ms) return null;
    const isNewBar = !this.candles.has(ms);
    this.candles.set(ms, { ...bar, timeMs: ms });
    this._markDirtyMs(ms, isNewBar);
    return bar;
  }

  upsertOscPoint(o) {
    const norm = normalizeOscPoint(o);
    if (!norm) return null;
    const ms = ChartDataStore.toMs(norm.time);
    if (!ms) return null;
    this.osc.set(ms, { ...norm, timeMs: ms });
    this._markDirtyMs(ms, false);
    if (norm.marker) {
      this._ingestAnnotationKey(ms, norm.marker, { pane: 'rsx' });
    }
    return norm;
  }

  replaceOscAndAnnotations(payload) {
    if (Array.isArray(payload.annotations)) {
      this.annotations.clear();
      this._ingestAnnotations(payload.annotations);
    }
    (payload.oscillators || []).forEach((o) => {
      this._ingestOsc(o, { allowOverwrite: true });
    });
    this._resetDirtyState();
  }

  getLatestDeltaForChart() {
    if (this._dirtyMs == null && !this._dirtyAnnotations) return null;

    const ms = this._dirtyMs;
    const candle = ms != null ? this.candles.get(ms) : null;
    const osc = ms != null ? this.osc.get(ms) : null;

    let fullAnnotations = null;
    if (this._dirtyAnnotations) {
      fullAnnotations = [];
      Array.from(this.annotations.values()).forEach((textMap) => {
        Array.from(textMap.values()).forEach((a) => {
          fullAnnotations.push({ ...a, time: ChartDataStore.msToChartSec(a.timeMs) });
        });
      });
      fullAnnotations.sort((a, b) => a.time - b.time);
    }

    const delta = {
      candle: candle ? { ...candle, time: ChartDataStore.msToChartSec(ms) } : null,
      osc: osc ? { ...osc, time: ChartDataStore.msToChartSec(ms) } : null,
      fullAnnotations,
      isNewBar: this._dirtyIsNewBar,
    };

    this._dirtyMs = null;
    this._dirtyIsNewBar = false;
    this._dirtyAnnotations = false;
    return delta;
  }

  getForLightweightCharts() {
    const sortedCandles = Array.from(this.candles.values()).sort((a, b) => a.timeMs - b.timeMs);
    const chartCandles = sortedCandles.map((c) => ({
      ...c,
      time: ChartDataStore.msToChartSec(c.timeMs),
    }));

    const chartOsc = sortedCandles.map((c) => {
      const oscPoint = this.osc.get(c.timeMs);
      if (oscPoint) {
        return { ...oscPoint, time: ChartDataStore.msToChartSec(c.timeMs) };
      }
      return { time: ChartDataStore.msToChartSec(c.timeMs) };
    });

    const chartAnnotations = [];
    Array.from(this.annotations.values()).forEach((textMap) => {
      Array.from(textMap.values()).forEach((a) => {
        chartAnnotations.push({ ...a, time: ChartDataStore.msToChartSec(a.timeMs) });
      });
    });
    chartAnnotations.sort((a, b) => a.time - b.time);

    return { candles: chartCandles, osc: chartOsc, annotations: chartAnnotations };
  }
}

if (typeof window !== 'undefined') {
  window.ChartDataStore = ChartDataStore;
}
