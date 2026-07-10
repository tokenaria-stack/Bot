/**
 * SettingsRenderer — routes legacy toolbar/settings UI to DDR manifest slotIds.
 * Does not rebuild HTML; binds existing checkboxes to DDRFactory.seriesMap.
 */
const SettingsRenderer = (() => {
  /** @type {Map<string, { slotIds: string[], legacyKeys?: string[] }>} */
  const toggleRegistry = new Map();

  const LEGACY_TOGGLE_DEFS = {
    rsx: {
      elementIds: ['tog-jurik'],
      defaultSlotIds: ['line_rsx', 'line_rsx_signal'],
      legacyKeys: ['rsx'],
    },
    volume: {
      elementIds: ['tog-volume'],
      defaultSlotIds: [],
      legacyKeys: ['volume'],
    },
  };

  const WOZDUH_PREF_SLOTS = {
    rsiPrice: { defaultSlotIds: [], legacyKeys: ['rsiPrice'] },
    rsiHl2: { defaultSlotIds: [], legacyKeys: ['rsiHl2'] },
    rsiVol: { defaultSlotIds: ['woz_fast', 'woz_slow'], legacyKeys: ['rsiVolFast', 'rsiVolSlow'] },
  };

  function ddrFactory() {
    return (typeof window !== 'undefined' && window.DDRFactory) ? window.DDRFactory : null;
  }

  function manifestComponents() {
    const manifest = ddrFactory()?.manifest;
    if (!manifest?.panes) return [];
    const out = [];
    for (const components of Object.values(manifest.panes)) {
      if (!Array.isArray(components)) continue;
      for (const c of components) {
        if (c?.id) out.push(c);
      }
    }
    return out;
  }

  function manifestSlotIds(predicate) {
    return manifestComponents()
      .filter((c) => {
        const kind = String(c.kind || 'line').toLowerCase();
        if (kind === 'marker' || c.dataMode === 'annotations') return false;
        return predicate(c);
      })
      .map((c) => c.id);
  }

  function resolveSlotIdsForAlias(alias) {
    const cached = toggleRegistry.get(alias);
    if (cached?.slotIds?.length) return cached.slotIds;

    const def = LEGACY_TOGGLE_DEFS[alias];
    if (!def) return [];

    if (alias === 'rsx') {
      const fromManifest = manifestSlotIds((c) => (
        c.id === 'line_rsx'
        || c.id === 'line_rsx_signal'
        || String(c.id).startsWith('line_rsx')
      ));
      return fromManifest.length ? fromManifest : def.defaultSlotIds;
    }

    return def.defaultSlotIds;
  }

  function resolveWozduhPrefSlots(prefKey) {
    const def = WOZDUH_PREF_SLOTS[prefKey];
    if (!def) return { slotIds: [], legacyKeys: [] };

    if (prefKey === 'rsiVol') {
      const fromManifest = manifestSlotIds((c) => (
        c.id === 'woz_fast' || c.id === 'woz_slow' || String(c.id).startsWith('woz_')
      ));
      return {
        slotIds: fromManifest.length ? fromManifest : def.defaultSlotIds,
        legacyKeys: def.legacyKeys,
      };
    }

    return { slotIds: def.defaultSlotIds, legacyKeys: def.legacyKeys };
  }

  function setDdrSlotsVisible(slotIds, visible) {
    const factory = ddrFactory();
    if (!factory?.cutoverActive) return false;
    let applied = false;
    for (const id of slotIds) {
      const series = factory.getSeries(id);
      if (!series) continue;
      series.applyOptions({ visible: visible !== false });
      applied = true;
    }
    return applied;
  }

  function setLegacySeriesVisible(context, legacyKeys, visible) {
    const chartData = ChartAdapter.getChartHandle(context);
    if (!chartData) return false;
    let applied = false;
    for (const key of legacyKeys || []) {
      if (key === 'rsx' && chartData.rsxSeries) {
        chartData.rsxSeries.applyOptions({ visible: visible !== false });
        chartData.rsxSignalSeries?.applyOptions({ visible: visible !== false });
        applied = true;
      } else if (key === 'volume' && chartData.volumeSeries) {
        chartData.volumeSeries.applyOptions({ visible: visible !== false });
        applied = true;
      } else if (chartData.wozduxSeries?.[key]) {
        chartData.wozduxSeries[key].applyOptions({ visible: visible !== false });
        applied = true;
      }
    }
    return applied;
  }

  function setToggleVisible(context, alias, visible) {
    const def = LEGACY_TOGGLE_DEFS[alias];
    if (!def) return false;

    const slotIds = resolveSlotIdsForAlias(alias);
    const ddrApplied = setDdrSlotsVisible(slotIds, visible);
    if (ddrApplied) return true;

    return setLegacySeriesVisible(context, def.legacyKeys, visible);
  }

  function setWozduhPrefVisible(context, prefKey, visible) {
    const { slotIds, legacyKeys } = resolveWozduhPrefSlots(prefKey);
    if (setDdrSlotsVisible(slotIds, visible)) return true;
    return setLegacySeriesVisible(context, legacyKeys, visible);
  }

  function applyWozduhPrefs(context, prefs) {
    if (!prefs || typeof prefs !== 'object') return;
    for (const prefKey of Object.keys(WOZDUH_PREF_SLOTS)) {
      if (typeof prefs[prefKey] !== 'boolean') continue;
      setWozduhPrefVisible(context, prefKey, prefs[prefKey]);
    }
  }

  function bindToggleElement(el, alias, context = 'live') {
    if (!el) return;
    const apply = () => {
      el.closest('.ind-toggle')?.classList.toggle('active', el.checked);
      setToggleVisible(context, alias, el.checked);
    };
    apply();
    el.addEventListener('change', apply);
  }

  function initToolbarToggles(context = 'live') {
    for (const [alias, def] of Object.entries(LEGACY_TOGGLE_DEFS)) {
      const slotIds = resolveSlotIdsForAlias(alias);
      toggleRegistry.set(alias, { slotIds, legacyKeys: def.legacyKeys });
      for (const id of def.elementIds) {
        bindToggleElement(document.getElementById(id), alias, context);
      }
    }
  }

  function refreshFromManifest() {
    for (const alias of Object.keys(LEGACY_TOGGLE_DEFS)) {
      toggleRegistry.set(alias, {
        slotIds: resolveSlotIdsForAlias(alias),
        legacyKeys: LEGACY_TOGGLE_DEFS[alias].legacyKeys,
      });
    }
  }

  function listManifestToggles() {
    return manifestComponents()
      .filter((c) => {
        const kind = String(c.kind || 'line').toLowerCase();
        return kind !== 'marker' && c.dataMode !== 'annotations';
      })
      .map((c) => {
        let title = c.id;
        try {
          const opts = typeof c.renderOptions === 'string'
            ? JSON.parse(c.renderOptions)
            : (c.renderOptions || {});
          if (opts.title) title = opts.title;
        } catch { /* noop */ }
        return { id: c.id, pane: c.pane, title };
      });
  }

  return {
    initToolbarToggles,
    refreshFromManifest,
    setToggleVisible,
    setWozduhPrefVisible,
    applyWozduhPrefs,
    resolveSlotIdsForAlias,
    listManifestToggles,
  };
})();

if (typeof window !== 'undefined') {
  window.SettingsRenderer = SettingsRenderer;
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = { SettingsRenderer };
}
