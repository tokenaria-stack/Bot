# Trading Bot — System Memory

**Перед написанием новых модулей ВСЕГДА перечитывай этот файл.**

> **Снэпшот MEMORY (июнь 2026):** Layer 1 LOCKED + **Layer 2 WIRED** + **Layer 3 SCALPER BRAIN** + **Paper/Sandbox** + **Dashboard Terminal** + **Dynamic Thresholds** + **Signal Matrix Toggles** + **Backtest Engine** + **SQLite Kline Cache (WAL)** + **Binance Vision Bulk Downloader (futures)** + **Dashboard Frontend Stable Architecture** (Live/Backtest isolation, Safe Mode patching, WS thread-safe broadcast, **chartInitialized HTTP-first gate**, **no live prefetch/fitContent**) + **LuxAlgo Trendlines Navigator** (multi-pane, index-based internal math + time DTO export, **full 1:1 chart export**). **Mainnet** USD-M Futures. Dashboard: Live Chart | **Statistics** | **Backtester**. **Read-Only** → virtual paper; **Sandbox** (`HYPER_SCALP_TEST=true`) — vetoes bypassed.

## Changelog / Статус (июнь 2026)

### [Live Chart Stability + History Genesis — Phase 5.36–5.41 — сессия]

#### [Phase 5.36 — Удаление fitContent и авто-префетча (frontend) — ✅]
**Проблема:** принудительный `fitContent` и фоновый `scheduleHistoryPrefetch` вызывали DDoS `/api/history`, визуальные глитчи и скачки viewport.

**Fix (`web/app.js`):**
| Изменение | Детали |
|-----------|--------|
| **`renderState`** | Убран автоматический `fitProfessionalChart(liveChartData)`; при `loadedCandles.length > 0` — только `applySeriesData()` + `forceSyncChartTimeScales()` |
| **Prefetch** | Удалены `scheduleHistoryPrefetch()`, `historyPrefetchTimer`, `isInitialPrefetch` и все вызовы |
| **`maybeLoadHistory`** | Только ручной скролл (`range.from < 10`); после merge — `applySeriesData()` + сдвиг logical range; **без** `fitContent` / `forceSyncChartTimeScales` |
| **`fitProfessionalChart`** | Функция **оставлена** — используется в backtest full reload |

**Константы:** `LIVE_STATE_CANDLE_LIMIT = 3000` (только initial `/api/state`); `LIVE_HISTORY_CHUNK_LIMIT = 5000` (scroll pagination, без cap 3000).

#### [Phase 5.37 — Thundering Herd gap-fill (backend) — ✅]
**Проблема:** параллельные HTTP-запросы до завершения первого gap fill спавнили десятки горутин `FetchHistoricalKlines`.

**Fix (`server/webserver.go`):**
```go
var activeGapFills sync.Map // key: symbol_interval
```
`scheduleKlineGapFill` — `LoadOrStore(taskKey)` перед `go func()`; `defer activeGapFills.Delete(taskKey)` в горутине.

#### [Phase 5.38 — Data Lock при setData (frontend) — ✅]
**Проблема:** Lightweight Charts синхронно вызывает `onVisibleLogicalRangeChanged` во время `setData` → фантомные запросы истории.

**Fix (`applySeriesData` в `web/app.js`):**
- `isUpdatingData = true` на весь блок `setData`
- `finally`: `setTimeout(() => { isUpdatingData = false; }, 0)` — флаг снимается после внутренних microtasks библиотеки
- `attachProfessionalChartSync` — guard `if (isUpdatingData || !range) return`

#### [Phase 5.39 — WS/HTTP race + микро-массив guard (frontend) — ✅]
**Проблема:** при смене ТФ `clearChartData()` очищал график, но WS-тики через `applyPriceBar` → `setData([1 bar])` рисовали «растянутую» свечу до прихода `/api/state`.

**Fix (`web/app.js`):**

| Механизм | Поведение |
|----------|-----------|
| **`handleWSMessage`** | `if (!chartInitialized) return` после `shouldPaintLiveChart()`; **удалён** `chartInitialized = true` перед `applyPriceBar` |
| **`renderState`** | `chartInitialized = true` **только после** `applySeriesData()` + `forceSyncChartTimeScales()` |
| **`clearChartData` / `switchLiveTimeframe`** | `chartInitialized = false` — WS «глухнет» до нового `renderState` |
| **Микро-массив guard** | Если `candles.length < 20 && hasMore` → `setTimeout(loadDashboard, 500)` + `return` (не кормить график live-буфером во время gap fill) |

**Правило:** HTTP `/api/state` — единственный источник «исторического фундамента»; WS рисует только **после** инициализации. Буфера WS-очереди **нет** — тики применяются синхронно в `onmessage`.

**Stale HTTP guard:** `currentLiveRequestId` в `loadDashboard()` — игнор устаревших ответов при быстрых кликах по ТФ.

#### [Phase 5.40 — Отмена warmup-trim на абсолютном старте истории (backend) — ✅]
**Проблема:** `indicatorWarmupBars = 100` на старших TF (1W, 1M) отрезал годы ценовых свечей (`candles[trim:]`).

**Fix (`server/webserver.go`):**
```go
func historyWarmupTrim(gotBars, requestedBars, warmupTrim int) int {
    if gotBars < requestedBars+warmupTrim { return 0 } // уперлись в genesis
    return warmupTrim
}
```
Применяется в: `buildMarketState`, `handleHistory`, `handleHistoryChunk` (SQLite + REST paths).

**`buildChartSeriesTrimmed`:** при `trim <= 0` или `len(candles) <= trim` — возвращает **все** свечи и **все** осцилляторы (раньше oscillators обнулялись). Индикаторы на первых барах — нулевые/сырые значения Falcon (без паники).

#### [Phase 5.41 — Binance Futures Genesis clamp (backtest) — ✅]
**`exchange/klines.go`:**
```go
const BinanceFuturesGenesisMs int64 = 1567900800000 // 2019-09-08 UTC
func ClampFuturesHistoryStartMs(startMs int64) int64
```
- `handleBacktestRun` — `effectiveStartMs = ClampFuturesHistoryStartMs(startMs)` до `FetchHistoricalKlines`
- `PadBacktestStartMs` — padding не уходит раньше genesis

**Правило:** на абсолютном старте истории **не обрезать** price candles ради warmup; на промежуточных чанках trim=100 сохраняется для выравнивания осцилляторов.

### [Data Layer + Navigator Export — Phase 5.31–5.35 — сессия]

#### [Phase 5.31 — Unified Data Layer & SQLite Optimization — ✅]
- **`data/history_db.go`:** `PRAGMA journal_mode=WAL`, `PRAGMA synchronous=NORMAL`, index `idx_klines_lookup`
- **`exchange/klines.go` — `FetchHistoricalKlines`:** load full range from SQLite → interval-aware `detectKlineGaps` → per-gap Binance REST fetch + `SaveKlines`; on API failure → forward-fill synthetic klines (not saved); **never aborts backtest**

#### [Phase 5.32 — Binance Vision Bulk Downloader — ✅]
- **`cmd/history_sync/main.go`:** CLI для массового импорта monthly ZIP с Binance Vision → `data.SaveKlines`
- Flags: `-symbol`, `-interval`, `-year`, `-month` (optional), `-db` (default `history.db`)
- `batchSize = 45000`; temp dir cleanup via `defer os.RemoveAll`

#### [Phase 5.33 — USD-M Futures Vision — ✅]
- Flag **`-market`** (default: `futures`, alt: `spot`)
- Futures URL: `https://data.binance.vision/data/futures/um/monthly/klines/...`
- Spot URL: `https://data.binance.vision/data/spot/monthly/klines/...`

#### [Phase 5.34 — Clean Spot + Test Futures Import — ✅]
- Удалены все spot-записи `BTCUSDT` из `historical_klines` + `VACUUM`
- **Важно:** spot и futures **нельзя смешивать** в одной таблице — бот торгует futures
- Futures CSV содержит header row `open_time` — `importCSV` пропускает первую нечисловую строку
- Тест: `BTCUSDT 15m 2023-01` futures → **2976** klines

#### [Phase 5.35 — Remove Destructive Downsampling — ✅]
**Проблема:** при >50k свечей пропадали trendlines на истории — navigator считался по полному `histKlines`, а `chartData` в JSON прореживался (`backtestChartStep` / `backtestMaxChartPoints=50000`). Фронт маппил `line.x1` → `candles[x1]` по укороченному массиву → линии отбрасывались.

**Fix:**
| Слой | Изменение |
|------|-----------|
| **`strategy/backtest.go`** | Удалены `backtestMaxChartPoints`, `backtestChartStep()`, `recordChart` downsampling — **каждая** свеча бэктеста в `chartData` 1:1 |
| **`web/app.js`** | Удалён `backtestMaxCandles = 50000` cap в `refreshBacktestIndicatorSeries` |
| **`strategy/trendline_navigator.go`** | DTO export **без** искусственных лимитов — все `CompletedLines` + active + все markers |
| **`server/webserver.go`** | Без изменений — передаёт `chartData` как есть |

**Правило:** индексы баров navigator (`x1/x2/index`) и массив свечей на фронте **обязаны** совпадать 1:1. **Запрещено** downsampling/truncation `chartData` при экспорте backtest JSON.

#### [Phase 5.26–5.30 — Navigator math & UX (кратко)]
- **5.26:** `backtestMaxChartPoints` поднят до 50k (позже **отменён** в 5.35)
- **5.27–5.29:** LuxAlgo pivot parity — `pivotSwingFloorBar = n-right-1`, left `>=`, right strict `>`
- **5.28:** internal geometry **index-only** (`linePriceAtIndex`, `slopePricePerBar`); `Time1/Time2` только в DTO export
- **5.30:** 500ms debounce auto-update navigator/RSX settings (`scheduleIndicatorSettingsAutoUpdate`)

### [Dashboard Frontend — Phase 5 Navigator Trendlines — сессия]

LuxAlgo Trendlines Navigator: три независимых движка (Long/Medium/Short) на панелях Price / RSX / Wozduh в Backtest. Отрисовка через canvas-primitive (`TrendlinePrimitive`), payload через `POST /api/backtest/run`.

#### [Phase 5.13 — Absolute Time Anchoring — ✅]
**Проблема:** при lazy-load истории (pagination, +1000 баров в начало) линии и маркеры «слетали» — DTO передавал относительные индексы `x1/x2/index`, которые смещались при prepend свечей на фронте.

**Fix — координаты по Unix open time (ms):**

| Слой | Изменение |
|------|-----------|
| **`strategy/trendline_navigator.go`** | `ChartPoint.Time`; `Trendline.Time1/Time2`; `TrendlineMarker.Time`; DTO: `time1`, `time2`, `time` (+ legacy `x1/x2/index`) |
| **Engine** | `Execute(highs, lows, closes []float64, barTimes []int64)`; `histTimes[]`; `ExtractBarTimes(klines)` из `kline.OpenTime`; `SynthesizeBarTimesMS` для тестов |
| **`FindPivots`** | принимает `barTimes []int64`, пишет `ChartPoint.Time` |
| **`web/trendline_plugin.js`** | `timeToCoordinate(line.time1/1000)`; clip линий за краем viewport если одна точка вне зоны |
| **`web/app.js`** | `mapNavigatorLinesForChart` — `time1/time2` (fallback index); `buildNavigatorMarkers` — `chartTime(marker.time)` |

**Не в scope 5.13:** background zones (`NavigatorZoneDTO`) — всё ещё index-based; могут дрейфовать при pagination.

#### [Phase 5.5–5.12 — Navigator pipeline (кратко)]
- **5.5:** TF buttons, popup Ok, dash/dotted styles, Bar Color через `candleSeries.setData()`
- **5.6–5.10:** Safe Mode payload injection, Ok handler (capture), DOM sync popups/matrix, `buildFinalBacktestPayload()`, deep merge settings
- **5.11:** Go JSON tags `BacktestRunSettings` (`matrix`, `navigators`, `risk`)
- **5.12:** Trendlines legend всегда видна в backtest; `applyNavigatorOverlays` → `setData([])` при disabled; source routing `NAVIGATOR_SOURCE_MAP` + backend `ui.Source = navigatorPaneToSource(pane)`

#### [Navigator Architecture — ключевые файлы]

| Файл | Роль |
|------|------|
| `strategy/trendline_navigator.go` | `NavigatorEngine`, `RunNavigatorAggregator`, `BuildAllNavigators`, DTO |
| `strategy/backtest_request.go` | `BacktestRunSettings`, `ResolveBacktestNavigators`, `ResolveBacktestMatrix` |
| `strategy/backtest.go` | `BuildAllNavigators(panes, klines, rsx, wozduh)` в run |
| `web/trendline_plugin.js` | Canvas primitive: lines + background zones |
| `web/app.js` | Legends, popups, `buildFinalBacktestPayload()`, `applyNavigatorOverlays`, Safe Mode |
| `web/index.html` | Static popups `#popup-price/rsx/wozduh`, matrix checkboxes |

#### [Navigator payload shape]
```json
{
  "symbol", "interval", "startDate", "endDate",
  "settings": {
    "matrix": { "useTrendlines": true, ... },
    "navigators": {
      "price": { "enabled": true, "useLong": true, "longLen": 60, ... },
      "rsx":   { ... },
      "wozduh": { ... }
    },
    "risk": { ... }
  }
}
```

