/**
 * WOZDUH-COLOR-OVERRIDES-1 — sparse local color overrides for Wozduh paint.
 * Factory colors stay in ui_config. This store holds deviations only.
 * Reset = delete override. Does not own visibility or demand.
 */
(function (global) {
  'use strict';

  const STORAGE_KEY = 'wozduh_color_prefs_v1';
  const LINE_FIELDS = ['color'];
  const CHANNEL_FIELDS = [
    'upperColor',
    'midColor',
    'lowerColor',
    'boundColor',
    'fillColor',
    'upperFillColor',
    'lowerFillColor',
  ];
  const FILL_FIELDS = new Set(['fillColor', 'upperFillColor', 'lowerFillColor']);
  const ALL_FIELDS = new Set([...LINE_FIELDS, ...CHANNEL_FIELDS]);

  const memoryStore = createMemoryStorage();
  let injectedStorage = null;

  function createMemoryStorage() {
    const map = Object.create(null);
    return {
      getItem(key) {
        return Object.prototype.hasOwnProperty.call(map, key) ? map[key] : null;
      },
      setItem(key, value) {
        map[key] = String(value);
      },
      removeItem(key) {
        delete map[key];
      },
    };
  }

  function setStorage(storage) {
    injectedStorage = storage || null;
  }

  function storage() {
    if (injectedStorage) return injectedStorage;
    try {
      if (typeof window !== 'undefined' && window.localStorage) return window.localStorage;
    } catch {
      /* private mode */
    }
    return memoryStore;
  }

  function normalizeHex(value) {
    if (typeof value !== 'string') return null;
    const trimmed = value.trim();
    if (!/^#[0-9A-Fa-f]{6}$/.test(trimmed)) return null;
    return trimmed.toUpperCase();
  }

  function fieldsForKind(kind) {
    return String(kind || '').toLowerCase() === 'channel' ? CHANNEL_FIELDS : LINE_FIELDS;
  }

  /**
   * Alpha from the factory fill string only. No module-owned default.
   * rgba(...,a) → a; rgb(...) or #RRGGBB → 1; unparseable → null (skip fill paint).
   */
  function parseRgbaAlpha(fill) {
    if (typeof fill !== 'string') return null;
    const s = fill.trim();
    if (!s) return null;
    if (normalizeHex(s)) return 1;
    const rgba = s.match(/rgba\(\s*\d+\s*,\s*\d+\s*,\s*\d+\s*,\s*([0-9]*\.?[0-9]+)\s*\)/i);
    if (rgba) {
      const alpha = Number(rgba[1]);
      return Number.isFinite(alpha) ? alpha : null;
    }
    if (/^rgb\(\s*\d+\s*,\s*\d+\s*,\s*\d+\s*\)$/i.test(s)) return 1;
    return null;
  }

  function fillRgbaFromHex(hex, factoryFill) {
    const alpha = parseRgbaAlpha(factoryFill);
    if (alpha == null) return null;
    const n = hex.slice(1);
    const r = parseInt(n.slice(0, 2), 16);
    const g = parseInt(n.slice(2, 4), 16);
    const b = parseInt(n.slice(4, 6), 16);
    return `rgba(${r},${g},${b},${alpha})`;
  }

  function expandShortHex(value) {
    if (typeof value !== 'string') return null;
    const m = value.trim().match(/^#([0-9A-Fa-f]{3})$/);
    if (!m) return null;
    const [a, b, c] = m[1];
    return `#${a}${a}${b}${b}${c}${c}`.toUpperCase();
  }

  function rgbToHex(r, g, b) {
    const h = (n) => Math.max(0, Math.min(255, Number(n) || 0)).toString(16).padStart(2, '0');
    return `#${h(r)}${h(g)}${h(b)}`.toUpperCase();
  }

  function parseRgbTriple(value) {
    if (typeof value !== 'string') return null;
    const m = value.trim().match(/^rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)/i);
    if (!m) return null;
    return [Number(m[1]), Number(m[2]), Number(m[3])];
  }

  /**
   * Display-only #RRGGBB for <input type="color">. Not a factory SSOT.
   * Named CSS goes through the browser computed style, never an app color table.
   */
  function toPickerHex(cssColor) {
    if (typeof cssColor !== 'string') return null;
    const s = cssColor.trim();
    if (!s) return null;
    const hex6 = normalizeHex(s);
    if (hex6) return hex6;
    const short = expandShortHex(s);
    if (short) return short;
    const rgb = parseRgbTriple(s);
    if (rgb) return rgbToHex(rgb[0], rgb[1], rgb[2]);
    try {
      if (typeof document === 'undefined' || typeof document.createElement !== 'function') {
        return null;
      }
      const el = document.createElement('span');
      el.style.color = s;
      const host = document.body || document.documentElement;
      if (host && typeof host.appendChild === 'function') host.appendChild(el);
      const computed = (typeof getComputedStyle === 'function')
        ? getComputedStyle(el).color
        : el.style.color;
      if (el.remove) el.remove();
      else if (el.parentNode && typeof el.parentNode.removeChild === 'function') {
        el.parentNode.removeChild(el);
      }
      if (!computed || computed === s) return null;
      const fromComputed = parseRgbTriple(computed) || normalizeHex(computed);
      if (Array.isArray(fromComputed)) return rgbToHex(fromComputed[0], fromComputed[1], fromComputed[2]);
      return fromComputed || null;
    } catch {
      return null;
    }
  }

  function pickerHexFor(kind, factoryColors, override, field) {
    const ov = override && typeof override === 'object' ? override[field] : null;
    const fromOverride = normalizeHex(ov);
    if (fromOverride) return fromOverride;
    const factory = factoryColors && typeof factoryColors === 'object' ? factoryColors[field] : null;
    return toPickerHex(factory);
  }

  function hasFactoryColor(factoryColors, field) {
    if (!factoryColors || typeof factoryColors !== 'object') return false;
    const v = factoryColors[field];
    return typeof v === 'string' && v.trim() !== '';
  }

  function factoryColorFields(kind, renderOpts) {
    const opts = renderOpts && typeof renderOpts === 'object' ? renderOpts : {};
    const out = {};
    const fields = fieldsForKind(kind);
    for (const field of fields) {
      if (!Object.prototype.hasOwnProperty.call(opts, field)) continue;
      if (typeof opts[field] !== 'string' || opts[field].trim() === '') continue;
      out[field] = opts[field];
    }
    return out;
  }

  function loadMap() {
    try {
      const raw = storage().getItem(STORAGE_KEY);
      if (!raw) return {};
      const parsed = JSON.parse(raw);
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return {};
      return parsed;
    } catch {
      return {};
    }
  }

  function persistMap(map) {
    const sparse = {};
    if (map && typeof map === 'object') {
      for (const [id, obj] of Object.entries(map)) {
        if (!id || !obj || typeof obj !== 'object' || Array.isArray(obj)) continue;
        const cleaned = {};
        for (const [field, value] of Object.entries(obj)) {
          if (!ALL_FIELDS.has(field)) continue;
          const hex = normalizeHex(value);
          if (!hex) continue;
          cleaned[field] = hex;
        }
        if (Object.keys(cleaned).length) sparse[id] = cleaned;
      }
    }
    if (!Object.keys(sparse).length) {
      storage().removeItem(STORAGE_KEY);
      return {};
    }
    storage().setItem(STORAGE_KEY, JSON.stringify(sparse));
    return sparse;
  }

  function overrideFor(id) {
    if (!id) return {};
    const row = loadMap()[id];
    return row && typeof row === 'object' ? { ...row } : {};
  }

  function setColor(id, field, hex) {
    if (!id || !ALL_FIELDS.has(field)) return overrideFor(id);
    const normalized = normalizeHex(hex);
    if (!normalized) return overrideFor(id);
    const map = loadMap();
    const row = { ...(map[id] && typeof map[id] === 'object' ? map[id] : {}) };
    row[field] = normalized;
    map[id] = row;
    persistMap(map);
    return overrideFor(id);
  }

  function resetProperty(id, field) {
    if (!id || !ALL_FIELDS.has(field)) return overrideFor(id);
    const map = loadMap();
    if (!map[id] || typeof map[id] !== 'object') {
      persistMap(map);
      return {};
    }
    delete map[id][field];
    persistMap(map);
    return overrideFor(id);
  }

  function resetComponent(id) {
    if (!id) return;
    const map = loadMap();
    delete map[id];
    persistMap(map);
  }

  function resetAll() {
    storage().removeItem(STORAGE_KEY);
  }

  /**
   * Color keys to apply when overrides exist. Empty object = do not touch series.
   */
  function paintPatch(kind, factoryColors, override) {
    const fields = fieldsForKind(kind);
    const src = override && typeof override === 'object' ? override : {};
    const factory = factoryColors && typeof factoryColors === 'object' ? factoryColors : {};
    const out = {};
    for (const field of fields) {
      if (!hasFactoryColor(factory, field)) continue;
      const hex = normalizeHex(src[field]);
      if (!hex) continue;
      if (FILL_FIELDS.has(field)) {
        const rgba = fillRgbaFromHex(hex, factory[field]);
        if (rgba) out[field] = rgba;
      } else {
        out[field] = hex;
      }
    }
    return out;
  }

  /** Restore factory strings after an override is deleted. */
  function factoryPatch(kind, factoryColors, onlyFields) {
    const allowed = fieldsForKind(kind);
    const want = Array.isArray(onlyFields) && onlyFields.length ? onlyFields : allowed;
    const factory = factoryColors && typeof factoryColors === 'object' ? factoryColors : {};
    const out = {};
    for (const field of want) {
      if (!allowed.includes(field)) continue;
      if (!hasFactoryColor(factory, field)) continue;
      out[field] = factory[field];
    }
    return out;
  }

  const api = {
    STORAGE_KEY,
    LINE_FIELDS,
    CHANNEL_FIELDS,
    FILL_FIELDS,
    hasFactoryColor,
    setStorage,
    normalizeHex,
    fieldsForKind,
    factoryColorFields,
    parseRgbaAlpha,
    fillRgbaFromHex,
    toPickerHex,
    pickerHexFor,
    loadMap,
    overrideFor,
    setColor,
    resetProperty,
    resetComponent,
    resetAll,
    paintPatch,
    factoryPatch,
  };

  global.WozduhColorPrefs = api;
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = api;
  }
})(typeof window !== 'undefined' ? window : globalThis);
