/**
 * FloatingMenu — fixed-to-viewport popovers (TV-like): open up from gear, drag, outside close.
 * Single document outside-click listener (idempotent). No chart-relative absolute positioning.
 */
const FloatingMenu = (() => {
  /** @type {WeakMap<HTMLElement, HTMLElement>} menu → last anchor */
  const anchors = new WeakMap();
  let outsideBound = false;
  /** @type {((menu: HTMLElement) => void | Promise<void>) | null} */
  let beforeOutsideClose = null;

  function setBeforeOutsideClose(fn) {
    beforeOutsideClose = typeof fn === 'function' ? fn : null;
  }

  function ensureOutsideClose() {
    if (outsideBound) return;
    outsideBound = true;
    document.addEventListener('mousedown', (event) => {
      if (window.__menuDragging) return;
      const target = event.target;
      if (!(target instanceof Element)) return;
      const toClose = [];
      document.querySelectorAll('.indicator-settings-menu:not([hidden])').forEach((menu) => {
        if (menu.contains(target)) return;
        const anchor = anchors.get(menu);
        if (anchor && (anchor === target || anchor.contains(target))) return;
        if (target.closest?.('.settings-toggle-btn, .wozduh-settings-toggle, .rsx-settings-toggle')) {
          // Gear click handles its own toggle.
          return;
        }
        toClose.push(menu);
      });
      if (!toClose.length) return;
      // ADR-014 / B1: flush pending indicator applies before close (Figma-style).
      void (async () => {
        for (const menu of toClose) {
          try {
            if (beforeOutsideClose) await beforeOutsideClose(menu);
          } catch (err) {
            console.warn('[FloatingMenu] beforeOutsideClose failed:', err);
          }
          menu.hidden = true;
        }
      })();
    }, true);
  }

  /** @returns {'up'|'down'} from data-float-direction (default up — gear / bottom-pane). */
  function floatDirection(menu) {
    return menu?.dataset?.floatDirection === 'down' ? 'down' : 'up';
  }

  /**
   * Open menu fixed to viewport. Direction from data-float-direction:
   * up (default) = above anchor; down = below (command-bar Ind, etc.).
   * @param {HTMLElement} menu
   * @param {HTMLElement} anchorBtn
   */
  function open(menu, anchorBtn) {
    if (!menu || !anchorBtn) return;
    ensureOutsideClose();
    anchors.set(menu, anchorBtn);

    document.querySelectorAll('.indicator-settings-menu').forEach((m) => {
      if (m !== menu) m.hidden = true;
    });

    const rect = anchorBtn.getBoundingClientRect();
    const dir = floatDirection(menu);
    menu.classList.add('indicator-settings-menu--floating');
    menu.style.position = 'fixed';
    menu.style.right = 'auto';
    menu.style.left = `${Math.max(8, rect.left)}px`;
    menu.style.zIndex = '20000';

    if (dir === 'down') {
      menu.style.bottom = 'auto';
      menu.style.top = `${rect.bottom + 4}px`;
    } else {
      // Grow upward from the gear top edge (avoids bottom clip on chart panes).
      menu.style.top = 'auto';
      menu.style.bottom = `${Math.max(8, window.innerHeight - rect.top)}px`;
    }

    menu.hidden = false;

    // Clamp horizontally if menu overflows the right edge.
    const menuW = menu.offsetWidth || 196;
    const maxLeft = window.innerWidth - menuW - 8;
    if (rect.left > maxLeft) {
      menu.style.left = `${Math.max(8, maxLeft)}px`;
    }

    // Clamp vertically when opening downward near the bottom of the viewport.
    if (dir === 'down') {
      const menuH = menu.offsetHeight || 120;
      let top = rect.bottom + 4;
      if (top + menuH > window.innerHeight - 8) {
        top = Math.max(8, window.innerHeight - menuH - 8);
      }
      menu.style.top = `${top}px`;
    }

    initDrag(menu);
  }

  function close(menu) {
    if (menu) menu.hidden = true;
  }

  function toggle(menu, anchorBtn) {
    if (!menu) return;
    if (!menu.hidden) {
      close(menu);
      return;
    }
    open(menu, anchorBtn);
  }

  function initDrag(menu) {
    if (!menu || menu._dragBound) return;
    const handle = menu.querySelector('.indicator-settings-menu__drag-handle');
    if (!handle) return;
    menu._dragBound = true;

    let dragging = false;
    let startX = 0;
    let startY = 0;
    let startLeft = 0;
    let startTop = 0;

    const onMove = (event) => {
      if (!dragging) return;
      const dx = event.clientX - startX;
      const dy = event.clientY - startY;
      const menuW = menu.offsetWidth || 196;
      const menuH = menu.offsetHeight || 240;
      const left = Math.max(8, Math.min(startLeft + dx, window.innerWidth - menuW - 8));
      const top = Math.max(8, Math.min(startTop + dy, window.innerHeight - menuH - 8));
      menu.style.left = `${left}px`;
      menu.style.top = `${top}px`;
    };

    const stopDrag = () => {
      if (!dragging) return;
      dragging = false;
      window.__menuDragging = false;
      document.removeEventListener('mousemove', onMove);
      document.removeEventListener('mouseup', stopDrag);
    };

    handle.addEventListener('mousedown', (event) => {
      if (event.button !== 0) return;
      event.preventDefault();
      event.stopPropagation();
      const rect = menu.getBoundingClientRect();
      // Switch from bottom-anchored open to top/left for free drag.
      menu.style.bottom = 'auto';
      menu.style.top = `${rect.top}px`;
      menu.style.left = `${rect.left}px`;
      dragging = true;
      window.__menuDragging = true;
      startX = event.clientX;
      startY = event.clientY;
      startLeft = rect.left;
      startTop = rect.top;
      document.addEventListener('mousemove', onMove);
      document.addEventListener('mouseup', stopDrag);
    });
  }

  /** Bind drag on every settings menu currently in the DOM (Woz + RSX). */
  function bindAll() {
    ensureOutsideClose();
    document.querySelectorAll('.indicator-settings-menu').forEach(initDrag);
  }

  return {
    open,
    close,
    toggle,
    initDrag,
    bindAll,
    setBeforeOutsideClose,
  };
})();

if (typeof window !== 'undefined') {
  window.FloatingMenu = FloatingMenu;
  window.openFloatingMenu = (menu, anchor) => FloatingMenu.open(menu, anchor);
  window.initFloatingMenuDrag = (menu) => FloatingMenu.initDrag(menu);
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = { FloatingMenu };
}
