/**
 * LegendRenderer — dumb DDR chrome: pane title + settings gear (no live metrics).
 */
const LegendRenderer = (() => {
  function paneTitle(paneId) {
    if (paneId === 'rsx') return 'RSX';
    if (!paneId) return '';
    return paneId.charAt(0).toUpperCase() + paneId.slice(1);
  }

  function toggleClass(paneId) {
    return `${paneId}-settings-toggle`;
  }

  function bindGear(btn, legendEl) {
    btn.addEventListener('click', (e) => {
      e.preventDefault();
      e.stopPropagation();
      const wrap = legendEl.closest('.chart-wrap');
      const menu = wrap?.querySelector('.indicator-settings-menu');
      if (!menu) return;
      menu.hidden = !menu.hidden;
    });
  }

  function mountFromManifest(_manifest) {
    document.querySelectorAll('.chart-legend[data-context="live"]').forEach((legendEl) => {
      const paneId = String(legendEl.dataset.pane || '').trim();
      if (paneId !== 'wozduh' && paneId !== 'rsx') return;

      const title = paneTitle(paneId);
      legendEl.innerHTML =
        `<span class="pane-title" style="font-weight:bold;color:var(--tv-text);margin-right:8px;">${title}</span>`
        + `<button type="button" class="settings-toggle-btn ${toggleClass(paneId)}" data-target="${paneId}" title="Settings" aria-label="Settings">⚙</button>`;

      const gear = legendEl.querySelector('.settings-toggle-btn');
      if (gear) bindGear(gear, legendEl);
    });
  }

  return { mountFromManifest };
})();

if (typeof window !== 'undefined') {
  window.LegendRenderer = LegendRenderer;
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = { LegendRenderer };
}
