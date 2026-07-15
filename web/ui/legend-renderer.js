/**
 * LegendRenderer — dumb DDR chrome: pane title + eye + settings gear.
 */
const LegendRenderer = (() => {
  function paneTitle(paneId) {
    if (paneId === 'wozduh') return 'Woz';
    if (paneId === 'rsx') return 'RSX';
    if (!paneId) return '';
    return paneId.charAt(0).toUpperCase() + paneId.slice(1);
  }

  function toggleClass(paneId) {
    return `${paneId}-settings-toggle`;
  }

  function bindEye(btn, legendEl) {
    btn.addEventListener('click', (e) => {
      e.preventDefault();
      e.stopPropagation();
      const wrap = legendEl.closest('.chart-wrap');
      const host = wrap?.querySelector('.lwc-host');
      if (!host) return;
      const hidden = host.style.visibility === 'hidden';
      host.style.visibility = hidden ? 'visible' : 'hidden';
      btn.classList.toggle('is-dimmed', !hidden);
      btn.title = hidden ? 'Show pane' : 'Hide pane';
    });
  }

  function bindGear(btn, legendEl) {
    btn.addEventListener('click', (e) => {
      e.preventDefault();
      e.stopPropagation();
      const wrap = legendEl.closest('.chart-wrap');
      const menu = wrap?.querySelector('.indicator-settings-menu');
      if (!menu) return;
      if (typeof FloatingMenu !== 'undefined') {
        FloatingMenu.toggle(menu, btn);
      } else {
        menu.hidden = !menu.hidden;
      }
    });
  }

  function mountFromManifest(_manifest) {
    document.querySelectorAll('.chart-legend[data-context="live"]').forEach((legendEl) => {
      const paneId = String(legendEl.dataset.pane || '').trim();
      if (paneId !== 'wozduh' && paneId !== 'rsx') return;

      const title = paneTitle(paneId);
      legendEl.innerHTML =
        `<span class="pane-title" style="font-weight:bold;color:var(--tv-text);margin-right:8px;">${title}</span>`
        + `<button type="button" class="visibility-toggle-btn" data-target="${paneId}" title="Hide pane" aria-label="Toggle pane visibility">👁</button>`
        + `<button type="button" class="settings-toggle-btn ${toggleClass(paneId)}" data-target="${paneId}" title="Settings" aria-label="Settings">⚙</button>`;

      const eye = legendEl.querySelector('.visibility-toggle-btn');
      const gear = legendEl.querySelector('.settings-toggle-btn');
      if (eye) bindEye(eye, legendEl);
      if (gear) bindGear(gear, legendEl);
    });

    if (typeof FloatingMenu !== 'undefined') FloatingMenu.bindAll();
  }

  return { mountFromManifest };
})();

if (typeof window !== 'undefined') {
  window.LegendRenderer = LegendRenderer;
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = { LegendRenderer };
}
