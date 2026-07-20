/**
 * Phase 19.5 — Timeframe toolbar (favorites, dropdown, live/backtest switching).
 */
const TimeframeController = (() => {
  let tfFavorites = [];

  function tfLabel(id) {
    return TF_DISPLAY[id] || id;
  }

  function tfSortKey(id) {
    const s = id.toLowerCase();
    if (s.includes('tick')) return 100 + (parseInt(s, 10) || 1);
    if (/^\d+s$/.test(s)) return 1000 + parseInt(s, 10);
    if (/^\d+m$/.test(s)) return 2000 + parseInt(s, 10);
    if (/^\d+h$/.test(s)) return 3000 + parseInt(s, 10);
    if (s === '1d') return 4001;
    if (s === '1w') return 4002;
    return 9000;
  }

  function sortFavorites() {
    tfFavorites.sort((a, b) => tfSortKey(a) - tfSortKey(b));
  }

  function loadFavorites() {
    try {
      const raw = localStorage.getItem(LS_FAV_KEY);
      tfFavorites = raw ? JSON.parse(raw) : [...DEFAULT_FAVS];
    } catch {
      tfFavorites = [...DEFAULT_FAVS];
    }
    tfFavorites = tfFavorites.filter((id) => id !== '1M');
    sortFavorites();
  }

  function saveFavorites() {
    sortFavorites();
    localStorage.setItem(LS_FAV_KEY, JSON.stringify(tfFavorites));
  }

  function isFavorite(id) {
    return tfFavorites.includes(id);
  }

  function toggleFavorite(id) {
    if (isFavorite(id)) {
      tfFavorites = tfFavorites.filter((f) => f !== id);
    } else {
      tfFavorites.push(id);
    }
    saveFavorites();
    renderTfBar();
    renderTfMenu();
  }

  function getActiveTfFromToolbar() {
    const activeBtn = document.querySelector('#tf-favorites .tf-btn.active');
    return activeBtn?.dataset?.tf || null;
  }

  function getActiveTf() {
    if (TabsController.isBacktestTfContext()) {
      return normalizeTf(getBacktestInterval() || backtestTf || getActiveTfFromToolbar());
    }
    return currentTf;
  }

  function setTfDropdownOpen(open) {
    const dd = document.getElementById('tf-dropdown');
    if (!dd) return;
    dd.hidden = !open;
    dd.classList.toggle('open', open);
  }

  function syncToolbar() {
    const activeTf = getActiveTf();
    const tfBtn = document.getElementById('tf-current-btn');
    const tfLabelEl = document.getElementById('timeframe-label');
    if (tfBtn) tfBtn.textContent = `${tfLabel(activeTf)} ▾`;
    if (tfLabelEl) tfLabelEl.textContent = tfLabel(activeTf);
    renderTfBar();
    renderTfMenu();
  }

  function renderTfBar() {
    const favEl = document.getElementById('tf-favorites');
    if (!favEl) {
      console.warn('[TimeframeController] #tf-favorites not found');
      return;
    }
    const activeTf = getActiveTf();
    favEl.innerHTML = '';
    tfFavorites.forEach((id) => {
      const btn = document.createElement('button');
      btn.type = 'button';
      btn.className = 'tf-btn' + (id === activeTf ? ' active' : '');
      btn.textContent = tfLabel(id);
      btn.dataset.tf = id;
      favEl.appendChild(btn);
    });
    const tfBtn = document.getElementById('tf-current-btn');
    const tfLabelEl = document.getElementById('timeframe-label');
    if (tfBtn) tfBtn.textContent = `${tfLabel(activeTf)} ▾`;
    if (tfLabelEl) tfLabelEl.textContent = tfLabel(activeTf);
  }

  function renderTfMenu() {
    const body = document.getElementById('tf-menu-body');
    if (!body) return;
    const activeTf = getActiveTf();
    body.innerHTML = '';

    Object.entries(TF_MENU).forEach(([group, items]) => {
      const label = document.createElement('div');
      label.className = 'tf-group-label';
      label.textContent = group;
      body.appendChild(label);

      items.forEach((item) => {
        const row = document.createElement('div');
        row.className = 'tf-menu-item' + (item.id === activeTf ? ' selected' : '');

        const name = document.createElement('button');
        name.type = 'button';
        name.className = 'tf-menu-name';
        name.textContent = item.label;
        name.dataset.tf = item.id;

        const star = document.createElement('button');
        star.type = 'button';
        star.className = 'tf-star' + (isFavorite(item.id) ? ' fav' : '');
        star.textContent = '★';
        star.title = 'Add to favorites';
        star.dataset.tf = item.id;

        row.appendChild(name);
        row.appendChild(star);
        body.appendChild(row);
      });
    });
  }

  function switchLiveTimeframe(tf) {
    const resolved = resolveTf(tf);
    if (!resolved) return;
    if (resolved === '1M') return;

    const changed = resolved !== currentTf;
    let viewportAnchor = null;
    const prevTf = currentTf;

    if (changed) {
      abortLiveStateFetch();
      window.projectionEpoch = (Number(window.projectionEpoch) || 0) + 1;
      if (ChartAdapter.isInitialized('live') && typeof ViewportManager !== 'undefined') {
        const captured = ViewportManager.capture('live');
        // Null capture / null intent → fresh camera (never synthetic restore on blank/poison scale).
        if (captured) {
          viewportAnchor = typeof ViewportManager.cameraIntentForTfSwitch === 'function'
            ? ViewportManager.cameraIntentForTfSwitch(captured, prevTf, resolved)
            : captured;
        }
      }
    }

    currentTf = resolved;
    localStorage.setItem(LS_TF_KEY, resolved);
    historyHasMore = true;
    disarmLiveHistoryScroll();
    syncToolbar();

    if (changed) {
      // Shot 11C: soft handoff after currentTf is set so the tick buffer binds the new TF.
      if (typeof prepareLiveTfHandoff === 'function') {
        prepareLiveTfHandoff();
      } else if (typeof clearChartData === 'function') {
        clearChartData({ keepProjection: true });
      }
    }

    loadDashboard({ userTfChange: changed, viewportAnchor });

    if (refreshTimer) clearInterval(refreshTimer);
    if (orderFlowPollTimer) clearInterval(orderFlowPollTimer);
    wsSubscribeTf(resolved);
    if (!WS.isOpen()) {
      startLivePollTimer();
    }
    if (isOrderFlowTf(resolved)) {
      orderFlowPollTimer = setInterval(pollOrderFlowState, 500);
      ChartAdapter.applyOrderFlowTimeScale(true);
    } else {
      ChartAdapter.applyOrderFlowTimeScale(false);
    }
    // Buffering overlay owned by loadDashboard (Atomic Publish) — do not clear here.
  }

  function switchBacktestTimeframe(tf, event) {
    if (event) {
      event.preventDefault();
      event.stopPropagation();
    }
    handleBacktestIntervalChange(tf);
  }

  function switchTimeframe(tf, event) {
    if (event) {
      event.preventDefault();
      event.stopPropagation();
    }
    const nextTf = resolveTf(tf);
    if (!nextTf) return;

    if (TabsController.isBacktestTfContext()) {
      switchBacktestTimeframe(nextTf, event);
      return;
    }

    switchLiveTimeframe(nextTf);
  }

  function applyCustomTf() {
    const input = document.getElementById('tf-custom-input');
    const val = input.value.trim();
    if (!val) return;
    input.value = '';
    switchTimeframe(val);
    setTfDropdownOpen(false);
  }

  function bindTfBarInteraction() {
    if (window.__tfBarInteractionBound) return;

    const tfBar = document.getElementById('tf-bar');
    const tfDropdown = document.getElementById('tf-dropdown');
    const tfCurrentBtn = document.getElementById('tf-current-btn');
    if (!tfBar || !tfDropdown || !tfCurrentBtn) {
      console.warn('[TimeframeController] TF bar elements missing (#tf-bar, #tf-dropdown, or #tf-current-btn)');
      return;
    }

    window.__tfBarInteractionBound = true;

    const handleTfBarClick = (e) => {
      if (e.target.closest('#tf-current-btn')) {
        e.preventDefault();
        e.stopPropagation();
        setTfDropdownOpen(tfDropdown.hidden);
        return;
      }

      const favBtn = e.target.closest('.tf-btn');
      if (favBtn?.dataset?.tf) {
        e.preventDefault();
        e.stopPropagation();
        switchTimeframe(favBtn.dataset.tf, e);
        return;
      }

      const menuBtn = e.target.closest('.tf-menu-name');
      if (menuBtn?.dataset?.tf) {
        e.preventDefault();
        e.stopPropagation();
        switchTimeframe(menuBtn.dataset.tf, e);
        setTfDropdownOpen(false);
        return;
      }

      const starBtn = e.target.closest('.tf-star');
      if (starBtn?.dataset?.tf) {
        e.preventDefault();
        e.stopPropagation();
        toggleFavorite(starBtn.dataset.tf);
      }
    };

    tfBar.addEventListener('click', handleTfBarClick, true);

    document.getElementById('tf-custom-add')?.addEventListener('click', () => {
      applyCustomTf();
    });

    document.getElementById('tf-custom-input')?.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') applyCustomTf();
    });

    document.addEventListener('click', (e) => {
      if (e.target.closest('#tf-bar')) return;
      setTfDropdownOpen(false);
    });
  }

  function init(options = {}) {
    const { useServerTf = false } = options;
    loadFavorites();
    if (!useServerTf) {
      currentTf = resolveTf(localStorage.getItem(LS_TF_KEY) || '1m') || '1m';
      if (currentTf === '1M') {
        currentTf = '1w';
        localStorage.setItem(LS_TF_KEY, currentTf);
      }
    }
    renderTfBar();
    renderTfMenu();
    bindTfBarInteraction();
  }

  return {
    init,
    syncToolbar,
    getActiveTf,
    getActiveTfFromToolbar,
    switchTimeframe,
    switchLiveTimeframe,
    switchBacktestTimeframe,
    loadFavorites,
    renderTfBar,
    renderTfMenu,
    setTfDropdownOpen,
    toggleFavorite,
    isFavorite,
  };
})();

if (typeof window !== 'undefined') {
  window.TimeframeController = TimeframeController;
}