#### [Navigator UI rules]
- **Legends:** 👁️ ⚙️ Trendlines — **всегда** в backtest на всех 3 панелях (не скрывать при unchecked targets)
- **Ok в popup:** global capture на `.btn-ok` → `buildFinalBacktestPayload()` → `window.currentBacktestPayload` → Safe Mode pipeline
- **Line filter:** по checkbox solid/dashed/dotted (не Term dropdown)
- **Source:** backend **форсирует** source из ключа pane (`price`→Price, `rsx`→RSX, `wozduh`→Wozduh)
- **Bar Color:** `applyNavigatorBarColors()` — tint свечей; preserve viewport в Safe Mode
- **Plugins:** `priceNavigatorPlugin`, `rsxNavigatorPlugin`, `wozduhNavigatorPlugin` на respective series

#### [Navigator DTO (ответ backtest)]
```json
{
  "navigators": {
    "price": {
      "lines": [{ "x1", "y1", "x2", "y2", "time1", "time2", "color", "style" }],
      "markers": [{ "index", "time", "price", "text", "color", "type" }],
      "barColors": [{ "index", "color" }],
      "backgroundZones": [{ "startIndex", "endIndex", "color" }]
    }
  }
}
```
**Рисовать линии/маркеры по `time1/time2/time` (секунды на фронте через `chartTime()`), не по index.**

### [Dashboard Frontend — Phase 3.9 + Safe Mode — сессия]

#### [High-Performance Safe Mode — `web/app.js`]
- **`window.__isSettingsUpdating`** — state lock: блокирует `maybeLoadBacktestHistory` и `restoreBacktestChartView` во время конвейера (защита от спама lazy loader).
- **Lean pipeline:** debounce 500ms → `POST /api/settings/indicators` → `POST /api/backtest/run` с **`patchIndicatorsOnly: true`** (без лишнего GET `/api/history/chunk`).
- **`applyBacktestIndicatorPatch()`** — surgical update: только RSX + trade markers + SL overlay series; **не** вызывает `candleSeries.setData()` / `applyPriceToChart()` если свечи уже на графике.
- **`canPatchBacktestIndicatorsOnly()`** — определяет режим patch vs full reload.

#### [Chart alignment + crosshair — `web/app.js`]
- **`CHART_PRICE_SCALE_MIN_WIDTH = 70`** — `minimumWidth` на right price scale всех панелей (Price / Wozduh / RSX) для вертикального выравнивания.
- **`syncPaneCrosshairs()`** — локальная bidirectional sync курсора **внутри** одного контекста (Price↔Wozduh↔RSX); Live и Backtest **не связаны**.
- Viewport restore: `getVisibleRange()` + deferred `setVisibleRange` (rAF + 10ms/50ms); при Safe Mode patch — **без** `setVisibleRange` / `fitContent`.

#### [Settings menus UX — `web/app.js`, `web/index.html`]
- Закрытие по **`mousedown`** вне меню (не `click`) — fix drag-to-select в input.
- **Enter** в полях RSX/Risk/Wozduh → blur + focus next field; на Save → click.
- Кнопки **Save** в RSX-меню (Live + Backtest) и Risk — закрывают меню после успешного сохранения.

#### [Backtest trade markers — visual cleanup — `web/app.js`]
- **SL line series удалена** (`slLineSeries`, `buildStopLossLineData`) — стоп-лосс на графике не рисуется.
- **`entryMarkerSeries`** (точная цена, `inBar`): LONG `#9C27B0` circle size 1; SHORT `#E91E63` circle size 1.
- **`candleSeries` markers:** вход — зелёные/красные стрелки above/below bar (сигнальные); выход — оранжевый circle `#FF9800` inBar (все выходы).

#### [Statistics tab — resizable history — `web/style.css`, `web/index.html`]
- **`.trade-history-wrap`:** `resize: vertical`, `overflow: auto`, min 150px, max 80vh, `border-top: 2px solid #333`.
- **`#equity-chart`:** `flex: 1 1 200px` — занимает освободившееся место при сжатии таблицы.

#### [WebSocket concurrency fix — `server/webserver.go`]
- **`WSClient`** — обёртка `{ Conn, mu sync.Mutex }` с thread-safe `WriteMessage` / `WriteJSON` / `Close`.
- **`clients map[*WSClient]bool`**; `broadcast()` и `pushMicroTicks()` — snapshot клиентов, запись **вне** `clientsMu`, per-client mutex.
- Fix: `panic: concurrent write to websocket connection` (gorilla/websocket).

### [Dashboard Frontend — Stable Architecture — сессия]

#### [Live vs Backtest isolation + UI Pipeline — `web/app.js`]
- **Абсолютная изоляция Live / Backtest:** отдельные `liveChartData` / `backtestChartData`, отдельные RSX-меню (`#rsx-wrap` / `#bt-rsx-wrap`). **`syncMultipleCharts` между вкладками — удалена навсегда.**
- **localStorage:** `dashboard_rsx_settings_live` / `dashboard_rsx_settings_backtest` (+ legacy keys для миграции).
- **Backtest UI Pipeline (Safe Mode):** debounce **500ms** → `POST /api/settings/indicators` → `POST /api/backtest/run` с `patchIndicatorsOnly: true` — **без** перерисовки свечей.
- **Viewport Preservation:** `getVisibleRange()`; при patch — не трогать price series и не вызывать `fitContent()` / `setVisibleRange()`.
- **Backtest trades viz:** entry dots на `entryMarkerSeries`; exit orange circles + signal arrows на `candleSeries` (см. §4 STABLE ARCHITECTURE).
- **Risk / RSX API:** строго **snake_case**; `coerceRsxSettingsForAPI()` перед POST.
- **Backtest entries (Go):** `buildBacktestRSXMarkersMap` → входы на L/LL/S/SS.

### [Backtest Engine + SQLite Cache — сессия]

#### [Backtester UI — `web/index.html`, `web/app.js`]
- **`#tab-backtest`:** форма (Symbol, TF, Start/End date), `#backtest-logs`, кнопка **Run Backtest**.
- **`#tab-stats`:** grid метрик (Total Trades, Win Rate, Net Profit, Profit Factor, Max DD, Recovery Factor).
- **Equity curve:** `#equity-chart` — Lightweight Charts LineSeries (`initEquityChart`).
- **Trade history:** `#trade-history-table` (Time, Side, Entry, Exit, PnL%, Duration).
- **`runBacktest()`** → `POST /api/backtest/run` → `renderBacktestStats()` → auto-switch на **Statistics** только при явном Run (`switchTab: true`); смена TF / авто-обновление RSX — **остаются на Backtest** (`switchTab: false`).

#### [Backtest Engine — `strategy/backtest.go`]
- **`BacktestEngine.Run(candles)`** — изолированный `ChiefAnalyst`, capital **$10 000**.
- **`chartData`:** **все** свечи бэктеста 1:1 в JSON (без downsampling — Phase 5.35).
- Прогон свечей: `UpdateKline` → `GenerateStreamingReport` (min **50** баров warmup).
- **Вход (IDLE):** `EvaluateScalpSignal` + `RiskManager.ValidateEntry` → virtual position (SL + TP 2R via `VirtualTakeProfitPrice`).
- **Выход:** SL/TP по High/Low бара; opposite signal ≥ dynamic threshold.
- **Метрики:** NetProfit (%), WinRate, ProfitFactor, MaxDrawdown (%), RecoveryFactor; equity каждые **100** баров + после сделок.
- **`ParseBacktestDateRange()`** — YYYY-MM-DD → ms UTC.

#### [API — `POST /api/backtest/run`]
- `BacktestRequest`: symbol, interval, startDate, endDate.
- Flow: `ResolveTimeframe` → `FetchHistoricalKlines` → `BacktestEngine.Run` → `BacktestResult` JSON.
- Требует `DashboardServer.rest` (Futures client); `data.InitDB()` при каждом запросе.

#### [SQLite History Cache — `data/history_db.go`]
- Драйвер: **`modernc.org/sqlite`** (pure Go, **без CGO**).
- Файл: **`history.db`** (в `.gitignore`); `InitDB()` в **`main.go`** при старте.
- Таблица `historical_klines` (PK: symbol, interval, open_time); index `idx_klines_time`.
- **`SaveKlines`** — транзакция + `INSERT OR IGNORE`; **`LoadKlines`** — SELECT по диапазону ms.
- **`ExpectedKlineCount`** — оценка полноты кэша.

#### [Smart fetch — `exchange/klines.go`]
- **`FetchHistoricalKlines`:** load SQLite range → **`detectKlineGaps`** (interval-aware step) → per-gap **fapi/v1/klines** fetch + `SaveKlines` → merge.
- On API failure for a gap: forward-fill synthetic bars (logged, **not** saved to DB); backtest continues.
- Логи: `[Klines] loading SQLite`, `[Klines] gap N/M`, `[Warning] API failed for gap`.

#### [Bulk history import — `cmd/history_sync/main.go`]
- Binance Vision monthly ZIPs → CSV → `data.SaveKlines`
- **Default market: futures** (`-market=futures`); spot только явно (`-market=spot`)
- **Не смешивать** spot и futures klines для одного symbol в `historical_klines`

### [Signal Matrix Toggles — сессия]

#### [Backend — `strategy/scoring_matrix.go`]
- **`ScoringMatrix`** (RWMutex): 13 toggles — `useRSX`, `useWozduhCross`, `useRedCross`, `useGeometry`, `useGeometryBounce`, `useGeometryTriangle`, `useDivergence`, `useFib`, `useExpRegime`, `useJurikTrend`, `useWozduhSpike`, `useAD`, `useAOCross` (default **all true**).
- **`scoreLong` / `scoreShort`:** каждый фактор обёрнут в проверку флага.
- **`POST /api/settings/matrix`** — JSON bool map → `SetScoringMatrix`.

#### [Frontend]
- Кнопка **🎛 Rules** в slim-header → modal **Signal Matrix** (чекбоксы).
- **`SCORING_MATRIX_KEY = 'bot_scoring_matrix'`** + `syncScoringMatrix()` → localStorage + POST.

### [Dashboard Terminal UI + Dynamic Thresholds — сессия]

#### [Slim Header + Tabs — `web/index.html`, `web/app.js`]
- **Вкладки:** `Live Chart` | `Live Statistics` | `Backtester`.
- **`#tab-live`:** индикаторы + 3 панели графиков (Price, Wozduh, RSX).
- **`.slim-header`:** `#bot-status`, `#sandbox-badge`, mini-bars L/S + пороги, кнопка **🎛 Rules** (Signal Matrix modal).
- **`updateScoringUI()`** — score + анимация шкал: `width = min(100, score/threshold×100)%`; bot status из trades или `masterState`.
- **Tab switch:** при возврате на Live Chart → `resizeAllCharts()` + `window.dispatchEvent('resize')`.

#### [Dynamic Thresholds — Backend + Frontend sync]
- **`strategy/thresholds.go`:** потокобезопасные пороги (`sync.RWMutex`), default **70**, clamp **10–200**.
  - `LongScoreThreshold()`, `ShortScoreThreshold()`, `SetScoreThresholds(long, short)`.
- **`strategy/scoring.go`:** `EvaluateScalpSignal` использует динамические пороги (отдельно long/short); vetoes **не** в scoring.
- **`strategy/master.go`:** opposite-signal exit сравнивает score с соответствующим порогом.
- **`POST /api/settings/thresholds`** — JSON `{"long": 70, "short": 70}` → обновление порогов на сервере.
- **Frontend:** `THRESHOLDS_KEY = 'bot_thresholds'` в **localStorage**; `syncThresholds()` на `change` инпутов + при старте страницы (restore → POST на бэкенд после рестарта сервера).

#### [FSM + Scoring architecture — актуально]
- **FSM:** `IDLE` ↔ `IN_POSITION` (состояние **HUNTING удалено**; macro 15m AO gate убран).
- **Hunt TF:** default `1m` (`DefaultHuntTimeframe`); вход на **close hunt TF**.
- **`EvaluateScalpSignal`:** чистый scoring (full matrix); порог ≥ dynamic threshold, победившая сторона > противоположной.
- **`strategy/risk.go` — `RiskManager.ValidateEntry()`:** Fee Barrier, CLIMAX, Macro Jurik, AI Veto; **sandbox bypasses all**.
- **Paper:** `readOnly` (пустые keys) → virtual entry/exit, markers в API; **live keys** → real orders.

#### [Scoring Matrix — toggles + weights (long / short зеркально)]
Флаги в **`strategy/scoring_matrix.go`**; баллы в **`strategy/scoring.go`**:
| Factor | Points |
|--------|--------|
| RSX L/LL or S/SS | +35/+45 |
| Wozduh vol cross lime/red | +35 |
| Red×Green cross | +35 |
| Geometry breakout | +30 |
| Divergence score | ± from `Report.Divergence` |
| Fib 0.618 active | +20 |
| Regime EXPANSION | +15 |
| Jurik tailwind | +20 bull / +15 recovery / +20 bear |
| Wozdux volume spike | +15 |
| Geometry bounce | +25 |
| Triangle | +10 |
| AD accumulation/distribution | +20 |
| AO cross zero | +15 |

### [Wozduh Panel + Chart UX + Trade Markers — сессия]

#### [Wozduh — `strategy/falcon.go` + dashboard]
- Панель **Wozduh**; полный набор линий по PineScript `RSIVolume_2graf.02[wozdux]`:
  - `rsiPrice` red, `emaRsi` green (EMA period **7** = `ll`), `rsiRsi` orange, `rsiHl2` purple
  - **wt11/wt22 (исправлено по Pine):** `rsi11 = RSI(VWEMA(close,24), 24)`; `wt11 = EMA(rsi11, 12)` blue; `wt22 = EMA(rsi11, 5)` aqua — **одна база rsi11**
  - `rsiHl2Vol` navy; `macdRsi` black (+50); `rsiAd` maroon (упрощённый; в Pine — cum+sum+cci)
  - **Vol channel:** SMA(wt22,24) ± 1.6185·σ; границы `#00FA9A` dashed
  - **Price channel:** SMA(rsiPrice,24) ± 1.6185·σ
  - **Cross dots:** `volCrossMarker` lime/red при wt11×wt22 cross → маркеры на `rsiVolSlow`
