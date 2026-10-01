/**
 * Which chart line a reading belongs to.
 * The color itself stays on the series factory. This file only names the lines.
 */
const StarInspectionSwatches = (() => {
  const LINE = {
    vwema: { seriesId: 'woz_rsi_hl2_vwema', field: 'color' },
    orange: { seriesId: 'woz_vol_rsi_ema5_chan', field: 'midColor' },
    ema5: { seriesId: 'woz_vol_rsi_ema5', field: 'color' },
    ema12: { seriesId: 'woz_vol_rsi_ema12', field: 'color' },
    rsi: { seriesId: 'woz_rsi_close', field: 'color' },
    closeMid: { seriesId: 'woz_rsi_close_chan', field: 'midColor' },
    ema7: { seriesId: 'woz_rsi_close_ema7', field: 'color' },
    macd: { seriesId: 'woz_macd_rsi_close', field: 'color' },
    rsx: { seriesId: 'line_rsx', field: 'color' },
    signal: { seriesId: 'line_rsx_signal', field: 'color' },
    width: { seriesId: 'woz_vol_rsi_ema5_chan', field: 'boundColor' },
    closeWidth: { seriesId: 'woz_rsi_close_chan', field: 'boundColor' },
  };

  const RELATION = {
    ema7Macd: ['ema7', 'macd'],
    ema7Mid: ['ema7', 'closeMid'],
    rsiMid: ['rsi', 'closeMid'],
    slope: ['vwema', 'orange'],
    rsx: ['rsx', 'signal'],
    orange: ['orange', 'orange'],
    vwema: ['vwema', 'vwema'],
    ema5: ['ema5', 'ema5'],
    ema12: ['ema12', 'ema12'],
    rsi: ['rsi', 'rsi'],
    ema7: ['ema7', 'ema7'],
    macd: ['macd', 'macd'],
    width: ['width', 'width'],
    closeWidth: ['closeWidth', 'closeWidth'],
  };

  function one(key) {
    const spec = LINE[key];
    return spec ? [spec] : [];
  }

  const FIELD = {
    Vwema: 'vwema', Slope: 'vwema', VwemaAccel: 'vwema', Distance: 'vwema',
    ChanMid: 'orange', ChanUp: 'orange', ChanDn: 'orange',
    MidSlope: 'orange', MidAccel: 'orange',
    Width: 'width', WidthChange: 'width',
    Ema5: 'ema5', Ema5Slope: 'ema5', Ema5Accel: 'ema5',
    Ema12: 'ema12', Ema12Slope: 'ema12',
    RsiClose: 'rsi', RsiCloseSlope: 'rsi', RsiCloseAccel: 'rsi',
    CloseMid: 'closeMid', CloseUp: 'closeMid', CloseDn: 'closeMid',
    CloseWidth: 'closeWidth', CloseWidthChange: 'closeWidth',
    Ema7: 'ema7', Ema7Slope: 'ema7', Ema7Accel: 'ema7',
    Macd: 'macd', MacdSlope: 'macd', MacdAccel: 'macd',
    Value: 'rsx', Accel: 'rsx', RSX: 'rsx', RSXSlope: 'rsx', RSXAccel: 'rsx',
    Signal: 'signal', SignalSlope: 'signal',
    RSXMinusSignal: 'rsx',
  };

  function specs(readingId) {
    const id = String(readingId || '');
    if (id.indexOf('rel.') === 0) {
      const leaf = id.slice(id.lastIndexOf('.') + 1);
      const pair = RELATION[leaf];
      if (!pair) return [];
      return pair.map((key) => LINE[key]).filter(Boolean);
    }
    const leaf = id.slice(id.lastIndexOf('.') + 1);
    const rsx = /RSX/.test(id) || leaf === 'rsx' || leaf === 'rsxMinus';
    if (rsx) {
      if (leaf === 'Signal' || leaf === 'SignalSlope' || leaf === 'signal') return one('signal');
      if (leaf === 'rsxMinus' || leaf === 'RSXMinusSignal') return [LINE.rsx, LINE.signal];
      return one('rsx');
    }
    if (FIELD[leaf]) return one(FIELD[leaf]);
    if (leaf === 'rsxMinus') return [LINE.rsx, LINE.signal];
    if (/Slope$/.test(leaf) || /Accel$/.test(leaf)) {
      return one(leaf.replace(/Slope$|Accel$/, ''));
    }
    if (leaf === 'orange') return one('orange');
    return one(leaf);
  }

  return { specs, LINE };
})();

if (typeof module !== 'undefined' && module.exports) {
  module.exports = StarInspectionSwatches;
}
