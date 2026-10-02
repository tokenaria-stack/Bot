/**
 * TOOL-DOCK-SHELL-V1. Right rail. One tool: show/hide #star-inspection.
 * Does not own Star/Lens/Evaluate.
 */
const ToolDock = (() => {
  const WIDTH = 40;
  let researchOpen = true;
  let root = null;

  function isOpen() {
    return researchOpen;
  }

  function chartInset() {
    const dock = WIDTH;
    if (!researchOpen) return dock;
    const panel = typeof document !== 'undefined' ? document.getElementById('star-inspection') : null;
    if (!panel || panel.hidden) return dock;
    const overlay = panel.classList.contains('is-overlay');
    if (overlay) return dock;
    const w = parseFloat(panel.style.width) || 380;
    return dock + w;
  }

  function apply() {
    if (typeof document === 'undefined') return;
    document.body.style.setProperty('--tool-dock-width', WIDTH + 'px');
    const panel = document.getElementById('star-inspection');
    if (panel) panel.hidden = !researchOpen;
    const btn = root && root.querySelector('[data-tool="research"]');
    if (btn) {
      btn.classList.toggle('is-open', researchOpen);
      btn.setAttribute('aria-pressed', researchOpen ? 'true' : 'false');
    }
    if (typeof StarInspection !== 'undefined' && typeof StarInspection.applyWidth === 'function') {
      StarInspection.applyWidth();
    }
  }

  function setOpen(on) {
    researchOpen = !!on;
    apply();
    return researchOpen;
  }

  function toggle() {
    return setOpen(!researchOpen);
  }

  function ensureDom() {
    if (typeof document === 'undefined' || root) return;
    const host = document.getElementById('workspace-main');
    if (!host) return;
    root = document.createElement('nav');
    root.id = 'tool-dock';
    root.setAttribute('aria-label', 'Tools');
    root.innerHTML =
      '<button type="button" class="tool-dock-btn is-open" data-tool="research" title="Research" aria-pressed="true" aria-label="Research">' +
      '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">' +
      '<rect x="4" y="5" width="10" height="14" rx="1"></rect>' +
      '<path d="M16 8h4M16 12h4M16 16h4"></path>' +
      '</svg></button>';
    host.appendChild(root);
    root.addEventListener('click', (ev) => {
      const btn = ev.target && ev.target.closest ? ev.target.closest('[data-tool="research"]') : null;
      if (!btn) return;
      toggle();
    });
  }

  function init() {
    ensureDom();
    apply();
  }

  function _resetForTests() {
    researchOpen = true;
    root = null;
  }

  return { init, isOpen, setOpen, toggle, chartInset, WIDTH, _resetForTests };
})();

if (typeof document !== 'undefined') {
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', () => ToolDock.init());
  else ToolDock.init();
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = ToolDock;
}
