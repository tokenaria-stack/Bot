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
   * Migrate legacy Falcon-menu keys (rsiVol, rsiPrice, …) onto component.id keys.
   * Pure data map — no hard-coded apply to series beyond known legacy aliases.
   */
  function migrateLegacyPrefs(prefs, components) {
    const next = { ...prefs };
    // Legacy single toggle for both wt lines.
    if (typeof prefs.rsiVol === 'boolean') {
      if (next.woz_fast === undefined) next.woz_fast = prefs.rsiVol;
      if (next.woz_slow === undefined) next.woz_slow = prefs.rsiVol;
    }
    const legacyToId = {
      rsiPrice: 'woz_rsi_price',
      emaRsi: 'woz_ema_rsi',
      rsiRsi: 'woz_rsi_rsi',
      rsiHl2: 'woz_rsi_hl2',
      macdRsi: 'woz_macd_rsi',
      rsiAd: 'woz_rsi_ad',
      rsiHl2Vol: 'woz_rsi_hl2_vol',
      priceChan: 'woz_price_chan_mid',
    };
    for (const [legacy, id] of Object.entries(legacyToId)) {
      if (typeof prefs[legacy] === 'boolean' && next[id] === undefined) {
        next[id] = prefs[legacy];
      }
    }
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

  function applyVisibility(components, prefs) {
    const factory = (typeof window !== 'undefined') ? window.DDRFactory : null;
    if (!factory?.cutoverActive || typeof factory.setSeriesVisible !== 'function') return;
    for (const c of components) {
      factory.setSeriesVisible(c.id, isChecked(prefs, c));
    }
  }

  function rebuildMenu(menu, components, prefs) {
    if (!menu) return;
    menu.replaceChildren();
    for (const c of components) {
      const label = document.createElement('label');
      label.className = 'menu-row';
      const input = document.createElement('input');
      input.type = 'checkbox';
      input.className = 'wozduh-chk';
      input.dataset.componentId = c.id;
      input.checked = isChecked(prefs, c);
      input.addEventListener('change', () => {
        const map = loadPrefsMap();
        map[c.id] = input.checked;
        savePrefsMap(map);
        const factory = (typeof window !== 'undefined') ? window.DDRFactory : null;
        if (factory?.cutoverActive && typeof factory.setSeriesVisible === 'function') {
          factory.setSeriesVisible(c.id, input.checked);
        }
      });
      label.appendChild(input);
      label.appendChild(document.createTextNode(` ${labelFor(c)}`));
      menu.appendChild(label);
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
  };
})();

if (typeof window !== 'undefined') {
  window.SettingsRenderer = SettingsRenderer;
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = { SettingsRenderer };
}
