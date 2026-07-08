/** Pure data transformation — no DOM, no Store. */

function normalizeStrategyThresholds(raw) {
  const long = Number(raw?.long ?? raw?.longThreshold);
  const short = Number(raw?.short ?? raw?.shortThreshold);
  return {
    long: Number.isFinite(long) && long > 0 ? long : DEFAULT_STRATEGY_THRESHOLDS.long,
    short: Number.isFinite(short) && short > 0 ? short : DEFAULT_STRATEGY_THRESHOLDS.short,
  };
}

function normalizeStrategyMatrix(raw) {
  if (!raw || typeof raw !== 'object') return { ...SCORING_MATRIX_DEFAULTS };
  return { ...SCORING_MATRIX_DEFAULTS, ...raw };
}

function normalizeTf(tf) {
  return resolveTf(tf);
}

/** Client-side TF resolver (mirrors server/timeframes.go aliases). */
function resolveTf(tf) {
  const raw = String(tf || '').trim();
  if (!raw) return '';

  const lower = raw.toLowerCase();
  const alias = {
    '1min': '1m', '1minute': '1m', m: '1m',
    '1hour': '1h', h: '1h',
    d: '1d', day: '1d',
    w: '1w', week: '1w',
  };
  if (alias[lower]) return alias[lower];
  if (TF_DISPLAY[raw]) return raw;

  const all = Object.values(TF_MENU).flat();
  const hit = all.find((item) => item.id === raw || item.id.toLowerCase() === lower);
  if (hit) return hit.id;

  if (/^\d+s$/i.test(raw)) return lower;
  if (/^\d+tick(s)?$/i.test(lower)) return lower === '1tick' ? '1tick' : lower;
  if (/^\d+M$/.test(raw)) return raw;
  if (/^\d+m$/i.test(raw) && raw !== 'M') return lower;
  if (/^\d+h$/i.test(lower)) return lower;
  if (/^\d+d$/i.test(lower)) return lower;

  return raw;
}

function rsxShowPivotsFrom(settings, fallback = true) {
  if (typeof settings?.show_pivots === 'boolean') return settings.show_pivots;
  if (typeof settings?.showPivots === 'boolean') return settings.showPivots;
  return fallback;
}

function clampRsxLength(val) {
  const n = parseInt(val, 10);
  if (!Number.isFinite(n)) return DEFAULT_RSX_LENGTH;
  return Math.min(MAX_RSX_LENGTH, Math.max(MIN_RSX_LENGTH, n));
}

function clampRsxDivLookback(val) {
  const n = parseInt(val, 10);
  if (!Number.isFinite(n)) return DEFAULT_RSX_LOOKBACK;
  return Math.min(MAX_RSX_DIV_LOOKBACK, Math.max(MIN_RSX_DIV_LOOKBACK, n));
}

function clampRsxSignalLength(val) {
  const n = parseInt(val, 10);
  if (!Number.isFinite(n)) return DEFAULT_RSX_SIGNAL_LENGTH;
  return Math.min(MAX_RSX_SIGNAL_LENGTH, Math.max(MIN_RSX_SIGNAL_LENGTH, n));
}

function clampRsxPivotRadius(val) {
  const n = parseInt(val, 10);
  if (!Number.isFinite(n)) return 2;
  return Math.min(10, Math.max(1, n));
}

