/**
 * Phase 28 — Backtest data plane: epoch guard, API I/O, store writes.
 */
const BacktestPipeline = (() => {
  let _epoch = 0;

  function isStale(epoch) {
    return epoch !== _epoch;
  }

  function shellFormContext() {
    const form = typeof BacktestController !== 'undefined' ? BacktestController.getFormValues() : {};
    const symbol = form.symbol || 'BTCUSDT';
    const interval = form.interval || '15m';
    const tf = typeof normalizeTf === 'function' ? normalizeTf(interval) : interval;

    const startStr = form.start || form.startDate;
    const endStr = form.end || form.endDate;

    let endMs = Date.now();
    let startMs = endMs - (30 * 24 * 60 * 60 * 1000);

    if (startStr) {
      startMs = new Date(`${startStr}T00:00:00Z`).getTime();
    }
    if (endStr) {
      endMs = new Date(`${endStr}T00:00:00Z`).getTime() + (24 * 60 * 60 * 1000 - 1);
    }

    const endTimeSec = Math.floor(endMs / 1000);
    const reqStartSec = Math.floor(startMs / 1000);
    const intervalMs = typeof getIntervalMs === 'function' ? getIntervalMs(tf) : 60 * 60 * 1000;
    const limit = Math.ceil((endMs - startMs) / intervalMs) + 100;

    return {
      form,
      symbol,
      interval,
      tf,
      endMs,
      startMs,
      endTimeSec,
      reqStartSec,
      limit,
    };
  }

  async function loadShell(options = {}) {
    const bumpEpoch = options.bumpEpoch !== false;
    const currentEpoch = bumpEpoch ? ++_epoch : (options.parentEpoch ?? _epoch);
    const force = options.force === true;
    const ctx = shellFormContext();

    if (!force) {
      const fp = backtestStore.getFingerprint();
      if (
        fp
        && fp.symbol === ctx.symbol
        && fp.interval === ctx.tf
        && fp.startSec === ctx.reqStartSec
        && fp.endSec === ctx.endTimeSec
        && backtestStore.hasBaseLayer()
      ) {
        return;
      }
    }

    ChartAdapter.ensureBacktestChart();

    try {
      if (typeof BacktestController !== 'undefined') BacktestController.setLoading(true);
      const params = new URLSearchParams({
        symbol: ctx.symbol,
        interval: ctx.tf,
        endTime: String(ctx.endMs),
        limit: String(Math.min(ctx.limit, 100000)),
      });
      if (typeof appendBacktestRsxSettingsToParams === 'function') {
        appendBacktestRsxSettingsToParams(params);
      }
      const { ok, data } = await API.fetchBacktestHistoryChunk(params);

      if (isStale(currentEpoch)) {
        console.log('[Pipeline] Stale drop');
        return;
      }

      if (!ok || !Array.isArray(data?.chartData) || data.chartData.length === 0) {
        console.warn('[Backtest] No history chunk data for shell load', {
          symbol: ctx.symbol,
          interval: ctx.tf,
          endTimeSec: ctx.endTimeSec,
        });
        if (typeof backtestHistoryHasMore !== 'undefined') {
          backtestHistoryHasMore = false;
        }
        return;
      }

      backtestStore.seal();
      backtestStore.replaceFromServer(
        chartPointsToStorePayload(data.chartData, data.annotations),
        ctx.tf,
      );
      backtestStore.unseal();

      if (typeof backtestHistoryHasMore !== 'undefined') {
        backtestHistoryHasMore = data.hasMore !== false;
      }
      backtestStore.setFingerprint(ctx.symbol, ctx.tf, ctx.reqStartSec, ctx.endTimeSec);

      if (typeof ChartProjection !== 'undefined') {
        ChartProjection.renderBacktest({ mode: 'full', reason: 'fresh_run' });
      }
    } catch (err) {
      console.error('Failed to load backtest shell:', err);
    } finally {
      if (typeof BacktestController !== 'undefined') BacktestController.setLoading(false);
    }
  }

  async function run(runOptions = {}) {
    const currentEpoch = ++_epoch;
    const {
      payload: initialPayload,
      tf,
      needsBaseReload = false,
      isNavRefresh = false,
      signal = null,
      skipSettingsPush = false,
      manageLoading = true,
    } = runOptions;

    if (!initialPayload) {
      throw new Error('BacktestPipeline.run requires payload');
    }

    if (!backtestStore.hasBaseLayer() || needsBaseReload) {
      console.log(`[Orchestrator] Base layer missing or outdated. Forcing reload for ${initialPayload.symbol} ${tf}`);
      await loadShell({ force: true, bumpEpoch: false, parentEpoch: currentEpoch });
      if (isStale(currentEpoch)) {
        console.log('[Pipeline] Stale drop');
        return;
      }
    }

    ChartAdapter.ensureBacktestChart();

    if (manageLoading && typeof BacktestController !== 'undefined') {
      BacktestController.setLoading(true);
    }

    let payload = { ...initialPayload };
    payload.simOnly = backtestStore.hasBaseLayer() && !needsBaseReload;

    try {
      if (!skipSettingsPush && typeof pushRsxSettingsToServer === 'function') {
        await pushRsxSettingsToServer(coerceRsxSettingsForAPI(RsxController.getSettings('backtest')));
        if (isStale(currentEpoch)) {
          console.log('[Pipeline] Stale drop');
          return;
        }
      }

      if (backtestStore.hasBaseLayer() && !needsBaseReload && !isNavRefresh) {
        payload.settings = {
          ...payload.settings,
          skipNavigators: true,
        };
      }

      if (!payload.settings?.matrix || !payload.settings?.navigators) {
        throw new Error('Backtest payload missing matrix or navigators in settings');
      }

      let result;
      for (let attempt = 0; attempt < 2; attempt++) {
        const { ok, status, result: respResult, rawText } = await API.runBacktest(payload, signal);
        result = respResult;

        if (isStale(currentEpoch)) {
          console.log('[Pipeline] Stale drop');
          return;
        }

        const errText = result?.error || result?.message || rawText || '';
        const notEnoughCandles = status === 400 && /not enough candles/i.test(errText);

        if (attempt === 0 && notEnoughCandles) {
          const expanded = BacktestController.expandBacktestStartDate(payload.startDate, payload.endDate, 90);
          if (expanded && expanded !== payload.startDate) {
            console.warn(`[Backtest] Auto-expanding startDate ${payload.startDate} → ${expanded} (retry after: ${errText})`);
            if (typeof buildFinalBacktestPayload === 'function') {
              payload = buildFinalBacktestPayload({
                symbol: payload.symbol,
                interval: payload.interval,
                startDate: expanded,
                endDate: payload.endDate,
              });
            } else {
              payload = { ...payload, startDate: expanded };
            }
            payload.simOnly = backtestStore.hasBaseLayer() && !needsBaseReload;
            if (backtestStore.hasBaseLayer() && !needsBaseReload && !isNavRefresh) {
              payload.settings = { ...payload.settings, skipNavigators: true };
            }
            BacktestController.setFormValues({ start: expanded });
            continue;
          }
        }

        if (result?._parseError) {
          console.error('[FALCON NETWORK] Server returned invalid JSON. Raw response:', rawText);
          throw new Error(`Server response is not valid JSON. Check console for raw text. Status: ${status}`);
        }

        if (ok) {
          break;
        }

        const errorText = errText || `HTTP error ${status}`;
        alert(`Ошибка Бэктеста:\n${errorText}`);
        return;
      }

      if (!payload.simOnly && result.chartData && result.chartData.length > 0) {
        console.log(`[Orchestrator] Applying full base layer from engine fallback (${result.chartData.length} candles)`);
        backtestStore.seal();

        const storePayload = typeof chartPointsToStorePayload === 'function'
          ? chartPointsToStorePayload(result.chartData, result.annotations)
          : { candles: result.chartData, oscillators: result.chartData, annotations: result.annotations };

        backtestStore.replaceFromServer(storePayload, tf);
        backtestStore.unseal();

        const newStartSec = backtestStore.firstCandleTimeSec();
        const newEndSec = backtestStore.lastCandleTimeSec();
        if (newStartSec != null) {
          backtestStore.setFingerprint(payload.symbol, tf, newStartSec, newEndSec);
        }
      } else {
        backtestStore.patchBacktestData(result, tf);
      }

      if (typeof backtestStore.setTrades === 'function') {
        backtestStore.setTrades(result.trades || []);
      }
      if (typeof backtestLastTrades !== 'undefined') {
        backtestLastTrades = result.trades || [];
      }

      if (typeof ChartProjection !== 'undefined') {
        if (!payload.simOnly && result.chartData && result.chartData.length > 0) {
          ChartProjection.renderBacktest({
            mode: 'full',
            reason: 'fresh_run',
            navigators: result.navigators,
          });
        } else {
          ChartProjection.renderBacktest({
            mode: 'overlay',
            reason: 'fresh_run',
            navigators: result.navigators,
          });
        }
      }

      if (typeof BacktestController !== 'undefined') {
        BacktestController.storeBacktestResult(result);
      }
      if (typeof NavigatorController !== 'undefined') {
        NavigatorController.renderChartLegends('backtest');
      }
    } catch (err) {
      if (err?.name === 'AbortError') {
        return;
      }
      const msg = err?.message || String(err);
      console.error('Backtest failed:', err);
      alert(`Ошибка Бэктеста:\n${msg}`);
    } finally {
      if (typeof BacktestController !== 'undefined') BacktestController.setLoading(false);
    }
  }

  return {
    get epoch() { return _epoch; },
    loadShell,
    run,
  };
})();

if (typeof window !== 'undefined') {
  window.BacktestPipeline = BacktestPipeline;
}