- UI: **Wozduh ⚙** popup; defaults = Pine inputs; **localStorage** `wozduh_visibility_prefs`

#### [RSX — lookback + macro P filter]
- `rsxLookback` в `/api/state` и `/api/history`; UI **RSX ⚙**; macro P только ±7 баров

#### [Chart UX — `web/app.js`]
- Sync time scale + crosshair **внутри одного контекста** (Live: 3 панели; Backtest: 3 панели) — **не между Live и Backtest**
- **Data Lock** (`isUpdatingData`); `applySeriesData` — `setTimeout(0)` defer; poll → `.update()` only
- **`chartInitialized` gate:** WS-тики игнорируются до успешного `renderState` (HTTP-first); сброс в `clearChartData`
- **Нет live prefetch** — история только по ручному скроллу (`maybeLoadHistory`, `range.from < 10`)
- **Нет auto `fitContent`** на live — только `forceSyncChartTimeScales`; `fitProfessionalChart` — backtest only
- `minBarSpacing: 0.001`

#### [Trade markers — price chart]
- `ChartTrade` с `kind` entry/exit; `sessionTrades` + `applyTradeMarkers()` — BUY/SELL arrows + EXIT circles.
- Persist trades в `/api/state` — маркеры переживают hard refresh.

#### [Veto — актуальная карта]

**`strategy/scoring.go`:** только scoring math; **без vetoes**.

**`strategy/risk.go` — `RiskManager.ValidateEntry()`:** Fee Barrier, CLIMAX, Macro (Jurik), AI Veto (memory≠nil). Sandbox → `nil` (bypass).

**`strategy/master.go` FSM:** `IDLE` ↔ `IN_POSITION`; hunt на close hunt TF; **ReadOnly** → paper virtual fills; **sandbox** → risk bypass.

**`.env`:** `HYPER_SCALP_TEST=true` → sandbox; keys = `BINANCE_*`. `memoryStore=nil` в main → AI Veto off.

### [Paper Trading — ✅ реализовано]

1. **ReadOnly** (пустые `BINANCE_*`) → virtual entry/exit без `CreateMarketOrder`
2. Virtual SL/TP на live 1m ticks; signal exit на close hunt TF
3. `ChartTrade` + `BroadcastMarker` → dashboard markers (entry/exit)
4. Trades в `/api/state` для истории на графике
5. Badge **`SANDBOX`** (sandbox mode) / paper via readOnly path

### [🔜 NEXT]

1. Expose `masterState` в `/api/state` для `#bot-status` (сейчас — derive из trades)
2. Qdrant in main + trade outcome logging
3. Backtest: progress streaming / partial cache fill для неполных диапазонов
4. Backtest: commission/slippage model

### [Dashboard Session — WS, Order Flow, RSX, UI fixes] *(исторический снэпшот)*

#### [Critical — Binance WebSocket]
- **Корневая причина «заморозки» графиков:** legacy URL `wss://fstream.binance.com/stream?streams=` подключался (101), но **не отдавал данные**.
- **Fix:** `exchange/network.go` → `BaseCombinedMarketMainURL` = `wss://fstream.binance.com/market/stream?streams=`.
- Smoke-тест: `exchange/ws_smoke_test.go` (`TestBinanceMarketCombinedStream`).

#### [Critical — Kline JSON parse]
- Binance на `/market/stream` шлёт OHLC иногда как **числа**, не строки → `[WS ERROR] kline parse: ... field .k.l`.
- **Fix:** `exchange/jsonflex.go` — тип `flexString` (string | number → float64).
- **Fix:** `flexString` для полей kline в `exchange/ws.go`.

#### [Live streaming — стандартные TF]
- `strategy/master.go` — callback `SetOnKlineBar` для всех TF.
- `server/webserver.go` — `BroadcastPriceBar(tf, candle)` для 15m/1h/4h; `BroadcastTick` остаётся для 1m + телеметрия.
- `ChartCandle` + `tickPayload`: поле `volume`; `ChartOscillator`: `color`, `marker`, `rsx`.

#### [Order Flow — подтверждено работает]
- Секунды (1s…) и тики (100ticks…) рендерятся после fix WS URL.
- `server/micro_broadcast.go` — push micro bar каждые 500ms.
- Frontend: `pollOrderFlowState`, overlay «Buffering…» при `tickBufferLen < 500`.

#### [RSX Jurik Dashboard — `strategy/rsx_chart.go`]
- **`BuildRSXChart(klines, rsxValues)`** — batch-обогащение после **одного** прохода `FalconEngine` (Jurik не ломается двойным Evaluate).
- **Вход RSX:** HLC3 `(H+L+C)/3` — уже в `FalconEngine.Evaluate`.
- **Цвет (без гистерезиса):** rising & RSX>50 → `#089981`; falling & RSX<50 → `#f23645`; иначе `#e1d2b5`.
- **Маркеры:** только на **5-bar pivot** (±2 бара); подтверждение на баре `i+2`, маркер на баре `i`.
- **Gatekeeper:** Pivot High + RSX>60 → `CheckClassicDivergence` с prev pivot ≤90 баров → `S`/`SS` или `P`; Pivot Low + RSX<40 → `L`/`LL` или `P`; зона 40–60 — без маркеров.
- **Адаптер:** ClassA/C → SS/LL; ClassB → S/L. Кастомный алгоритм дивергенций **удалён** — только универсальный модуль `indicators.CheckClassicDivergence`.
- Live WS: `rsxColor` через `analyst.JurikRSXColor()` (= `RSXColor`).

#### [Dashboard UI — `web/app.js`, `web/index.html`]
- **Terminal layout:** toolbar + **slim-header** (status, scores, thresholds) + **tabs-nav** + tab panels.
- **Избранные TF:** сортировка по возрастанию (`tfSortKey`: ticks → seconds → minutes → hours → days).
- **3 панели:** Price (+ Volume histogram) | Wozdux | RSX — с **resize handles** (`pane-resize`, localStorage `dashboard_pane_heights`).
- **Thresholds UI:** localStorage `bot_thresholds` ↔ `POST /api/settings/thresholds` ↔ `strategy/thresholds.go`.
- **RSX frontend:** одна `rsxSeries`, `{ time, value, color }` per-point (LWC 4.2); маркеры `setMarkers` из `marker` (P/L/LL/S/SS).
- **RSX levels:** price lines 30 (OS), 50 (MID), 70 (OB) — пунктир.
- **`mapRSXData` / `mergeOsc`:** явное сохранение `color`, `marker`, `rsx` (fix потери полей при merge).

#### [Chart scales — fix исчезновения графиков]
- **Price chart:** `PriceScaleMode.Logarithmic`, `autoScale: true` — через `createPriceChartOptions()`.
- **Wozdux + RSX:** `PriceScaleMode.Normal`, `autoScale: true`, `scaleMargins: { top: 0.05, bottom: 0.05 }` — через **независимые** `createWozduxChartOptions()` / `createRSXChartOptions()` (без shared objects).
- **Удалено (ломало рендер):** `autoScale: false` + `setVisibleRange()`, общий `chartOptions` с log-scale для индикаторов, `applyIndicatorScales()`.

#### [JS bugs fixed]
- `Uncaught SyntaxError: Identifier 'RULER_IDLE' has already been declared` — дублирующий блок констант линейки удалён.
- Двойной `falcon.Evaluate()` в `buildChartSeries` — заменён на один проход + `evaluated[]`.

### [UI & Dashboard] *(базовый снэпшот)*
- TradingView-стайл интерфейс (`#131722`, Jurik/Wozdux palette).
- **Slim header:** bot status, sandbox badge, L/S mini score bars + threshold inputs.
- **Tabs:** Live Chart | Live Statistics | Backtester.
- Логарифмическая шкала (Log/Auto) и переключатель типа графика (Candles/Bars/Line).
- Dropdown таймфреймов с **Избранным** (localStorage) + custom TF input.
- **Ruler v2:** IDLE → MEASURING → FIXED; сброс по третьему клику / Escape.
- Lazy load истории (scroll-left) с dedupe + хронологической сортировкой.
- Cache-busting (`_t=`) на `/api/state` и `/api/history`.

### [Data & Backend]
- **Mainnet:** Binance USD-M Futures (`https://fapi.binance.com`, `wss://fstream.binance.com`).
- **`/api/state`**, **`/api/history`**, **`POST /api/settings/thresholds`**, **`POST /api/settings/matrix`**, **`GET/POST /api/settings/indicators`**, **`GET/POST /api/settings/risk`**, **`POST /api/backtest/run`**, **`/ws`**.
- **SQLite cache:** `history.db` — klines для backtest (см. `data/history_db.go`).
- `/api/history` lazy load + `indicatorWarmupBars = 100` для бесшовных осцилляторов (**trim отменяется** на абсолютном старте истории через `historyWarmupTrim`).
- **`activeGapFills sync.Map`** — дедупликация фоновых gap-fill горутин per `symbol_interval`.
- **`BinanceFuturesGenesisMs`** (`exchange/klines.go`) — floor start date для backtest (2019-09-08).
- Фильтрация `Close == 0`, NaN/Inf; dedupe по `time`; `endTime` sec→ms для Binance.
- **Order Flow:** `domain.TickBuffer` (100k aggTrades), `LiquidationBuffer` (1k), WS `@aggTrade` + `@forceOrder`, `SynthesizeMicroCandles` для tick/second TF.
- **RSX chart series:** `BuildRSXChart` → `ChartOscillator{color, marker, rsx}`; pivot markers + classic divergence adapter.

### [Security]
- **Read-Only режим:** пустые API keys → публичные klines/WS + dashboard; `RecoverState` и торговля пропускают private API.

## Актуальный снэпшот (обязательные правила)

Ядро бота **реализовано и работает на Binance USD-M Futures Mainnet**. Ниже — зафиксированные правила, которые **ОБЯЗАТЕЛЬНЫ** для всего проекта.

---

## Dashboard Frontend — STABLE ARCHITECTURE (июнь 2026)

> **КРИТИЧЕСКОЕ:** Перед любыми правками `web/app.js` / `web/index.html` перечитай этот блок. Нарушение этих правил приводит к «тихим» падениям API, сбросу зума и конфликтам вкладок.

### 1. Абсолютная изоляция Live vs Backtest

| Правило | Детали |
|---------|--------|
| **Контексты** | Live (`#tab-live`) и Backtest (`#tab-backtest`) — **полностью независимые** runtime-контексты |
| **Графики** | `liveChartData` и `backtestChartData` — отдельные инстансы Lightweight Charts; **никогда** не шарить series между вкладками |
| **Синхронизация** | **`syncMultipleCharts` между Live и Backtest — отменена навсегда.** Sync time scale допустим **только внутри** одного контекста (Price \| Wozduh \| RSX) |
| **RSX settings** | Раздельное состояние: `liveRsxSettings` / `backtestRsxSettings` |
| **localStorage** | `dashboard_rsx_settings_live` / `dashboard_rsx_settings_backtest` (ключи `LS_RSX_SETTINGS_LIVE_KEY` / `LS_RSX_SETTINGS_BACKTEST_KEY`) |
| **RSX UI** | Live: `#rsx-wrap`; Backtest: `#bt-rsx-wrap` — floating/fixed меню независимы |
| **TF toolbar** | `getActiveTf()` / `switchTimeframe(tf, event)` маршрутизирует в `switchLiveTimeframe` или `switchBacktestTimeframe` по активной вкладке |
| **Live init gate** | `chartInitialized` — WS блокируется до `renderState`; сброс в `clearChartData` при смене TF |
| **Tab switch → Live** | При возврате на Live: `pushRsxSettingsToServer(liveRsxSettings)` — восстановление live-настроек на сервере (сервер хранит RSX как singleton) |
| **Tab switch → Backtest** | Backtest использует свои сохранённые настройки; перед run/pipeline — push backtest RSX на сервер |

### 2. Конвейер реактивности Backtest (UI Pipeline — Safe Mode)

Любое изменение RSX-полей в `#bt-rsx-wrap` запускает **строгую асинхронную цепочку**:

```
input/change (delegation на document)
  → onBacktestRsxFieldEdit
  → debounce 500ms
  → runBacktestAutoUpdatePipeline
      window.__isSettingsUpdating = true
      A. syncRsxIndicatorSettings('all', 'backtest')
           → coerceRsxSettingsForAPI → POST /api/settings/indicators
      B. runBacktest(false, {
           preserveView: true,
           skipSettingsPush: true,
           switchTab: false,
           patchIndicatorsOnly: true,
         })
           → POST /api/backtest/run
           → applyBacktestIndicatorPatch()   // RSX + markers ONLY, no candle setData
      window.__isSettingsUpdating = false (finally)
```

| Механизм | Значение |
|----------|----------|
| **State lock** | `window.__isSettingsUpdating` — блокирует lazy history + viewport restore |
| **Delegation** | `initBacktestAutoUpdateDelegation()` — document-level listener |
| **In-flight guard** | `backtestAutoUpdateInFlight` + retry 400ms |
| **Patch mode** | `canPatchBacktestIndicatorsOnly()` / `applyBacktestIndicatorPatch()` |
| **Loading** | `#backtest-loading` — visibility only |
| **Числа в JSON** | `coerceRsxSettingsForAPI()` — обязательно |

**`refreshBacktestIndicatorSeries()`** — legacy helper для TF change; в RSX pipeline **не вызывается** (данные RSX приходят из backtest run response).

