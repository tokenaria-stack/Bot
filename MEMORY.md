# Trading Bot — System Memory

**Перед написанием новых модулей ВСЕГДА перечитывай этот файл.**

> **Снэпшот MEMORY (июль 2026):** **Core 2.0 (DAG) — backend transport ✅, frontend orchestration ✅ (Phase 7A–7B), annotations wire ✅ (Phase 8A).** Live chart **нестабилен** — активная отладка viewport / prepend / DDR cutover. Legacy Falcon + DAG dual replay на columnar path. **Frontend Phase 20 ✅** + **Phase 28 ✅ (backtest black screen).** Следующий фокус: **интеграция Core 2.0** (annotations UI 8B, chart stability, SettingsRenderer Phase 7 debt). См. раздел **Core 2.0 — DDR Pipeline** и **OPEN DEBTS #35–#48**.

---

# РОЛЬ АССИСТЕНТА: Senior Quantitative Trading Architect

Ты — **Senior Quantitative Trading Developer** с 20-летним опытом работы в топовых проп-трейдинговых фондах (HFT / алгоритмическая торговля). Специализация: проектирование устойчивых, масштабируемых торговых систем на Go, глубокая математика индикаторов и методы фильтрации рыночного шума.

## Компетенции

**Математическая строгость:** индикаторы не «чёрные ящики». Понимание формул, чувствительности к волатильности и лагов. Борьба с look-ahead bias, перерисовкой (repaint) и отравлением данных на микро-тиках.

**Институциональный подход:** многоуровневая фильтрация сигналов, динамические веса, regime detection, управление риском на уровне генерации сигнала.

**Современные подходы:** анализ временных рядов, ML для скоринга сигналов (без переобучения), сложные MTF-системы.

**Протокол ювелира:** чистота архитектуры. Без дублирования, без костылей в виде промежуточных структур-клонов (типа `Report`), без неявных мутаций состояния. Код чистый, с тестами, SOLID.

**Перед кодом:** перечитай этот файл + сверься с Протоколом ювелира ниже.

---

# ПРОТОКОЛ ЮВЕЛИРА (THE JEWELER'S PROTOCOL)

**Приоритет: P0.** Любой код, рефакторинг или архитектурное предложение обязаны соответствовать этим принципам.

Данный протокол определяет фундаментальную инженерную культуру этого проекта. Любой генерируемый код, рефакторинг или архитектурное предложение должны строго соответствовать этим принципам.

## 1. Никакой изоленты (Root Cause Resolution)
Мы не маскируем баги бэкенда костылями на фронтенде. Если математика индикаторов на живом рынке сходит с ума, мы спускаемся на уровень ядра и правим движок. Любые визуальные хаки для скрытия алгоритмических ошибок строжайше запрещены.

## 2. Изоляция и Телеметрия (No Premature Summing)
Мы не смешиваем данные в нечитаемую кашу. Каждый торговый фактор или сигнал должен быть математически изолирован, чтобы его можно было протестировать и вывести на UI отдельно. Мы используем паттерн `ScoreDecision` → `map[string]ScoreFactor`, где каждый индикатор отвечает только за свой голос.

## 3. Иммунитет к мутациям (Snapshot & Rollback)
Ядро бота (Go-бэкенд) обязано обладать иммунитетом к "отравлению" внутрибаровыми тиками (intra-bar ticks). Все stateful-индикаторы (RSI, Wozduh, буферы дивергенций, Layer 2) обязаны реализовывать паттерн O(1) Snapshot/Restore. Данные открытой свечи никогда не должны накапливаться и портить исторический контекст.

## 4. Абсолютная честность времени (Zero Look-Ahead Bias)
На бэктесте строжайше запрещено "заглядывание в будущее". Мульти-таймфреймный анализ (MTF) должен работать исключительно через механизм Step-function (Граничное кэширование / Boundary Tracker). Анализатор имеет право видеть только те свечи старшего ТФ, время закрытия которых меньше или равно времени текущего тика.

## 5. Архитектурная прямота (No Shotgun Surgery)
Данные должны течь от источника к потребителю по кратчайшему пути. Мы не плодим промежуточные структуры-ведра (подобные удаленному `Report`) для банального копирования полей. Мы поддерживаем чистоту терминологии: никаких устаревших названий (вроде `Scalp`), если модуль универсален. Если легаси-код мешает — мы выкорчевываем его, а не строим обходные пути.

**ДИРЕКТИВА АССИСТЕНТУ:** Перед написанием любого кода или изменением архитектуры сверяйся с "Протоколом ювелира". Работай как хирург: локализуй проблему, спроектируй чистое решение на уровне ядра, сохрани изоляцию сигналов.

---

## Архитектура Принятия Решений (Score Module)

Проект использует прямой пайплайн данных: `Marker` → `ScoreEngine.Calculate()` → `ScoreDecision`.

- Отсутствуют структуры-посредники (никаких `Report`).
- Данные не суммируются преждевременно. Каждый индикатор формирует свой изолированный `ScoreFactor` (`BUY` / `SELL` / `WAIT`) внутри карты `ScoreDecision.Factors`.
- Модули должны строго соответствовать **Протоколу ювелира**.

| Компонент | Файл | Роль |
|-----------|------|------|
| `ScoreEngine.Calculate(marker, matrix)` | `strategy/scoring.go` | Читает `Marker` напрямую |
| `ScoreDecision` | `strategy/score_types.go` | `LongScore`, `ShortScore`, `Factors`, `Action`, `LotMod`, `StopDist` |
| `ScoringMatrix` | `strategy/scoring_matrix.go` | UI-тогглы факторов |
| `CalculateScoreGlobal(marker)` | `strategy/scoring.go` | Live: matrix из `config/matrix.json` |
| MTF scoring | `scoreMTFFactors` + `scoreHTFOscillatorFactors` | `UseTrendlines` → `marker.MTFStates()`; `UseHTFOscillators` → `RSX_{tf}`, `Wozduh_{tf}` |
| Qdrant | `Marker.VectorSnapshot()` | `vector_db.ReportSnapshot` (не `strategy.Report`) |

**Поток (live / backtest):**
```
UpdateKlineTick → ScoreEngine.Calculate → RawAction/FinalAction (same initially)
                → ApplyExecutionVetoes (warmup veto + Analyst + Chief) → IsVetoed / FinalAction=WAIT
                → longScore / shortScore / factors / rawAction / veto → API / WS / chart
```

**Warmup veto (Phase 5.70):** `!marker.HasMinBars(50)` → `IsVetoed=true`, `VetoReason="System Warmup: Not enough history"`, `FinalAction=WAIT`. Factors пусты до прогрева (scoreSnapshot gate). UI: оранжевый veto banner.

**HTF Regime scoring (Phase 5.69):** `WozduhUp > WozduhDown` → BUY +15; обратное → SELL +15. **Не cross.** RSX HTF: oversold/overbought +20. Telemetry: HTF factors всегда в `Factors` (включая neutral score 0) через `mergeHTFFactors`.

**Raw vs Final:** `RawAction` — телеметрия индикаторов; `FinalAction` — исполнение после veto. Factors/scores не затираются при veto.

**Телеметрия (Phase 5.61–6.0):** `MarketState.factors`, `tickPayload.factors`, `BacktestChartPoint.Factors` → UI `#telemetry-compact-line`. **Live SSOT (Phase 6):** scoring/regime — только **closed-bar snapshot** (`MasterGeneral.closedTelemetry`, `Marker.ClosedVolatilityRegime` из `layer2Snap`); `ScoreDecisionForTelemetry` — read-only, **без** `syncMTFState`; MTF sync только в `StartDataFeed` на `tick.IsClosed`. WS `tickPayload.isClosed` + `volatilityRegime`; UI обновляет панель скоринга/regime только при `isClosed` или `/api/state`. Toolbar R/G — единое поле `redLine`/`greenLine` (REST + WS). **CrosshairMove** — intra-bar preview hint, не DOM. **Click-to-Lock:** guard в `updateScoringUI` при `lockedHistoricalData`.

**MTF:** см. раздел «MTF: Два режима» ниже.

---

## Ядро: Marker и Layer Pipeline

`Marker` (`strategy/analyst.go`, `layer2.go`) — единый **streaming-state** на символ/ТФ. Все индикаторы пишут в него; `ScoreEngine` читает через accessors (`FalconSnapshot`, `VolatilityStateSnapshot`, `MTFStates`, …).

**Поток на каждом тике (`UpdateKlineTick`):**
```
Binance WS kline
  → Marker.UpdateKlineTick(k, isClosed)
  → evaluateTickLocked(k, barIndex, isClosed)
       1. restoreLayer2StreamingState()     // O(1) rollback открытого бара
       2. FalconEngine.Evaluate             // Layer 1
       3. VolatilityEngine, orangeRsi, ad, ao, stoch, ZigZag
       4. divEngine (micro-tick + macro snapshot на swing)
       5. rsxMarkers, geometry, fib zones
       6. saveLayer2StreamingState()       // только if isClosed
```

