# Trading Bot — System Memory

**Перед написанием новых модулей ВСЕГДА перечитывай этот файл.**

> **Снэпшот MEMORY (июнь 2026):** **Ядро ~95% готово** — Live MTF walk-forward, Safe Boot, HTF Regime scoring, live telemetry WS, gap-fill merge, Falcon/RSX snapshot isolation. Следующий фокус: **калибровка весов** + A/B backtest. Правила в `.cursor/rules/`.

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

**Телеметрия (Phase 5.61–5.72):** `MarketState.factors`, `tickPayload.factors`, `BacktestChartPoint.Factors` → UI `#scoring-factors-list`. **Live:** `SetOnTelemetry` на каждый WS-тик рабочего TF (intra-bar + close); `ScoreDecisionForTelemetry` синхронизирует MTF **до** scoring на bar close. UI: серые inactive factors при WAIT/veto (`.factor-row--inactive`).

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
| `/api/backtest/run` | POST | `BacktestRequest` → `BacktestResult` с `chartData[]` (**`ChartPoint.factors`**), `trades`, `equityCurve`, `navigators` |
| `/api/settings/*` | GET/POST | thresholds, matrix, indicators, risk, navigators |
| `/api/cache/clear` | POST | сброс HTF cache + gap-fill flags |
| `/ws` | WS | `wsEnvelope{ type, data }` |

**WebSocket типы:**
- `tick` → `tickPayload`: OHLCV, osc lines (1m), `longScore`, `shortScore`, **`factors`**, `brainStatus`, `aiStatus`
- `marker` → `markerPayload`: trade entry/exit на графике

**Live dual-channel (HTTP bootstrap + WS push):**
1. `boot()` → `fetchBootstrapState` (без localStorage TF) → `initCharts` → `renderState`
2. `GET /api/state` → `enrichFromAnalyst` → `master.ScoreDecisionForTelemetry`
3. WS `tick` → `SetOnTelemetry` → `BroadcastTick` с полным `ScoreDecision` на **каждом** тике рабочего TF
4. WS игнорируется до `chartInitialized`
5. `pollLatestState` дополняет scoring на REST

