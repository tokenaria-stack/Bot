/**
 * UI-VISUAL-GRAMMAR-V1.
 * Paints a color position that the server already froze.
 * Does not rank a visible list, does not read path into a state color,
 * and does not subtract a relation.
 */
const StarInspectionGrammar = (() => {
  const DEFAULT_PALETTE = {
    low: [212, 148, 61],
    mid: [120, 123, 134],
    high: [38, 198, 218],
  };

  function clamp01(n) {
    if (n == null) return null;
    const v = Number(n);
    if (!Number.isFinite(v)) return null;
    if (v < 0) return 0;
    if (v > 1) return 1;
    return v;
  }

  function mix(a, b, t) {
    return [
      Math.round(a[0] + (b[0] - a[0]) * t),
      Math.round(a[1] + (b[1] - a[1]) * t),
      Math.round(a[2] + (b[2] - a[2]) * t),
    ];
  }

  function rgbOr(value, fallback) {
    if (!Array.isArray(value) || value.length !== 3) return fallback.slice();
    const out = value.map((part) => {
      const n = Number(part);
      if (!Number.isFinite(n)) return null;
      return Math.max(0, Math.min(255, Math.round(n)));
    });
    if (out.some((part) => part == null)) return fallback.slice();
    return out;
  }

  function paletteOr(palette) {
    const source = palette || {};
    return {
      low: rgbOr(source.low, DEFAULT_PALETTE.low),
      mid: rgbOr(source.mid, DEFAULT_PALETTE.mid),
      high: rgbOr(source.high, DEFAULT_PALETTE.high),
    };
  }

  function hexToRgb(hex) {
    const match = /^#?([0-9a-fA-F]{6})$/.exec(String(hex || '').trim());
    if (!match) return null;
    const n = parseInt(match[1], 16);
    return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
  }

  function rgbToHex(rgb) {
    return '#' + rgb.map((part) => part.toString(16).padStart(2, '0')).join('');
  }

  function paletteFromHex(hexes) {
    const source = hexes || {};
    return {
      low: hexToRgb(source.low) || DEFAULT_PALETTE.low.slice(),
      mid: hexToRgb(source.mid) || DEFAULT_PALETTE.mid.slice(),
      high: hexToRgb(source.high) || DEFAULT_PALETTE.high.slice(),
    };
  }

  function defaultHex() {
    return {
      low: rgbToHex(DEFAULT_PALETTE.low),
      mid: rgbToHex(DEFAULT_PALETTE.mid),
      high: rgbToHex(DEFAULT_PALETTE.high),
    };
  }

  function dotColor(position, palette) {
    const t = clamp01(position);
    if (t == null) return null;
    const stops = paletteOr(palette);
    const rgb = t < 0.5 ? mix(stops.low, stops.mid, t / 0.5) : mix(stops.mid, stops.high, (t - 0.5) / 0.5);
    return 'rgb(' + rgb[0] + ', ' + rgb[1] + ', ' + rgb[2] + ')';
  }

  function arrowGlyph(kind, value, ok) {
    if (!ok || (kind !== 'slope' && kind !== 'accel' && kind !== 'sign')) return '';
    const v = Number(value);
    if (!Number.isFinite(v) || v === 0) return '→';
    if (kind === 'accel') return v > 0 ? '↗' : '↘';
    return v > 0 ? '↑' : '↓';
  }

  function formatValue(value, ok, family) {
    if (!ok) return '—';
    const v = Number(value);
    if (!Number.isFinite(v)) return '—';
    const digits = Math.abs(v) >= 100 ? 2 : 2;
    const body = Math.abs(v) < 1 && v !== 0 ? v.toFixed(4) : v.toFixed(digits);
    if (family === 'signed') {
      if (v > 0) return '+' + body;
      if (v === 0) return '0.00';
      return body;
    }
    if (v === 0) return '0.00';
    return body;
  }

  function formatPath(value, ok) {
    if (!ok || value == null || !Number.isFinite(Number(value))) return '—';
    return Number(value).toFixed(3) + ' ATR';
  }

  return {
    dotColor, arrowGlyph, formatValue, formatPath, clamp01,
    paletteFromHex, defaultHex, rgbToHex, hexToRgb,
  };
})();

if (typeof module !== 'undefined' && module.exports) {
  module.exports = StarInspectionGrammar;
}