function coerceRsxSettingsForAPI(settings, defaults = defaultRsxSettings()) {
  const pivotRadius = Number(clampRsxPivotRadius(Number(settings?.pivot_radius ?? settings?.pivotRadius)));
  const minPriceDelta = Number(settings?.min_price_delta_ratio ?? settings?.minPriceDeltaRatio ?? 0);
  const minOscDelta = Number(settings?.min_osc_delta ?? settings?.minOscDelta ?? 0);
  return {
    length: Number(clampRsxLength(Number(settings?.length))),
    div_lookback: Number(clampRsxDivLookback(Number(settings?.div_lookback))),
    signal_length: Number(clampRsxSignalLength(Number(settings?.signal_length))),
    source: settings?.source === 'hlc3' ? 'hlc3' : 'close',
    pivot_radius: pivotRadius,
    pivotRadius,
    div_method: settings?.div_method === 'fractal' ? 'fractal' : 'tv',
    min_price_delta_ratio: Number.isFinite(minPriceDelta) ? minPriceDelta : 0,
    min_osc_delta: Number.isFinite(minOscDelta) ? minOscDelta : 0,
    minPriceDeltaRatio: Number.isFinite(minPriceDelta) ? minPriceDelta : 0,
    minOscDelta: Number.isFinite(minOscDelta) ? minOscDelta : 0,
    show_pivots: rsxShowPivotsFrom(settings, rsxShowPivotsFrom(defaults, true)),
    showPivots: rsxShowPivotsFrom(settings, rsxShowPivotsFrom(defaults, true)),
  };
}

function normalizeRsxSettingsFromAPI(raw, defaults = defaultRsxSettings()) {
  if (!raw || typeof raw !== 'object') return coerceRsxSettingsForAPI(defaults, defaults);
  const num = (v, fallback) => {
    const n = Number(v);
    return Number.isFinite(n) ? n : fallback;
  };
  const divMethod = raw.div_method || raw.divMethod || defaults.div_method;
  return coerceRsxSettingsForAPI({
    length: num(raw.length ?? raw.rsxLength, defaults.length),
    div_lookback: num(raw.div_lookback ?? raw.divLookback, defaults.div_lookback),
    signal_length: num(raw.signal_length ?? raw.signalLineLength, defaults.signal_length),
    source: raw.source || raw.rsxSource || defaults.source,
    pivot_radius: num(raw.pivot_radius ?? raw.pivotRadius, defaults.pivot_radius),
    div_method: divMethod,
    min_price_delta_ratio: num(raw.min_price_delta_ratio ?? raw.minPriceDeltaRatio, defaults.min_price_delta_ratio),
    min_osc_delta: num(raw.min_osc_delta ?? raw.minOscDelta, defaults.min_osc_delta),
    show_pivots: rsxShowPivotsFrom(raw, rsxShowPivotsFrom(defaults, true)),
  }, defaults);
}

function chartTime(raw) {
  const t = Number(raw);
  if (!Number.isFinite(t)) return null;
  return t >= 1e12 ? Math.floor(t / 1000) : Math.floor(t);
}

function isWarmupOscValue(value) {
  return value == null || !Number.isFinite(value) || value === 0 || value === 50;
}

function isValidOHLC(open, high, low, close) {
  return (
    Number.isFinite(open) && Number.isFinite(high)
    && Number.isFinite(low) && Number.isFinite(close)
    && open > 0 && high > 0 && low > 0 && close > 0
  );
}

function normalizeCandle(c) {
  if (!c) return null;
  const time = chartTime(c?.time ?? c?.Time);
  const open = Number(c.open ?? c.Open);
  const high = Number(c.high ?? c.High);
  const low = Number(c.low ?? c.Low);
  const close = Number(c.close ?? c.Close);
  if (time == null) return null;
  if (!isValidOHLC(open, high, low, close)) return null;
  return {
    time,
    open,
    high,
    low,
    close,
    volume: Number.isFinite(Number(c.volume ?? c.Volume)) ? Number(c.volume ?? c.Volume) : 0,
  };
}

function toCandles(raw) {
  return (raw || [])
    .map(normalizeCandle)
    .filter(Boolean);
}

function toLineClose(candles) {
  return candles.map((c) => ({ time: Number(c.time), value: Number(c.close) }));
}

function toLine(raw, key) {
  return (raw || []).map((p) => {
    const time = chartTime(p?.time);
    if (time == null) return { time: 0 };
    const value = Number(p[key]);
    if (isWarmupOscValue(value)) return { time: Number(time) };
    return { time: Number(time), value };
  });
}