**Смена TF в Backtest** (`#bt-interval` или toolbar):

```
switchBacktestTimeframe(tf, event) → preventDefault + stopPropagation
  → handleBacktestIntervalChange
      → clear candles/osc
      → runBacktest(true, { switchTab: false })   // full reload, fitContent OK
```

**Run Backtest (кнопка):** `runBacktest(true)` → `switchTab('tab-stats')`.

### 3. Viewport Preservation + Chart Alignment

| ✅ Разрешено | ❌ Запрещено при Safe Mode patch |
|-------------|----------------------------------|
| Не вызывать `candleSeries.setData()` | `applyPriceToChart` / `applyOscillatorToChart` (Wozduh) |
| `getVisibleRange()` для full reload | `getVisibleTimeRange()` |
| `minimumWidth: 70` на все price scales | `fitContent()` при settings patch |
| `syncPaneCrosshairs()` внутри контекста | `setVisibleRange()` при `__isSettingsUpdating` |
| Deferred restore (rAF + 10ms) только при **full** reload | `reinitBacktestChart()` при RSX change |

**Time scale sync** внутри контекста: `subscribeVisibleLogicalRangeChange` → sync all 3 panes (Price/Wozduh/RSX).

### 4. Отрисовка сделок Backtest (Lightweight Charts)

| Элемент | Реализация |
|---------|------------|
| **Entry (exact price)** | `entryMarkerSeries` — `inBar` circle: LONG `#9C27B0`, SHORT `#E91E63`, size 1 |
| **Entry (signal)** | `candleSeries` markers — green/red arrows below/above bar (`buildBacktestTradeMarkers`) |
| **Exit** | `candleSeries` — orange circle `#FF9800`, `inBar`, size 1 (все выходы) |
| **RSX signals** | `rsxSeries.setMarkers()` — L/LL/S/SS/P |
| **Spike markers** | Wozduh vol cross dots на price chart |

**Stop-Loss line series — удалена.** SL не рисуется на графике (логика SL остаётся в Go: `position_stop.go`).

### 4b. Trendlines Navigator (Backtest — Phase 5)

| Правило | Детали |
|---------|--------|
| **Time anchoring** | DTO экспортирует `time1/time2/time` (ms); internal math — **bar indices** (`x1/x2`). Фронт: `chartTime(time1)` primary, `navigatorBarIndexToTime(x1, candles)` fallback — **требует 1:1 candles ↔ backtest bars** |
| **No export limits** | Все completed + active lines и все markers в JSON; **запрещён** downsampling `chartData` |
| **Lazy load** | При prepend свечей **не** пересчитывать координаты по index; plugin рисует по time |
| **Payload** | `buildFinalBacktestPayload()` — единый объект перед каждым backtest run; inject в `window.currentBacktestPayload` |
| **Safe Mode** | Ok в navigator popup → debounce pipeline → POST backtest с полным `settings.navigators` |
| **Clear canvas** | Disabled pane / пустой DTO → `navigatorPlugin.setData([])` |
| **Source routing** | Линии Price/RSX/Wozduh — на **своей** series; backend форсирует `Source` из pane key |
| **Background zones** | ⚠️ ещё index-based (`startIndex/endIndex`) — known limitation |

### 5. Settings Menus UX (RSX / Wozduh / Risk / Navigator)

- Закрытие вне меню: **`mousedown`** на document + `isPanelSettingsInteractionTarget()` (не `click`).
- **Enter** в input/select → blur → next field; на Save → programmatic click.
- **Save** закрывает меню (`menu.hidden = true`) после успешного POST.
- RSX Save: Live → `syncRsxIndicatorSettings`; Backtest → `runBacktestAutoUpdatePipeline`.

### 6. Statistics Tab

- **`.trade-history-wrap`:** vertical resize (150px–80vh), scroll overflow.
- **`#equity-chart`:** flex-grow — расширяется при уменьшении таблицы истории.

### 7. Risk API и RSX Settings (snake_case)

**Endpoints:**

| Method | Path | Payload |
|--------|------|---------|
| GET/POST | `/api/settings/indicators` | RSX: `length`, `div_lookback`, `signal_length`, `source`, `pivot_radius`, `div_method` |
| GET/POST | `/api/settings/risk` | `risk_per_trade`, `atr_multiplier`, `stop_loss_type`, … |

**Правила:**

- Frontend → Backend: **строго snake_case**; все numeric fields — **JSON numbers**, не strings
- `coerceRsxSettingsForAPI()` — единая точка приведения типов перед `JSON.stringify`
- **`stop_loss_type: "fractal_atr"`** — стоп-лосс = fractal ± ATR (`strategy/position_stop.go`, `strategy/risk_settings.go`)
- Альтернатива: `"fixed_pct"` (UI select `#risk-stop-loss-type`)
- RSX `div_method`: `"fractal"` \| `"tv"` — разные алгоритмы pivot/divergence markers

**Backend singleton:** сервер держит одну копию RSX settings; frontend **обязан** push нужного контекста перед backtest и restore live при tab switch.

### 8. Dashboard WebSocket Broadcast (`server/webserver.go`)

| Правило | Детали |
|---------|--------|
| **WSClient** | `{ Conn *websocket.Conn, mu sync.Mutex }` — единственная точка записи в сокет |
| **Clients map** | `map[*WSClient]bool` + `clientTF map[*WSClient]string` |
| **broadcast()** | Snapshot клиентов под `clientsMu`, `WriteMessage` **вне** map lock |
| **pushMicroTicks()** | То же через `WriteJSON` |
| **Read path** | `conn.ReadMessage()` в goroutine handleWS — без mutex (read-only) |

gorilla/websocket: **один writer** на connection; нарушение → panic.

### 9. Backtest Engine — логика входов (Go)

- **`buildBacktestRSXMarkersMap(candles)`** — precompute L/LL/S/SS/P по `CloseTime`
- **Вход:** на баре с маркером L/LL (long) или S/SS (short) — **не** через порог scoring matrix ≥70
- **`position_stop.go`:** SL по `GetRiskSettings().StopLossType`; для `fractal_atr` — DownFractal/UpFractal ± ATR×multiplier
- Chart data в ответе backtest включает RSX markers для frontend overlay

### 10. Анти-паттерны (НЕ возвращаться)

1. Глобальная sync Live↔Backtest charts
2. `getVisibleTimeRange()` 
3. `fitContent()` / `setVisibleRange()` при Safe Mode patch (`patchIndicatorsOnly`)
4. `candleSeries.setData()` при RSX settings change (только patch RSX + markers)
5. POST RSX settings со string-числами
6. TF click без `stopPropagation` → переход на Stats tab
7. `runBacktest(true)` / `switchTab: true` при auto-update RSX
8. Прямой `conn.WriteMessage` без `WSClient.mu` (concurrent write panic)
9. Lazy history fetch при `window.__isSettingsUpdating === true`
10. Восстановление жёлтой SL line series на графике (удалена намеренно)
11. Downsampling / truncation `chartData` в backtest API (`backtestChartStep`, `backtestMaxChartPoints`) — ломает navigator index↔candle alignment
12. Смешивание spot и futures klines в `historical_klines` для одного symbol
13. **`chartInitialized = true` в `handleWSMessage`** — WS не должен рисовать до HTTP `renderState`
14. **`scheduleHistoryPrefetch` / auto `fitContent` на live** — вызывают DDoS и viewport glitches
15. **Warmup trim price candles** на genesis chunk (`candles[trim:]` когда `len < limit+warmup`)

---

### 1. Binance USDⓈ-M Futures (единственный канал исполнения)

| Правило | Детали |
|---------|--------|
| **SDK** | Строго `github.com/adshao/go-binance/v2/futures` |
| **REST** | Только `fapi` (`https://fapi.binance.com`; testnet только при явном `isTestnet=true`) |
| **WebSocket** | **`wss://fstream.binance.com/market/stream?streams=`** (market combined). Legacy `/stream?streams=` — **не использовать** (нет данных). + `@aggTrade`, `@forceOrder`, klines |
| **Spot API** | **ЗАПРЕЩЁН** — никаких `/api/v3/...` в `exchange/` |
| **Ключи** | Mainnet Futures keys в `.env` (`BINANCE_API_KEY`, `BINANCE_SECRET_KEY`); **пустые = Read-Only** (аналитика без торговли и **без trade markers**). `EXCHANGE_*` **не читается** config |
| **Шорты** | `GetPositionAmt()` → `positionAmt < 0` = SHORT, `> 0` = LONG |
| **Плечо** | `execution.RiskManager` считает `Leverage` + cap `MaxLeverage`; **`ChangeLeverage(symbol, lev)`** вызывается в `huntForEntry` **перед** `CreateMarketOrder` |
| **Баланс** | **`GetFuturesBalance("USDT")`** → `AvailableBalance` с `/fapi/v3/balance`; mock `1000` **удалён** |

### 2. Cold Start Recovery (`MasterGeneral.RecoverState`)

- Вызывается в **`main.go` сразу после `NewMasterGeneral`**, до WS и Event Loop.
- `GetPositionAmt("BTCUSDT")` → если позиция есть: восстанавливает `IN_POSITION`, `positionQty`, `targetSide`.
- Стартовые точки для трейлинга: **`currentStopPrice = 0`** (LONG) или **`math.MaxFloat64`** (SHORT) — чтобы первый live-тик сразу поставил актуальный стоп.
- Отменяет stale ордера (`CancelAllOpenOrders`).

### 3. Условные стопы — только Algo Order API (ошибка -4120)

> **КРИТИЧЕСКОЕ ПРАВИЛО:** `STOP_MARKET` через `NewCreateOrderService()` **не работает** для закрытия позиции.

Стоп-лоссы — **исключительно** `NewCreateAlgoOrderService()` → `POST /fapi/v1/algoOrder`:

```
AlgoType(CONDITIONAL)
Type(AlgoOrderTypeStopMarket)
TriggerPrice(...)          // НЕ StopPrice()
WorkingType(MARK_PRICE)
ClosePosition(true)        // без Quantity
```

**Fallback:** если `ClosePosition` отклонён → `Quantity` + `ReduceOnly(true)`.

**Отмена:** `CancelAllOpenOrders` отменяет **и** обычные ордера, **и** algo (`NewCancelAllAlgoOpenOrdersService`).

### 4. Tick-by-Tick сопровождение позиции

| Режим | Канал | Когда | Что делает |
|-------|-------|-------|------------|
| **Закрытие свечи** | `Tick1mCh` | hunt TF + `IsClosed` | `IDLE` → `huntForEntry` (scoring + risk) |
| **Live-тик** | `TickLiveCh` | каждое WS-обновление `1m` | `IN_POSITION` → `managePosition()` / virtual SL-TP |

- Вход в сделку — **только на закрытии 1m** (`huntForEntry`).
- Сопровождение (`managePosition`) — **в реальном времени** на каждом тике незакрытой 1m-свечи (~4 раза/сек на Futures WS).

### 5. Throttling и фильтр шума (API protection)

| Механизм | Значение | Где |
|----------|----------|-----|
| **minStep** | `0.2%` от цены (`Close * 0.002`) | trailing: стоп двигается только если улучшение ≥ minStep |
| **orderCooldown** | `3 секунды` (`lastOrderTime`) | блок cancel+create стопа; защита от HTTP 429 |
| **positionSyncInterval** | `5 секунд` | периодический `GetPositionAmt` (обнаружение SL/TP на бирже) |

При ошибке создания стопа — `lastOrderTime = now` (penalty pause).

### 6. Гибридный трейлинг-стоп (Круиз / Форсаж)

Данные для трейлинга — **`1m` microReport**; алгоритмический выход — **`15m` macroReport.LatestAO**.

| Режим | Условие | Стоп |
|-------|---------|------|
| **⛴️ Круиз** | RSI в норме | `DownFractal - 0.5 ATR` (LONG) / `UpFractal + 0.5 ATR` (SHORT) |
| **🚀 Форсаж** | LONG: RSI ≥ 80; SHORT: RSI ≤ 20 | `Close ± 1.0 ATR` (агрессивный параболический трейлинг) |

`Report.UpFractal` / `Report.DownFractal` — последние отфильтрованные фракталы Б. Вильямса (`lastConfirmedFractals`).

### 7. MTF-распределение (Top-Down)

| TF | Роль |
|----|------|
| **`1m`** (default hunt) | Микро-вход (`huntForEntry` + **`EvaluateScalpSignal`**); live-трейлинг; full Layer 2 Report |
| **`3m`** | Analyst warm-up / dashboard TF |
| **`15m`** | Analyst warm-up / dashboard TF |
| **`1h`** | Прогрев истории / аналитик |
| **`4h`** | Прогрев истории / macro context |

**`validateHunting()` / HUNTING state — удалены.** Вход только через Layer 3 score ≥ dynamic threshold + `RiskManager`.

**Таймфрейм `1s` удалён** — Binance Futures не поддерживает 1s klines.

### 8. Баланс, плечо и вход в сделку (`huntForEntry`) — Layer 3 Scoring

Порядок вызовов при входе (на **close hunt TF**, default `1m`):

```
GenerateMarketReport() (hunt TF)
  → EvaluateScalpSignal(ctx, report, feeRate, memoryStore)   // Layer 3 scoring (dynamic thresholds)
  → entryRisk.ValidateEntry(report, side)                    // Fee/CLIMAX/Macro/AI vetoes
  → if Action == BUY/SELL:
      ReadOnly → openVirtualPosition (paper)
      else → GetFuturesBalance → sizing → ChangeLeverage → CreateMarketOrder → algo stop
      → IN_POSITION
```