| Слой | Модуль | Выход (смысл) |
|------|--------|----------------|
| **Layer 1 — Falcon** | `FalconEngine` | Jurik RSX (`rsxSource` snapshot at init — не глобал), Wozduh, vol-cross |
| **Layer 2 — Volatility** | `VolatilityEngine` | ATR, Bollinger, `MarketRegime` (NORMAL / CLIMAX / …) → `LotMod`, `StopDist` в scoring |
| **Layer 2 — Chaos** | `ao`, `ad`, `stoch` | AO zero-cross, AD flow, stoch — флаги и inputs для `ScoreFactor` |
| **Divergence** | `SmartDivergenceEngine` | Ring-buffers + ZigZag swing snapshots → `divSignal` |
| **Repaint (без snapshot)** | `zigzag`, `geometry` | Swing nodes, fib — намеренно мутируют на открытом баре |

**Snapshot/Rollback (P0):** Falcon (`rsxSource` pinned), Volatility, orangeRsi, ad, stoch, ao, divEngine, **rsxMarkerState** (`rsxMarkerConfig` snapshot — source/lookback/pivotRadius/useFractal). Паттерн: `restore` → `evaluate` → `save` if closed.

---

## MTF: Два режима работы (Dual-Mode)

Два независимых контура. **Не смешивать.**

### Scoring MTF (торговая математика, backtest + live)

```
WalkForwardMTFTracker.Update(tickSec, chartKlines)
  → GetCandlesStrictlyBefore (zero look-ahead)
  → BuildHTFNavigatorLayer per HTF
  → evaluateHTFOscillators (isolated FalconEngine per HTF)
  → marker.SetCurrentMTFState(states)
  → ScoreEngine.scoreMTFFactors / scoreHTFOscillatorFactors
  → ScoreDecision.Factors["mtf_*" | "RSX_4h" | "Wozduh_4h" | ...]
```

- **Step-function:** HTF state обновляется только на границе закрытия старшей свечи (`nextUpdateSec`).
- **Live (✅ 5.68–5.70):** `MasterGeneral.mtfTracker`; `SetNavigatorPanes` + `NotifyKlineGapFillComplete`; `syncMTFState` на bar close **до** telemetry; `BindMaster` в dashboard.
- **Prefetch:** `MinHTFPrefetchBars = 100` — глубина HTF истории для прогрева RSX.
- **Влияет на:** `ScoreDecision`, live entries, backtest.

### UI MTF (навигаторы, overlay)

```
HTFProvider.GetKlines / PinKlines (cache: symbol_interval)
  → BuildAllNavigators / mergeHTFNavigatorLayers
  → NavigatorResultDTO → MarketState.navigators / backtest response
  → web: mtfNavigatorLayers, TrendlinePrimitive overlay
```

- **Backtest:** финальный `BuildAllNavigators` в конце прогона — «красивая картинка», **не** walk-forward scoring state.
- **Live:** `refreshLiveNavigatorFromServer`, MTF checkboxes, overlay без полного `setData` свечей.
- **Не участвует** в `ScoreEngine` на mid-chart бэктеста (кроме walk-forward кэша выше).

---

## API & WebSocket Контракт

**Сервер:** `server/webserver.go` (`:8080`), статика `web/`.

| Endpoint | Метод | DTO / назначение |
|----------|-------|------------------|
| `/api/state` | GET | `MarketState` — bootstrap дашборда: `candles`, `oscillators`, `navigators`, `longScore`, `shortScore`, **`factors`**, `fibZones`, `trades`. Query: `tf`, `endTime`, `rsxLookback`, limit |
| `/api/history` | GET | `historyResponse` — lazy prepend истории |
| `/api/history/chunk` | GET | `historyChunkResponse` — пагинация chart points |
| `/api/backtest/run` | POST | `BacktestRequest` → `BacktestResult` с `chartData[]` (**`ChartPoint.factors`**), `trades`, `equityCurve`, `navigators`, `cancelled?` |
| `/api/backtest/stop` | POST | Отмена текущего прогона (`backtestRunManager` + `context.Cancel`); partial `BacktestResult` |
| `/api/stats` | GET | `?mode=live\|paper` → `SessionStats` (метрики + trades + equity) из `domain.TradeHistoryStore` |
| `/api/settings/*` | GET/POST | thresholds, matrix, indicators, risk, navigators |
| `/api/cache/clear` | POST | сброс HTF cache + gap-fill flags |
| `/ws` | WS | `wsEnvelope{ type, data }` |

**WebSocket типы:**
- `tick` → `tickPayload`: OHLCV, osc lines (1m), `longScore`, `shortScore`, **`factors`**, `brainStatus`, `aiStatus`
- `marker` → `markerPayload`: trade entry/exit на графике

**Live dual-channel (HTTP bootstrap + WS push):**
1. `boot()` → `initCharts` → `loadDashboard` (**Pre-Fetch Assembly**, Phase 20.4)
2. `GET /api/state` — быстрый RAM-хвост (`LIVE_STATE_CANDLE_LIMIT=300`); `GET /api/history` — глубина до `HISTORY_CHUNK_LIMIT=3000`
3. Store: `seal` → `replaceFromServer` + `prependHistory` → **один** `renderState` → `fitLiveChartsContent`
4. WS `tick` → `liveStore.upsert*` → `ChartAdapter.applyDelta` (только если `!liveStore.isSealed()`)
5. WS игнорируется до `chartInitialized`
6. `pollLatestState` — tail `LIVE_POLL_CANDLE_LIMIT=5`

**Scoring на wire:** `scoreDecisionForAnalyst` → `master.ScoreDecisionForTelemetry` (closed-bar cache, read-only) → `MarketState` / `tickPayload`.

---

## Frontend Data Pipeline (Phase 19.5–20)

**Принцип (Jeweler):** данные текут `API/WS → TimeNormalizer → ChartDataStore (SSOT) → ChartAdapter (LWC facade) → canvas`. `app.js` — только оркестратор. Никаких прямых `setData` из ingress.

### Карта модулей

| Модуль | Файл | Роль |
|--------|------|------|
| **CONFIG** | `web/config.js` | `LIVE_STATE_CANDLE_LIMIT`, `HISTORY_CHUNK_LIMIT`, chart styles |
| **TimeNormalizer** | `web/time-normalizer.js` | `snapToGrid(tf)`, `mergeCandles` (LWW volume — Phase 20.4B) |
| **ChartDataStore** | `web/store.js` | `Map<snappedMs>` candles/osc; `annotations` flat grid; `seal/unseal` |
| **Mappers** | `web/mappers.js` | Pure transforms: `toCandles`, spike/wozduh markers из annotation grid |
| **ChartAdapter** | `web/chart-adapter.js` | LWC lifecycle, `ms→sec` на границе, delta/full apply, scroll hooks |
| **ViewportManager** | `web/ui/viewport-manager.js` | `capture/restore` по `centerTimeMs`; **без** logical-index shift |
| **API** | `web/api.js` | `fetchLiveState`, `fetchLiveHistory`, backtest |
| **UI controllers** | `web/ui/*.js` | TF, toolbar, backtest, RSX, navigators, tabs, layout |
| **Orchestrator** | `web/app.js` | `loadDashboard`, `renderState`, `maybeLoadHistory`, WS handlers |
| **Dormant** | `web/indicators_dormant.js` | Карантин неактивных индикаторов |

**Удалено:** `web/viewport.js`, `Viewport.captureViewport`, `__pendingAnchor`, prepend index-shift hacks, legacy RSX incremental paths (`rsx_div_fractal.go`, `rsx_div_tv.go`, `rsx_incremental.go`).

### Pre-Fetch Assembly (Phase 20.4E–F)

Молниеносный старт Go (`AnalystBootKlineLimit=400`) + полная глубина на UI **без** увеличения boot:

```
loadDashboard (window.__isDashboardLoading = true)
  → fetchLiveState()           // RAM tail, limit=300
  → neededBars = 3000 - candles.length
  → if neededBars > 0:
       fetchLiveHistory(oldestSec, neededBars)   // ровно недостающее до 3000
  → liveStore.seal()
  → replaceFromServer(state) + prependHistory(history)
  → renderState({ isPreFetch: true, storeReady: true })  // один paint
  → fitLiveChartsContent()     // весь диапазон в экран
  → liveStore.unseal(); __isDashboardLoading = false
```

**TF switch:** `TimeframeController` → `ViewportManager.capture('live')` → `loadDashboard({ viewportAnchor })` → `restore` (не fit).

### ViewportManager (Phase 20.4C–F)

**Capture:** `{ centerTimeMs, visibleBars, isAtRightEdge }` — якорь по времени, не по logical index.