function normalizeOscPoint(p) {
  const time = chartTime(p?.time ?? p?.Time);
  if (time == null) return null;
  const res = { time: Number(time) };
  const fields = [
  ['jurik', p.jurik],
  ['rsx', p.rsx],
  ['rsx_signal', p.rsx_signal ?? p.rsxSignal],
  ['rsiPrice', p.rsiPrice],
  ['rsiHl2', p.rsiHl2],
  ['rsiVolFast', p.rsiVolFast],
  ['rsiVolSlow', p.rsiVolSlow],
  ['volCrossMarker', p.volCrossMarker],
  ['color', p.color],
  ['marker', p.marker],
  ['volumeSpikeUp', p.volumeSpikeUp],
  ['volumeSpikeDown', p.volumeSpikeDown],
  ];
  for (const [key, value] of fields) {
    if (value !== undefined) res[key] = value;
  }
  return res;
}

function chartPointsToOsc(points) {
  return (points || [])
    .map((p) => normalizeOscPoint({
      time: chartTime(p.time ?? p.Time),
      jurik: p.jurik ?? p.Jurik,
      rsx: p.rsx ?? p.Rsx,
      rsx_signal: p.rsx_signal ?? p.rsxSignal ?? p.RsxSignal,
      rsiPrice: p.rsiPrice ?? p.RsiPrice,
      rsiHl2: p.rsiHl2 ?? p.RsiHl2,
      rsiVolFast: p.rsiVolFast ?? p.wozduh_up ?? p.RsiVolFast,
      rsiVolSlow: p.rsiVolSlow ?? p.wozduh_down ?? p.RsiVolSlow,
      volCrossMarker: p.volCrossMarker ?? p.VolCrossMarker,
      color: p.color ?? p.Color,
      marker: p.marker ?? p.Marker,
      volumeSpikeUp: p.volumeSpikeUp ?? p.VolumeSpikeUp,
      volumeSpikeDown: p.volumeSpikeDown ?? p.VolumeSpikeDown,
    }))
    .filter(Boolean);
}

function chartPointsToCandles(points) {
  return toCandles((points || []).map((p) => ({
    time: p.time ?? p.Time,
    open: p.open ?? p.Open,
    high: p.high ?? p.High,
    low: p.low ?? p.Low,
    close: p.close ?? p.Close,
    volume: p.volume ?? p.Volume,
  })));
}

function chartPointsToStorePayload(points, annotations) {
  return {
    candles: chartPointsToCandles(points),
    oscillators: chartPointsToOsc(points),
    annotations,
  };
}

function toVolumeBars(candles) {
  return (candles || []).map((c) => ({
    time: c.time,
    value: c.volume || 0,
    color: c.close >= c.open ? CHART_STYLES.volumeBar.upColor : CHART_STYLES.volumeBar.downColor,
  }));
}

function mapRSXSignalData(osc) {
  return (osc || []).map((d) => {
    const time = chartTime(d?.time);
    if (time == null) return { time: 0 };
    const raw = d.rsx_signal ?? d.rsxSignal;
    const value = parseFloat(raw);
    if (isWarmupOscValue(value)) return { time: Number(time) };
    return { time: Number(time), value };
  });
}

function mapRSXData(osc) {
  return (osc || []).map((d) => {
    const time = chartTime(d?.time);
    if (time == null) return { time: 0 };
    const value = parseFloat(d.rsx);
    if (isWarmupOscValue(value)) {
      return { time: Number(time) };
    }
    return {
      time: Number(time),
      value,
      color: d.color || RSX_DEFAULT_COLOR,
      marker: d.marker || '',
    };
  });
}