- **`feeRate`**: default `0.0012` (Binance Futures taker approx.) в `MasterGeneral`.
- **`memoryStore`**: опциональный `*vector_db.MemoryStore`; в `main.go` пока `nil`.
- **`SetOnTrade` → `BroadcastMarker`:** маркер на dashboard **до** отправки ордера, но **только если не ReadOnly** и scoring=BUY/SELL.
- При `WAIT` / veto — остаётся в `HUNTING`, ордер **не** отправляется, **маркер не шлётся**.
- При ошибке balance / leverage / market order — переход в `IDLE`.

### 9. Поток исполнения (актуальный)

```
main.go
  → NewBinanceExchange (futures, fapi)
  → ChiefAnalyst[1m|15m|1h|4h] + REST warm-up (fapi/v1/klines)
  → NewMasterGeneral(analysts, rm, exchangeClient, memoryStore)  // memoryStore может быть nil
  → RecoverState()
  → WsClient (fstream) → StartDataFeed → Run()

TradeSignal → GetFuturesBalance → EvaluateSignal → ChangeLeverage → CreateMarketOrder (fapi)
           → CreateStopMarketOrder (fapi/v1/algoOrder)
```

## Роль

Твоя роль: **Senior Go Developer**, эксперт по **Quant/AI-трейдингу**, алготрейдингу, техническому анализу (в частности, **Волны Эллиотта**), стратегиям **«Торгового Хаоса»** и **Enterprise-архитектуре** торговых систем.

## Обзор архитектуры (Enterprise MTF)

Проект построен как **многослойная торговая система**: изолированная математика, потоковые данные с биржи, MTF-аналитика с AI-памятью, конечный автомат принятия решений и отдельный риск-фильтр перед исполнением.

```
Binance Futures WS (fstream)     REST fapi (прогрев + ордера)
      │                                    │
      ▼                                    ▼
 exchange/ws.go                   exchange/klines.go + binance.go
      │ WsTick → OutCh                       │
      ▼                                    ▼
 MasterGeneral.StartDataFeed      ChiefAnalyst[tf] (кэш Kline + streaming pipeline)
      │                              │
      ├─ UpdateKline (каждый тик) → evaluateTickLocked:
      │       FalconEngine → VolatilityEngine → SmartDivergence → ZigZag → Geometry → Fibonacci
      ├─ UpdateIndicators() — no-op (AO streaming в evaluateTickLocked)
      ├─ TickLiveCh  → managePosition (IN_POSITION, live)
      ├─ Tick1mCh    → macro / hunting / entry (на close)
      ▼                              ▼
 MasterGeneral.Run (FSM)       indicators/ + vector_db/
      │                              │
      ├─ GenerateMarketReport() ← Report (legacy + Layer 2 + Falcon)
      ├─ EvaluateScalpSignal() ← Layer 3 scoring (+ optional AI veto)
      ▼
 TradeSignal → execution.RiskManager → BinanceExchange (fapi + algo)
```

### Архитектурные концепции

#### 1. Слой индикаторов (`indicators/`) — ✅ PIPELINE BASELINE (Layer 1)

Полностью **изолированная потоковая математика** — **без go-talib**, **O(1) память** на тик.

**Базовые интерфейсы (`types.go`):**
- **`Indicator`** — `Update(val float64) float64` + `Value() float64` (RSI, MACD, SMA, EMA, RMA, AO, …)
- **`CandleIndicator`** — `UpdateCandle(high, low, close float64) float64` + `Value()` (ATR, Stochastic, AD, …)

**Композиция:** выход одного модуля → вход другого (`macd.Update(rsi.Update(price))`).

**Модули (все экспортированы, streaming-first):**

| Файл | Streaming-типы | Batch (*Values) |
|------|----------------|-----------------|
| `ma.go` | SMA, EMA, RMA | SMAValues, EMAValues |
| `utils.go` | RollingSum, RollingStDev | ExtractPrices |
| `oscillators.go` | RSI, MACD, Stochastic | RSIValues, MACDValues, StochValues |
| `volatility.go` | BollingerBands, ATR | BollingerBandsValues, ATRValues |
| `chaos.go` | AO, WilliamsFractals | AOValues, AOValuesFromKlines, WilliamsFractalPeaks |
| `volume.go` | AD, CumSum, VolumeWeightedEMA | — |
| `jurik.go` | JurikRSX | JurikRSXValues |
| `zigzag.go` | ZigZag (ATR + fractals + RSI sensitivity) | — |
| `fibonacci.go` | `FibonacciEngine`, `FibZone`, `VectorizeReport` inputs | `CalculatePriceZones`, `CalculateTimeZones`, `FindConfluence` |
| `peaks.go` | — | FindExtremes, FilterPeaksByATR |
| `geometry.go` | Trendline (наклонные, пробои, треугольники) | — |
| `divergence.go` | SmartDivergenceEngine (Snapshots) | AnalyzeMacro / AnalyzeMicro |
| `divergence_classic.go` | — | CheckClassicDivergence, CheckTripleDivergence (A/B/C) |

**Расширенные API (не Indicator, по замыслу):**
- `WilliamsFractals.UpdateCandle(h, l) → FractalStatus` — 5-bar ring buffer
- `ZigZag.UpdateCandle(h, l, c, rsi) → ZigZagUpdate`; `Value()` = цена последнего узла
- `VolumeWeightedEMA.Update(price, volume)` — dual-input
- `MACD.Signal()`, `MACD.Histogram()`; `Stochastic.K()`, `Stochastic.D()`; `BollingerBands.Bands()`

**Стандарт нового индикатора:** O(1) ring buffer или prev-value → `NewXxx()` → `var _ Indicator = (*Xxx)(nil)` → опционально `XxxValues()` только для истории/тестов.

**Запрет:** никакой торговой логики, входов/выходов и вызовов биржи (Правило 5). **go-talib удалён** из проекта.

**Документация baseline:** `README.md` + `indicators/doc.go`.

#### 2. MTF-аналитика (`strategy/analyst.go`, `layer2.go`, `geometry_tracker.go`) + Falcon

**`ChiefAnalyst`** — один экземпляр на таймфрейм, **потокобезопасен** (`sync.RWMutex`).

- Кэш свечей: `UpdateKline`, `GetKlines` (defensive copy).
- **`evaluateTickLocked`** (на каждый тик, `layer2.go`): единый streaming pipeline:
  - `FalconEngine.Evaluate` → Jurik, Red, **Green**, Black, Blue
  - `VolatilityEngine.Evaluate(..., JurikRSX)` → regime, ATR, SafeStopDist, LotModifier
  - `SmartDivergenceEngine` — micro ticks + snapshots на ZigZag confirm
  - `ZigZag.UpdateCandle` → swing nodes
  - `geometryTracker` — trendlines, touches, **IsBullishBreakout** (resistance breakout)
  - `FibonacciEngine.CalculatePriceZones` — на завершении волны (2-й+ ZigZag node)
  - streaming **AO** (`indicators.AO`) — mid price (hl2)/2
  - `RedLineCrossGreenUp` — `prevRed <= prevGreen && curRed > curGreen && curRed < 40`
  - `JurikValue`, `JurikIsRising` — macro Jurik filter для scoring
- **Replay:** при перезаписи формирующейся свечи — `replayStreamingLocked()` (полный reset всех streaming engines).
- **`GenerateMarketReport()`** — legacy batch (RSI, MACD, ATR, fractals) + **полный Layer 2 Report**.
- **Интеграция с `vector_db` (Qdrant):** фракталы, `BackfillHistory`, `CheckAndSaveFractal`, `PredictNextMovement`; 🔜 trade outcomes через `SaveTradeOutcome`.
- **`UpdateIndicators()`** — пустой no-op (совместимость с `StartDataFeed`); AO только streaming.
- **Не торгует**, **не импортирует** `exchange` для ордеров — только данные и отчёты.

#### 2b. Стратегия Falcon — Layer 3 (`strategy/falcon.go`) — ✅ ИНТЕГРИРОВАН

**`FalconEngine`** — приборная панель Wozdux из **5 сигналов** на каждый тик:

| Поле | Pipeline | Параметры | Смысл |
|------|----------|-----------|-------|
| `JurikRSX` | JurikRSX(hlc3) | length=14 | Главный трендовый фильтр (0–100) |
| `RedLine` | RSI(hl2) | length=14 | Давление «здесь и сейчас» |
| **`GreenLine`** | **EMA(RSI(close))** | RSI=14, EMA=12 | Динамическая Wozdux Green (support/resistance) |
| `BlackLine` | MACD(RSI(close)) + 50 | RSI=24, MACD 7/24/9 | Импульс |
| `BlueLine` | EMA(RSI(VWEMA(close, vol))) | VWEMA=24, RSI=24, EMA=12 | Поток объёма |

**Cross trigger:** `Report.RedLineCrossGreenUp` — Red пробивает Green снизу вверх в зоне `< 40`.

**Интеграция:** `ChiefAnalyst` → `Report.Falcon` + `Report.RedLineCrossGreenUp` → **`EvaluateScalpSignal`** (+35 баллов).

#### 2c. Volatility Engine — Layer 2 (`strategy/volatility.go`) — ✅ ДВИЖОК ГОТОВ

**`VolatilityEngine`** — режим рынка + динамический стоп + множитель лота на каждом тике.

| Поле | Источник |
|------|----------|
| `ATR` | streaming `indicators.ATR(14)` |
| `baselineATR` | `SMA(20)` от ATR |
| `baselineVol` | `SMA(20)` от volume |

**`Evaluate(high, low, close, volume, primaryOscillator)`** → `VolatilityState`:

| Regime | Условие | LotModifier | SafeStopDist |
|--------|---------|-------------|--------------|
| **CLIMAX** | ATR > baseline×2 **и** Vol > avgVol×2 **и** osc > 80 или < 20 | **0.3** | ATR × 2.0 |
| **SQUEEZE** | ATR < baseline×0.8 | **1.2** | ATR × 1.5 |
| **EXPANSION** | остальное (default) | **1.0** | ATR × 1.5 |

Warmup-safe: пока baseline = 0 → EXPANSION без паник. `primaryOscillator` = Jurik RSX или RSI.

**🔜 Интеграция:** ✅ `ChiefAnalyst` → `Report.Volatility`; ✅ `huntForEntry` → `LotModifier`, `SafeStopDist` через `EvaluateScalpSignal`.

#### 3. Top-Down оркестрация (`strategy/master.go`) — ✅ ЯДРО РЕАЛИЗОВАНО

**`MasterGeneral`** — **конечный автомат (State Machine)** с гибридной синхронизацией.

| Состояние | Смысл |
|-----------|--------|
| **`IDLE`** | Ожидание; hunt на close hunt TF |
| **`IN_POSITION`** | Позиция открыта (live или virtual); live `managePosition` + трейлинг + signal exit |

**Гибридная синхронизация:**

- Мастер **спит** в `Run()` и просыпается по **`Tick1mCh`** (закрытие 1m) и **`TickLiveCh`** (каждый live-тик 1m).
- **Закрытие hunt TF:** `huntForEntry` (`IDLE`).
- **Live 1m:** `managePosition` / `manageVirtualPosition` (`IN_POSITION`).

**`StartDataFeed(ctx, wsOutCh)`** — Consumer: `UpdateKline` на каждом тике; `UpdateIndicators()` no-op на закрытии; для `1m` — non-blocking ping в `TickLiveCh` (всегда) и `Tick1mCh` (только close).

#### 3b. Layer 3 Scoring Brain (`strategy/scoring.go`, `strategy/thresholds.go`, `strategy/scoring_matrix.go`) — ✅ РЕАЛИЗОВАН

**`EvaluateScalpSignal(ctx, report, feeRate, memory)`** — чистая бизнес-логика; vetoes — в **`RiskManager`**.

**Dynamic thresholds (`strategy/thresholds.go`):** long/short entry thresholds, UI via `POST /api/settings/thresholds`.

**Signal toggles (`strategy/scoring_matrix.go`):** per-rule enable/disable, UI via `POST /api/settings/matrix`; default all enabled.

**Выход:** `ScalpDecision{ Action, Score, LongScore, ShortScore, LotMod, StopDist, Reason }`.

**`huntForEntry`:** вызывает scoring; при `BuyAction` — sizing через `calculateSafePositionSize` × `LotMod`, SL = `Close - StopDist`.

#### 4. Потоковые данные (`exchange/ws.go`) — ✅ PRODUCER

Независимый **WebSocket-клиент** (без импорта `strategy` — защита от циклических зависимостей).

- **`WsClient`** — combined stream Futures: **`1m`, `15m`, `1h`, `4h`** (`fstream`, testnet: `stream.binancefuture.com`).
- Парсинг JSON → **`WsTick`** `{ Timeframe, Kline, IsClosed }`.
- Публичный буферизованный канал **`OutCh`** (1000) — единственная точка выхода Producer.
- **`MasterGeneral`** — Consumer через `StartDataFeed`.

## Главный Аналитик (Chief Analyst)

Центральная концепция архитектуры — **Главный Аналитик**: модульный «мозг», который **не торгует сам**, а **собирает, нормализует и интерпретирует** данные из специализированных подсистем, после чего передаёт сводку в слой принятия решений.

Сессия реализовала **MTF-архитектуру**: по одному `ChiefAnalyst` на TF. Live-свечи приходят из **`exchange/ws.go`** через роутер Мастера, история для прогрева — из **`exchange/klines.go`** (REST).

Математика — в `indicators/`; устаревший `strategy/chaos.go` **удалён**.

### Текущие способности ChiefAnalyst (`strategy/analyst.go`)

