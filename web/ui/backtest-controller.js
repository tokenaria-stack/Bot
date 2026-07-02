/**
 * Phase 19.5.4 — Backtest form, loading UI, and stats dashboard (dumb UI controller).
 */
const BacktestController = (() => {
  let runCallback = null;
  let stopCallback = null;
  let intervalChangeCallback = null;
  let statsMode = 'backtest';
  let lastBacktestResult = null;
  let runActive = false;

  function parseBacktestDateInput(value) {
    if (!value) return null;
    const d = new Date(`${value}T00:00:00Z`);
    return Number.isNaN(d.getTime()) ? null : d;
  }

  function formatBacktestDateInput(d) {
    return d.toISOString().slice(0, 10);
  }

  function limitBacktestDateRange(interval, startDate, endDate) {
    const end = parseBacktestDateInput(endDate) || new Date();
    return {
      startDate,
      endDate: endDate || formatBacktestDateInput(end),
      limited: false,
    };
  }

  function expandBacktestStartDate(startDate, endDate, days) {
    const start = parseBacktestDateInput(startDate);
    if (!start || !Number.isFinite(days) || days <= 0) return startDate;
    const expanded = new Date(start);
    expanded.setUTCDate(expanded.getUTCDate() - days);
    return formatBacktestDateInput(expanded);
  }

  function applyDateRangeLimits(interval) {
    const startEl = document.getElementById('bt-start');
    const endEl = document.getElementById('bt-end');
    const endDate = endEl?.value || formatBacktestDateInput(new Date());
    const startDate = startEl?.value || '';
    const clamped = limitBacktestDateRange(interval, startDate, endDate);
    if (startEl && clamped.startDate) startEl.value = clamped.startDate;
    if (endEl && clamped.endDate) endEl.value = clamped.endDate;
    return clamped;
  }

  function setDefaultBacktestDates() {
    const end = new Date();
    const start = new Date(end);
    start.setDate(start.getDate() - 30);
    const fmt = (d) => d.toISOString().slice(0, 10);
    const startEl = document.getElementById('bt-start');
    const endEl = document.getElementById('bt-end');
    if (startEl && !startEl.value) startEl.value = fmt(start);
    if (endEl && !endEl.value) endEl.value = fmt(end);
  }

  function shiftBacktestDate(inputId, deltaMonths) {
    const el = document.getElementById(inputId);
    if (!el) return;
    const seed = el.value || new Date().toISOString().slice(0, 10);
    const d = new Date(`${seed}T12:00:00Z`);
    d.setUTCMonth(d.getUTCMonth() + deltaMonths);
    el.value = d.toISOString().slice(0, 10);
  }

  function initBacktestDateNav() {
    document.querySelectorAll('[data-date-nav]').forEach((btn) => {
      btn.addEventListener('click', (e) => {
        e.preventDefault();
        const id = btn.dataset.dateNav;
        const dir = Number(btn.dataset.dir) || 0;
        if (id && dir) shiftBacktestDate(id, dir);
      });
    });
  }

  function formatStatNum(value, digits = 2) {
    const n = Number(value);
    if (!Number.isFinite(n)) return '0.00';
    return n.toFixed(digits);
  }

  function setStatValue(id, value, digits = 2, colorize = false) {
    const el = document.getElementById(id);
    if (!el) return;
    el.textContent = formatStatNum(value, digits);
    if (!colorize) return;
    el.classList.remove('positive', 'negative');
    const n = Number(value);
    if (n > 0) el.classList.add('positive');
    else if (n < 0) el.classList.add('negative');
  }

  function formatStatTime(sec) {
    const n = Number(sec);
    if (!Number.isFinite(n) || n <= 0) return '—';
    return new Date(n * 1000).toLocaleString();
  }

  function renderStats(data, mode = statsMode) {
    if (!data) {
      setStatValue('stat-total-trades', 0, 0, false);
      setStatValue('stat-win-rate', 0, 2, false);
      setStatValue('stat-net-profit', 0, 2, true);
      setStatValue('stat-profit-factor', 0, 2, false);
      setStatValue('stat-max-drawdown', 0, 2, false);
      setStatValue('stat-recovery-factor', 0, 2, false);
      ChartAdapter.setEquityData([]);
      const tbody = document.getElementById('trade-history-body');
      if (tbody) {
        const hint = mode === 'backtest'
          ? 'Run a backtest to populate statistics'
          : 'No trades yet';
        tbody.innerHTML = `<tr><td colspan="8" class="stats-empty">${hint}</td></tr>`;
      }
      return;
    }

    setStatValue('stat-total-trades', data.totalTrades, 0, false);
    setStatValue('stat-win-rate', data.winRate, 2, false);
    setStatValue('stat-net-profit', data.netProfit, 2, true);
    setStatValue('stat-profit-factor', data.profitFactor, 2, false);
    setStatValue('stat-max-drawdown', data.maxDrawdown, 2, false);
    setStatValue('stat-recovery-factor', data.recoveryFactor, 2, false);

    if (Array.isArray(data.equityCurve)) {
      ChartAdapter.setEquityData(
        data.equityCurve.map((p) => ({ time: p.time, value: p.value })),
      );
      ChartAdapter.fitEquityContent();
    }

    const tbody = document.getElementById('trade-history-body');
    if (!tbody) return;

    const trades = (data.trades || []).map(normalizeTradeRow);
    if (trades.length === 0) {
      const hint = mode === 'backtest'
        ? 'Run a backtest to populate statistics'
        : 'No trades yet';
      tbody.innerHTML = `<tr><td colspan="8" class="stats-empty">${hint}</td></tr>`;
      return;
    }

    tbody.innerHTML = trades.map((t) => {
      const pnlClass = t.pnl >= 0 ? 'pnl-pos' : 'pnl-neg';
      const pnlSign = t.pnl >= 0 ? '+' : '';
      const feeParts = [];
      if (Number.isFinite(t.fee) && t.fee > 0) feeParts.push(`$${formatStatNum(t.fee, 2)}`);
      if (Number.isFinite(t.slippagePct) && t.slippagePct > 0) {
        feeParts.push(`${formatStatNum(t.slippagePct, 3)}% slip`);
      }
      const feeCell = feeParts.length ? feeParts.join(' / ') : '—';
      return `<tr>
      <td>${formatStatTime(t.entryTime)}</td>
      <td>${formatStatTime(t.exitTime)}</td>
      <td>${t.side}</td>
      <td>${formatStatNum(t.entryPrice, 2)}</td>
      <td>${formatStatNum(t.exitPrice, 2)}</td>
      <td>${feeCell}</td>
      <td class="${pnlClass}">${pnlSign}${formatStatNum(t.pnl, 2)}%</td>
      <td>${t.exitReason}</td>
    </tr>`;
    }).join('');
  }

  async function refreshStatsForMode(mode) {
    statsMode = mode || 'backtest';
    document.querySelectorAll('.stats-mode-selector .mode-btn').forEach((btn) => {
      btn.classList.toggle('active', btn.dataset.mode === statsMode);
    });

    if (statsMode === 'backtest') {
      renderStats(lastBacktestResult, statsMode);
      return;
    }

    try {
      const payload = await API.fetchStats(statsMode);
      renderStats(payload, statsMode);
    } catch (err) {
      console.warn('Stats fetch failed:', err);
      renderStats({
        totalTrades: 0,
        winRate: 0,
        netProfit: 0,
        maxDrawdown: 0,
        profitFactor: 0,
        recoveryFactor: 0,
        trades: [],
        equityCurve: [],
      }, statsMode);
    }
  }

  function initStatsModeSelector() {
    const root = document.querySelector('.stats-mode-selector');
    if (!root) return;
    root.querySelectorAll('.mode-btn').forEach((btn) => {
      btn.addEventListener('click', () => {
        const mode = btn.dataset.mode;
        if (mode) refreshStatsForMode(mode);
      });
    });
  }

  function setRunActive(active) {
    runActive = active;
    const runBtn = document.getElementById('btn-run-backtest');
    const stopBtn = document.getElementById('btn-stop-backtest');
    if (runBtn) {
      runBtn.disabled = active;
      runBtn.textContent = active ? 'Running…' : 'Run';
    }
    if (stopBtn) stopBtn.disabled = !active;
  }

  function getFormValues() {
    const symbol = document.getElementById('bt-symbol')?.value.trim() || 'BTCUSDT';
    const interval = document.getElementById('bt-interval')?.value || '15m';
    const start = document.getElementById('bt-start')?.value || '';
    const end = document.getElementById('bt-end')?.value || '';
    return { symbol, start, end, interval };
  }

  function setFormValues({ symbol, start, end, interval } = {}) {
    if (symbol != null) {
      const el = document.getElementById('bt-symbol');
      if (el) el.value = symbol;
    }
    if (start != null) {
      const el = document.getElementById('bt-start');
      if (el) el.value = start;
    }
    if (end != null) {
      const el = document.getElementById('bt-end');
      if (el) el.value = end;
    }
    if (interval != null) {
      const el = document.getElementById('bt-interval');
      if (el) el.value = interval;
    }
  }

  function setLoading(visible) {
    const el = document.getElementById('backtest-loading');
    if (!el) return;
    el.classList.toggle('hidden', !visible);
  }

  function onRunRequested(callback) {
    runCallback = callback;
  }

  function onStopRequested(callback) {
    stopCallback = callback;
  }

  function onIntervalChange(callback) {
    intervalChangeCallback = callback;
  }

  function setInterval(interval) {
    const tf = String(interval || '').trim();
    if (!tf) return null;
    const el = document.getElementById('bt-interval');
    if (!el) return tf;
    const hasOption = [...el.options].some((o) => o.value === tf);
    if (!hasOption) {
      const opt = document.createElement('option');
      opt.value = tf;
      opt.textContent = TF_DISPLAY[tf] || tf;
      el.appendChild(opt);
    }
    el.value = tf;
    return tf;
  }

  function initIntervalHandler() {
    const el = document.getElementById('bt-interval');
    if (!el || el.dataset.autoHandlerBound === '1') return;
    el.dataset.autoHandlerBound = '1';
    el.addEventListener('change', () => {
      intervalChangeCallback?.(el.value);
    });
  }

  function init() {
    setDefaultBacktestDates();
    initBacktestDateNav();
    initStatsModeSelector();
    initIntervalHandler();
    document.getElementById('btn-run-backtest')?.addEventListener('click', () => runCallback?.());
    document.getElementById('btn-stop-backtest')?.addEventListener('click', () => stopCallback?.());
  }

  function storeBacktestResult(result) {
    lastBacktestResult = result;
    if (statsMode === 'backtest') {
      renderStats(result, 'backtest');
    }
  }

  return {
    init,
    getFormValues,
    setFormValues,
    setLoading,
    setRunActive,
    renderStats,
    onRunRequested,
    onStopRequested,
    onIntervalChange,
    setInterval,
    applyDateRangeLimits,
    expandBacktestStartDate,
    refreshStatsForMode,
    storeBacktestResult,
    getStatsMode: () => statsMode,
    isRunActive: () => runActive,
  };
})();

window.BacktestController = BacktestController;