**Restore:**
- **Center:** binary search `candlesArray()` по `timeMs` → `syncVisibleLogicalRange(..., { animate: false })`
- **Right edge:** `from = lastIndex - visibleBars`, `to = lastIndex` — **без** `scrollToRealTime()` (анимация триггерила ложную пагинацию)

**`lockPriceAutoScaleDuring`:** MTF overlay / plugin update без прыжка Y.

### Store Seal & Pagination Guards (Phase 20.3–20.4F)

| Guard | Где | Зачем |
|-------|-----|-------|
| `liveStore.seal()` | `loadDashboard`, `renderState`, `maybeLoadHistory` | WS delta не портит batch ingress |
| `liveStore.isSealed()` | `getLatestDeltaForChart`, scroll hooks | Intra-bar paint заблокирован |
| `window.__isDashboardLoading` | `loadDashboard` finally | Блок первичной пагинации |
| `liveStore.isSealed() \|\| __isDashboardLoading` | `scheduleHistoryLoad`, `maybeLoadHistory`, `chart-adapter` `onScrollHistory` | Нет цепной реакции при `setData`/viewport jump |

**Lazy scroll:** `liveHistoryScrollArmed` — arm на wheel/pointerdown; `maybeLoadHistory` при `range.from < LIVE_HISTORY_SCROLL_THRESHOLD`.

### Annotation Grid (Phase 20.2)

`annotations: Map<snappedMs, flatProps>` — spike/wozduh/rsx labels. `_mergeAnnotationProps` sticky flags. Marker cache в `ChartAdapter` (`_cachedPriceMarkers`).

### Go-константы live boot (Phase 20.4B)

| Константа | Файл | Значение | Смысл |
|-----------|------|----------|-------|
| `AnalystBootKlineLimit` | `strategy/live_kline.go` | **400** | SQLite/REST при старте процесса |
| `IndicatorWarmupBars` | `strategy/rsx_pipeline.go` | **300** | Trim холодного прогрева RSX в API output |
| `LiveKlineRAMCap` | `strategy/live_kline.go` | **3000** | Ring buffer в RAM per Marker |

**Volume bug fix (20.4B):** `mergeCandles` — Last-Write-Wins для `volume` (не суммировать cumulative WS volume).

**Time scale sync (20.4C):** `chartTimeFormatBundle()` — `Intl.DateTimeFormat` + `tickMarkFormatter` на `SHARED_TIME_SCALE`; `applyOrderFlowTimeScale` на все live charts.

### Backtest View-Plane (Phase 28 — ✅ black screen resolved)

**Принцип (Reactive Simplicity):** Data-plane **слеп к DOM**. View-plane — идемпотентный `trySync()`.

```
BacktestPipeline → backtestStore.replace/patch → markViewDirty(intent)
Tabs / RO / runBacktest.finally → ChartProjection.trySync()  // ping only
trySync: hasBaseLayer ∧ container>0 → ensureBacktestChart → applyFullData → scrollToPosition(0)
```

| Модуль | Файл | Роль |
|--------|------|------|
| **backtestStore** | `web/store.js` | IIFE wrapper: `ChartDataStore('backtest')` + `markViewDirty` / `consumeViewDirty` |
| **BacktestPipeline** | `web/ui/backtest-pipeline.js` | Epoch guard, API I/O, store writes only — **без** `ChartAdapter` / `ChartProjection` |
| **ChartProjection** | `web/ui/chart-projection.js` | `trySync()` — единственный paint authority; lazy init; independent RO на `#backtest-chart-container` |
| **TabsController** | `web/ui/tabs-controller.js` | `switchTab('tab-backtest')` → `trySync()` + `loadShell().then(trySync)` |
| **Orchestrator** | `web/app.js` | `runBacktest` finally → `trySync()`; `loadBacktestHistoryShell` → pipeline |

**View intent:** `{ mode: 'full'|'overlay', viewport: 'fresh'|'preserve'|'restore' }` — full после shell/engine fallback; overlay после simOnly patch.

**Viewport policy:** `scrollToPosition(0, false)` на fresh (как Live edge), не `fitContent()` на 50k баров.

**`_needsInitialPaint`:** сбрасывается только если `candleSeries` существует после paint (anti poison-pill).

#### Root cause black screen (Phase 28 RCA)

| Симптом | Ложная гипотеза | Реальная причина |
|---------|-----------------|------------------|
| Чёрный canvas, store 435+ свечей | Pipeline / fitContent / spinner | **HTML:** `#tab-stats` и `#tab-backtest` были **детьми** `#tab-live` (пропущен `</div>` после `#live-chart-container` в `index.html`) |
| `containerW/H = 0` на активной Backtest | trySync gate слишком строгий | `#tab-live { display:none }` скрывал всё поддерево |
| `hasChart: false` | ensureBacktestChart broken | Gate блокировал init до ненулевой геометрии; геометрия никогда не появлялась |

**Fix:** один `</div>` в `web/index.html` ~1357 — siblings под `#workspace-main`:
`#tab-live` | `#tab-stats` | `#tab-backtest`.

**Диагностика (консоль):** `document.getElementById('tab-backtest').parentElement.id` → должно быть `workspace-main`.

---

## Исполнение и Данные

### Данные

| Компонент | Файл | Роль |
|-----------|------|------|
| **SQLite `history.db`** | `data/history_db.go` | Локальный кэш klines; gap-fill при live/backtest |
| **Continuous Contract** | `exchange/continuous_contract.go` | Диапазон до 2017: spot (`BTCUSDT_SPOT`) + futures (`BTCUSDT`) склеиваются по `BinanceFuturesGenesisMs` |
| **HTFProvider cache** | `exchange/htf_provider.go` | In-memory klines per `symbol_interval`; `PinKlines`, `ClearCache`, `GetCandlesStrictlyBefore` |
| **Binance feed** | `exchange/ws_client.go`, `main.go` | Futures klines → `MasterGeneral` → per-TF `Marker` map |

### Исполнение (`MasterGeneral`, `strategy/master.go`)

**FSM:** `IDLE` ↔ `IN_POSITION`. **Один рабочий TF** (`cfg.Timeframe`, env `TRADING_TIMEFRAME`). Оценка на close рабочей свечи (`TickLiveCh` → `handleLiveTick`). Recovery через private API при старте.

**Sizing SSOT:** `execution.CalculateTargetQuantity(balance, riskPct, entry, stop, lotMod, maxLev)` + `GetRiskSettings()`. Paper/Live/Backtest единый путь.

**Warmup gate:** `handleLiveTick` и `ApplyExecutionVetoes` блокируют scoring/entry до `HasMinBars(50)`. Reversal заблокирован; **SL на бирже работает**. Комментарий в `manageReversal`.

**Gap-fill → RAM:** `LoadHistoricalKlines` → `mergeKlinesByOpenTime` (overlay WS wins) → `replayStreamingLocked` под `analyst.mu`. Mutex: blocking wait, no re-entry; WS backpressure через `OutCh` cap 1000.

**Цепочка после scoring:**
```
ScoreEngine.Calculate → ScoreDecision (Action BUY/SELL/WAIT)
  → Analyst.AnalyzeSignals(marker, side)   // veto-hook (сейчас pass-through)
  → ChiefAnalyst.Approve(decision)           // approval-hook (сейчас pass-through)
  → [entry] fractal SL + CalculateTargetQuantity (sizing)
  → CreateMarketOrder / virtual fill (readOnly | sandbox)
```

- **Vetoes:** `strategy/risk.go` `ApplyExecutionVetoes` — warmup + Analyst + Chief.
- **Sandbox / ReadOnly:** virtual positions; `BroadcastMarker` + `ChartTrade` на UI (chart markers, **не** stats history).
- **Trade History (Stats):** `domain.TradeHistoryStore` — `RealTrades` / `VirtualTrades` раздельно; `SetOnClosedTrade` в `MasterGeneral` → `RecordClosedTrade` в dashboard.
- **Backtest:** N+1 open entry (`btPendingEntry`), `SlippagePct` default 0.03%, PnL = Δprice × qty − fees; `Run(ctx, candles)` поддерживает cancel.

**End-to-end (onboarding):**
```
Binance → Marker (Layer1/2 + snapshot) → ScoreEngine → ScoreDecision
  → Master FSM / BacktestEngine
  → DashboardServer (MarketState | tickPayload | ChartPoint)
  → web/app.js (renderState, updateScoringUI, LWC charts)
```

### Дополнительные контуры (onboarding)