function rsxMarkerStyle(marker) {
  const m = String(marker || '').toUpperCase();
  if (m === 'S' || m === 'SS') {
    return { position: 'aboveBar', color: '#b71c1c', shape: 'circle', size: 1 };
  }
  if (m === 'L' || m === 'LL') {
    return { position: 'belowBar', color: '#004d40', shape: 'circle', size: 1 };
  }
  if (m === 'P') {
    return { position: 'belowBar', color: '#1565c0', shape: 'circle', size: 1 };
  }
  return { position: 'belowBar', color: '#2962ff', shape: 'circle', size: 1 };
}

function normalizeAnnotationPane(pane) {
  const key = String(pane || 'rsx').trim().toLowerCase();
  if (key === 'price' || key === 'wozduh') return key;
  return 'rsx';
}

function annotationToNativeMarker(ann) {
  const rawTime = ann?.time ?? ann?.Time;
  const time = chartTime(rawTime);
  if (time == null || !Number.isFinite(time)) return null;
  const label = String(ann?.label ?? ann?.Label ?? ann?.text ?? '').toUpperCase();
  const style = rsxMarkerStyle(label);
  const useRsxStyle = ['S', 'SS', 'L', 'LL', 'P'].includes(label);
  return {
    time: Number(time),
    position: useRsxStyle ? style.position : (ann?.position || 'belowBar'),
    color: useRsxStyle ? style.color : (ann?.color || '#26a69a'),
    shape: useRsxStyle ? style.shape : (ann?.shape || 'circle'),
    size: useRsxStyle ? (style.size ?? 1) : undefined,
    text: label,
    _rawTime: rawTime,
  };
}

function navigatorBarIndexToTime(index, candles) {
  const idx = Number(index);
  if (!Number.isInteger(idx) || idx < 0 || !candles || idx >= candles.length) return null;
  return chartTime(candles[idx].time);
}

function mapNavigatorBackgroundZones(zones) {
  return (zones || []).map((zone) => {
    const time1 = chartTime(zone.startTime);
    const time2 = chartTime(zone.endTime);
    if (time1 == null || time2 == null) return null;
    return {
      startTime: Math.floor(time1),
      endTime: Math.floor(time2),
      time1: Math.floor(time1),
      time2: Math.floor(time2),
      color: zone.color,
    };
  }).filter(Boolean);
}

function navigatorBarColorMap(barColors, candles) {
  const colorByTime = new Map();
  if (!barColors) return colorByTime;

  if (Array.isArray(barColors)) {
    barColors.forEach((entry) => {
      if (!entry?.color) return;
      const t = chartTime(entry.time)
        ?? navigatorBarIndexToTime(entry.index, candles);
      if (t != null) colorByTime.set(t, entry.color);
    });
    return colorByTime;
  }

  if (typeof barColors === 'object') {
    Object.entries(barColors).forEach(([rawTime, color]) => {
      const t = chartTime(rawTime);
      if (t != null && color) colorByTime.set(t, color);
    });
  }
  return colorByTime;
}

function barSlopeFromLine(line) {
  const y1 = Number(line.y1);
  const y2 = Number(line.y2);
  const x1 = Number(line.x1);
  const x2 = Number(line.x2);
  if (!Number.isFinite(y1) || !Number.isFinite(y2) || !Number.isFinite(x1) || !Number.isFinite(x2)) {
    return 0;
  }
  const dx = x2 - x1;
  if (dx === 0) return 0;
  return (y2 - y1) / dx;
}

