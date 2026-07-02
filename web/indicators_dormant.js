// Dormant indicator math quarantine (Phase 17.1).
// This file intentionally holds optional Wozduh line definitions and mappers,
// while active rendering remains in app.js.
(function initDormantIndicators(global) {
  const WOZDUX_DORMANT_LINE_STYLES = {
    emaRsi: { color: 'green', lineWidth: 2, title: 'EMA(RSI)', priceLineVisible: false, lastValueVisible: false },
    rsiRsi: { color: 'orange', lineWidth: 2, title: 'RSI(RSI)', priceLineVisible: false, lastValueVisible: false },
    macdRsi: { color: 'black', lineWidth: 2, title: 'MACD(RSI)', priceLineVisible: false, lastValueVisible: false },
    rsiAd: { color: 'maroon', lineWidth: 3, title: 'RSI(AD)', priceLineVisible: false, lastValueVisible: false },
    rsiHl2Vol: { color: 'navy', lineWidth: 2, title: 'RSI(HL2*Vol)', priceLineVisible: false, lastValueVisible: false },
    priceChanMid: { color: 'maroon', lineWidth: 2, title: 'Price Chan Mid', priceLineVisible: false, lastValueVisible: false },
    priceChanUp: {
      color: 'blue',
      lineWidth: 1,
      lineStyle: global.LightweightCharts?.LineStyle?.Dashed,
      title: 'Price Chan Up',
      priceLineVisible: false,
      lastValueVisible: false,
    },
    priceChanDn: {
      color: 'blue',
      lineWidth: 1,
      lineStyle: global.LightweightCharts?.LineStyle?.Dashed,
      title: 'Price Chan Dn',
      priceLineVisible: false,
      lastValueVisible: false,
    },
  };

  const WOZDUH_DORMANT_MENU_ITEMS = [
    { prefKey: 'emaRsi', keys: ['emaRsi'], default: false },
    { prefKey: 'rsiRsi', keys: ['rsiRsi'], default: false },
    { prefKey: 'rsiHl2Vol', keys: ['rsiHl2Vol'], default: false },
    { prefKey: 'macdRsi', keys: ['macdRsi'], default: false },
    { prefKey: 'rsiAd', keys: ['rsiAd'], default: false },
    { prefKey: 'priceChan', keys: ['priceChanMid', 'priceChanUp', 'priceChanDn'], default: false },
  ];

  const WOZDUH_DORMANT_LINE_CONFIG = {
    emaRsi: { dataKey: 'emaRsi', style: WOZDUX_DORMANT_LINE_STYLES.emaRsi },
    rsiRsi: { dataKey: 'rsiRsi', style: WOZDUX_DORMANT_LINE_STYLES.rsiRsi },
    macdRsi: { dataKey: 'macdRsi', style: WOZDUX_DORMANT_LINE_STYLES.macdRsi },
    rsiAd: { dataKey: 'rsiAd', style: WOZDUX_DORMANT_LINE_STYLES.rsiAd },
    rsiHl2Vol: { dataKey: 'rsiHl2Vol', style: WOZDUX_DORMANT_LINE_STYLES.rsiHl2Vol },
    priceChanMid: { dataKey: 'priceChanMid', style: WOZDUX_DORMANT_LINE_STYLES.priceChanMid },
    priceChanUp: { dataKey: 'priceChanUp', style: WOZDUX_DORMANT_LINE_STYLES.priceChanUp },
    priceChanDn: { dataKey: 'priceChanDn', style: WOZDUX_DORMANT_LINE_STYLES.priceChanDn },
  };

  // Quarantined mapper: kept for future scoring research, not used in active render flow.
  function mapDormantWozduhLines(points, chartTimeFn, isWarmupFn) {
    const out = {};
    Object.keys(WOZDUH_DORMANT_LINE_CONFIG).forEach((key) => {
      out[key] = (points || []).map((p) => {
        const time = chartTimeFn ? chartTimeFn(p?.time ?? p?.Time) : null;
        if (time == null) return { time: 0 };
        const value = Number(p?.[key]);
        if (!Number.isFinite(value) || (isWarmupFn && isWarmupFn(value))) return { time: Number(time) };
        return { time: Number(time), value };
      });
    });
    return out;
  }

  global.DormantIndicators = Object.assign({}, global.DormantIndicators || {}, {
    WOZDUX_DORMANT_LINE_STYLES,
    WOZDUH_DORMANT_MENU_ITEMS,
    WOZDUH_DORMANT_LINE_CONFIG,
    mapDormantWozduhLines,
  });
}(window));
