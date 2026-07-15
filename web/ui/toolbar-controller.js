/**
 * Phase 19.5.6 — Status bar, toolbar toggles, and live overlays.
 */
const ToolbarController = (() => {
  let cachedSandboxMode = false;

  function setTextIfChanged(el, next) {
    if (el && el.textContent !== next) el.textContent = next;
  }

  function fmt(v) {
    return typeof v === 'number' && Number.isFinite(v) ? v.toFixed(2) : '—';
  }

  function fmtVolume(v) {
    if (!Number.isFinite(v) || v <= 0) return '—';
    if (v >= 1e9) return `${(v / 1e9).toFixed(2)} B`;
    if (v >= 1e6) return `${(v / 1e6).toFixed(2)} M`;
    if (v >= 1e3) return `${(v / 1e3).toFixed(2)} K`;
    return v.toFixed(2);
  }

  function resolveTfLabel(state) {
    const tf = state.timeframe || state.tradingTimeframe
      || (typeof getActiveTf === 'function' ? getActiveTf() : null)
      || '1m';
    return TF_DISPLAY[tf] || tf;
  }

  function plotTip(plots, id) {
    if (!plots || typeof plots !== 'object') return null;
    const v = Number(plots[id]);
    return Number.isFinite(v) ? v : null;
  }

  function updateHeaderData(state) {
    if (!state) return;

    if (typeof state.sandboxMode !== 'undefined') {
      cachedSandboxMode = !!state.sandboxMode;
    }
    const isSandbox = typeof state.sandboxMode !== 'undefined'
      ? !!state.sandboxMode
      : cachedSandboxMode;

    if (state.symbol) {
      setTextIfChanged(document.getElementById('symbol'), state.symbol || 'BTCUSDT');
    }
    if (state.timeframe || state.tradingTimeframe) {
      setTextIfChanged(document.getElementById('timeframe-label'), resolveTfLabel(state));
    }

    if ('volatilityRegime' in state) {
      const regime = state.volatilityRegime || '';
      const regimeEl = document.getElementById('regime');
      if (regimeEl) {
        setTextIfChanged(regimeEl, regime || '—');
        const regimeClass = regime ? `regime meta-val ${regime}` : 'regime meta-val';
        if (regimeEl.className !== regimeClass) regimeEl.className = regimeClass;
      }
    }

    const plots = state.plots;
    // Tip SSOT: DDR plot ids (Stage 5). Legacy redLine/greenLine ignored.
    const jurik = plotTip(plots, 'line_rsx') ?? (state.jurik != null ? Number(state.jurik) : null);
    // Header R = legacy RedLine semantics → RSI(HL2); G = EMA(RSI).
    const red = plotTip(plots, 'woz_rsi_hl2') ?? plotTip(plots, 'woz_rsi_price');
    const green = plotTip(plots, 'woz_ema_rsi');

    if (jurik != null) {
      setTextIfChanged(document.getElementById('jurik-val'), fmt(jurik));
    }
    if (red != null) {
      setTextIfChanged(document.getElementById('red-val'), fmt(red));
    }
    if (green != null) {
      setTextIfChanged(document.getElementById('green-val'), fmt(green));
    }

    const sandboxEl = document.getElementById('sandbox-badge');
    if (sandboxEl && sandboxEl.classList.contains('active') !== isSandbox) {
      sandboxEl.classList.toggle('active', isSandbox);
    }
  }

  function updateRsxValue(val, color) {
    const el = document.getElementById('rsx-val');
    if (!el) return;
    const n = Number(val);
    if (!Number.isFinite(n)) {
      el.textContent = '—';
      return;
    }
    el.textContent = n.toFixed(1);
    if (color) el.style.color = color;
  }

  function updateOscHeader(pt) {
    if (!pt) return;
    updateHeaderData({
      plots: pt.plots,
      jurik: pt.rsx ?? pt.jurik,
    });
    const rsxVal = plotTip(pt.plots, 'line_rsx') ?? parseFloat(pt.rsx ?? pt.jurik);
    if (Number.isFinite(rsxVal)) {
      updateRsxValue(rsxVal, pt.color || RSX_DEFAULT_COLOR);
    }
  }

  function setBuffering(isVisible) {
    const el = document.getElementById('orderflow-buffer');
    if (!el) return;
    el.style.display = isVisible ? 'flex' : 'none';
  }

  function updateVolume(candles) {
    const el = document.getElementById('volume-val');
    if (!el || !candles?.length) return;
    const last = candles[candles.length - 1];
    el.textContent = fmtVolume(last.volume);
    el.style.color = last.close >= last.open
      ? ((typeof ChartTheme !== 'undefined') ? ChartTheme.bull : TV.green)
      : ((typeof ChartTheme !== 'undefined') ? ChartTheme.bear : TV.red);
  }

  function isSpikeEnabled() {
    return document.getElementById('tog-spike')?.checked ?? true;
  }

  function isFibEnabled() {
    return document.getElementById('tog-fib')?.checked ?? false;
  }

  function setRulerActive(active) {
    document.getElementById('ruler-btn')?.classList.toggle('active', !!active);
  }

  function init() {
    if (typeof SettingsRenderer !== 'undefined') {
      SettingsRenderer.initToolbarToggles('live');
    } else {
      const toggles = {
        'tog-jurik': 'rsx',
        'tog-volume': 'volume',
      };
      Object.entries(toggles).forEach(([id, seriesKey]) => {
        const el = document.getElementById(id);
        if (!el) {
          console.warn(`[ToolbarController] #${id} not found`);
          return;
        }
        const applyVisibility = () => {
          el.closest('.ind-toggle')?.classList.toggle('active', el.checked);
          ChartAdapter.setToggleSeriesVisible('live', seriesKey, el.checked);
        };
        applyVisibility();
        el.addEventListener('change', applyVisibility);
      });
    }

    const togSpike = document.getElementById('tog-spike');
    if (togSpike) {
      togSpike.addEventListener('change', (e) => {
        e.target.closest('.ind-toggle')?.classList.toggle('active', e.target.checked);
        ChartAdapter.applyAllMarkers();
      });
    } else {
      console.warn('[ToolbarController] #tog-spike not found');
    }

    const togFib = document.getElementById('tog-fib');
    if (togFib) {
      togFib.addEventListener('change', (e) => {
        e.target.closest('.ind-toggle')?.classList.toggle('active', e.target.checked);
        if (typeof ChartAdapter.renderFib === 'function') {
          ChartAdapter.renderFib(typeof lastFibZones !== 'undefined' ? lastFibZones : []);
        }
      });
    } else {
      console.warn('[ToolbarController] #tog-fib not found');
    }

    const rulerBtn = document.getElementById('ruler-btn');
    if (rulerBtn) {
      rulerBtn.addEventListener('click', () => {
        if (typeof toggleRuler === 'function') toggleRuler();
      });
    } else {
      console.warn('[ToolbarController] #ruler-btn not found');
    }

    document.addEventListener('keydown', (e) => {
      if (e.key !== 'Escape') return;
      if (document.querySelector('.fullscreen-pane')) {
        if (typeof exitFullscreenPane === 'function') exitFullscreenPane();
        return;
      }
      if (typeof ruler !== 'undefined' && ruler.active && typeof resetRuler === 'function') {
        resetRuler();
      }
    });

    const chartTypeGroup = document.getElementById('chart-type-group');
    if (chartTypeGroup) {
      chartTypeGroup.querySelectorAll('.seg-btn').forEach((btn) => {
        btn.addEventListener('click', () => {
          ChartAdapter.setChartType(btn.dataset.chart);
          chartTypeGroup.querySelectorAll('.seg-btn').forEach((b) => b.classList.remove('active'));
          btn.classList.add('active');
        });
      });
    } else {
      console.warn('[ToolbarController] #chart-type-group not found');
    }

    document.getElementById('btn-clear-cache')?.addEventListener('click', async () => {
      if (!confirm('Очистить кэш базы данных и памяти на сервере?')) return;
      try {
        const resp = await fetch('/api/cache/clear', { method: 'POST' });
        if (resp.ok) {
          alert('Кэш успешно очищен!');
        } else {
          alert('Ошибка очистки: ' + await resp.text());
        }
      } catch (err) {
        console.error('Ошибка:', err);
      }
    });
  }

  return {
    init,
    updateHeaderData,
    updateRsxValue,
    updateOscHeader,
    setBuffering,
    updateVolume,
    isSpikeEnabled,
    isFibEnabled,
    setRulerActive,
    getSandboxMode: () => cachedSandboxMode,
  };
})();

window.ToolbarController = ToolbarController;