function mapNavigatorLinesForChart(lines, candles) {
  const minTime = candles?.length ? chartTime(candles[0].time) : null;
  return (lines || []).map((line) => {
    let time1 = chartTime(line.time1);
    let time2 = chartTime(line.time2);
    if (time1 == null) time1 = navigatorBarIndexToTime(line.x1, candles);
    if (time2 == null) time2 = navigatorBarIndexToTime(line.x2, candles);
    if (time1 == null || time2 == null) return null;
    if (minTime != null) {
      if (time2 < minTime) return null;
      if (time1 < minTime) {
        const y1 = Number(line.y1);
        const y2 = Number(line.y2);
        if (Number.isFinite(y1) && Number.isFinite(y2) && time2 !== time1) {
          line = {
            ...line,
            y1: y1 + (y2 - y1) * (minTime - time1) / (time2 - time1),
          };
        }
        time1 = minTime;
      }
    }
    return {
      time1: Math.floor(time1),
      time2: Math.floor(time2),
      x1: Math.floor(time1),
      y1: line.y1,
      x2: Math.floor(time2),
      y2: line.y2,
      barX1: line.x1,
      barX2: line.x2,
      slope: Number.isFinite(line.slope) ? line.slope : barSlopeFromLine(line),
      isActive: line.isActive === true,
      color: line.color,
      style: line.style,
    };
  }).filter(Boolean);
}

function buildSpikeMarkersFromGrid(annotationMap, { showSpike = true } = {}) {
  if (!showSpike || !annotationMap) return [];
  const markers = [];
  annotationMap.forEach((ann, ms) => {
    if (!ann.spikeUp && !ann.spikeDown) return;
    const time = ChartDataStore.msToChartSec(ms);
    if (ann.spikeUp) {
      markers.push({ time, position: 'belowBar', color: TV.green, shape: 'circle', text: '▲' });
    }
    if (ann.spikeDown) {
      markers.push({ time, position: 'aboveBar', color: TV.red, shape: 'circle', text: '▼' });
    }
  });
  return markers.sort((a, b) => a.time - b.time);
}

function buildWozduhMarkersFromGrid(annotationMap) {
  if (!annotationMap) return [];
  const markers = [];
  annotationMap.forEach((ann, ms) => {
    if (!ann.volCross) return;
    markers.push({
      time: ChartDataStore.msToChartSec(ms),
      position: 'inBar',
      color: ann.volCross,
      shape: 'circle',
      size: 1,
    });
  });
  return markers.sort((a, b) => a.time - b.time);
}

/** @deprecated Use buildSpikeMarkersFromGrid — kept for callers passing annotation Map. */
function buildSpikeMarkers(annotationMapOrOsc) {
  if (annotationMapOrOsc instanceof Map) {
    return buildSpikeMarkersFromGrid(annotationMapOrOsc);
  }
  return buildSpikeMarkersFromGrid(new Map());
}

function normalizeTradeRow(t) {
  const pnl = Number(t.pnl ?? 0);
  return {
    entryTime: t.entryTime ?? 0,
    exitTime: t.exitTime ?? t.time ?? 0,
    side: t.side || '—',
    entryPrice: t.entryPrice,
    exitPrice: t.exitPrice,
    fee: t.fee,
    slippagePct: t.slippagePct,
    pnl,
    exitReason: t.exitReason || t.reason || '—',
    duration: t.duration,
  };
}

if (typeof window !== 'undefined') {
  window.Mappers = {
    chartTime,
    isWarmupOscValue,
    isValidOHLC,
    normalizeCandle,
    normalizeOscPoint,
    chartPointsToOsc,
    chartPointsToCandles,
    chartPointsToStorePayload,
    toCandles,
    toLine,
    toLineClose,
    toVolumeBars,
    mapRSXData,
    mapRSXSignalData,
    annotationToNativeMarker,
    normalizeAnnotationPane,
    rsxMarkerStyle,
    mapNavigatorLinesForChart,
    mapNavigatorBackgroundZones,
    navigatorBarColorMap,
    navigatorBarIndexToTime,
    normalizeStrategyMatrix,
    normalizeStrategyThresholds,
    normalizeRsxSettingsFromAPI,
    coerceRsxSettingsForAPI,
    normalizeTf,
    resolveTf,
    normalizeTradeRow,
    buildSpikeMarkers,
    buildSpikeMarkersFromGrid,
    buildWozduhMarkersFromGrid,
  };
}
