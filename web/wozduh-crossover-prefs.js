/**
 * WOZDUH-CROSSOVER-PAINT-1 — sparse local chrome prefs for Wozduh crossover dots.
 * Factory lives in ui_config. This store holds deviations only.
 */
(function (global) {
  'use strict';

  const STORAGE_KEY = 'wozduh_crossover_prefs_v1';
  const SHAPES = Object.freeze(['circle', 'square', 'triangle', 'star4']);
  const OUTLINE_STYLES = Object.freeze(['solid', 'dashed']);

  const FACTORY = Object.freeze([
    Object.freeze({
      id: 'woz_vol_rsi_ema12_x_ema5',
      title: 'Volume RSI EMA12 × EMA5',
      plotA: 'woz_vol_rsi_ema12',
      plotB: 'woz_vol_rsi_ema5',
      visible: true,
      shape: 'circle',
      size: 8,
      upFill: '#00E676',
      downFill: '#FF1744',
      outlineColor: '#000000',
      outlineWidth: 1,
      outlineStyle: 'solid',
    }),
    Object.freeze({
      id: 'woz_rsi_hl2_vwema_x_ema5',
      title: 'RSI VWEMA(HL2) × Volume RSI EMA5',
      plotA: 'woz_rsi_hl2_vwema',
      plotB: 'woz_vol_rsi_ema5',
      visible: false,
      shape: 'square',
      size: 8,
      upFill: '#00E676',
      downFill: '#FF1744',
      outlineColor: '#000000',
      outlineWidth: 1,
      outlineStyle: 'solid',
    }),
    Object.freeze({
      id: 'woz_rsi_hl2_vwema_x_ema12',
      title: 'RSI VWEMA(HL2) × Volume RSI EMA12',
      plotA: 'woz_rsi_hl2_vwema',
      plotB: 'woz_vol_rsi_ema12',
      visible: false,
      shape: 'triangle',
      size: 8,
      upFill: '#00E676',
      downFill: '#FF1744',
      outlineColor: '#000000',
      outlineWidth: 1,
      outlineStyle: 'solid',
    }),
    Object.freeze({
      id: 'woz_rsi_hl2_vwema_x_ema5_chan_mid',
      title: 'RSI VWEMA(HL2) × EMA5 channel mid',
      plotA: 'woz_rsi_hl2_vwema',
      plotB: 'woz_vol_rsi_ema5_chan_mid',
      visible: false,
      shape: 'star4',
      size: 8,
      upFill: '#00E676',
      downFill: '#FF1744',
      outlineColor: '#000000',
      outlineWidth: 1,
      outlineStyle: 'solid',
    }),
  ]);

  const FACTORY_BY_ID = Object.freeze(Object.fromEntries(FACTORY.map((p) => [p.id, p])));
  const PAIR_IDS = Object.freeze(FACTORY.map((p) => p.id));

  const memoryStore = createMemoryStorage();
  let injectedStorage = null;
  const listeners = [];

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

  function sameFactory(id, field, value) {
    const fac = FACTORY_BY_ID[id];
    if (!fac) return false;
    if (field === 'upFill' || field === 'downFill' || field === 'outlineColor') {
      const hex = normalizeHex(value);
      return hex && hex === normalizeHex(fac[field]);
    }
    if (field === 'size' || field === 'outlineWidth') {
      return Number(value) === Number(fac[field]);
    }
    return value === fac[field];
  }

  function persistMap(map) {
    const sparse = {};
    if (map && typeof map === 'object') {
      for (const id of PAIR_IDS) {
        const row = map[id];
        if (!row || typeof row !== 'object' || Array.isArray(row)) continue;
        const cleaned = {};
        if (typeof row.visible === 'boolean' && row.visible !== FACTORY_BY_ID[id].visible) {
          cleaned.visible = row.visible;
        }
        if (SHAPES.includes(row.shape) && row.shape !== FACTORY_BY_ID[id].shape) {
          cleaned.shape = row.shape;
        }
        if (OUTLINE_STYLES.includes(row.outlineStyle) && row.outlineStyle !== FACTORY_BY_ID[id].outlineStyle) {
          cleaned.outlineStyle = row.outlineStyle;
        }
        const size = Number(row.size);
        if (Number.isFinite(size) && size > 0 && size <= 32 && !sameFactory(id, 'size', size)) {
          cleaned.size = size;
        }
        const ow = Number(row.outlineWidth);
        if (Number.isFinite(ow) && ow >= 0 && ow <= 8 && !sameFactory(id, 'outlineWidth', ow)) {
          cleaned.outlineWidth = ow;
        }
        for (const field of ['upFill', 'downFill', 'outlineColor']) {
          const hex = normalizeHex(row[field]);
          if (hex && !sameFactory(id, field, hex)) cleaned[field] = hex;
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

  function notify(kind) {
    for (let i = 0; i < listeners.length; i++) {
      try { listeners[i](kind); } catch { /* */ }
    }
  }

  function onChange(fn) {
    if (typeof fn !== 'function') return () => {};
    listeners.push(fn);
    return () => {
      const i = listeners.indexOf(fn);
      if (i >= 0) listeners.splice(i, 1);
    };
  }

  function factoryFor(id) {
    return FACTORY_BY_ID[id] || null;
  }

  function resolved(id) {
    const fac = FACTORY_BY_ID[id];
    if (!fac) return null;
    const ov = loadMap()[id];
    const row = ov && typeof ov === 'object' ? ov : {};
    const hex = (field) => normalizeHex(row[field]) || fac[field];
    const shape = SHAPES.includes(row.shape) ? row.shape : fac.shape;
    const outlineStyle = OUTLINE_STYLES.includes(row.outlineStyle) ? row.outlineStyle : fac.outlineStyle;
    const size = Number.isFinite(Number(row.size)) && Number(row.size) > 0 ? Number(row.size) : fac.size;
    const outlineWidth = Number.isFinite(Number(row.outlineWidth)) && Number(row.outlineWidth) >= 0
      ? Number(row.outlineWidth)
      : fac.outlineWidth;
    return {
      id: fac.id,
      title: fac.title,
      plotA: fac.plotA,
      plotB: fac.plotB,
      visible: typeof row.visible === 'boolean' ? row.visible : fac.visible,
      shape,
      size,
      upFill: hex('upFill'),
      downFill: hex('downFill'),
      outlineColor: hex('outlineColor'),
      outlineWidth,
      outlineStyle,
    };
  }

  function allResolved() {
    return PAIR_IDS.map(resolved);
  }

  function isVisible(id) {
    const row = resolved(id);
    return !!(row && row.visible);
  }

  function demandPlotIds() {
    const ids = new Set();
    for (const row of allResolved()) {
      if (!row.visible) continue;
      ids.add(row.plotA);
      ids.add(row.plotB);
    }
    return [...ids];
  }

  function patch(id, fields) {
    if (!FACTORY_BY_ID[id] || !fields || typeof fields !== 'object') return resolved(id);
    const map = loadMap();
    const row = { ...(map[id] && typeof map[id] === 'object' ? map[id] : {}) };
    if (Object.prototype.hasOwnProperty.call(fields, 'visible') && typeof fields.visible === 'boolean') {
      row.visible = fields.visible;
    }
    if (SHAPES.includes(fields.shape)) row.shape = fields.shape;
    if (OUTLINE_STYLES.includes(fields.outlineStyle)) row.outlineStyle = fields.outlineStyle;
    if (fields.size != null && Number.isFinite(Number(fields.size))) row.size = Number(fields.size);
    if (fields.outlineWidth != null && Number.isFinite(Number(fields.outlineWidth))) {
      row.outlineWidth = Number(fields.outlineWidth);
    }
    for (const field of ['upFill', 'downFill', 'outlineColor']) {
      if (fields[field] == null) continue;
      const hex = normalizeHex(fields[field]);
      if (hex) row[field] = hex;
    }
    map[id] = row;
    persistMap(map);
    notify(Object.prototype.hasOwnProperty.call(fields, 'visible') ? 'demand' : 'paint');
    return resolved(id);
  }

  function resetPair(id) {
    if (!FACTORY_BY_ID[id]) return;
    const map = loadMap();
    delete map[id];
    persistMap(map);
    notify('demand');
  }

  function resetAll() {
    storage().removeItem(STORAGE_KEY);
    notify('demand');
  }

  /** Isolate DDR hydrate tests from factory-on pair-1 demand. */
  function disableAllForTests() {
    setStorage(createMemoryStorage());
    const map = {};
    for (const id of PAIR_IDS) map[id] = { visible: false };
    persistMap(map);
  }

  const api = {
    STORAGE_KEY,
    SHAPES,
    OUTLINE_STYLES,
    FACTORY,
    PAIR_IDS,
    setStorage,
    factoryFor,
    resolved,
    allResolved,
    isVisible,
    demandPlotIds,
    patch,
    resetPair,
    resetAll,
    disableAllForTests,
    onChange,
    loadMap,
    normalizeHex,
  };

  global.WozduhCrossoverPrefs = api;
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = api;
  }
})(typeof window !== 'undefined' ? window : globalThis);
