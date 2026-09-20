/**
 * SettingsRenderer — DDR manifest → visibility toggles (Great Purge Stage 4).
 * No hardcoded line ids: checkboxes are generated from component.configurable.
 */
const SettingsRenderer = (() => {
  const PREFS_LIVE_KEY = (typeof WOZDUH_PREFS_LIVE_KEY !== 'undefined')
    ? WOZDUH_PREFS_LIVE_KEY
    : 'wozduh_visibility_prefs_live';
  const PREFS_LEGACY_KEY = (typeof WOZDUH_PREFS_KEY !== 'undefined')
    ? WOZDUH_PREFS_KEY
    : 'wozduh_visibility_prefs';

  function parseRenderOpts(raw) {
    if (!raw) return {};
    if (typeof raw === 'object') return raw;
    try {
      const parsed = JSON.parse(raw);
      return parsed && typeof parsed === 'object' ? parsed : {};
    } catch {
      return {};
    }
  }

  /** @returns {object[]} configurable Wozduh scalar lines from UIManifest.panes */
  function collectConfigurable(manifest) {
    const panes = manifest?.panes;
    if (!panes || typeof panes !== 'object') return [];
    const out = [];
    for (const comps of Object.values(panes)) {
      if (!Array.isArray(comps)) continue;
      for (const c of comps) {
        if (!c || c.configurable !== true) continue;
        if (String(c.hostId || '') !== 'wozduh') continue;
        if (String(c.kind || 'line').toLowerCase() === 'marker') continue;
        if (String(c.kind || '').toLowerCase() === 'plot') continue;
        if (c.dataMode === 'annotations') continue;
        if (!c.id) continue;
        out.push(c);
      }
    }
    return out;
  }

  function defaultVisibleFor(component) {
    const opts = parseRenderOpts(component.renderOptions);
    if (typeof opts.defaultVisible === 'boolean') return opts.defaultVisible;
    return true;
  }

  function labelFor(component) {
    const opts = parseRenderOpts(component.renderOptions);
    const title = opts.title != null ? String(opts.title).trim() : '';
    return title || component.id;
  }

  function loadPrefsMap() {
    try {
      const raw = localStorage.getItem(PREFS_LIVE_KEY) || localStorage.getItem(PREFS_LEGACY_KEY);
      if (!raw) return {};
      const prefs = JSON.parse(raw);
      return prefs && typeof prefs === 'object' ? prefs : {};
    } catch {
      return {};
    }
  }

  /**
   * One-shot remap of legacy Falcon keys and retired Wozduh plot IDs onto
   * canonical component.id keys. Old IDs are not kept as runtime aliases.
   */
  function migrateLegacyPrefs(prefs, components) {
    const next = { ...prefs };
    const remap = (oldId, newId) => {
      if (oldId === newId) return;
      if (typeof next[newId] !== 'boolean' && typeof prefs[oldId] === 'boolean') {
        next[newId] = prefs[oldId];
      }
      delete next[oldId];
    };
    remap('woz_fast', 'woz_vol_rsi_ema12');
    remap('woz_slow', 'woz_vol_rsi_ema5');
    remap('woz_rsi_price', 'woz_rsi_close');
    remap('woz_ema_rsi', 'woz_rsi_close_ema7');
    remap('woz_rsi_rsi', 'woz_rsi_rsi_close');
    remap('woz_macd_rsi', 'woz_macd_rsi_close');
    remap('woz_rsi_hl2_vol', 'woz_rsi_hl2_vwema');
    remap('woz_vol_chan', 'woz_vol_rsi_ema5_chan');
    remap('woz_price_chan', 'woz_rsi_close_chan');
    // Legacy Falcon single toggle for both volume-RSI EMA lines.
    if (typeof prefs.rsiVol === 'boolean') {
      if (typeof next.woz_vol_rsi_ema12 !== 'boolean') next.woz_vol_rsi_ema12 = prefs.rsiVol;
      if (typeof next.woz_vol_rsi_ema5 !== 'boolean') next.woz_vol_rsi_ema5 = prefs.rsiVol;
    }
    const legacyToId = {
      rsiPrice: 'woz_rsi_close',
      emaRsi: 'woz_rsi_close_ema7',
      rsiRsi: 'woz_rsi_rsi_close',
      rsiHl2: 'woz_rsi_hl2',
      macdRsi: 'woz_macd_rsi_close',
      rsiAd: 'woz_rsi_ad',
      rsiHl2Vol: 'woz_rsi_hl2_vwema',
      priceChan: 'woz_rsi_close_chan',
    };
    for (const [legacy, id] of Object.entries(legacyToId)) {
      if (typeof prefs[legacy] === 'boolean' && typeof next[id] !== 'boolean') {
        next[id] = prefs[legacy];
      }
    }
    const collapse = (newId, oldIds) => {
      if (typeof next[newId] === 'boolean') return;
      const flags = oldIds.map((id) => prefs[id]).filter((v) => typeof v === 'boolean');
      if (!flags.length) return;
      next[newId] = flags.some(Boolean);
    };
    collapse('woz_vol_rsi_ema5_chan', [
      'woz_vol_rsi_ema5_chan_mid', 'woz_vol_rsi_ema5_chan_up', 'woz_vol_rsi_ema5_chan_dn',
      'woz_vol_chan_mid', 'woz_vol_chan_up', 'woz_vol_chan_dn',
    ]);
    collapse('woz_rsi_close_chan', [
      'woz_rsi_close_chan_mid', 'woz_rsi_close_chan_up', 'woz_rsi_close_chan_dn',
      'woz_price_chan_mid', 'woz_price_chan_up', 'woz_price_chan_dn',
    ]);
    for (const c of components) {
      if (typeof next[c.id] !== 'boolean') {
        next[c.id] = defaultVisibleFor(c);
      }
    }
    return next;
  }

  function savePrefsMap(prefs) {
    try {
      localStorage.setItem(PREFS_LIVE_KEY, JSON.stringify(prefs));
    } catch {
      /* quota / private mode */
    }
  }

  function isChecked(prefs, component) {
    if (typeof prefs[component.id] === 'boolean') return prefs[component.id];
    return defaultVisibleFor(component);
  }

  const WozduhColorPrefsApi = (typeof WozduhColorPrefs !== 'undefined')
    ? WozduhColorPrefs
    : (typeof require === 'function'
      ? (() => { try { return require('../wozduh-color-prefs.js'); } catch { return null; } })()
      : null);

  function ddrFactory() {
    return (typeof window !== 'undefined') ? window.DDRFactory : null;
  }

  function componentKind(component) {
    return String(component.kind || 'line').toLowerCase();
  }

  function factoryColorsFor(component) {
    if (!WozduhColorPrefsApi) return {};
    return WozduhColorPrefsApi.factoryColorFields(
      componentKind(component),
      parseRenderOpts(component.renderOptions),
    );
  }

  function pickerHex(component, field) {
    if (!WozduhColorPrefsApi) return '#000000';
    const hex = WozduhColorPrefsApi.pickerHexFor(
      componentKind(component),
      factoryColorsFor(component),
      WozduhColorPrefsApi.overrideFor(component.id),
      field,
    );
    return hex ? hex.toLowerCase() : '#000000';
  }

  function refreshColorInputs(root, components) {
    if (!root || !Array.isArray(components)) return;
    const byId = new Map(components.map((c) => [c.id, c]));
    const inputs = typeof root.querySelectorAll === 'function'
      ? root.querySelectorAll('input[type="color"]')
      : [];
    for (const input of inputs) {
      const id = input.dataset && input.dataset.componentId;
      const field = input.dataset && input.dataset.field;
      const c = byId.get(id);
      if (!c || !field) continue;
      input.value = pickerHex(c, field);
    }
  }

  function paintColor(component, field, hex) {
    const factory = ddrFactory();
    if (!factory || typeof factory.setWozduhColor !== 'function') return;
    factory.setWozduhColor(component.id, field, hex);
  }

  function bindVisibilityCheckbox(input, component) {
    input.addEventListener('change', () => {
      const map = loadPrefsMap();
      map[component.id] = input.checked;
      savePrefsMap(map);
      const factory = ddrFactory();
      if (factory?.cutoverActive && typeof factory.setSeriesVisible === 'function') {
        factory.setSeriesVisible(component.id, input.checked);
      }
    });
  }

  function visibilityCheckbox(component, prefs) {
    const input = document.createElement('input');
    input.type = 'checkbox';
    input.className = 'wozduh-chk';
    input.dataset.componentId = component.id;
    input.checked = isChecked(prefs, component);
    bindVisibilityCheckbox(input, component);
    return input;
  }

  function nameSpan(c) {
    const text = document.createElement('span');
    text.className = c.id === 'woz_vol_rsi_ema5'
      ? 'wozduh-style-label wozduh-pane-owner-label'
      : 'wozduh-style-label';
    text.textContent = labelFor(c);
    return text;
  }

  function appendLineRow(menu, component, prefs) {
    const name = labelFor(component);
    const row = document.createElement('div');
    row.className = 'wozduh-component-row';
    row.dataset.componentId = component.id;
    const vis = document.createElement('label');
    vis.className = 'wozduh-vis';
    vis.appendChild(visibilityCheckbox(component, prefs));
    vis.appendChild(nameSpan(component));
    row.appendChild(vis);
    row.appendChild(colorInput(component, 'color', `${name} color`));
    row.appendChild(defaultButton('Default', `Default ${name} color`, () => {
      const factory = ddrFactory();
      if (factory && typeof factory.resetWozduhColor === 'function') {
        factory.resetWozduhColor(component.id, 'color');
      }
      const input = row.querySelector?.('input[type="color"]')
        || [...(row.children || [])].find((c) => c.type === 'color');
      if (input) input.value = pickerHex(component, 'color');
    }));
    menu.appendChild(row);
  }

  function appendChannelGroup(menu, component, prefs) {
    const name = labelFor(component);
    const wrap = document.createElement('div');
    wrap.className = 'wozduh-style-channel';
    wrap.dataset.componentId = component.id;
    const header = document.createElement('div');
    header.className = 'wozduh-component-row wozduh-component-row--channel-header';
    const vis = document.createElement('label');
    vis.className = 'wozduh-vis';
    vis.appendChild(visibilityCheckbox(component, prefs));
    vis.appendChild(nameSpan(component));
    header.appendChild(vis);
    header.appendChild(defaultButton('Default', `Default ${name} colors`, () => {
      const factory = ddrFactory();
      if (factory && typeof factory.resetWozduhComponentColors === 'function') {
        factory.resetWozduhComponentColors(component.id);
      }
      refreshColorInputs(wrap, [component]);
    }));
    wrap.appendChild(header);
    const factory = factoryColorsFor(component);
    const fields = [
      ['boundColor', 'Boundary'],
      ['upperColor', 'Upper'],
      ['upperFillColor', 'Upper fill'],
      ['midColor', 'Middle'],
      ['lowerFillColor', 'Lower fill'],
      ['lowerColor', 'Lower'],
      ['fillColor', 'Fill'],
    ];
    for (const [field, fieldLabel] of fields) {
      if (WozduhColorPrefsApi && typeof WozduhColorPrefsApi.hasFactoryColor === 'function') {
        if (!WozduhColorPrefsApi.hasFactoryColor(factory, field)) continue;
      } else if (!factory[field]) {
        continue;
      }
      const row = document.createElement('div');
      row.className = 'wozduh-style-row wozduh-style-row--nested';
      const text = document.createElement('span');
      text.className = 'wozduh-style-label';
      text.textContent = fieldLabel;
      row.appendChild(text);
      row.appendChild(colorInput(component, field, `${name} ${fieldLabel.toLowerCase()} color`));
      wrap.appendChild(row);
    }
    menu.appendChild(wrap);
  }

  function defaultButton(label, title, onClick) {
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'wozduh-style-default';
    btn.textContent = label;
    btn.title = title;
    btn.addEventListener('click', (e) => {
      e.preventDefault();
      e.stopPropagation();
      onClick();
    });
    return btn;
  }

  function colorInput(component, field, accessibleName) {
    const input = document.createElement('input');
    input.type = 'color';
    input.className = 'wozduh-style-color';
    input.dataset.componentId = component.id;
    input.dataset.field = field;
    input.value = pickerHex(component, field);
    input.title = accessibleName;
    input.setAttribute?.('aria-label', accessibleName);
    input.addEventListener('input', () => {
      paintColor(component, field, input.value);
    });
    return input;
  }

  function applyVisibility(components, prefs) {
    const factory = (typeof window !== 'undefined') ? window.DDRFactory : null;
    if (!factory?.cutoverActive || typeof factory.setSeriesVisible !== 'function') return;
    if (typeof factory.beginVisibilityBatch === 'function') factory.beginVisibilityBatch();
    try {
      for (const c of components) {
        factory.setSeriesVisible(c.id, isChecked(prefs, c));
      }
    } finally {
      if (typeof factory.endVisibilityBatch === 'function') factory.endVisibilityBatch();
    }
  }

  function rebuildMenu(menu, components, prefs) {
    if (!menu) return;
    menu.replaceChildren();

    const handle = document.createElement('div');
    handle.className = 'indicator-settings-menu__drag-handle';
    handle.title = 'Drag to move';
    handle.textContent = '⋮⋮⋮ Woz Settings';
    menu.appendChild(handle);

    for (const c of components) {
      if (componentKind(c) === 'channel') appendChannelGroup(menu, c, prefs);
      else appendLineRow(menu, c, prefs);
    }

    const allBtn = defaultButton('Default all colors', 'Default all Wozduh colors', () => {
      const factory = ddrFactory();
      if (factory && typeof factory.resetAllWozduhColors === 'function') {
        factory.resetAllWozduhColors();
      }
      refreshColorInputs(menu, components);
    });
    allBtn.className = 'wozduh-style-default wozduh-style-default-all';
    menu.appendChild(allBtn);

    const ok = document.createElement('button');
    ok.type = 'button';
    ok.className = 'risk-save-btn';
    ok.textContent = 'Ok';
    ok.addEventListener('click', (e) => {
      e.preventDefault();
      e.stopPropagation();
      menu.hidden = true;
    });
    menu.appendChild(ok);

    menu._dragBound = false;
    if (typeof FloatingMenu !== 'undefined') {
      FloatingMenu.initDrag(menu);
    } else if (typeof initFloatingMenuDrag === 'function') {
      initFloatingMenuDrag(menu);
    }
  }

  function mountFromManifest(manifest) {
    const components = collectConfigurable(manifest);
    if (!components.length) return;
    let prefs = migrateLegacyPrefs(loadPrefsMap(), components);
    savePrefsMap(prefs);
    document.querySelectorAll('#osc-wrap .wozduh-settings-menu').forEach((menu) => {
      rebuildMenu(menu, components, prefs);
    });
    applyVisibility(components, prefs);
  }

  /** Legacy bridge: prefs object may still use Falcon keys — migrate then apply by id. */
  function applyWozduhPrefs(_context, prefs) {
    const manifest = (typeof window !== 'undefined') ? window.DDRFactory?.manifest : null;
    const components = collectConfigurable(manifest);
    if (!components.length) return;
    const map = migrateLegacyPrefs(prefs && typeof prefs === 'object' ? prefs : loadPrefsMap(), components);
    savePrefsMap(map);
    applyVisibility(components, map);
  }

  function initToolbarToggles() {
    const manifest = (typeof window !== 'undefined') ? window.DDRFactory?.manifest : null;
    if (manifest) mountFromManifest(manifest);
  }

  function refreshFromManifest() {
    const manifest = (typeof window !== 'undefined') ? window.DDRFactory?.manifest : null;
    if (manifest) {
      mountFromManifest(manifest);
      return;
    }
    if (typeof window !== 'undefined' && window.WozduhController) {
      applyWozduhPrefs('live', window.WozduhController.getPrefsForChart('live'));
    }
  }

  function setToggleVisible() {
    return false;
  }

  return {
    applyWozduhPrefs,
    initToolbarToggles,
    refreshFromManifest,
    setToggleVisible,
    mountFromManifest,
    collectConfigurable,
    migrateLegacyPrefs,
    rebuildMenu,
  };
})();

if (typeof window !== 'undefined') {
  window.SettingsRenderer = SettingsRenderer;
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = { SettingsRenderer };
}