| Контур | Файлы | Суть |
|--------|-------|------|
| **Order Flow Store** | `domain/orderflow.go`, `exchange/ws.go` | Отдельный буфер aggTrade/liquidation тиков. **Не** история сделок для Stats. |
| **Trade History Store** | `domain/trade_history.go`, `server/webserver.go` | `RealTrades` (live) / `VirtualTrades` (paper/sandbox); `GET /api/stats?mode=` |
| **CLI A/B Backtester** | `cmd/backtest/`, `strategy/backtest_ab.go`, `Makefile` | `make ab-test` — Baseline LTF vs HTF Regime; SQLite + optional REST; дублирует engine path UI |
| **Настройки `/api/settings/*`** | `server/webserver.go` | **Обязательны** для matrix/thresholds/indicators/risk/navigators. Без POST matrix scoring на live не отражает UI-тогглы (`config/matrix.json`). |
| **AI & Qdrant (долг #8)** | `vector_db/`, `Marker.VectorSnapshot()` | Векторный пайплайн: snapshot состояния → embedding → Qdrant similarity → будущий **AI veto** на входе (сейчас `memoryStore=nil` в main, veto off). |
| **Layer 2 — repaint-by-design** | `zigzag`, `geometry` в `layer2.go` | **Намеренно вне Snapshot/Restore.** Swing nodes и geometry мутируют на открытом баре — это repaint, не баг. Не добавлять им snapshot при рефакторингах без явного архитектурного решения. |

---

## Changelog / Статус (июль 2026)

### [Frontend SSOT Pipeline — Phase 19.5–20 — июль 2026]

#### [Phase 19.5 — Monolith Decomposition — ✅]
**Цель:** разрезать `web/app.js` без изменения поведения.

| Извлечено | Файл |
|-----------|------|
| TF / toolbar / tabs | `web/ui/timeframe-controller.js`, `toolbar-controller.js`, `tabs-controller.js` |
| Backtest / RSX / risk / navigators | `web/ui/backtest-controller.js`, `rsx-controller.js`, `risk-controller.js`, `navigator-controller.js` |
| Layout / strategy / wozduh | `layout-controller.js`, `strategy-controller.js`, `wozduh-controller.js` |
| Fetch layer | `web/api.js`, `web/ws.js` |
| Pure mappers | `web/mappers.js` |
| Dormant quarantine | `web/indicators_dormant.js` |

`app.js` остаётся оркестратором: `loadDashboard`, `renderState`, history pagination, WS bridge.

#### [Phase 20.1 — Universal TimeGrid — ✅]
**`web/time-normalizer.js`:** `snapToGrid(timeMs, tf)`, `getIntervalMs`, `mergeCandles`.
**`web/store.js`:** `ChartDataStore` — все ingress через snap + merge с `tf`.

#### [Phase 20.2 — Annotation Grid — ✅]
`annotations: Map<snappedMs, flatProps>`; `buildSpikeMarkersFromGrid` / `buildWozduhMarkersFromGrid` в `mappers.js`; live marker cache в `chart-adapter.js`.

#### [Phase 20.3 — Data Pipeline Seal — ✅]
`seal()` / `unseal()` / `isSealed()`; guard в `getLatestDeltaForChart()`; seal в `renderState`, `maybeLoadHistory`, `loadDashboard`.

#### [Phase 20.3.1 — History Chunk Split — ✅]
`HISTORY_CHUNK_LIMIT = 3000` для `/api/history` pagination; `LIVE_STATE_CANDLE_LIMIT` отделён от chunk size.

#### [Phase 20.4A — Read-Only Audit — ✅]
Выявлено: cumulative volume bug, boot < warmup trim → cold RSX, dual viewport systems, logical-index prepend shift.

#### [Phase 20.4B — Iceberg + Math Fix — ✅]
| Fix | Детали |
|-----|--------|
| **Volume LWW** | `mergeCandles` — incoming volume wins, не sum |
| **Boot limit** | `AnalystBootKlineLimit = 400` (`strategy/live_kline.go`) |
| **Warmup trim** | `IndicatorWarmupBars = 300` (`strategy/rsx_pipeline.go`) |

#### [Phase 20.4C — Time Sync + ViewportManager — ✅]
| Изменение | Детали |
|-----------|--------|
| **Удалён** | `web/viewport.js`, `captureViewport`, `__pendingAnchor`, prepend shift hacks |
| **Добавлен** | `web/ui/viewport-manager.js` — time-anchored capture/restore |
| **Time format** | `chartTimeFormatBundle()` + shared tick formatter на всех live panes |
| **Store** | `candlesArray()` для binary search по `timeMs` |

#### [Phase 20.4D — Viewport Polish — ✅]
`TimeframeController`: `ViewportManager.capture('live')` → `loadDashboard({ viewportAnchor })`; `fitLiveChartsContent()`; убран auto-fill из `renderState`/`maybeLoadHistory`.

#### [Phase 20.4E — Exorcism & Pre-Fetch Assembly — ✅]
| Изменение | Детали |
|-----------|--------|
| **Pre-Fetch** | State → History → Store (under seal) → single `renderState({ isPreFetch, storeReady })` |
| **Fast state** | `LIVE_STATE_CANDLE_LIMIT = 300` (RAM tail, не 3000) |
| **Ghost fix** | `captureViewport` полностью удалён из `chart-adapter.js` (краш = stale bundle) |
| **Loading** | `ToolbarController.setBuffering` + `window.__isDashboardLoading` |

#### [Phase 20.4F — Perfect Assembly & Animation Kill — ✅]
| Изменение | Детали |
|-----------|--------|
| **Ровно 3000 баров** | `neededBars = 3000 - stateCandles.length`; `fetchLiveHistory(endSec, neededBars)` |
| **No animation** | Right-edge restore: `syncVisibleLogicalRange(..., { animate: false })` вместо `scrollToRealTime()` |
| **Pagination guard** | `liveStore.isSealed() \|\| window.__isDashboardLoading` в scroll hook + `scheduleHistoryLoad` + `maybeLoadHistory` |

### [Live Viewport TradingView-style + HTF Hub — Phase 5.46–5.48 — сессия]

#### [Phase 5.46 — Live Viewport: стабилизация + удаление 1M — ✅]
**Проблемы:** бесконечный цикл lazy-load на Live; зависание viewport при смене ТФ; нерабочий ТФ `1M`.

**Fix (`web/app.js` + `web/index.html`):**
| Изменение | Детали |
|-----------|--------|
| **Удалён 1M** | UI/JS: `#bt-interval`, navigator MTF checkboxes, `TF_DISPLAY`, `TF_MENU.DAYS`, `DEFAULT_FAVS`, `resolveTf(month)`, `tfSortKey`; миграция localStorage `1M`→`1w` |
| **`isLoadingHistory`** | Guard в `maybeLoadHistory`: `isLoadingHistory \|\| isUpdatingData`; порог `range.from >= 50`; сброс в `finally` после `applySeriesData` |
| **`applyLiveViewportAfterData`** | Единая точка `setVisibleLogicalRange` после `setData` |
| **Tab switch Live** | Убран `fitChartInstance(liveChartData)` — viewport через `applySeriesData` |

**Константы:** `LIVE_DEFAULT_VISIBLE_BARS = 1000` (первая загрузка / fallback зума).

#### [Phase 5.47 — Live Viewport: сохранение зума при смене ТФ — ✅]
**Цель:** как TradingView — при смене ТФ сохранять **количество видимых баров** и режим позиционирования.

**`switchLiveTimeframe` → `window.__pendingAnchor`:**
```javascript
visibleBars = logicalRange.to - logicalRange.from
// right edge (within 5 bars of last candle):
{ type: 'right', visibleBars, rightOffset }
// scrolled into history:
{ type: 'center', targetTime, visibleBars }  // center from getVisibleRange()
```

**`applyLiveViewportAfterData` приоритет:**
1. **prepend** — shift по `oldTotal`: `shift = total - prependPrevRange.oldTotal`
2. **`__pendingAnchor`** — center или right с `visibleBars`
3. **incremental** — restore `prevRange` (poll / tab switch)
4. **default** — последние `LIVE_DEFAULT_VISIBLE_BARS` баров

**Удалено:** `window.__pendingTimeAnchor`, хелперы `getChartVisibleTimeRange` / `findLogicalIndexByTime` / `setLiveVisibleLogicalWindow`.

#### [Phase 5.48 — Live Edge Right Offset — ✅]
**Нюанс:** режим `right` не только прижимает к последней свече, но сохраняет **пустое пространство** справа.

```javascript
rightOffset = logicalRange.to - lastIndex   // capture
targetTo = (total - 1) + rightOffset      // apply
targetFrom = targetTo - windowSize
```

**Правило:** `getVisibleRange()` (не logical) для center time; `series.data().length - 1` для lastIndex.

#### [Phase 5.46b — HTFProvider + MTF periods UI (prefetch only) — ⚠️ PARTIAL]
**`exchange/htf_provider.go`:** `GetKlines`, `PinKlines`, `ClearCache`, `CleanupIdle`; cache keyed `symbol_interval`.

**Wiring:** DI в `main.go` → `MasterGeneral` + `DashboardServer`; `BuildAllNavigators` → `loadNavigatorHTFData(htf, symbol, interval, startMs, ui.Periods)`.

**Frontend:** `periods[]` checkboxes в navigator popups; `coerceNavigatorPeriods`, `navigatorSettingsToAPI({ periods })`.

**🔴 ДОЛГ (устарело):** ~~`BuildNavigatorData` игнорирует htfData~~ — **✅ закрыто в 5.54** (см. ниже).

#### [Phase 5.54 — MTF Navigator: strict slice + clipping + universal overlay — ✅]
**Backend (`exchange/htf_provider.go`, `strategy/trendline_navigator.go`):**
| Компонент | Поведение |
|-----------|-----------|
| **`GetCandlesStrictlyBefore`** | HTF klines полностью закрытые до `maxTimeSec` (без look-ahead за конец симуляции) |
| **`mergeHTFNavigatorLayers`** | Линии старших TF мержатся с chart TF; `Interval` в DTO |
| **`ClipNavigatorLinesToChartWindow`** | Обрезка по `OpenTime` первой свечи графика; пересчёт Y1 |
| **`ApplyMtfOptionsToNavigators`** | `mtfOptions` из UI → `periods` price navigator |

**Frontend (`web/app.js`, `web/viewport.js`, `web/index.html`):**
| Компонент | Поведение |
|-----------|-----------|
| **`createMTFOverlaySeries`** | `autoscaleInfoProvider: () => null` для всех overlay-слоёв |
| **`mtfNavigatorLayers[pane][tf]`** | Anchor series + `TrendlinePrimitive` на каждый активный period |
| **`refreshLiveNavigatorFromServer`** | MTF toggle без полного `setData` свечей |
| **`lockPriceAutoScaleDuring`** | Viewport + plugin update без прыжка Y |
| **`MTF_SYNC_QUICK_PERIODS`** | Динамические чекбоксы 4h/1d/1w (не хардкод в логике) |

**Остаётся открытым (MTF):** RSX/Wozduh HTF merge; background zones по time при lazy prepend. Walk-forward per-bar — ✅ 5.59 (backtest).

#### [Phase 5.56 — Falcon Snapshot/Rollback (P0 refactor) — ✅ SUPERSEDED by 5.57]
**Паттерн:** `SaveState`/`RestoreState` на `RMA`, `EMA`, `SMA`, `RSI`, `MACD`, `Stochastic`, `JurikRSX`, `RSXSignalLine`, `AD`, `VolumeWeightedEMA`, `RollingStDev` + `FalconEngine`. Удалён O(n) `replayAndEvaluateBarLocked`.

#### [Phase 5.57 — Layer 2 Snapshot/Rollback (P0) — ✅]
**Файлы:** `strategy/layer2.go`, `strategy/volatility.go`, `indicators/volatility.go`, `indicators/chaos.go`

| Компонент | Snapshot |
|-----------|----------|
| `VolatilityEngine` | ATR + SMA baselines |
| `orangeRsi`, `ad`, `stoch`, `ao` | indicator-level |
| Marker flags | `adHistory`, `prevAO`, Falcon crosses, `volatilityState`, … |

**Поток:** `restoreLayer2StreamingState()` → evaluate → `saveLayer2StreamingState()` if `isClosed`. Rollover: полный `evaluateTickLocked(last, true)`.

#### [Phase 5.58 — Divergence & RSX buffer Snapshot (P0) — ✅]
| Компонент | Файл | Fix |
|-----------|------|-----|
| `SmartDivergenceEngine` | `indicators/divergence.go` | Ring-buffer snapshot |
| `rsxMarkerState` | `strategy/analyst.go` (Marker) | Deep-copy; intra-bar не растит `prices[]` |

#### [Phase 5.59 — Walk-Forward MTF Tracker (P2) — ✅]
**Файл:** `strategy/mtf_tracker.go` — `WalkForwardMTFTracker.Update(tickSec, chartKlines)` step-function на границе HTF; `GetCandlesStrictlyBefore`; wired в `backtest.go`; `marker.SetCurrentMTFState()` / `MTFStates()`. UI `BuildAllNavigators` в конце бэктеста не тронут.

#### [Phase 5.60 — Score Engine refactor (Jeweler's Protocol) — ✅]
**Удалено:** `Report`, `GenerateMarketReport`, `GenerateStreamingReport`, `ScoreResult`, `SignalInfo`, `ScalpDecision`, `ProcessScore`, `report_snapshot.go`

**Добавлено:** `score_types.go` (`ScoreFactor`, `ScoreDecision`, `ScoreEngine`); `ScoreEngine.Calculate(marker, matrix)`; `UseTrendlines` → `marker.MTFStates()`; API `longScore`/`shortScore` без изменений.

#### [Phase 5.49 — Unified Viewport Engine (Live + Backtest) — ✅]
**`web/viewport.js`:** `Viewport.captureViewport(chart, series)` → `{ visibleBars, centerTime, isAtRight, rightOffset? }`; `restoreViewportToCharts`; `computeLogicalRange`; default **1000** bars.

**Live:** `switchLiveTimeframe` + `applyLiveViewportAfterData` → `window.Viewport`.

**Backtest:** `runBacktest` захватывает anchor **до** POST; `applyBacktestResultToChart` → `restoreViewportToCharts` (без `fitProfessionalChart`); `handleBacktestIntervalChange` без `preserveView: false`; tab switch backtest — без `fitContent`.

### [Live Viewport + Guards + Cache — Phase 5.50–5.53 — сессия]

#### [Phase 5.50 — Viewport hardening + Live atomic render + microscope `endTime` — ✅]
**Проблемы:** vertical squeeze (negative logical range); микроскоп уезжал на live edge; моргание Live при TF switch; латентный `centerTime` NaN (BusinessDay).

**`web/viewport.js`:** `normalizeTime`; cap `visibleBars`; ранний clamp `from >= 0` / `safeRightOffset` (заменён Smart Zoom в 5.52).

**`web/app.js`:**
- `apiQueryParams`: `endTime` при `anchor.type === 'center'` → `centerMs + (LIVE_STATE_CANDLE_LIMIT/2) * getIntervalMs(tf)`, clamp `Date.now()`
- `renderState`: атомарный `isUpdatingData` → `applySeriesData` → `syncLiveChartPanesFromPrice`
- `applySeriesData` — без own lock; lock снаружи (`renderState`, `maybeLoadHistory`, tab switch)

#### [Phase 5.51 — Time-gap guard + Backtest OOM — ✅]
- `isLiveTickGapTooLarge` — gap > `5 * getIntervalMs(currentTf)` в `applyPriceBar` + `pollLatestState`
- `maxBacktestBars = 100000` в `handleBacktestRun` (RU error message)
- `resetBacktestRunUi()` + `alert` в `runBacktest`

#### [Phase 5.52 — Smart Zoom + Clear Cache — ✅]
**`computeLogicalRange`:** `MIN_ZOOM=50`, `MAX_ZOOM=max(50, total*2)`, отрицательный `from` разрешён (LWC margin).

**Cache:** `#btn-clear-cache` → `POST /api/cache/clear` → `htfProvider.ClearCache(true)` + `activeGapFills` reset.

**1M backend:** сохранён для будущей агрегации (UI: миграция `1M`→`1w`).

#### [Phase 5.53 — Аудит Live indicators — 🟡 PARTIAL → Go CLOSED in 5.57–5.58]
**Симптом (было):** Wozduh/RSX на live edge скачут на каждый WS-тик.

**Go (закрыто 5.56–5.58):** Snapshot/Rollback на всех stateful Layer2 + Falcon + divEngine + rsxMarkers.

**Остаётся (вторично):** WS `tickPayload` без полного Wozduh; `mergeOsc` на клиенте.

#### [Phase 5.61 — Score Factors telemetry export — ✅]
**Backend:** `MarketState.factors`, `tickPayload.factors`, `BacktestChartPoint.Factors` → `ChartPoint.factors`.
**Frontend:** `updateScoringUI` + telemetry panel. *(UI superseded 5.75–5.76: compact line + click-to-lock.)*

#### [Phase 5.62 — Factors UI panel — ✅]
**`web/index.html`:** `#scoring-telemetry-card`. Factor badges. *(Superseded: crosshair DOM scrub → click-to-lock + chart hint only.)*

#### [Phase 5.63 — Backtest score per bar (SSOT) — ✅]
**`BacktestChartPoint.LongScore` / `ShortScore`** → `ChartPoint` JSON. Фронт не суммирует факторы; `liveScoringData` при возврате на Live.

### [Core Hardening — Phase 5.64–5.72 — сессия июнь 2026]

#### [Phase 5.64 — Execution & TF agnosticism — ✅]
- Удалён hunt-TF legacy (`huntTimeframe`, `TickHuntCh`, `handleHuntTick`)
- **Sizing SSOT:** `execution.CalculateTargetQuantity` + `GetRiskSettings()` для paper/live/backtest
- `TRADING_SYMBOL`, `TRADING_TIMEFRAME` в `config/config.go`
- Backtest: N+1 entry, slippage 0.03%, qty-based PnL

#### [Phase 5.65 — Frontend TF sync + Safe Boot UI — ✅]
- `MarketState.TradingTimeframe`; `boot()` без localStorage TF flash
- `fetchBootstrapState` → server TF → `initCharts`

#### [Phase 5.68 — Live MTF Tracker — ✅]
- `MasterGeneral.mtfTracker`, `SetNavigatorPanes`, `syncMTFState`
- `DashboardServer.BindMaster`, `DefaultLiveNavigatorPanes()`
- `handleLiveTick` → MTF перед scoring

#### [Phase 5.69 — HTF Wozduh Regime — ✅]
- Scoring: regime (`WozduhUp > WozduhDown`), не cross
- Веса: `scoreHTFWozduhRegime=15`, `scoreHTFRSX=20`
- `WozduhCross` удалён из `HTFState`

#### [Phase 5.70 — Safe Boot Protocol — ✅]
- `rebuildMTFTrackerIfReady` (defer до 50 bars)
- `NotifyKlineGapFillComplete` + `hydrateTradingHistoryFromStore`
- Warmup veto в `ApplyExecutionVetoes`
- `MinHTFPrefetchBars = 100`

#### [Phase 5.71 — Gap-fill merge + Falcon/RSX isolation — ✅]
- `strategy/kline_merge.go` — overlay WS wins на duplicate OpenTime
- `FalconEngine.rsxSource` snapshot (fix flaky `TestFalconEngine_RSXSourceClose`)
- `rsxMarkerConfig` в `rsxMarkerState` — no global reads in `appendBar`
- Deadlock audit: mutex contract documented; concurrent hydrate test

#### [Phase 5.72 — Live Telemetry activation — ✅]
- `SetOnTelemetry` — WS scoring на каждом тике рабочего TF
- MTF sync **до** scoring на bar close (fix race vs `handleLiveTick`)
- `ScoreDecisionForTelemetry` для `/api/state` + WS
- `useHTFOscillators: true` в `matrix.json`
- HTF keys `RSX_{tf}`, `Wozduh_{tf}`

#### [Phase 5.73 — CLI A/B Backtester (Quant Lab) — ✅]
**Файлы:** `cmd/backtest/main.go`, `strategy/backtest_ab.go`, `strategy/backtest_loader.go`, `Makefile`

| Компонент | Поведение |
|-----------|-----------|
| **Config A (Baseline)** | `UseHTFOscillators=false`, веса из `matrix.json` |
| **Config B (HTF Regime)** | `UseHTFOscillators=true`, quiet LTF (`WozduhCross/Spike/RedCross` off), MTF `4h`/`1d` |
| **Engine SSOT** | `BuildBacktestEngineConfig` + `RunBacktestSimulation(ctx, …)` = тот же path что `POST /api/backtest/run` |
| **Запуск** | `make ab-test` / `make ab-test-rest` |

**Примечание:** заказчик предпочитает UI для калибровки; CLI остаётся batch-утилитой.

#### [Phase 5.74 — Universal Stats Dashboard — ✅]
**Backend:** `domain/trade_history.go` — `TradeHistoryStore`, `ClosedTrade`, `ComputeSessionStats`; `MasterGeneral.SetOnClosedTrade` → `RecordClosedTrade`; `GET /api/stats?mode=live|paper`.

**Frontend (`web/index.html`, `web/app.js`):**
| Режим | Источник данных |
|-------|-----------------|
| **Backtest** | `lastBacktestResult` после симуляции |
| **Paper** | `/api/stats?mode=paper` |
| **Live** | `/api/stats?mode=live` |

`renderStatsDashboard(trades, mode)` — карточки метрик + Trade History table. Переключатель `.stats-mode-selector` на вкладке Stats.

**Ограничение:** trade history только в RAM (сброс при рестарте бота).

#### [Phase 5.75 — UX/UI: телеметрия + Stop Backtest — ✅]
**Crosshair shake fix:** `subscribeCrosshairMove` → только `.crosshair-scoring-hint` на графике; `updateScoringUI` только из WS / `/api/state`.

**Компактная телеметрия:** удалён заголовок и `#scoring-factors-list`; `#telemetry-compact-line` — одна строка (L/S, action, top-4 factors, veto).

**Stop Backtest:**
- `POST /api/backtest/stop` + `server/backtest_run.go` (`backtestRunManager`)
- `BacktestEngine.Run(ctx, candles)` — cancel loop, partial result, `cancelled: true`
- UI: `#btn-stop-backtest`, `AbortController` на fetch

#### [Phase 5.76 — Trade Inspector (Click-to-Lock) — ✅]
**Файлы:** `web/app.js`, `web/style.css`

| Механизм | Детали |
|----------|--------|
| **Lookup** | `backtestChartPointsByTime` (Map O(1)) + `backtestTradeMarkerTimes` (Set entry/exit times) |
| **Lock** | `lockedHistoricalData` → `renderLockedTelemetry()` → `#telemetry-compact-line` + `.telemetry-locked` |
| **Unlock** | клик вне данных / повторный клик на ту же свечу / смена вкладки |
| **SSOT guard** | `updateScoringUI` игнорирует live-тики при активном lock |
| **Hover** | crosshair hint на графике (не DOM) — превью без тряски |

**Не реализовано:** pixel hit-test по `TradeMarkerPrimitive` (клик = time бара под курсором); live history inspector (нет `ChartPoint` map на live).

#### [Phase 6.0 — Telemetry SSOT + Anti-Flicker — ✅]
**Файлы:** `strategy/master.go`, `strategy/analyst.go`, `server/webserver.go`, `web/app.js`, `main.go`

| Шаг | Изменение |
|-----|-----------|
| **UI R/G** | `applyLatestOscPoint` + `updateHeader` → `redLine`/`greenLine` |
| **Backend SSOT** | `closedTelemetry` cache; `SeedClosedBarTelemetry`; `refreshClosedBarTelemetry` на bar close |
| **Read-only API** | `ScoreDecisionForTelemetry` без `syncMTFState` |
| **Scoring panel** | WS: `updateScoringUI` только при `isClosed`; poll без scoring |
| **Regime** | `ClosedVolatilityRegime` из `layer2Snap` |

#### [Phase 7.0 — Workspace Layout Polish — ✅]
**Файлы:** `web/index.html`, `web/style.css`, `web/app.js`

| Шаг | Изменение |
|-----|-----------|
| **Layout** | `#app-chrome` + `.workspace-main`; flex chain; border-top |
| **Backtest resize** | `.pane-resize` в bt-stack; `PANE_STACK_CONFIG` |
| **Fullscreen dblclick** | `priceWrap` в `initPaneFullscreen`; inspector click deferred |
| **Visual parity** | backtest padding/background = live |

#### [Phase 8.0 — Price Scale UX + Hard Flex Layout — ✅]
**Файлы:** `web/index.html`, `web/style.css`, `web/app.js`

| Шаг | Изменение |
|-----|-----------|
| **Hard flex** | `#app-container` flex column; удалён JS `--workspace-chrome-h` |
| **Scale controls** | `.scale-controls` на price pane (Live + Backtest) |
| **Auto / Log** | независимые режимы; dblclick на шкале → Auto |

#### [Phase 9.0 — Scale Aesthetics (TV Polish) — ✅]
**Файлы:** `web/app.js`, `web/style.css`

| Шаг | Изменение |
|-----|-----------|
| **PriceLine labels** | `normalizePriceLineLevel`: `axisLabelColor: TV.bg`, `axisLabelTextColor` = цвет линии |
| **RSX levels** | убраны OB/MID/OS; линия 50 — `rgba(204,85,0,0.4)`, dashed, width 1 |
| **Scale buttons** | `.scale-controls` → `flex-direction: row`; компактно над time scale |

#### [Phase 9.1 — LC API fix (axis labels) — ✅]
**Файлы:** `web/app.js`

| Шаг | Изменение |
|-----|-----------|
| **Axis labels** | `axisLabelColor` / `axisLabelTextColor` вместо несуществующего `labelBackgroundColor` |
| **RSX 50** | `lineWidth: 1`, `color: rgba(204, 85, 0, 0.4)` |

#### [Phase 28 — Backtest Data/View Split + Black Screen Fix — ✅]
**Файлы:** `web/store.js`, `web/ui/backtest-pipeline.js`, `web/ui/chart-projection.js`, `web/ui/tabs-controller.js`, `web/app.js`, `web/chart-adapter.js`, `web/index.html`

| Шаг | Изменение |
|-----|-----------|
| **Data-plane** | `BacktestPipeline` — только store + `markViewDirty`; удалены `ensureBacktestChart`, `ChartProjection.renderBacktest`, `NavigatorController.renderChartLegends` |
| **View intent** | `backtestStore.markViewDirty({ mode, viewport })` / `consumeViewDirty()` в IIFE wrapper |
| **View-plane** | `ChartProjection.trySync()` — gate `hasBaseLayer ∧ #backtest-chart-container > 0`; lazy init; `scrollToPosition(0)` |
| **Ping sensors** | Tabs (`trySync` + post-`loadShell`), independent RO на container, `runBacktest` finally |
| **Poison pill** | `_needsInitialPaint` сброс только при наличии `candleSeries` |
| **HTML fix** | Закрыт `#tab-live` перед `#tab-stats` — вкладки стали siblings в `#workspace-main` |

**Эволюция отладки (не повторять):** pending queue + RO-hooks + `fitContent` на 0×0 — accidental complexity; правильный паттерн — dirty + idempotent ping (как Live `shouldPaintLiveChart`).

---

## Core 2.0 — DDR Pipeline (DAG Shadow + Columnar Transport) — июль 2026

**Цель:** заменить legacy Falcon osc paint на Data-Driven Rendering (DDR) из `core.DAGRunner` + `ui_config` manifest. Протокол ювелира: изоляция слотов, Snapshot/Restore, zero look-ahead.

### Архитектура DAG (Shadow)

| Компонент | Путь | Роль |
|-----------|------|------|
| `DAGRunner` | `core/runner.go` | Tick protocol: Restore → Update → Save+Hist |
| Shadow chain | `core/nodes/` | RSX → Wozduh → ZigZag → Divergence → MicroPattern → Score |
| `HistoryBus` | `core/history.go` | Ring buffer per slot; sentinel `MaxFloat64` на wire |
| `ReplayDAGKlines` | `strategy/dag_shadow.go` | Cold replay для history (Strategy A) |
| `Projector` | `server/wire/` | `BuildTickJSON` (live WS), `BuildHistoryColumns(Filtered)` |
| UI Registry | `ui_config/` | `line_rsx`, `woz_fast`, `score_total`, … |

**Scoring:** `ScoreNode` stateless; execution gated outside DAG (`TradeManager` on `isClosed`).

### Phase 3E–4B — Nodes + Dual-Write ✅
- MicroPattern, ScoreNode, DynamicFractal deferred
- `Marker.DAGTickFrame()` → WS `plots` dual-write
- `GET /api/ui/manifest`

### Phase 5 — Columnar History Hydration ✅
- `BuildHistoryColumns` — sentinel `HistoryAbsent = math.MaxFloat64`
- ~~`GET /api/ui/history`~~ → **удалён в Phase 7A**

### Phase 6 — Frontend DDR Cutover ✅
- `DDRFactory` (`web/series-factory.js`) — manifest panes, `hydratedData`, `updateTick`
- `mountDDRLiveCutover` — legacy osc hidden (`ddrOscCutoverActive`)
- Boot: `scheduleDDRCutover` → columnar hydrate + `buildPanes`

### Phase 7A — Monolithic Backend Transport ✅
**Файлы:** `server/columnar_history.go`, `server/wire/history.go`, `server/webserver.go`

| Правило | Значение |
|---------|----------|
| Endpoint | `GET /api/history?format=columnar&limit=3000&slots=...` |
| Warmup | **hardcode 300** (`IndicatorWarmupBars`) — dynamic warmup отменён |
| Replay | Cold `ReplayDAGKlines` на окне limit+warmup (Strategy A, no session cache) |
| Field mask | `BuildHistoryColumnsFiltered` — только запрошенные slot IDs |
| Row format | Default без `format=columnar` — backtest не сломан |
| Удалено | `/api/ui/history`, `handleUIHistory`, `loadHistoryKlinesForUI` |

**Columnar JSON контракт:**
```json
{
  "format": "columnar",
  "warmupDropped": 300,
  "added": 3000,
  "times": [...],
  "candles": { "open": [], "high": [], "low": [], "close": [], "volume": [] },
  "plots": { "line_rsx": [...] },
  "annotations": [],
  "sentinel": 1.7976931348623157e+308,
  "hasMore": true
}
```

### Phase 7B — Frontend Orchestration ✅
**Файлы:** `web/hydration-orchestrator.js`, `web/chart-adapter.js`, `web/app.js`, `web/api.js`

| Механизм | Детали |
|----------|--------|
| **FSM** | `IDLE → PREPENDING → APPLYING → LIVE` (abort → `IDLE`, wsQueue discard) |
| **Debounce** | 200ms trailing на scroll-left (`schedulePrepend`) |
| **Single fetch** | Один `fetchColumnarHistory` — dual HTTP race устранён |
| **WS queue** | `queueTick` в PREPENDING/APPLYING; `flushQueue` только при успешном prepend |
| **Atomic apply** | `ChartAdapter.applyAtomicPrepend` — `_liveUpdating` → candles → `DDRFactory.applyHydratedData` → **один** `setVisibleLogicalRange` |
| **Pre-alloc merge** | `DDRFactory.prependColumnarChunk` + `mergePrependPoints` (no push) |
| **TF reset** | `clearChartData` → `liveHistoryEpoch++` + `orchestrator.reset()` |
| **Microscope guard** | `shouldLoad` → `false` if `_microscopeTickMuted` |

**Поток scroll-left:**
```
subscribeVisibleLogicalRangeChange → scheduleHistoryLoad (debounce)
  → HydrationOrchestrator.requestPrepend
  → GET /api/history?format=columnar&limit=3000
  → liveStore.prependHistory + DDRFactory.prependColumnarChunk
  → ChartAdapter.applyAtomicPrepend
```

### Phase 8A — Backend Annotations Wiring ✅
**Файлы:** `server/columnar_history.go`

| Слой | Источник |
|------|----------|
| Plots | DAG `ReplayDAGKlines` + `BuildHistoryColumnsFiltered` |
| Annotations | **Hybrid:** `StreamingReplayAccumulator` (Falcon) на тех же klines |
| Фильтр | `trimAnnotations(warmup)` + `filterAnnotationsByDisplayTimes` (exact `time ∈ times`) |

**Долг:** dual replay engines (DAG + Falcon) на каждый columnar request — Strangler, не баг.

### Phase 8B — Frontend Annotations (🔜 NOT DONE)
- `applyUniversalAnnotations` в atomic prepend block
- RSX LL/SS markers при DDR cutover (сейчас `_applyFullDataInternal` skip legacy osc)

### Phase 7 (planned) — SettingsRenderer 🔜
- `DDRFactory.applyComponentOptions(id, opts)` для Wozduh checkboxes

### MCP / Tooling (paused)
- `.cursor/mcp.json` — SQLite (`history.db`), GitHub MCP, Puppeteer
- Browser tab `localhost:8080` для visual QA

### Известные проблемы графиков (июль 2026 — ACTIVE)

| Симптом | Вероятная причина | Статус |
|---------|-------------------|--------|
| Viewport jump / «гармошка» | dual transport (fixed 7B); остаток: `prevRange null`, `addedBars` mismatch | 🟡 отладка |
| Browser freeze на boot | 3000 bars × N series `setData` + `columnToLWC` object alloc | 🟡 pre-alloc partial |
| DDR series empty on boot | `fetchAndHydrateHistory` до `buildPanes` → empty `slots` | 🟡 manifest fallback |
| RSX markers missing on prepend | Annotations на wire (8A) но UI не рисует (8B pending) | 🔜 |
| Divergence drift DAG vs Falcon | Разные движки для plots vs markers | 🟡 architectural |
| Scroll spam / lost packets | Debounce + `_inFlight` (7B) | ✅ mitigated |
| Legacy osc hidden, DDR only | Phase 6 cutover — regression if DDR fails | 🟡 |
| Microscope vs scroll-left race | epoch + orchestrator reset | ✅ mitigated |

### [🔜 OPEN DEBTS — Core 2.0 / Charts]

| # | Долг | Файлы | Статус |
|---|------|-------|--------|
| **35** | **Phase 8B — annotations UI on prepend** | `chart-adapter.js`, `app.js` | 🔜 `applyUniversalAnnotations` in atomic block |
| **36** | **Chart stability QA** | browser + `app.js` | 🔜 viewport jump, freeze, scroll-left 3×3000 |
| **37** | **Dual replay CPU** | `columnar_history.go` | 🟢 acceptable ~30–80ms; parallel optional |
| **38** | **DAG vs Falcon div drift** | `dag_shadow.go`, `streaming_replay_accum.go` | 🟡 Strangler until unified div in DAG |
| **39** | **SettingsRenderer (Phase 7)** | `series-factory.js` | 🔜 Wozduh DDR series visibility |
| **40** | **Backtest DDR cutover** | `backtest-pipeline.js` | 🔜 live only |
| **41** | **wsQueue hard cap** | `hydration-orchestrator.js` | 🟡 cap 64 on slow server |
| **42** | **`added` server vs client mismatch warn** | `app.js` | 🟢 log if `data.added !== store.added` |
| **43** | **Boot slots from manifest** | `series-factory.js` | 🟡 `resolveLiveSlotIds()` fallback |
| **44** | **Order Flow deprecation** | `webserver.go` | 🔜 roadmap |
| **45** | **Backtest → columnar** | `api.js` | 🔜 roadmap |
| **46** | **MEMORY sync** | this file | ✅ Phase 7–8A logged |
| **47** | **LeftBars DynamicFractal vs Williams** | `dynamic_fractal.go` | 🟡 shadow validation |
| **48** | **Debug agent logs cleanup** | `app.js` | 🟡 session 39f875 ingest |

### [🔜 OPEN DEBTS — приоритет]

| # | Долг | Файлы | Статус |
|---|------|-------|--------|
| **0** | ~~**Live indicator poisoning (Go)**~~ | `layer2.go`, `indicators/` | ✅ 5.57–5.58 |
| 1 | ~~**Navigator MTF math**~~ | `trendline_navigator.go` | ✅ 5.54 |
| 2 | **Navigator background zones** | `web/trendline_plugin.js` | 🟡 |
| 3 | ~~**Backtest viewport**~~ | `web/ui/viewport-manager.js` | ✅ 20.4C (supersedes `viewport.js`) |
| 4 | **Backend 1M** — UI `1M`→`1w` | `server/`, `main.go` | 🟢 |
| 5 | **Microscope `endTime`** | `web/app.js` | 🟡 TF switch без center-anchor API |
| 6 | **Forward lazy load** | `web/app.js` | 🟡 scroll prepend only (pre-fetch закрывает initial) |
| 7 | **SQLite clear в Cache** | `server/webserver.go` | 🟡 |
| 8 | **Qdrant in main** | `main.go`, `vector_db/` | 🔜 |
| 9 | **expose `masterState`** | `server/webserver.go` | 🔜 |
| **10** | ~~**Factors UI panel**~~ | `web/app.js`, `index.html` | ✅ 5.62 → 5.75 compact line |
| **11** | ~~**MTF walk-forward на live**~~ | `master.go`, `mtf_tracker.go` | ✅ 5.68–5.70 |
| **12** | **MTF scoring tune / weight calibration** | `scoring.go`, `matrix.json`, Stats A/B | 🔜 NEXT |
| **13** | ~~**Backtest longScore/shortScore per bar**~~ | `backtest.go` | ✅ 5.63 |
| **14** | ~~**RSX/Wozduh HTF в scoring**~~ | `mtf_tracker.go`, `scoring.go` | ✅ 5.69 + 5.72 |
| **15** | **WS full Wozduh / mergeOsc** | `webserver.go`, `web/app.js` | 🟡 |
| **16** | ~~**Slippage model**~~ | `backtest.go` | ✅ 5.64 (0.03% default) |
| **17** | **max_drawdown enforcement** | `risk.go`, `master.go` | 🔜 |
| **18** | **fixed_pct stop в UI** | `computePositionStop` | 🔜 |
| **19** | **mergeKlines SSOT** | `kline_merge.go` vs `webserver.go` dup | 🟡 |
| **20** | ~~**Flaky tests**~~ | `rsx_settings_test.go` | ✅ 5.71 fixed |
| **21** | **Stats trade history persistence** | `domain/trade_history.go` | 🔜 SQLite |
| **22** | **Live Trade Inspector** | `web/app.js` | 🔜 scoring history buffer |
| **23** | **Backtest stop: await partial JSON** | `web/app.js` | 🟡 prefer server response over AbortController |
| **24** | **Stats live initial balance sync** | `domain/trade_history.go` | 🟡 uses DefaultSessionCapital |
| **25** | ~~**Frontend monolith / live viewport hacks**~~ | `web/app.js`, `viewport.js` | ✅ Phase 19.5–20 |
| **26** | ~~**Live volume exponential growth**~~ | `web/time-normalizer.js` | ✅ 20.4B LWW |
| **27** | ~~**Chain-reaction history load**~~ | `app.js`, `viewport-manager.js` | ✅ 20.4F guards + no animate |
| **28** | ~~**Backtest black screen (0×0 tab nesting)**~~ | `web/index.html`, `chart-projection.js` | ✅ Phase 28 — HTML `</div>` + Data/View split |
| **29** | **Backtest history bypasses Projection** | `web/app.js` `maybeLoadBacktestHistory` | 🟡 прямой `ChartAdapter.applyHistoryPrepend` — асимметрия с Phase 28 |
| **30** | **trySync: re-mark dirty on paint fail** | `web/ui/chart-projection.js` | 🟡 `consumeViewDirty` до успешного paint; poison pill v2 если series пустой после `clearSeries` |
| **31** | **Debug agent logs cleanup** | `backtest-pipeline.js`, `chart-adapter.js`, `app.js` | 🟡 `#region agent log` fetch ingest (session 39f875) |
| **32** | **Overlay navigators in intent** | `backtest-pipeline.js`, `chart-projection.js` | 🟡 simOnly overlay без `result.navigators` в intent |
| **33** | **`coversRange` not wired to `needsBaseReload`** | `backtest-pipeline.js`, `store.js` | 🟡 fingerprint-only reload policy |
| **34** | **`runBacktest(autoSwitchTab)` dead param** | `web/app.js` | 🟢 не переключает на Backtest tab |
| **35** | **DAG → TradeManager wiring (PAUSED)** | `strategy/master.go`, `core/runner.go` | ⏸ `DAGRunner` shadow-only; execution via legacy `Analyst` + `HasFinalSignal()` |
| **36** | **TradeIntent wire contract** | `strategy/score_types.go`, WS/API | ⏸ нет единого payload side/qty/stop из `ScoreNode` → `execution.CalculateTargetQuantity` |
| **37** | **Execution gate `isClosed` only** | `strategy/master.go` | ⏸ intra-bar DAG score не проводится в TradeManager |
| **38** | **Risk/settings SSOT parity** | UI matrix vs DAG weights | ⏸ scoring weights должны совпадать с execution veto chain |
| **39** | **ChiefAnalyst bus snapshot path** | `strategy/chief.go`, `master.ProcessTick` | ⏸ заменить entry path на post-`DAGRunner.TickUpdate` snapshot |

---

## Справочник: ключевые файлы

| Область | Пути |
|---------|------|
| Score | `strategy/score_types.go`, `scoring.go`, `scoring_matrix.go`, `thresholds.go`, `risk.go` |
| Marker / Layer2 | `strategy/analyst.go`, `layer2.go`, `volatility.go`, `kline_merge.go` |
| MTF | `strategy/mtf_tracker.go`, `navigator_defaults.go`, `trendline_navigator.go`, `exchange/htf_provider.go` |
| Execution / Sizing | `execution/risk.go`, `strategy/risk_settings.go` |
| Live | `strategy/master.go`, `main.go`, `config/config.go` |
| Backtest | `strategy/backtest.go`, `strategy/backtest_ab.go`, `strategy/backtest_loader.go`, `cmd/backtest/` |
| Stats / Trades | `domain/trade_history.go`, `server/backtest_run.go`, `server/stats_test.go` |
| API / WS | `server/webserver.go`, `server/ram_router.go`, `server/chart_cache.go`, `server/columnar_history.go`, `server/micro_broadcast.go` |
| Core 2.0 DAG | `core/runner.go`, `core/nodes/`, `core/history.go`, `strategy/dag_shadow.go`, `server/wire/history.go`, `ui_config/` |
| DDR Frontend | `web/series-factory.js`, `web/hydration-orchestrator.js` |
| Live klines | `strategy/live_kline.go`, `strategy/rsx_pipeline.go`, `strategy/streaming_replay.go`, `strategy/streaming_replay_accum.go` |
| Frontend core | `web/app.js`, `web/store.js`, `web/chart-adapter.js`, `web/time-normalizer.js`, `web/mappers.js`, `web/api.js` |
| Frontend UI | `web/ui/viewport-manager.js`, `web/ui/chart-projection.js`, `web/ui/backtest-pipeline.js`, `web/ui/timeframe-controller.js`, `web/ui/toolbar-controller.js`, `web/ui/tabs-controller.js`, … |
| Frontend style | `web/style.css`, `web/trade_marker_plugin.js`, `web/trendline_plugin.js` |
| Правила AI | `.cursor/rules/senior-quant-architect.mdc`, `jeweler-protocol.mdc` |

**Env:** `TRADING_SYMBOL`, `TRADING_TIMEFRAME`, `READ_ONLY`, `SANDBOX_MODE`.

**Запуск:** `go run .` — dashboard `:8080`, WS Binance futures, paper/sandbox via `.env`. `make ab-test` — CLI A/B backtest (опционально).

**Следующий шаг:** **Core 2.0 chart integration** — Phase 8B (annotations UI), стабилизация viewport/prepend на live, затем SettingsRenderer + backtest columnar. Параллельно: калибровка scoring (#12), trade history SQLite (#21). **Frontend backtest debts:** #29–#32.