**Scoring на wire:** `scoreDecisionForAnalyst` → `master.ScoreDecisionForTelemetry` (MTF sync) → `ApplyExecutionVetoes` → `MarketState` / `tickPayload`.

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
- **Sandbox / ReadOnly:** virtual positions; `BroadcastMarker` + `ChartTrade` на UI.
- **Backtest:** N+1 open entry (`btPendingEntry`), `SlippagePct` default 0.03%, PnL = Δprice × qty − fees.

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
| **Order Flow Store** | `domain/orderflow.go`, `exchange/ws.go` | Отдельный буфер aggTrade/liquidation тиков; агрегация в synthetic klines для tick-TF (`IsOrderFlowTimeframe`). Не смешивать с OHLCV `Marker` — параллельный data path в `DashboardServer.loadOrderFlowKlines`. |
| **Настройки `/api/settings/*`** | `server/webserver.go` | **Обязательны** для matrix/thresholds/indicators/risk/navigators. Без POST matrix scoring на live не отражает UI-тогглы (`config/matrix.json`). |
| **AI & Qdrant (долг #8)** | `vector_db/`, `Marker.VectorSnapshot()` | Векторный пайплайн: snapshot состояния → embedding → Qdrant similarity → будущий **AI veto** на входе (сейчас `memoryStore=nil` в main, veto off). |
| **Layer 2 — repaint-by-design** | `zigzag`, `geometry` в `layer2.go` | **Намеренно вне Snapshot/Restore.** Swing nodes и geometry мутируют на открытом баре — это repaint, не баг. Не добавлять им snapshot при рефакторингах без явного архитектурного решения. |

---

## Changelog / Статус (июнь 2026)

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
| `rsxMarkerState` | `strategy/rsx_incremental.go` | Deep-copy; intra-bar не растит `prices[]` |

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
**Frontend:** `updateScoringUI` + `#scoring-factors-list`; backtest crosshair → historical factors.

#### [Phase 5.62 — Factors UI panel — ✅]
**`web/index.html`:** `#scoring-telemetry-card`. **`web/style.css`:** factor badges. Backtest: crosshair scrubbing via `backtestChartPointsByTime`.

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
- UI: gray inactive factors при WAIT/veto; HTF keys `RSX_{tf}`, `Wozduh_{tf}`

### [🔜 OPEN DEBTS — приоритет]

| # | Долг | Файлы | Статус |
|---|------|-------|--------|
| **0** | ~~**Live indicator poisoning (Go)**~~ | `layer2.go`, `indicators/` | ✅ 5.57–5.58 |
| 1 | ~~**Navigator MTF math**~~ | `trendline_navigator.go` | ✅ 5.54 |
| 2 | **Navigator background zones** | `web/trendline_plugin.js` | 🟡 |
| 3 | ~~**Backtest viewport**~~ | `web/viewport.js` | ✅ 5.49 |
| 4 | **Backend 1M** — UI `1M`→`1w` | `server/`, `main.go` | 🟢 |
| 5 | **Microscope `endTime`** | `web/app.js` | 🟡 |
| 6 | **Forward lazy load** | `web/app.js` | 🟡 |
| 7 | **SQLite clear в Cache** | `server/webserver.go` | 🟡 |
| 8 | **Qdrant in main** | `main.go`, `vector_db/` | 🔜 |
| 9 | **expose `masterState`** | `server/webserver.go` | 🔜 |
| **10** | ~~**Factors UI panel**~~ | `web/app.js`, `index.html` | ✅ 5.62 + 5.72 gray WAIT |
| **11** | ~~**MTF walk-forward на live**~~ | `master.go`, `mtf_tracker.go` | ✅ 5.68–5.70 |
| **12** | **MTF scoring tune / weight calibration** | `scoring.go`, `matrix.json` | 🔜 NEXT |
| **13** | ~~**Backtest longScore/shortScore per bar**~~ | `backtest.go` | ✅ 5.63 |
| **14** | ~~**RSX/Wozduh HTF в scoring**~~ | `mtf_tracker.go`, `scoring.go` | ✅ 5.69 + 5.72 |
| **15** | **WS full Wozduh / mergeOsc** | `webserver.go`, `web/app.js` | 🟡 |
| **16** | ~~**Slippage model**~~ | `backtest.go` | ✅ 5.64 (0.03% default) |
| **17** | **max_drawdown enforcement** | `risk.go`, `master.go` | 🔜 |
| **18** | **fixed_pct stop в UI** | `computePositionStop` | 🔜 |
| **19** | **mergeKlines SSOT** | `kline_merge.go` vs `webserver.go` dup | 🟡 |
| **20** | **Flaky tests** | `rsx_settings_test.go` | ✅ 5.71 fixed |

---

## Справочник: ключевые файлы

| Область | Пути |
|---------|------|
| Score | `strategy/score_types.go`, `scoring.go`, `scoring_matrix.go`, `thresholds.go`, `risk.go` |
| Marker / Layer2 | `strategy/analyst.go`, `layer2.go`, `volatility.go`, `kline_merge.go` |
| MTF | `strategy/mtf_tracker.go`, `navigator_defaults.go`, `trendline_navigator.go`, `exchange/htf_provider.go` |
| Execution / Sizing | `execution/risk.go`, `strategy/risk_settings.go` |
| Live | `strategy/master.go`, `main.go`, `config/config.go` |
| Backtest | `strategy/backtest.go` |
| API / WS | `server/webserver.go`, `server/micro_broadcast.go`, `server/config/matrix.json` |
| Frontend | `web/app.js`, `web/viewport.js`, `web/style.css` |
| Правила AI | `.cursor/rules/senior-quant-architect.mdc`, `jeweler-protocol.mdc` |

**Env:** `TRADING_SYMBOL`, `TRADING_TIMEFRAME`, `READ_ONLY`, `SANDBOX_MODE`.

**Запуск:** `go run .` — dashboard `:8080`, WS Binance futures, paper/sandbox via `.env`.

**Следующий шаг:** калибровка весов scoring (`LongScoreThreshold`, HTF Regime A/B) + валидация телеметрии на live с MTF checkboxes (`4h`/`1d`/`1w`).