| Область | Реализация |
|---------|------------|
| **Потокобезопасность** | `sync.RWMutex` — безопасное кэширование и обновление массива свечей (`UpdateKline`, `GetKlines`, defensive copy) |
| **Векторная память (AI)** | Полная интеграция с Qdrant (`vector_db/`): векторизация окон свечей, `BackfillHistory`, сохранение подтверждённых фракталов (`CheckAndSaveFractal`), поиск исторических совпадений (`PredictNextMovement`, top-3) |
| **Фильтрация шума** | Фракталы Билла Вильямса (окно=2) + `filteredFractalPeaks`: отсечение ложных пробоев через динамический ATR (`FilterPeaksByATR`) |
| **Интеграция Falcon** | `FalconEngine` per TF: warmup → `UpdateKline` → `Report.Falcon` (+ GreenLine, cross) |
| **Layer 2 streaming** | Volatility, SmartDivergence, ZigZag, Geometry, Fibonacci — ✅ wired в `evaluateTickLocked` |
| **Streaming AO** | `indicators.AO` в pipeline; `Report.LatestAO` из streaming-кэша (не batch) |
| **Интеграция Хаоса** | **15m macro FSM** на `Report.LatestAO` (streaming на 1m/15m через pipeline) |
| **MTF-отчёт (`Report`)** | ✅ **`GenerateMarketReport()`** — legacy + **Layer 2 + Falcon + Jurik macro** |

Аналитик **не торгует** и **не вызывает Exchange API** — только собирает контекст рынка для своего таймфрейма.

### Структура `Report` и `GenerateMarketReport()`

Метод **`GenerateMarketReport()`** собирает текущие индикаторы в единый пакет данных для **`MasterGeneral`**. Требует **минимум 50 свечей** в кэше.

| Поле | Источник / смысл | Статус |
|------|------------------|--------|
| `Timeframe` | TF данного `ChiefAnalyst` | ✅ |
| `Close` | последняя цена закрытия | ✅ |
| `RSI` | `RSIValues(close, 14)` — legacy | 🔄 трейлинг FOMO |
| `MACD` | `MACDValues(close, 12, 26, 9)` — legacy | 🔄 |
| `ATR` | `ATRValues` batch — трейлинг-стопы | ✅ |
| `IsOverbought` / `IsOversold` | RSI >= 70 / <= 30 — legacy | 🔄 |
| `LatestAO` | **streaming AO** из `evaluateTickLocked` | ✅ |
| `UpFractal` / `DownFractal` | `lastConfirmedFractals` — трейлинг Круиз | ✅ |
| **`Falcon`** | Jurik, Red, **Green**, Black, Blue | ✅ |
| **`Volatility`** | `VolatilityEngine` — Regime, ATR, SafeStopDist, LotModifier | ✅ |
| **`Divergence`** | `SmartDivergenceEngine` — macro + micro combined | ✅ |
| **`ZigZag`** | Direction + LastNode | ✅ |
| **`Geometry`** | Touches, breakouts, triangle, **`IsBullishBreakout`** | ✅ |
| **`FibZones`** | Kill-zones от `FibonacciEngine` (на завершении волны) | ✅ |
| **`RedLineCrossGreenUp`** | Red × Green cross в зоне < 40 | ✅ |
| **`JurikValue`** | `Falcon.JurikRSX` (macro filter) | ✅ |
| **`JurikIsRising`** | `curJurik > prevJurik` | ✅ |

**Falcon-подполя:** `JurikRSX`, `RedLine`, **`GreenLine`**, `BlackLine`, `BlueLine` — см. `strategy/falcon.go`.

**Vector DB projection:** `Report.VectorSnapshot()` → `vector_db.ReportSnapshot` → `VectorizeReport()` (6 dims, без import cycle).

Цены извлекаются через **`indicators.ExtractPrices(klines)`** (legacy batch). Falcon — **streaming** в `UpdateKline`.

| Компонент | Файл / пакет | Ответственность |
|-----------|----------------|-----------------|
| **Аналитик** | `strategy/analyst.go`, `layer2.go`, `geometry_tracker.go` | Кэш свечей, **full streaming pipeline**, Report |
| **Falcon** | `strategy/falcon.go` | Jurik + Wozdux (5 линий) → `FalconSignals` |
| **Volatility** | `strategy/volatility.go` | Regime + SafeStopDist + LotModifier → `Report.Volatility` |
| **Scoring** | `strategy/scoring.go`, `strategy/thresholds.go`, `strategy/scoring_matrix.go` | **`EvaluateScalpSignal`**, dynamic thresholds, rule toggles |
| **Backtest** | `strategy/backtest.go` | **`BacktestEngine.Run`**, virtual PnL + stats |
| **History cache** | `data/history_db.go` | SQLite kline cache (`modernc.org/sqlite`) |
| **Entry Risk** | `strategy/risk.go` | **`RiskManager.ValidateEntry`** — entry vetoes |
| **Report snapshot** | `strategy/report_snapshot.go` | `Report.VectorSnapshot()` для Qdrant |
| **Geometry tracker** | `strategy/geometry_tracker.go` | Trendlines от ZigZag → `GeometryState` |
| **Мастер** | `strategy/master.go` | FSM, **scoring entry**, trailing, algo exit |
| **Dashboard** | `server/webserver.go`, `web/` | API + terminal UI + backtest/stats tabs |
| **Sizing риск** | `execution/risk.go` | `RiskManager.EvaluateSignal` — sizing по SL distance + MaxLeverage cap |
| **Профили риска** | `strategy/risk.go` | `ValidateSignal` по профилям (legacy multi-strategy) |
| **Индикаторы** | `indicators/` | ✅ **Pipeline Baseline (Layer 1):** потоковая математика O(1), composable Indicator/CandleIndicator; batch `*Values` для warm-up; **без go-talib** |
| **Пики** | `indicators/peaks.go` | Универсальный поиск локальных экстремумов (HIGH/LOW) оконным методом с последующей очисткой от рыночного шума через индикатор ATR |
| **Дивергенции** | `indicators/divergence.go` | Smart Divergence Engine (Snapshots, Macro/Micro) |
| **Векторная память** | `vector_db/` | Qdrant: фракталы + **trade outcomes** + **AI win-rate lookup** |
| **WebSocket** | `exchange/ws.go` | Producer: Futures kline streams → `WsTick` в `OutCh` |
| **Биржа** | `exchange/` | REST fapi (ордера, klines, position), WS fstream, Normalizer, Algo stops |
| **Конфиг** | `config/` | Секреты и настройки **только из `.env`** (godotenv) |

Поток данных (актуальный):

```
exchange/ws.go (Producer: WsClient.OutCh, fstream)
    → MasterGeneral.StartDataFeed
    → ChiefAnalyst[tf].UpdateKline (каждый тик)
        → evaluateTickLocked: Falcon + Volatility + Divergence + ZigZag + Geometry + Fib + AO
    → UpdateIndicators() [no-op]
    → TickLiveCh (live 1m) / Tick1mCh (close 1m) → MasterGeneral.Run
    → GenerateMarketReport() → Report
    → EvaluateScalpSignal(ctx, report, feeRate, memoryStore)   // dynamic long/short thresholds
    → entryRisk.ValidateEntry(report, side)                  // Fee/CLIMAX/Macro/AI vetoes
    → huntForEntry: paper virtual OR market order + algo stop
    → execution.RiskManager.EvaluateSignal × LotMod → BinanceExchange (fapi + algoOrder)
```

REST-прогрев: `exchange/klines.go` → `GET /fapi/v1/klines` → история в `ChiefAnalyst`.

## Слой Индикаторов (Indicators Layer) — Pipeline Baseline v1

Статус: **✅ BASELINE LOCKED** — Layer 1 готов к подключению Layer 2 (контекст/Elliott) и Layer 3 (Falcon).

### Архитектура пайплайна

- **Потоковая обработка:** каждый тик вызывает `Update` / `UpdateCandle`; состояние хранится в структуре индикатора.
- **O(1) память:** ring buffer (SMA, RollingStDev, Stochastic, WilliamsFractals) или prev-value (EMA, RMA).
- **Композиция:** индикаторы вкладываются друг в друга (MACD от RSI, Bollinger от любого потока).
- **Batch `*Values`:** только для прогрева истории и тестов; Layer 2 переведёт `ChiefAnalyst` на live streaming instances.
- **go-talib:** **удалён** из `go.mod` и всего пакета `indicators/`.

### Файлы

| Файл | Содержимое |
|------|------------|
| **`types.go`** | `Indicator`, `CandleIndicator` |
| **`doc.go`** | Package docs — Pipeline Baseline standard |
| **`ma.go`** | `SMA`, `EMA`, `RMA` + `SMAValues`, `EMAValues` |
| **`utils.go`** | `ExtractPrices`, `RollingSum`, `RollingStDev` |
| **`oscillators.go`** | `RSI`, `MACD`, `Stochastic` + `*Values` |
| **`volatility.go`** | `BollingerBands`, `ATR` + `*Values` |
| **`chaos.go`** | `AO`, `WilliamsFractals`, `AOValues`, `OHLCFromKlines` |
| **`volume.go`** | `AD`, `CumSum`, `VolumeWeightedEMA` |
| **`zigzag.go`** | `ZigZag` — adaptive ATR + fractals + RSI sensitivity |
| **`fibonacci.go`** | `FibonacciEngine`, `FibZone`, kill-zones (ATR padding), time zones, confluence |
| **`jurik.go`** | `JurikRSX` + `JurikRSXValues` |
| **`geometry.go`** | `Trendline`, `DetectTriangle`, volume-confirmed breakout |
| **`peaks.go`** | `FindExtremes`, `FilterPeaksByATR` |
| **`divergence.go`** | `SmartDivergenceEngine`, `Snapshot`, `DivSignal` |
| **`divergence_classic.go`** | `CheckClassicDivergence`, `CheckTripleDivergence` (классы A/B/C) |

### План перехода Layer 1 → 2 → 3

| Слой | Статус | Содержание |
|------|--------|------------|
| **Layer 1** `indicators/` | ✅ **LOCKED** | Pipeline Baseline: streaming O(1), composable, no go-talib |
| **Layer 2** аналитика | ✅ **WIRED** | Geometry + Smart Divergence + Volatility + ZigZag + Fib → `ChiefAnalyst`/`Report` |
| **Layer 3** Falcon + Scoring | ✅ **РЕАЛИЗОВАНО** | FalconEngine + **`EvaluateScalpSignal`** + `huntForEntry` wiring |

### Архитектурное правило

> В `indicators/` — **только математика и обёртки**. Никакой торговой логики, входов/выходов и вызовов биржи (см. Правило 5).

---

## Карта блоков `indicators/` — назначение и потребители

> **Как читать:** «Блок» = файл или логическая группа. «Модуль-потребитель» = кто вызывает в runtime.

### A. Фундамент пайплайна (строительные блоки)

| Блок | Файл | Для чего | Кто использует |
|------|------|----------|----------------|
| **Интерфейсы** | `types.go`, `doc.go` | Контракт `Indicator` / `CandleIndicator`; стандарт Layer 1 | Все streaming-индикаторы; новые модули |
| **Скользящие средние** | `ma.go` — SMA, EMA, RMA | Базовое сглаживание O(1); RMA = Wilder (RSI, ATR) | `oscillators`, `volatility`, `chaos`, `falcon`, `zigzag` |
| **Статистика окна** | `utils.go` — RollingSum, RollingStDev | Сумма / σ за N баров без O(n) памяти | `BollingerBands`; будущий volume-RSI |
| **Конвертер данных** | `utils.go` — `ExtractPrices` | `[]Kline` → OHLCV slices для batch | `ChiefAnalyst.GenerateMarketReport` (legacy) |

### B. Осцилляторы и качество тренда

| Блок | Файл | Для чего | Кто использует |
|------|------|----------|----------------|
| **RSI / MACD / Stoch** | `oscillators.go` | Классические осцилляторы; composable pipeline | `FalconEngine` (Red/Black/Blue); legacy `Report` (batch) |
| **Jurik RSX** | `jurik.go` | «Noise-free RSI» 0–100; главный фильтр тренда | **`FalconEngine`**, **Smart Divergence Snapshot** |

### C. Волатильность и объём

| Блок | Файл | Для чего | Кто использует |
|------|------|----------|----------------|
| **ATR** | `volatility.go` | True Range + RMA; динамические стопы | `Report.ATR`; `managePosition` trailing; `FilterPeaksByATR`; `ZigZag` |
| **Bollinger Bands** | `volatility.go` | SMA ± k·σ | 🔜 Layer 2 (пока не подключён) |
| **AD / CumSum** | `volume.go` | Накопление CLV / running sum | ✅ BurgundyAD в Divergence snapshots |
| **VolumeWeightedEMA** | `volume.go` | VWAP-подобная цена с EMA | **`FalconEngine` BlueLine** |

### D. Торговый хаос и структура рынка

| Блок | Файл | Для чего | Кто использует |
|------|------|----------|----------------|
| **Awesome Oscillator** | `chaos.go` | SMA(hl2,5) − SMA(hl2,34); макро-тренд | ✅ **streaming AO** в pipeline; **15m FSM** |
| **Williams Fractals** | `chaos.go` | 5-bar ring; up/down fractal | `filteredFractalPeaks` → `Report` fractals; `ZigZag`; Qdrant |
| **Adaptive ZigZag** | `zigzag.go` | Свинги с ATR + RSI sensitivity | ✅ Smart Divergence + Geometry + Fib wave anchors |
| **Fibonacci** | `fibonacci.go` | Retracement/Extension kill-zones, time zones, confluence | ✅ `ChiefAnalyst` → `Report.FibZones` |

### E. Геометрия и паттерны (Layer 2)

| Блок | Файл | Для чего | Кто использует |
|------|------|----------|----------------|
| **Trendline / Triangle** | `geometry.go` | Наклонные линии от `Peak`; касания; volume breakout; `DetectTriangle` | ✅ `geometry_tracker.go` → `Report.Geometry` |
| **Пики + ATR-фильтр** | `peaks.go` | Локальные экстремумы; отсечение шума | fractals, `geometry.NewTrendline`, Qdrant |
| **Smart Divergence** | `divergence.go` | 8 индикаторов на ZigZag-snapshot; macro + micro | ✅ `layer2.go` → `Report.Divergence` |
| **Classic Divergence** | `divergence_classic.go` | A/B/C batch (2–3 пика) | тесты; fallback batch |

### F. Сборка сигналов (strategy)

| Блок | Файл | Для чего | Кто использует |
|------|------|----------|----------------|
| **FalconEngine** | `strategy/falcon.go` | Jurik + Wozdux dashboard (**5 линий**) | ✅ `ChiefAnalyst` → `Report.Falcon` |
| **VolatilityEngine** | `strategy/volatility.go` | Regime SQUEEZE/EXPANSION/CLIMAX | ✅ `Report.Volatility` + scoring |
| **ChiefAnalyst** | `analyst.go` + `layer2.go` | MTF кэш, full streaming pipeline, Report | `MasterGeneral` все TF |
| **Scoring Brain** | `strategy/scoring.go` | `EvaluateScalpSignal`, vetoes, scoring matrix | ✅ `huntForEntry` |
| **MasterGeneral** | `strategy/master.go` | FSM, **scoring entry**, trailing, ордера, AI memory | `main.go` event loop |

### Поток Layer 2 + Layer 3 (актуальный)

```
Kline tick → ChiefAnalyst.UpdateKline → evaluateTickLocked
           → FalconEngine (Jurik, Red, Green, Black, Blue)
           → VolatilityEngine → SmartDivergence (micro + ZigZag snapshots)
           → ZigZag → geometryTracker → FibonacciEngine
           → GenerateMarketReport() → Report
           → EvaluateScalpSignal(ctx, report, feeRate, memoryStore)
           → huntForEntry (BUY) → RiskManager × LotMod → exchange
```

### Legacy vs Falcon (переходный период)

| Задача | Сейчас | Статус |
|--------|--------|--------|
| Микро-вход hunt TF | **`EvaluateScalpSignal`** (BUY + SELL, dynamic thresholds) | ✅ |
| Макро gate 15m AO | **удалён** (HUNTING / validateHunting) | — |
| Jurik macro veto | `Report.JurikValue` + `JurikIsRising` | ✅ |
| Стоп / лот | `Volatility.SafeStopDist` + `LotModifier` | ✅ |
| Трейлинг | Fractals + ATR + RSI (legacy batch) | ✅ (пока) |
| SHORT entry | `SellAction` в scoring + `ValidateEntry("SELL")` | ✅ |

---

## Layer 2 — Аналитика — ✅ WIRED

### 0. GeometryState (`strategy/geometry_tracker.go`)

| Поле | Смысл |
|------|-------|
| `ResistanceTouches` / `SupportTouches` | Касания наклонных |
| `ResistanceBreakout` / `SupportBreakout` | Volume-confirmed пробой |
| **`IsBullishBreakout`** | Пробой сопротивления вверх → **+30** в scoring |
| `BreakoutStrength` | 1 + Touches |
| `TriangleKind` | symmetrical / ascending / descending |

### 1. Geometry Module (`indicators/geometry.go`)

**Назначение:** наклонные линии поддержки/сопротивления от узлов ZigZag, подсчёт касаний, пробой с подтверждением объёмом, распознавание треугольников.

**Ключевые типы:**
- **`Trendline`** — две точки `Peak{Index, Value}`; `Equation()` → y = mx + b; `ValueAt(index)`
- **`UpdateTouches(barIndex, high, low, atr)`** — касание если цена в пределах **0.5×ATR** от линии без пробоя
- **`CheckBreakout(index, close, open, volume, avgVolume, isResistance) (bool, int)`** — пробой + vol > avg×1.5 + направление свечи; сила = **1 + Touches**
- **`DetectTriangle(resistance, support) string`** → `"symmetrical"` | `"ascending"` | `"descending"` | `""`

**Потребитель:** ✅ `geometry_tracker.go` на каждом тике + ZigZag swing nodes → `Report.Geometry`.

---

### 2. Smart Divergence Engine (`indicators/divergence.go`)

**Назначение:** confluence-дивергенции цены vs **8 индикаторов** на снапшотах ZigZag-пиков + микро-физика (velocity/acceleration) внутри фрактала.

**`Snapshot`** (на каждый ZigZag-узел):

| Поле | Источник (маппинг Falcon/Wozdux) |
|------|----------------------------------|
| `Jurik` | `Falcon.JurikRSX` |
| `OrangeRSI` | RSI(close, 14) — отдельный поток в `layer2.go` |
| `RedRSI` | `Falcon.RedLine` |
| `BlueVolume` | `Falcon.BlueLine` |
| `BurgundyAD` | `indicators.AD` |
| `AO` | `Report.LatestAO` / streaming AO |
| `MACD` | `Falcon.BlackLine` |
| `Stoch` | streaming Stochastic |

**`SmartDivergenceEngine`:**
- Ring buffer **5 Snapshot** + **5 micro-тиков** Orange/Red
- `UpdateSnapshot(snap)` — новый пик ZigZag
- `UpdateMicroTick(orange, red)` — каждый тик
- **`AnalyzeMacro() DivSignal`** — score **−100…+100**, Description

**Macro веса (медвежья дивергенция на High, бычья — зеркально на Low):**

| Индикатор | Баллы |
|-----------|-------|
| Jurik | 15 |
| OrangeRSI | 10 |
| BlueVolume (fakeout) | 25 |
| BurgundyAD | 20 |
| AO / MACD | 20 каждый |

**Каскады:** подтверждение [0] vs [2] → ×**1.5**; тройная волна 5 → ×**2.0**.  
**Скрытая дивергенция:** Higher Low + падающий индикатор → **+30** (продолжение тренда).

**MicroScanner (`AnalyzeMicro` / `AnalyzeMicroCombined`):**

| Паттерн | Условие | Баллы |
|---------|---------|-------|
| **Saucer** | RSI < 30; V отрицательная, растёт к 0; A > 0 | +15 |
| **V-Spike** | резкая смена V −→+; экстремальное A | +20 |

**Legacy:** классические A/B/C — `divergence_classic.go` (`CheckClassicDivergence`, `CheckTripleDivergence`).

**Потребитель:** ✅ `layer2.go` — snapshots на ZigZag confirm, micro ticks каждый тик → `Report.Divergence`.

---

### 3. Volatility Engine (`strategy/volatility.go`)

См. **§2c** выше. Потоковый `Evaluate` на каждом тике; не путать с `indicators/volatility.go` (Bollinger/ATR Layer 1).

---

### 4. Fibonacci Engine (`indicators/fibonacci.go`) — ✅

**`FibonacciEngine`:** `CalculatePriceZones` (retracement 0.382–0.786, extension 1.272–2.618, ATR kill-zones), `CalculateTimeZones`, `FindConfluence`.

**Wiring:** на завершении ZigZag-волны (2-й+ узел) → `Report.FibZones`; активный 0.618 → **+20** в scoring.

---

### Layer 2 — актуальный поток (✅)

```
Kline tick → evaluateTickLocked (layer2.go)
           → FalconEngine → VolatilityEngine → SmartDivergence.UpdateMicroTick
ZigZag confirm → Snapshot → UpdateSnapshot → geometry.onSwingNode → fib wave
           → Report { Falcon, Volatility, Divergence, ZigZag, Geometry, FibZones, Jurik*, RedLineCross* }
           → EvaluateScalpSignal → huntForEntry
```

---

## Vector DB — AI Memory (`vector_db/`) — ✅ РАСШИРЕН

### Anti-cycle pattern

`vector_db` **не импортирует** `strategy`. Проекция: `Report.VectorSnapshot()` → `vector_db.ReportSnapshot` → `VectorizeReport()`.

### `db.go` — методы

| Метод | Payload | Назначение |
|-------|---------|------------|
| `SavePattern` | price, is_up_fractal | Фракталы (legacy) |
| **`SaveTradeOutcome`** | **action, pnl, is_win** | Исходы сделок для AI-памяти |
| `SearchSimilarPatterns` | — | k-NN по embedding |
| `InitCollection` | — | Cosine distance collection |

### `pattern.go` — embeddings

| Функция | Размер | Содержимое |
|---------|--------|------------|
| `VectorizeCandles` | len(klines) | Нормализованные closes (фракталы) |
| **`VectorizeReport`** | **6** | Jurik/100, DivScore/100, Regime (−1/0/1), Red/100, Blue/100, FibActive |

### `memory.go` — `MemoryStore`

```go
NewMemoryStore(db, collectionName)
PredictWinRate(ctx, ReportSnapshot, k) → (winRate, count, err)
```

**AI Veto:** в **`RiskManager.ValidateEntry`** (не в scoring): если `count >= 3` и `winRate < 0.40` → block entry.

**`main.go`:** `memoryStore` пока `nil` в `NewMasterGeneral`; 🔜 подключить Qdrant + коллекцию trade outcomes.

---

## Слой Исполнения (Execution Layer)

Модуль `exchange/` работает **исключительно через Binance USDⓈ-M Futures API**.

### 1. Конфигурация

- **`config/config.go`** — `.env` через `godotenv`: `BINANCE_TEST_API_KEY`, `BINANCE_TEST_SECRET_KEY`.
- **`exchange/binance.go`** — SDK `github.com/adshao/go-binance/v2/futures`; `futures.UseTestnet = true` перед `NewClient`.
- Подпись HMAC SHA256 — внутри SDK для всех signed endpoints.

### 2. Методы `BinanceExchange` (fapi)

| Метод | Endpoint | Назначение |
|-------|----------|------------|
| `Ping` | `/fapi/v1/ping` | Проверка связи |
| `CreateMarketOrder` | `/fapi/v1/order` MARKET | Вход / алгоритмический выход |
| `CreateLimitOrder` | `/fapi/v1/order` LIMIT | Лимитные ордера |
| `CreateStopMarketOrder` | **`/fapi/v1/algoOrder`** STOP_MARKET | Стоп-лосс / трейлинг (Algo API) |
| `GetPositionAmt` | `/fapi/v2/positionRisk` | Sync позиции (+/- для short) |
| `GetFuturesBalance` | `/fapi/v3/balance` | `AvailableBalance` USDT (или другой asset) |
| `ChangeLeverage` | `/fapi/v1/leverage` | Установка плеча перед входом |
| `CancelAllOpenOrders` | cancel all + **cancel all algo** | Перед перестановкой стопа |
| `GetKlines` | `/fapi/v1/klines` | REST-прогрев аналитиков |

### 3. Normalizer (`exchange/normalizer.go`)

- `LoadLimitsFromFutures()` → `GET /fapi/v1/exchangeInfo` через SDK.
- `FormatPrice` / `FormatQuantity` — TickSize / StepSize, округление qty **строго вниз**.

### 4. Статус Execution Layer

| Статус | Описание |
|--------|----------|
| ✅ | Futures REST + WS, Normalizer, Market/Limit, **Algo STOP_MARKET**, position sync, cancel algo, **ChangeLeverage**, **GetFuturesBalance** |
| 🔜 | `ChangeMarginType` (isolated/cross) на бирже |

## Слой Риск-менеджмента

### `execution/risk.go` — sizing (используется Мастером)

```
GetFuturesBalance("USDT") → EvaluateSignal → OrderRequest → ChangeLeverage → CreateMarketOrder
```

- **`EvaluateSignal`**: расчёт qty по дистанции до SL и `%` риска на сделку.
- **`MaxLeverage`**: cap плеча; при превышении — уменьшение position size.
- **`OrderRequest.Leverage`**: передаётся в **`ChangeLeverage`** перед market entry.

### `strategy/risk.go` — профильная валидация (legacy / multi-strategy)

Система поддерживает **мультистратегийность**: каждая стратегия живёт по своим правилам риска.

| Сущность | Назначение |
|----------|------------|
| **`TradeSignal`** | Входящий сигнал: `StrategyID`, `Symbol`, `Side`, `EntryPrice`, `StopLoss`, `TakeProfit`, `RiskPercentage` |
| **`RiskProfile`** | Правила для одной стратегии: `MaxRiskPerTrade`, `RequireStopLoss` |
| **`RiskManager.profiles`** | `map[string]RiskProfile` — профиль на каждый `StrategyID` |

Примеры профилей:

- **Скальпинг** (`aggressive`): `RequireStopLoss=false`, высокий `MaxRiskPerTrade` — допускает сигналы без жёсткого стопа.
- **Среднесрок** (`conservative`): `RequireStopLoss=true`, низкий `MaxRiskPerTrade` — стоп обязателен, риск на сделку ограничен.

Регистрация: **`AddProfile(strategyID, profile)`** — добавление или обновление профиля.

### 3. Потокобезопасность (Concurrency-Safe)

Ядро бота спроектировано для **асинхронной работы в Go** (множество горутин-стратегий и MTF-аналитиков).

- `RiskManager` использует **`sync.RWMutex`**:
  - **Write lock** — `AddProfile` (изменение мапы профилей).
  - **Read lock** — `ValidateSignal` (чтение профиля при шквале одновременных сигналов).
- Безопасная обработка параллельных `ValidateSignal` от независимых горутин без data race.

### 4. Три столпа валидации (`ValidateSignal`)

Каждый `TradeSignal` проходит **три жёсткие проверки**. При провале — `error` с префиксом `rejected:` (ордер **не** уходит на биржу).

| # | Проверка | Условие отклонения |
|---|----------|-------------------|
| **1. Stop-Loss** | Профиль требует стоп (`RequireStopLoss=true`) | `StopLoss <= 0` → `rejected: stop-loss is mandatory` |
| **2. Лимит риска** | Защита депозита | `RiskPercentage > MaxRiskPerTrade` → `rejected: risk exceeds strategy limits` |
| **3. Sanity Check цен** | Логика SL/TP относительно входа | **BUY:** SL < Entry, TP > Entry (если заданы). **SELL:** SL > Entry, TP < Entry. Иначе → `rejected: invalid SL/TP levels` |

Дополнительно: неизвестный `StrategyID` → `rejected: unknown strategy ID`.

### 5. Текущий статус Risk Layer

| Статус | Описание |
|--------|----------|
| ✅ Готово | **`strategy/risk.go`** — `RiskManager`, `TradeSignal`, `RiskProfile`, `ValidateSignal`, тесты в `risk_test.go` |

## Текущий фокус (Next Step)

| Слой | Статус |
|------|--------|
| ✅ Indicators (Layer 1) | Pipeline Baseline v1 + Jurik RSX + FibonacciEngine |
| ✅ Layer 2 wiring | Volatility + Divergence + ZigZag + Geometry + Fib → Report |
| ✅ Layer 3 Scoring | `EvaluateScalpSignal` + dynamic thresholds + `huntForEntry` |
| ✅ Entry Risk | `RiskManager.ValidateEntry` (vetoes; sandbox bypass) |
| ✅ Paper/Sandbox | Virtual fills (readOnly) + sandbox badge + trade markers in API |
| ✅ Dashboard Terminal | Tabs + slim-header + mini bars + thresholds + Signal Matrix |
| ✅ Backtest Engine | Real historical replay + RSX marker entries + Statistics UI + equity curve |
| ✅ Dashboard Frontend | Live/Backtest isolation + Safe Mode patch + crosshair sync + trade markers + **Navigator Trendlines (time-anchored)** |
| ✅ Dashboard WS | Thread-safe `WSClient` broadcast (`server/webserver.go`) |
| ✅ SQLite Kline Cache | `history.db` (WAL) + gap-aware `FetchHistoricalKlines` + Vision bulk import (`cmd/history_sync`) |
| ✅ LuxAlgo Navigator | `trendline_navigator.go` — multi-engine Long/Medium/Short + absolute time DTO |
| ✅ Streaming AO | Pipeline; batch AO removed |
| ✅ AI Memory (code) | `MemoryStore`, `SaveTradeOutcome`, `VectorizeReport` |
| 🔜 Qdrant in main | Подключить `NewDBClient` + `MemoryStore` + trade outcome logging |
| 🔜 Post-trade loop | SaveTradeOutcome после закрытия позиции |
| ✅ Execution | Futures fapi + fstream, Algo stops, Normalizer |
| ✅ Execution sizing | `EvaluateSignal` × `LotModifier` |
| ✅ Analyst | Full Report + Qdrant fractals |
| ✅ WebSocket | `exchange/ws.go` (Futures Producer) |
| ✅ Master | FSM `IDLE`↔`IN_POSITION` + scoring entry + trailing |
| ✅ main.go | Analysts 1m/3m/15m/1h/4h; sandbox via `HYPER_SCALP_TEST` |
| 🔜 | `ChangeMarginType` (isolated/cross) |

**СТРОГОЕ РАЗДЕЛЕНИЕ:** Аналитики **не** вызывают Exchange API. FSM, риск и ордера — **эксклюзивно** `MasterGeneral` + `execution.RiskManager` + `exchange/`.

## Правила

### Правило 1

Строгая обработка **ВСЕХ** ошибок в коде — никакого игнорирования `err`.

### Правило 2

Использование **интерфейсов** для абстракции бирж, хранилищ и внешних сервисов (Binance, Bybit, Qdrant и т.д.).

### Правило 3

Все чувствительные настройки и ключи берутся **строго из `.env`**.

### Правило 4 (КРИТИЧЕСКОЕ)

Перед написанием новых модулей **ВСЕГДА** перечитывать этот файл `MEMORY.md`.

**Никогда** не удалять рабочий код без явного согласия пользователя.

### Правило 5 (Архитектура)

- `indicators/` — только расчёт и обёртки индикаторов; **без** бизнес-логики входа/выхода. Параметры индикаторов задаются через **гибкую конфигурацию** (например, `ChaosConfig` для AO: fast/slow period), чтобы настраиваться **индивидуально под шум каждого таймфрейма**.
- `vector_db/` — только работа с Qdrant (паттерны, trade outcomes, win-rate lookup); **без** торговых решений и **без** import `strategy`.
- `strategy/analyst.go` — **только** сигналы и отчёты по своему TF; **full streaming pipeline** (`layer2.go`); опирается на `indicators/` и `vector_db/`; **запрещены** прямые вызовы Exchange API.
- `strategy/layer2.go`, `strategy/geometry_tracker.go` — streaming wiring Layer 2; без FSM и ордеров.
- `strategy/scoring.go` — **только** `EvaluateScalpSignal` (Layer 3 scoring math).
- `strategy/thresholds.go` — dynamic long/short entry thresholds (thread-safe).
- `strategy/scoring_matrix.go` — per-rule scoring toggles (thread-safe).
- `strategy/backtest.go` — **`BacktestEngine`** historical simulation + RSX marker entries + stats + **`BuildAllNavigators`**
- `strategy/backtest_request.go` — **`BacktestRunSettings`** decode (matrix, navigators, risk).
- `strategy/trendline_navigator.go` — LuxAlgo Navigator engine + time-anchored DTO; **без** FSM и ордеров.
- `strategy/position_stop.go` — **`ComputeStopLossPrice`** (fractal_atr / fixed_pct)
- `strategy/risk_settings.go` — thread-safe **`RiskSettings`** singleton.
- `data/history_db.go` — SQLite kline cache only; **no trading logic**.
- `strategy/risk.go` — **`RiskManager.ValidateEntry`** (entry vetoes) + legacy `ValidateSignal` по профилям.
- `strategy/report_snapshot.go` — проекция `Report` → `vector_db.ReportSnapshot` (anti-cycle).
- `strategy/falcon.go` — **только** сборка pipeline Jurik + Wozdux (`FalconEngine`, `FalconSignals`); без FSM и ордеров.
- `strategy/volatility.go` — **только** regime + SafeStopDist + LotModifier (`VolatilityEngine`); без FSM и ордеров.
- `indicators/geometry.go`, `indicators/divergence.go` — **только** математика паттернов; без торговых решений.
- `exchange/ws.go` — **только** WebSocket Producer; **запрещён** импорт `strategy` (анти-цикл).
- **`web/app.js`** — Live/Backtest **изолированы**; Safe Mode pipeline + Viewport rules + **Navigator overlays (time-anchored)**; **запрещена** cross-tab chart sync и `candleSeries.setData` при RSX patch.
- **`web/trendline_plugin.js`** — canvas primitive для trendlines; координаты **только по time**, не index.
- **`server/webserver.go`** — dashboard HTTP + **WSClient** thread-safe broadcast; **не** писать в `*websocket.Conn` напрямую из нескольких goroutines.
- `strategy/master.go` — **эксклюзивно**: FSM, `RecoverState`, `StartDataFeed`, `Run`, `TradeSignal`, `execution.RiskManager`, исполнение ордеров, live trailing.
- `strategy/risk.go` — **ValidateSignal** по профилям (legacy); **ValidateEntry** — entry vetoes; **без** прямых вызовов Exchange API для ордеров.

### Правило 6 (Полномочия)

**СТРОГОЕ РАЗДЕЛЕНИЕ ПОЛНОМОЧИЙ:** Аналитики (`strategy/analyst.go`) работают параллельно в горутинах и занимаются исключительно сбором данных, расчётом индикаторов, поиском пиков и формированием отчётов/сигналов. Они **НЕ имеют доступа** к биржевым ордерам. Принятие финальных торговых решений, делегирование в **`RiskManager`**, проверка глобальной синхронизации таймфреймов и отправка ордеров на биржу — **эксклюзивная и единоличная обязанность Мастера/Генерала** (`strategy/master.go`).

## Структура проекта

```
trading_bot/
├── README.md            # Pipeline Baseline (Layer 1 standard)
├── MEMORY.md            # Память ИИ, архитектурные правила и статус проекта
├── .env                 # Секреты (API ключи Binance, Qdrant)
├── main.go              # Init: config, data.InitDB(), analysts, RecoverState, WS, Run
├── cmd/
│   └── history_sync/    # Binance Vision bulk kline importer (-market futures|spot)
├── history.db           # SQLite kline cache (generated, gitignored; futures data only)
├── data/                # Persistent cache layer
│   └── history_db.go    # InitDB, SaveKlines, LoadKlines (modernc.org/sqlite)
├── web/                 # Dashboard frontend (terminal UI)
│   ├── index.html       # Toolbar + slim-header + tabs + backtest/stats + navigator popups
│   ├── app.js           # Charts, thresholds, matrix, backtest pipeline, navigator overlays
│   └── trendline_plugin.js  # TrendlinePrimitive canvas (time-anchored lines + zones)
├── server/              # HTTP dashboard + API
│   ├── webserver.go     # /api/state, /api/backtest/run, /api/settings/*, /ws
│   ├── webserver_indicators_test.go
│   └── micro_broadcast.go
├── config/
│   └── config.go
├── execution/           # Position sizing (EvaluateSignal, MaxLeverage)
│   └── risk.go
├── exchange/
│   ├── exchange.go
│   ├── binance.go       # Futures fapi: market/limit, algo stop, position, balance, leverage
│   ├── klines.go        # GET /fapi/v1/klines + FetchHistoricalKlines (SQLite-aware)
│   ├── normalizer.go    # /fapi/v1/exchangeInfo limits
│   ├── network.go       # FuturesWSCombinedURL → /market/stream
│   ├── jsonflex.go      # flexString — OHLC string|number в kline JSON
│   ├── ws.go            # fstream combined klines → WsTick
│   └── ws_smoke_test.go
├── indicators/          # Layer 1 — потоковая математика (Pipeline Baseline)
│   ├── doc.go           # Package docs, baseline standard
│   ├── types.go         # Indicator, CandleIndicator
│   ├── ma.go            # SMA, EMA, RMA
│   ├── utils.go         # ExtractPrices, RollingSum, RollingStDev
│   ├── oscillators.go   # RSI, MACD, Stochastic
│   ├── volatility.go    # BollingerBands, ATR
│   ├── chaos.go         # AO, WilliamsFractals
│   ├── volume.go        # AD, CumSum, VolumeWeightedEMA
│   ├── jurik.go           # Jurik RSX — noise-free RSI
│   ├── zigzag.go          # Adaptive ZigZag (ATR + RSI)
│   ├── fibonacci.go       # FibonacciEngine: kill-zones, time zones, confluence
│   ├── geometry.go        # Trendline, triangles, volume breakout
│   ├── peaks.go           # Экстремумы и фильтрация шума по ATR
│   ├── divergence.go      # SmartDivergenceEngine (Snapshots)
│   ├── divergence_classic.go # Classic A/B/C divergence (batch)
│   ├── jurik_test.go
│   ├── geometry_test.go
│   ├── divergence_test.go
│   ├── divergence_smart_test.go
│   ├── chaos_test.go
│   └── volume_test.go
├── vector_db/           # AI-память и база паттернов
│   ├── db.go            # Qdrant: SavePattern, SaveTradeOutcome, SearchSimilarPatterns
│   ├── pattern.go       # VectorizeCandles + VectorizeReport (6-dim)
│   ├── memory.go        # MemoryStore.PredictWinRate
│   └── pattern_test.go
└── strategy/            # Мозг бота (бизнес-логика)
    ├── analyst.go       # ChiefAnalyst: кэш TF, Report, Qdrant fractals
    ├── layer2.go        # evaluateTickLocked: full streaming pipeline
    ├── layer2_test.go
    ├── geometry_tracker.go  # Trendlines → GeometryState
    ├── report_snapshot.go   # Report → vector_db.ReportSnapshot
    ├── falcon.go        # FalconEngine: Jurik + Wozduh (Pine-aligned wt11/wt22, channels, cross)
    ├── falcon_cross_test.go
    ├── falcon_test.go
    ├── volatility.go    # VolatilityEngine: regime + stop + lot modifier
    ├── volatility_test.go
    ├── scoring.go       # EvaluateScalpSignal — Layer 3 scoring (no vetoes)
    ├── thresholds.go    # Dynamic long/short entry thresholds
    ├── scoring_matrix.go # Per-rule scoring toggles
    ├── backtest.go      # BacktestEngine — RSX marker entries + virtual PnL + stats + navigators
    ├── backtest_request.go  # BacktestRunSettings, ResolveBacktestNavigators/Matrix
    ├── trendline_navigator.go  # LuxAlgo Navigator engine + time-anchored DTO
    ├── trendline_navigator_test.go
    ├── position_stop.go # ComputeStopLossPrice (fractal_atr, fixed_pct)
    ├── risk_settings.go # RiskSettings singleton (stop_loss_type, atr_multiplier)
    ├── backtest_test.go
    ├── scoring_test.go
    ├── thresholds_test.go
    ├── scoring_matrix_test.go
    ├── master.go        # MasterGeneral: FSM, scoring entry, trailing, AI hook
    ├── rsx_chart.go     # BuildRSXChart: цвет + pivot/divergence markers для dashboard
    ├── rsx_chart_test.go
    ├── risk.go          # RiskManager: профили риска, ValidateSignal
    └── risk_test.go
```
