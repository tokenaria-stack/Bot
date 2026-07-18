# Trading Bot — System Memory

**Перед написанием новых модулей ВСЕГДА перечитывай этот файл.**

> **Снэпшот MEMORY (июль 2026):** **Core 5.0 Data Plane SSOT (Phases A–G ✅)** поверх **Core 4.10 Self-Healing + Frame Double-Commit Fix ✅** поверх **Core 4.0 Great Purge (Stages 1–5 + TV Floating UI) ✅** поверх **Core 3.5 Projection (11A–11E) + Core 3.0 FE (10A–10B) + Data Foundation (9A–9J).**  
> **Core 5.0:** Ingress SSOT (`exchange/ingress.go`); Grace 5s + монотонный UPSERT + WAL-checkpoint; Boot FSM; micro-candles purge; `cmd/repair_volumes` healer; **Phase F Legacy Strategies Purge ✅**; **Phase G Rebrand ✅** — словарь `market.Frame` / `market.Runtime` / `streaming`+`snapshot`; пакеты `market/` (state) + `decision/` (contracts) + `strategy/` = doc.go beacon; DAG `exchange → market → decision → execution`.  
> Инвариант: **State → Projection → Transport**. Tip Ownership (History closed XOR Live forming). Discard axis = `window.projectionEpoch`.  
> Charts = columnar REST (closed-only tip strip) + `BroadcastChartTick`/`RouteChartTick` (DAG, strict per-TF routing, case-sensitive `1m`≠`1M`). TF camera = Sticky Live Edge / Microscope router.  
> Scale = `ScaleController` SSOT (`chart_scale_prefs_v2`, default Auto ON) + re-arm after `setData`.  
> **ColumnarStore Self-Healing (4.5):** `_intervalSec` + chronology gap-detect (`appendTick` → `{gapDetected}` вместо тихой склейки); WS `onReconnect` → `loadDashboard()`.  
> **Frame double-commit fixed (4.8):** `lastCommittedOpenTime` guard в `UpdateKlineTick` — вероятный root cause RSX tip-spike (#67), warmup depth был ложной гипотезой (диагностика: `market/continuity_test.go`).  
> **Cold boot camera (4.10):** `_commitFreshCamera` — только layout-independent `applyOptions({barSpacing, rightOffset:0})`, без `setVisibleLogicalRange`/`fitContent` (оба ломаются на контейнере 0×0).  
> Default `ENGINE_MODE=ChartOnly`. Delivery path без Falcon. Trading — только `ENGINE_MODE=live` (после новых стратегий).  
> **TF mechanics CLOSED.** **Wozduh = DAG bus only** (Falcon Evaluate gated). **Legend = chrome only** (no per-tick HTML metrics). Floating menus = `position:fixed` viewport.  
> **NEXT:** **#76 ScoreNodes** (не удалять `market/falcon.go`); **#67 Live Confirm** RSX tip-spike после 4.8. Затем #68/#69. Ops `cmd/repair_volumes` при остановленном боте.

---

## Core 2.3 — Development Plan (ACTIVE)

**Статус:** Этап 1 (UI camera) ✅ 6A–8; **Data Foundation ✅ 9A–9J**; **Core 3.5 Projection ✅ 11A–11E**; **Core 4.0 Great Purge ✅ Stages 1–5 + TV Floating UI**; ChartOnly delivery — current default.

**Цель UI:** устранить баги LWC без Scene Graph.  
**Цель Data:** разделить Owners (RAM realtime ≠ SQLite archive); убить Falcon-export на delivery path.

**Критерий успеха (1 год):** новый слой (Elliott / Footprint) = новый Layer, без правки Adapter / Store / Scheduler.

### Пять законов (UI paint)

1. Только `ChartAdapter` говорит с LWC (`setData` / `update` / range).
2. Никто не читает Store напрямую для paint — только через **окно** (`extractWindow` / Provider).
3. **RenderContext** вместо Scene: window (~3000) + viewport + geometry + visibleLayers + epoch.
4. Конвейер: `Store → Window (~3000) → Geometry → Series → Adapter`.
5. Только `RenderScheduler` инициирует paint (`isBusy` / epoch; булевый `isRenderLocked` удалён в Shot 2).

### Пять законов (Data / Jeweler — Shots 9x)

1. **RAM ≠ SQLite:** Analyst = realtime; SQLite = archive ledger. Здоровый RAM ≠ здоровый tip БД.
2. **Честная труба:** REST → только то, что отдала биржа. Дыра = дыра. Zero synthesize в ledger/ядро.
3. **Single Writer:** runtime UPSERT только через `PersistenceQueue` (WS closed + catch-up + gap-fill enqueue).
4. **HistoryProvider:** единственный owner окна истории для chart REST (`GetWindow` = SQLite ∪ RAM).
5. **Projector:** единственный packer slot→wire для live plots (WS) и columnar history; navigators берут серии из DAG, не из Falcon export.

### Этап 1 — UI Stabilization

| Shot | Содержание | Статус |
|------|------------|--------|
| 1–5 | extractWindow, epoch, geometry, shiftCamera, zombies | ✅ |
| 6A | F3 camera; TF `viewportAnchor`; `hostId` DDR | ✅ |
| 6B+ | Active Driver / slave scroll | 🔜 |
| 7–8 | Camera contracts; live-edge visibility | ✅ partial |

### Этап Data Foundation (Shots 9A–9J) — ✅ CLOSED

| Shot | Содержание | Статус |
|------|------------|--------|
| **9A** | `HistoryProvider.GetWindow` = SQLite ∪ RAM overlay | ✅ |
| **9B** | `BroadcastChartTick` atomic OHLCV+plots all TFs; Falcon telemetry frozen | ✅ |
| **9C** | `PersistenceQueue` + UPSERT; closed bar async archive | ✅ |
| **9D** | `SQLiteTipNeedsCatchUp` independent of RAM gap-fill | ✅ |
| **9E** | `FetchClosedRange` sterile pipe; delete `FetchHistoricalKlines`+synthesize; sole writer | ✅ |
| **9F** | `EngineMode` ChartOnly\|Live; gate Falcon/Score/`Run`; hot-path log silence | ✅ |
| **9G** | Audit: 503 = phantom `chartExportPoints` (not panic) | ✅ analysis |
| **9H** | Navigators from DAG; purge ExportChartSeries/`chartExportPoints`; lightweight `/api/state` | ✅ |
| **9I** | DAG Div annotations via Projector; purge `legacyChartAnnotationsFromKlines`; Falcon-free chart REST/WS | ✅ |
| **9J** | Live header sterilization: `enrichFromDAG` / tick tip from DAG slots; no FalconSnapshot/ScoreEngine in server UI | ✅ |

**Synchronous Slice Rule:** один индексный диапазон на `times`, все `candles.*`, все `plots[id]`.  
**Render window:** LWC soft cap 15000 (`RENDER_WINDOW_LIMIT`); Store может расти.  
**Camera (Shot 6A):** F3 only — после F2. Prepend → `shiftCamera(addedBars, viewportRange)`; TF restore → `ViewportManager.restore(anchor)`; fresh → `fitContent()` на price only. **Запрет** глобальных `isLocked`.  
**DDR routing:** `component.hostId` → `hostMap[hostId]` (`rsx` / `wozduh`). Pane id (`pane_osc` / `pane_score`) — группировка манифеста, не chart target.  
**Budget:** F1/F2 priority frames через RenderScheduler RAF split; камера = фаза F3.

### Этап 2 / 3 (будущее) + Core 3.0–3.5 Frontend

| Shot | Содержание | Статус |
|------|------------|--------|
| **10A** | Declarative Auto/Log scale (`ScaleController`, localStorage, manual Y-drag sync) | ✅ |
| **10B** | Zero-gap handoff: WS-first + tick buffer around `replaceMonolith` | ✅ |
| **11A** | History Tip Protocol: `dropFormingTip` before `ReplayDAGKlines` (closed XOR forming) | ✅ |
| **11B** | `window.projectionEpoch` SSOT discard axis (replaces requestId + historyEpoch) | ✅ |
| **11C** | Atomic Publish: soft TF handoff (`prepareLiveTfHandoff`); one `markDirty(full)` after monolith+DDR | ✅ |
| **11D** | Scale default Auto ON; Smart Camera router (Sticky Edge / Microscope / Zoom-OUT) | ✅ |
| **11D-FIX** | Sticky Live Edge priority before zoom direction | ✅ |
| **11D-HOTFIX** | `updateTick` sentinel filter; `ScaleController.applyAll()` after candle `setData` | ✅ |
| **11D-VIEWPORT** | Live Edge preserves `visibleBars` + `rightOffset` (no hard 150 reset) | ✅ **TF mechanics CLOSED** |
| **11E** | Delta Integrity: NewBar boundary → delta chain in `RenderScheduler` | ✅ |

### Core 4.0 — Great Purge (Wozduh → DAG + DDR UI + Floating) — ✅ Stages 1–5 + TV UI

| Stage | Содержание | Статус |
|-------|------------|--------|
| **1 Primitives** | MACD Snap fields; RollingSum/CumSum `SaveState`; `TestMACD_SaveRestore_IntraBarRollback` | ✅ |
| **2 Slots** | 15× `SlotWozduh*` atoms перед `SlotCount` (`core/slots.go`) | ✅ |
| **3 WozduhNode** | Полная Falcon Woz math → bus only; golden parity vs Falcon; Jurik остаётся в RSXNode; `falcon.go` **сохранить** | ✅ |
| **4 Manifest/UI** | `Configurable` на `UIComponent`; полный `wozduh_layout.go`; data-driven `settings-renderer.js` | ✅ |
| **5 Wire purge** | Falcon line fields strip из tip/`MarketState`; tip via `plots`; Falcon Evaluate только за `EngineAllowsStrategies()`; toolbar/legend без per-tick tip HTML | ✅ |
| **TV Floating UI** | Legend title **Woz** + 👁 (`visibility` on `.lwc-host`); gear → `FloatingMenu`; Woz menu drag-handle + Ok; fixed-up open; outside close; drag | ✅ |

**Канон UI chrome (не ломать):**
- `LegendRenderer` — только title + eye + gear (никаких live `leg-val-*` thrashing).
- `FloatingMenu` (`web/ui/floating-menu.js`) — единственный owner open/drag/outside; `position:fixed` к окну (не chart-absolute).
- Open: `left = anchor.left`, `bottom = innerHeight - anchor.top` (меню растёт **вверх** от шестерёнки).
- Eye: `host.style.visibility` toggle — layout/LWC resize observers живы.
- Outside-click listener — один глобальный (`outsideBound` idempotent).
- Woz settings: handle `⋮⋮⋮ Woz Settings` + Ok closes; RSX static HTML Save/handle; `bindAll`/`initDrag` на оба.
- Composition root: `web/boot.js` (не `app.js` / `app.legacy.js`).

**Projection laws (Shots 11x — canon):**

1. **Tip Ownership:** REST history = closed bars only (`dropFormingTip`); forming tip = WS `BroadcastChartTick` only.
2. **ProjectionEpoch:** one discard axis for TF / load / buffer / hydrate / WS.
3. **Atomic Publish:** TF switch keeps old LWC frame under buffering until one full paint (no `setData([])` wipe).
4. **CameraIntent:** `tipVisible ≠ isAtRightEdge`; Live Edge carries `visibleBars`+`rightOffset`, never `barSpacing` across TF; Microscope = zoom-IN off-edge + healthy spacing.
5. **Delta Integrity:** `isNewBar` never coalesces away (`deltas[]` chain → sequential `applyDelta`).

- Layer Interface, culling, ChartDataProvider  
- ChunkLedger, SceneFrame/Object Graph — только по необходимости  
- Live `tick.annotations` upsert in ColumnarStore (optional); HistoryBus tip for navigators (#64); toolbar tip from DAG (#65 FE residual)
- **IIR tip SSOT (#67):** align live Analyst warmup depth with history Replay (kill tip cliff vs TV) — **NEXT**
- Osc fixed scale bounds in manifest (#68)
- MemoryBudget (#69) — deferred (prune-right interfered; revisit after #67)
- Order Flow (#44) — **amputated** until strategy settings  
- **Stage 6 ScoreNodes (#76)** — later; keep `market/falcon.go`
- `ENGINE_MODE=live` re-enable when trading stack reconfigured
- Great Purge Stages 1–5 + TV Floating UI — ✅ (see Core 4.0 table above)

### Core 4.4–4.10 — Continuity Audit Chain (RSX tip-spike + WS gaps) — ✅

**Триггер:** дыры на графике от тиковых свечей + мутация (шип) RSX на стыке History/Live, отличная от TV. Расследование шло слоями сверху вниз (transport → frontend store → backend Marker), каждый слой диагностировался тестом/трейсером **до** правки кода.

| Shot | Содержание | Статус |
|------|------------|--------|
| **4.3 Golden Audit** | `server/golden_audit_test.go`: SQLite vs Binance REST для одного закрытого бара — OHLC+RSX идентичны бит-в-бит; **volume отличался** (не влияет на RSX) | ✅ backend data plane стерилен |
| **4.4 Frontend Continuity Audit** | Диагностика (без правок): `ColumnarStore.appendTick` не проверял разрыв хронологии; `boot.js` `initLiveWebSocket` не слушал `onClose`/`onReconnect` | ✅ найдено |
| **4.5 Self-Healing** | `ColumnarStore.setTfInterval` + gap-check в `appendTick` (`(time-lastTime) > intervalSec*1.5` → `{gapDetected}`, ничего не пушит); `replaceMonolith` нормализует `times` через `chartTime`; `boot.js`: `syncStoreTfInterval` перед всеми тик-путями, `onReconnect` → `beginLiveTickBuffer()+loadDashboard()`, `pushLiveTickDelta` реагирует на `gapDetected` → `loadDashboard()` (throttle 10s + `__isDashboardLoading` guard против шторма) | ✅ |
| **4.6 Tracer Bullet** | 4 временных `console.log` (WS Receive → Store Append → Store Saved → LWC Update) — доказали, что JS-конвейер стерилен: искажённое `line_rsx` приходит **уже неверным по WS**. Логи удалены после диагностики | ✅ diagnostic only |
| **4.7 Marker State Audit** | Диагностика (без правок) `core/runner.go`/`strategy/analyst.go`: Restore→Update→Save протокол корректен (нет накопления ошибки IIR); найден **двойной коммит** — `UpdateKlineTick` при переходе на новый бар безусловно повторно коммитил уже закрытый (`isClosed=true`) предыдущий бар → каждый бар проходил через DAG **дважды** | ✅ root cause found |
| **4.8 Marker Double-Commit Fix** | `strategy/analyst.go`: поле `Marker.lastCommittedOpenTime`; страховочный коммит в `UpdateKlineTick` теперь только если `last.OpenTime != lastCommittedOpenTime`; `strategy/layer2.go`: `markTailCommittedLocked` после `warmupStreaming`/`replayStreamingLocked`; сброс в `resetStreamingEngines` | ✅ вероятный fix #67 |
| **4.9 TF Interval Parser Fix** | `boot.js` `installGlobalShims` шимил `window.getIntervalMs = () => 60000` для **любого** TF (перебивал честный `TimeNormalizer.getIntervalMs`) → на TF>1m gap-check (4.5) ложно срабатывал на каждый штатный новый бар → reload-шторм. Fix: честный regex-парсер `parseTfIntervalMs` как fallback | ✅ |
| **4.10 Cold Boot Camera Fix** | `chart-compositor.js` `_commitFreshCamera`: `setVisibleLogicalRange`/`fitContent` делят на ширину DOM-контейнера — на cold boot контейнер ещё `0×0` → NaN-коллапс LWC до ручного Auto Scale. Fix: только `timeScale().applyOptions({barSpacing, rightOffset:0})` — layout-independent | ✅ |

**Канон (новые инварианты):**
- **ColumnarStore не клеит дыры молча.** Разрыв хронологии — сигнал (`gapDetected`), не самостоятельное решение; лечит composition root (`loadDashboard`), не Store (никакого `loadDashboard` внутри `columnar-store.js`).
- **UpdateKlineTick — один коммит на бар.** `lastCommittedOpenTime` — единственный источник правды о том, что бар уже прошёл Save; страховочный коммит на границе баров — только если официальный `isClosed=true` был пропущен (обрыв сети), не рутинный путь.
- **Cold boot camera — только width-independent API.** `setVisibleLogicalRange`/`fitContent` запрещены до первого реального layout; см. debt #80 (тот же риск в `ViewportManager.restore`, не тронут в 4.10).
- **TF-интервал — SSOT только `TimeNormalizer.getIntervalMs`.** Любой fallback/шим обязан быть честным парсером, не константой (прецедент 4.9).

**Файлы:** `web/columnar-store.js`, `web/boot.js`, `web/ws.js`, `web/chart-compositor.js`, `strategy/analyst.go`, `strategy/layer2.go`, `server/webserver.go` (`RouteChartTick`/`routeTick`), `server/golden_audit_test.go`, `strategy/continuity_test.go`.

### Core 5.0 — Data Plane SSOT (Phases A–G) — ✅ CLOSED

**Цель:** одна свеча = один жизненный цикл = одна каноническая версия. Триггер: Golden Audit (volume drift SQLite 21.257 vs Binance 48.47) + гонка boot (REST recovery до WS connect) + долг #19 (3 копии merge).

| Phase | Содержание | Статус |
|-------|------------|--------|
| **A. Ingress SSOT** | `exchange/ingress.go`: `Authority` (Estimated 0 / Settled 1 / Final 2 = WS x=true), правило merge (выше — целиком; ниже — discard+метрика; равные — High=MAX/Low=MIN/Volume=MAX/Close=incoming), `Validate` (typed `RejectReason`: invalid_time/invalid_range/negative_volume/future_bar; reject, никаких тихих правок), `IngressMetrics` (атомики), Edge-ledger `openTime→Authority` (FIFO 4096, вне ledger = Settled, НЕ в SQLite, НЕ в Kline). Удалены оба дубля `mergeKlinesByOpenTime` (strategy/kline_merge.go целиком + server/webserver.go) — все 4 вызова на `exchange.MergeKlineSeries` (RAM live = Final, REST/SQLite = Settled) | ✅ долг #19 закрыт |
| **B. Boundary Policies + WAL** | `data.KlineSettleGraceMs=5000` в `CapKlineEndToLastClosed` (REST никогда не запрашивает бар моложе 5s после закрытия — root cause volume drift); монотонный UPSERT `high=MAX, low=MIN, volume=MAX` (firewall, не бизнес-логика); `PRAGMA wal_autocheckpoint=1000` + `data.CheckpointWAL()` (`wal_checkpoint(TRUNCATE)`) каждые 5 мин из воркера PersistenceQueue — WAL 178MB утечка диска устранена | ✅ |
| **C. Boot FSM** | `strategy/boot_controller.go`: Phase 0 Connecting (WS первым, тики в буфер cap 4096, Marker не тронут) → Phase 1 Loading (SQLite+REST через Ingress) → Phase 2 Reconciling (буфер реплеится по порядку через `MasterGeneral.routeTick` — единый канонический путь тика, вынесен из StartDataFeed) → Phase 3 Live (StartDataFeed + gap-fill/catch-up лупы ПОСЛЕ выхода в Live). Чистка main.go: `agentBootLog`/hardcode debug-путь удалены | ✅ |
| **D. Bar Source Seam + Purge** | Контракт шва задокументирован в `exchange/ingress.go` (см. канон ниже). Выкорчеваны: `server/micro_candles.go`, `server/micro_broadcast.go`, `IsOrderFlowTimeframe`/`loadOrderFlowKlines`/`orderFlowWarmupBars`/`TickBufferLen`/`d.orderFlow` из webserver.go, `StartMicroBroadcast` из main.go. Тиковые TF в меню (`timeframes.go`) остаются как «розетка» — вернут данные с TickBarBuilder | ✅ |
| **E. Data Repair + Repo Cleanup** | `cmd/repair_volumes/main.go` — healer застрявших объёмов: REST re-fetch (7d, 1m/3m/5m/15m) → `SaveKlines` (монотонный UPSERT сам поднимает volume) → `CheckpointWAL`. Repo: удалены `backups/` (38MB dump), `server/history.db*`, `server/data/`, бинарники `trading_bot`/`strategy.test` (51MB), tracked-артефакты `server/config/matrix.json`, `server/server/`, `.DS_Store`. `.gitignore` переписан (`.cursor/*` с негациями `!rules/`, `!mcp.json.example`) | ✅ (healer создан, НЕ запускался — запускать при остановленном боте) |
| **F. Legacy Strategies Purge** | Удалены ScoreEngine/matrix/thresholds/chief/risk/position_*/rsx_signal_memory/backtest_ab/entry_audit/`cmd/backtest`; trade-FSM из `master.go` + `EnableTrading` из streaming_replay; settings `/api/settings/{matrix,thresholds,risk}`; Qdrant-хвосты из `analyst.go`; wire/FE delivery L/LL/S/SS off (`AnnotationFromDivState` no-op). Оставлены: `score_types.go`, `falcon.go`, `execution/`, `vector_db/`, navigator HH/LL, `EngineAllowsStrategies`. Verify: `go build ./...` + `go vet ./...` ✅ | ✅ |

**Phase F — итог (выполнено 18.07.2026):**
- **Ядро сохранено:** data-plane Frame/streaming+snapshot/Boot/MTF/DAG/chart RSX colors, `falcon.go` (#76), `score_types` sockets, navigator HH/LL geometry.
- **Удалено:** legacy scoring + trade FSM + risk_settings + A/B backtest CLI + matrix/threshold/risk APIs + FE chrome; chart surface L/LL/S/SS (math `indicators/divergence_rsx` + `SlotDivState` остаются, wire/UI не публикуют).

**Phase G — Rebrand (G1–G4 ✅, выполнено 19.07.2026):**
- **G1 Vocabulary ✅:** `Marker`→`Frame`, `MasterGeneral`→`Runtime`, `layer2.go`→`streaming.go`+`snapshot.go`; идентификаторы Analyst / MasterGeneral / Layer2 вычищены из Go-кода (JSON-тег `json:"marker"` и chart-label поле `Marker string` сохранены для FE).
- **G2 Package Move ✅:** data-plane → `market/`; contracts → `decision/`; `strategy/` = `doc.go` beacon only.
- **G3 Docs ✅:** `MEMORY.md` + `market/doc.go` + `decision/doc.go` + `strategy/doc.go`.
- **G4 Audit ✅:** lexical + DAG `exchange → market → decision → execution` (`decision`↛`market`, `market`↛`server`, `exchange`↛`market`/`decision`).

**Словарь Core 5.0 (канон):**
| Было | Стало | Пакет | Роль |
|------|-------|-------|------|
| `Marker` | `Frame` | `market` | Per-TF streaming state («что происходит») |
| `MasterGeneral` | `Runtime` | `market` | Data runtime / tick routing |
| `layer2.go` | `streaming.go` + `snapshot.go` | `market` | O(1) Snapshot/Restore intra-bar |
| `score_types` | `ScoreDecision` / `ScoreFactor` | `decision` | Контракты («что делать») |
| `strategy/` (код) | beacon `doc.go` | `strategy` | Исторический placeholder |

**Канон (новые инварианты Core 5.0):**
- **Источник данных приоритетнее значения данных.** Merge решает по Authority, не по полям. WS-финал (x=true) никогда не проигрывает REST. Полевая эвристика (MAX/MIN) — только при равном доверии.
- **Bar Source Seam:** в ingress-pipeline входят ТОЛЬКО закрытые канонические бары (`exchange.Kline`); способ агрегации (время/тики/объём) — приватная деталь продюсера. Forming-тики (x=false) обходят pipeline (телеметрия Frame, Core 4.8). Time-бары = клайны биржи (канон TV), никакой самосборки из трейдов.
- **Boot: WS первым.** REST recovery никогда больше не «истина» поверх пропущенных WS-баров. Один канонический путь тика — `Runtime.routeTick` (live и boot-replay).
- **Import DAG:** `exchange → market → decision → execution`. `market` отвечает «что происходит»; `decision` — «что делать» (без импорта `market`).
- **SQLite firewall ≠ лечение.** MAX/MIN в UPSERT — последний рубеж; корень (Grace) — на границе REST.

**Контракт будущего `TickBarBuilder` (НЕ реализован — ждёт воскрешения aggTrade, долг #44):**
- Источник: существующее кольцо `domain.TickBuffer` (RAM-only, фиксированный cap, тики никогда не персистятся).
- Билдер держит ТОЛЬКО текущий строящийся бар (инкрементально: count/OHLCV) — не историю свечей (старый micro_candles пересинтезировал всё кольцо на каждый запрос — за это и выкорчеван).
- Готовый закрытый бар → `exchange.Kline` → Ingress pipeline (Merge/Validate) → дальше система его не отличает от time-бара.
- Свой уровень авторитета (например, `AuthorityAggregated`): для локальных баров НЕТ REST-recovery — дыра в тиках это честная дыра.
- Зависимость: подписка `@aggTrade` в `exchange/ws.go` (закомментирована, долг #44) + `OrderFlowStore` sink.

**Файлы:** `exchange/ingress.go`, `data/history_db.go`, `data/persistence_queue.go`, `market/boot_controller.go`, `market/runtime.go` (`routeTick`), `main.go`, `market/ram_history.go`, `market/frame.go`, `market/streaming.go`, `market/snapshot.go`, `decision/score_types.go`, `strategy/doc.go`, `server/history_provider.go`, `server/webserver.go`, `cmd/repair_volumes/main.go`, `.gitignore`.

**После Phase F+G:** стерильные ядра — БД, live, фронтенд, бэктест (chart replay only); legacy strategy code вычищен; границы пакетов зафиксированы. Новые стратегии — в `decision/` на канон `ScoreDecision`/`Factors`.

### Project Renaissance (база Phase 0)

| Файл | Роль |
|------|------|
| `web/boot.js` | Composition root; ProjectionEpoch; tick buffer; Atomic `loadDashboard` / `prepareLiveTfHandoff` |
| `web/chart-core.js` | Sterile ChartAdapter; Scale re-arm after `setData`; one-way TimeScale |
| `web/chart-compositor.js` | Sole live paint; F1/F2/F3; delta chain flush |
| `web/render-scheduler.js` | NewBar boundary coalesce (`deltas[]`) |
| `web/series-factory.js` | DDRFactory; `columnToLWC` + `updateTick` sentinel parity |
| `web/ui/viewport-manager.js` | Camera router + Live Edge `visibleBars`/`rightOffset` |
| `web/ui/scale-controller.js` | Auto/Log SSOT (`chart_scale_prefs_v2`, default Auto ON) |
| `web/app.legacy.js` / `adapter.legacy.js` | Quarantined |
| `web/ui/floating-menu.js` | TV-like fixed menus: open-up, drag, outside close |
| `web/ui/legend-renderer.js` | Pane chrome: Woz/RSX title + eye + gear |
| `web/ui/settings-renderer.js` | DDR Configurable → Woz checkboxes + Ok + drag handle |

**Paint path (Core 2.3 Shot 6A):**
```
Store → extractWindow → RenderScheduler (F1 RAF → F2 RAF)
  → ChartCompositor.flush
      F1: price + annotations
      F2: DDR plots + navigator
      F3: shiftCamera | ViewportManager.restore | fitContent(price)
  → ChartAdapter + DDRFactory
```

---

## Core 2.3 — Data Foundation Canon (Shots 9A–9J) — июль 2026

### Владельцы (Owners)

```
Биржа
  ├── WS  → Analyst RAM (realtime) → BroadcastChartTick (Projector plots + Div annotations)
  └── REST FetchClosedRange → PersistenceQueue → SQLite (archive ledger)
SQLite ∪ RAM
  └── HistoryProvider.GetWindow → columnar / (legacy JSON history)
Navigators
  └── ExtractDAGNavigatorSeries(ReplayDAGKlines) → BuildAllNavigators
Annotations
  └── SlotDivState (DAG) → Projector → wire.Annotation[] (columnar + WS)
```

| Owner | Ответственность | Запрещено |
|-------|-----------------|-----------|
| Analyst RAM | Live klines + DAG tick | Писать SQLite напрямую; synthetic bars |
| SQLite | Immutable-ish archive OHLCV | Решения, scoring, synthesize |
| PersistenceQueue | Единственный runtime `SaveKlines` | — |
| HistoryProvider | Merge SQLite∪RAM для chart windows | — |
| Projector | Slot → wire keys + DivState → markers | Считать дивергенции |
| EngineMode | ChartOnly vs Live trading stack | — |

### EngineMode (`ENGINE_MODE`)

| Mode | Default | Поведение |
|------|---------|-----------|
| **ChartOnly** | ✅ yes | Нет `SetScoringMatrix` load; нет `Master.Run`; Falcon/div/fib/geometry gated в `evaluateTickLocked`; DAG всегда; `/api/state` enrich telemetry off |
| **Live** | env `ENGINE_MODE=live` | Полный Falcon Layer2 + ScoreMatrix + `Master.Run` |

Файлы: `strategy/engine_mode.go`, `config.Config.EngineMode`, `main.go`.

### Ingestion (Shot 9E)

| API | Контракт |
|-----|----------|
| `FetchClosedRange` | **Один** futures REST call; ≤1000 bars; no SQLite; no synthesize |
| `FetchClosedRangePages` | Цикл страниц + pause; всё ещё sterile |
| ~~`FetchHistoricalKlines`~~ | **УДАЛЁН** (+ detectGaps, synthesizeForwardFill, spot gap REST) |
| Boot | `LoadRAMHistory` = DB ∪ FetchClosedRangePages → RAM only |
| Tip catch-up | `SQLiteTipNeedsCatchUp` → FetchClosedRange → `AppendClosedBars` |
| RAM gap-fill | FetchClosedRangePages → `LoadHistoricalKlines` + enqueue archive |

### `/api/state` (Shot 9H)

- **Не** генерирует Candles/Oscillators для live paint.
- `navigators=1`: klines → `ExtractDAGNavigatorSeries` (SlotJurikRSX + SlotWozduhSlow) → navigators.
- Полный state: metadata + optional navigators; ChartOnly без Falcon/Score/Fib в payload.
- Charts: `GET /api/history?format=columnar` + WS `BroadcastChartTick`.

### Annotations (Shot 9I) + Header (Shot 9J)

- DAG `DivergenceNode`: ZigZag swings × oscillator slot → `SlotDivScore` / `SlotDivState` (чистая математика, без Falcon).
- Projector: rising-edge `SlotDivState` → `wire.Annotation` (цвет/форма); манифест `ann_rsx_div` (`dataMode=annotations`).
- Columnar + legacy JSON history + WS tick: **без** `legacyChartAnnotationsFromKlines` / StreamingReplay.
- StreamingReplay остаётся только для backtest/lab.
- **Header:** `enrichFromDAG` / `BroadcastChartTick` → `SlotJurikRSX` (+ Wozduh vol slots на tick). `RedLine`/`GreenLine` = 0 (нет price-RSI слотов). ChartOnly: scores/actions/fib/regime empty. Live UI: `LongScore` = `SlotTotalScore` only (no ScoreEngine).

### Удалённый легаси (не возвращать)

- `ExportChartSeriesForWindow` / `chartExportPoints` / `buildLiveChartFromRAM` / `buildRAMChartExport` / `buildTailPollChartFromRAM`
- `legacyChartAnnotationsFromKlines` + Falcon chart annotations path
- `enrichFromAnalyst` / `scoreDecisionForAnalyst` / `FalconSnapshot` / `FibZonesSnapshot` на UI path
- `FetchHistoricalKlines` + forward-fill synthesize
- `NewFalconEngine()` ad-hoc в UI enrich
- Hot-path DEBUG spam (`LoadKlines` SELECT logs, HistoryProvider GetWindow spam)

### Оставшийся Falcon (не chart delivery / не UI header)

| Место | Зачем ещё жив | Приоритет |
|-------|---------------|-----------|
| Marker Layer2 / MTF / scoring | `ENGINE_MODE=live` trading stack only | OK gated |
| Streaming replay / backtest | Lab path | OK until Live chart rewrite |
| `boot.js` header wire | backend fills `jurik` on state/tick; modern boot may not call `updateHeaderData` | 🟡 optional FE hook |

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

## 6. Розетки, а не электростанции (Sockets, not Power Plants)
Архитектура продумывается на шаг вперёд: проектируем розетки (интерфейсы, контракты, политики — Bar Source Seam в `exchange/ingress.go`, `IngressPolicy`, Authority), но не строим электростанции (реализации без реального потребителя). Спекулятивные FSM состояний свечи, Revision/Version-поля, per-exchange settlement registry, Renko/Kagi/Volume-билдеры без консьюмера — запрещены. Когда потребитель появляется, реализация вставляется в готовую розетку, не трогая ядро (пример: контракт `TickBarBuilder` в Core 5.0 Phase D).

**ДИРЕКТИВА АССИСТЕНТУ:** Перед написанием любого кода или изменением архитектуры сверяйся с "Протоколом ювелира". Работай как хирург: локализуй проблему, спроектируй чистое решение на уровне ядра, сохрани изоляцию сигналов.

---

## Архитектура Принятия Решений (Score Module)

Канон после Phase F+G: `Frame` (market state) → будущий `ScoreEngine.Calculate()` → `ScoreDecision` (`decision/`). Legacy scoring implementation purged; types remain as sockets.

- Отсутствуют структуры-посредники (никаких `Report`).
- Данные не суммируются преждевременно. Каждый индикатор формирует свой изолированный `ScoreFactor` (`BUY` / `SELL` / `WAIT`) внутри карты `ScoreDecision.Factors`.
- Модули должны строго соответствовать **Протоколу ювелира**.
- **Import rule:** `decision` не импортирует `market` (контракты чистые).

| Компонент | Файл | Роль |
|-----------|------|------|
| `ScoreDecision` / `ScoreFactor` | `decision/score_types.go` | Контракты решения; `ScoreEngine` — пустая розетка |
| `Frame` | `market/frame.go` | State («что происходит») |
| `Runtime` | `market/runtime.go` | Tick routing / data plane |
| Falcon (bus) | `market/falcon.go` | Keep until #76 ScoreNodes |
| Qdrant socket | `vector_db/` | Pattern memory; no live strategy consumer |

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

## Ядро: Frame и Streaming Pipeline

`Frame` (`market/frame.go`, `streaming.go`, `snapshot.go`) — единый **streaming-state** на символ/ТФ («что происходит»). Индикаторы пишут в него; будущий ScoreEngine (`decision/`) читает через accessors (`FalconSnapshot`, `VolatilityStateSnapshot`, `MTFStates`, …). `Runtime` (`market/runtime.go`) маршрутизирует тики.

**Поток на каждом тике (`UpdateKlineTick`):**
```
Binance WS kline
  → Frame.UpdateKlineTick(k, isClosed)
  → evaluateTickLocked(k, barIndex, isClosed)
       1. restoreStreamingState()           // O(1) rollback открытого бара
       2. FalconEngine.Evaluate             // gated in ChartOnly
       3. VolatilityEngine, orangeRsi, ad, ao, stoch, ZigZag
       4. divEngine (micro-tick + macro snapshot на swing)
       5. rsxMarkers, geometry, fib zones
       6. saveStreamingState()              // только if isClosed
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
| **Binance feed** | `exchange/ws_client.go`, `main.go` | Futures klines → `Runtime` → per-TF `Frame` map |

### Исполнение (`Runtime`, `market/runtime.go`)

**ChartOnly default:** trade FSM purged in Phase F. Data plane (`routeTick`, Boot, MTF, chart broadcast) живёт в `Runtime`. Trading — только после новых стратегий в `decision/` + `ENGINE_MODE=live`.

**Sizing SSOT (сокет):** `execution.CalculateTargetQuantity(...)` — остаётся; legacy risk_settings purged.

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

### [Core 2.3 Этап 1 — Shots 5–6A — июль 2026]

#### [Shot 5 — Zombie Eradication & RSX Settings — ✅]
- `DDRFactory.seriesMap` хранит `{ chart, series }`; `clear()` → `chart.removeSeries(series)` перед Map clear (orphan LWC series eradicated).
- `boot.js`: `resolveLiveRsxSettings()` → `RsxController.getSettings('live')`; `syncRsxIndicatorSettings()` с seq + promise-chain dedup (no `DEFAULT_RSX_SETTINGS` hardcode).
- Fresh viewport: `fitContent()` только на price chart (не `scrollToPosition(0)`).

#### [Shot 6A — F3 Physics, TF Anchor Pipeline, Canonical hostId — ✅]
**Файлы:** `web/chart-compositor.js`, `web/boot.js`, `web/series-factory.js`, `ui_config/rsx_layout.go`, `ui_config/wozduh_layout.go`, `ui_config/score_layout.go`, `core/manifest.go` (`HostID` уже был).

| Механизм | Детали |
|----------|--------|
| **F3 camera** | `_commitPrependCamera` / `_commitFullCamera` строго после `runF2`; только при `phase === 'F2' \|\| !phase` |
| **Prepend** | `shiftCamera(addedBars, intent.viewportRange)` — range снят orchestrator'ом **до** merge; null-safe |
| **Full / TF** | `intent.anchor` → `ViewportManager.restore`; иначе `viewport:'fresh'` → `fitContent(price)` |
| **TF pipeline** | `TimeframeController` → `loadDashboard({ viewportAnchor })` → `markDirty({ viewport:'restore', anchor })` |
| **hostId** | RSX lines → `"rsx"`; Wozduh lines + Score hist/line → `"wozduh"`; `buildPanes({ rsx, wozduh }, panes)` |

**Канон:** нет глобальных locks; рассинхрон лечится порядком исполнения (F1→F2→F3). `boot.js` только связывает; камера — зона Compositor.

**Next (6B):** Active Driver / TimeScaleCoordinator — DOM wheel proxy с slave → master.

---

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
| UI Registry | `ui_config/` | `HostID` routing: `line_rsx`→`rsx`; `woz_*`/`score_*`→`wozduh` |

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
- **Shot 6A:** `buildPanes(hostMap, panes)` routes by `component.hostId` (not pane id). Host map: `{ rsx, wozduh }`. Backend: `core.UIComponent.HostID` + `ui_config/*_layout.go`.

### Core 4.0 — Great Purge (Wozduh bus + TV chrome) ✅
**Инвариант:** Wozduh math живёт в `WozduhNode` → `SlotWozduh*`; UI читает только DDR plots/manifest. `market/falcon.go` **не удалять** до Stage 6 ScoreNodes (#76).

| Кусок | Канон |
|-------|-------|
| Primitives | MACD / RollingSum / CumSum Snapshot-Restore; intra-bar rollback test |
| Slots | `SlotWozduhRsiPrice` … `SlotWozduhVolCross` (+ Fast/Slow) before `SlotCount` |
| Node | Golden parity vs Falcon Evaluate; Jurik stays in RSXNode |
| Manifest | `UIComponent.Configurable`; full `wozduh_layout.go` |
| Wire | Tip via `plots`; no Falcon line DTO on ChartOnly tip path |
| Legend | Title + 👁 + ⚙ only — no per-tick metric HTML |
| Floating | `FloatingMenu.open`: `fixed`, grow up via `bottom`; one outside listener; drag on handle |
| Eye | `chart-wrap .lwc-host` → `style.visibility` (not `display`) |

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
| **Microscope guard** | WS ticks only (legacy `_microscopeTickMuted` in processTick). **Never** block `shouldLoad` REST prepend (#5) |

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
| Prepend F1/F2 indicator desync | Camera shift before DDR `setData` (master sync on stale slaves) | ✅ Shot 6A F3 |
| TF jump / lost anchor | `loadDashboard()` ignored `viewportAnchor` | ✅ Shot 6A pipeline |
| Wozduh lines on RSX pane | Blind `pane_osc → rsx` mapping | ✅ Shot 6A `hostId` |
| Orphan LWC series after TF | `DDRFactory.clear()` Map-only, no `removeSeries` | ✅ Shot 5 |
| Viewport jump / «гармошка» | dual transport (fixed 7B); residual edge cases | 🟡 QA |
| Browser freeze на boot | 3000 bars × N series `setData` + `columnToLWC` object alloc | 🟡 pre-alloc partial |
| DDR series empty on boot | `fetchAndHydrateHistory` до `buildPanes` → empty `slots` | 🟡 manifest fallback |
| RSX markers missing on prepend | Annotations на wire (8A) но UI не рисует (8B pending) | 🔜 |
| Slave panes dead scroll | `handleScroll:false`; need Active Driver (Shot 6B) | 🔜 |
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
| **39** | ~~**SettingsRenderer (Phase 7)**~~ — DDR `Configurable` + `settings-renderer.js` + Legend eye/gear + `FloatingMenu` (Core 4.0) | `settings-renderer.js`, `legend-renderer.js`, `floating-menu.js`, `wozduh_layout.go` | ✅ Great Purge Stage 4 + TV UI |
| **40** | **Backtest DDR cutover** | `backtest-pipeline.js` | 🔜 live only |
| **41** | **wsQueue hard cap** | `hydration-orchestrator.js` | 🟡 cap 64 on slow server |
| **42** | **`added` server vs client mismatch warn** | `boot.js` | 🟢 log if `data.added !== store.added` |
| **43** | **Boot slots from manifest** | `series-factory.js` | 🟡 `resolveLiveSlotIds()` fallback |
| **49** | **Active Driver / slave scroll (Shot 6B)** | `chart-core.js`, `boot.js` | 🔜 wheel proxy → master |
| **50** | ~~**F1/F2 prepend desync**~~ | `chart-compositor.js` | ✅ Shot 6A F3 |
| **51** | ~~**TF viewportAnchor dropped**~~ | `boot.js` | ✅ Shot 6A |
| **52** | ~~**Manifest pane→chart misroute**~~ | `ui_config/`, `series-factory.js` | ✅ Shot 6A hostId |
| **53** | ~~**Orphan LWC series (zombies)**~~ | `series-factory.js` | ✅ Shot 5 removeSeries |
| **44** | **Order Flow** — **amputated** (no `@aggTrade`/`@forceOrder` WS; `loadOrderFlowKlines` stub; no `OrderFlowStore` alloc). Restore with strategy settings UI later | `exchange/ws.go`, `main.go`, `server/webserver.go`, `domain/orderflow.go` | ⏸ amputated; re-enable w/ settings |
| **45** | **Backtest → columnar** | `api.js` | 🔜 roadmap |
| **46** | **MEMORY sync** | this file | ✅ Core 4.0 Great Purge + TV Floating UI logged (июль 2026) |
| **47** | **LeftBars DynamicFractal vs Williams** | `dynamic_fractal.go` | 🟡 shadow validation |
| **54** | **CameraState SSOT** — камера всё ещё LWC-derived at `capture`; Shot 7 ужесточил контракты (`restore` scalpel, atomic F1, `_cameraGesturing`). Полный SSOT (`CameraState` → Adapter → LWC, никогда наоборот) — **только если** TF/edge регрессии вернутся. | `viewport-manager.js`, `chart-compositor.js`, `chart-core.js` | 🟡 deferred; сейчас дешевле, чем после роста bypass-путей |
| **55** | ~~**PersistenceQueue (P0b)**~~ — closed bar → async batch UPSERT SQLite; не sync из Analyst | `data/persistence_queue.go`, `data/history_db.go`, `main.go` | ✅ Shot 9C |
| **56** | ~~**SQLiteNeedsCatchUp (P0c)**~~ — SQLite tip self-heal independent of RAM gap-fill; 9E: FetchClosedRange → PersistenceQueue | `data/sqlite_tip.go`, `strategy/sqlite_catchup.go`, `main.go` | ✅ Shot 9D+9E |
| **57** | ~~**HistoryProvider / Sync Gap delivery**~~ | `server/history_provider.go` | ✅ Shot 9A GetWindow = SQLite∪RAM |
| **58** | ~~**Atomic chart tick (hollow PriceBar)**~~ | `server/webserver.go`, `main.go`, `strategy/master.go` | ✅ Shot 9B BroadcastChartTick all TFs; Falcon/Score telemetry frozen |
| **59** | ~~**Ingestion sterilization (P0)**~~ — FetchClosedRange sterile pipe; FetchHistoricalKlines+synthesize deleted; PersistenceQueue sole runtime writer | `exchange/klines.go`, `strategy/kline_gap.go`, `strategy/ram_history.go`, `data/persistence_queue.go` | ✅ Shot 9E |
| **60** | ~~**EngineMode ChartOnly/Live (P0)**~~ — gate Falcon/Score/Run; DAG stays; hot-path log silence | `strategy/engine_mode.go`, `strategy/layer2.go`, `main.go`, `data/history_db.go`, `server/` | ✅ Shot 9F |
| **61** | ~~**Navigator DAG + Export purge (P0)**~~ — `/api/state` navigators from DAG; kill ExportChartSeries/chartExportPoints; lightweight MarketState | `strategy/dag_navigator_series.go`, `server/webserver.go`, `server/chart_cache.go` | ✅ Shot 9H |
| **62** | ~~**Columnar annotations still Falcon**~~ — Projector packs `SlotDivState` → wire markers; `legacyChartAnnotationsFromKlines` deleted; legacy JSON history also DAG-only | `server/wire/annotation.go`, `projector.go`, `columnar_history.go`, `chart_cache.go` | ✅ Shot 9I |
| **63** | ~~**Non-columnar `/api/history` JSON Falcon**~~ — `buildHistoryChartSeriesTrimmed` now OHLC + DAG oscillators/annotations (no StreamingReplay) | `server/chart_cache.go` | ✅ Shot 9I |
| **64** | **Navigators full ReplayDAGKlines each request** — CPU on `navigators=1` | `dag_navigator_series.go` | 🟡 later: live HistoryBus tail |
| **65** | ~~**Toolbar/header MarketState Falcon**~~ — DAG tip/plots; Falcon line fields stripped (Core 4.0 Stage 5); no per-tick legend HTML metrics | `server/webserver.go`, `legend-renderer.js`, `toolbar-controller.js` | ✅ 9J + Great Purge Stage 5. Residual: optional `ToolbarController.updateHeaderData` hook |
| **66** | **HTFProvider / signalAnalyst alloc in ChartOnly** — idle objects | `main.go` | 🟢 optional skip |
| **67** | **IIR Tip SSOT** — RSX spike on History/Live boundary. Warmup-depth hypothesis (400 vs 3000) **disproved** (`continuity_test.go`: bit-identical). Real root cause found + fixed in 4.8: `Frame.UpdateKlineTick` double-committed each closed bar into DAG (Restore→Update→Save ×2) | `market/frame.go` (`lastCommittedOpenTime`), `market/streaming.go` / `snapshot.go` | 🟡 fix landed Core 4.8 — **pending live confirm** (re-open if spike persists) |
| **68** | **Osc fixed scale bounds** — RSX/Wozduh manifests lack TV-like `[-5,105]` / `autoscaleInfoProvider`; scale depends on data extremes | `ui_config/rsx_layout.go`, `wozduh_layout.go`, DDR RenderOpts | 🟡 after #67 |
| **69** | **MemoryBudget / WindowPolicy** — ColumnarStore can grow unbounded on left-scroll; paint window 15k separate. **Deferred** — prune-right island interfered with microscope; revisit after #67 | `columnar-store.js`, `config.js` | ⏸ deferred |
| **70** | **ScaleController only binds price** — osc slave `right` scales not in `applyAll`; rely on setData + sentinel. Optional: register rsx/wozduh or re-arm after DDR hydrate | `scale-controller.js`, `chart-core.js`, `chart-compositor.js` | 🟢 residual |
| **71** | **FrameDiagnostics** — one snapshot per publish (epoch, barCount, autoScale, cameraMode, forming) for camera/scale regressions | `boot.js` / compositor | 🟢 cheap insurance |
| **72** | ~~**TF camera accordion / Live Edge wipe**~~ | `viewport-manager.js`, `timeframe-controller.js` | ✅ 11D–11D-VIEWPORT; **TF mechanics CLOSED** |
| **73** | ~~**Y-axis squash / sentinel live tick**~~ | `series-factory.js`, `chart-core.js`, `scale-controller.js` | ✅ 11D-HOTFIX |
| **74** | ~~**Wozduh Falcon→DAG bus**~~ — slots + WozduhNode + golden parity; Falcon Evaluate gated; tip via plots | `core/slots.go`, `core/nodes/wozduh.go`, `ui_config/wozduh_layout.go`, `strategy/falcon.go` (keep) | ✅ Core 4.0 Stages 1–5 |
| **75** | ~~**TV Floating UI**~~ — fixed-up menus, eye hide `.lwc-host`, drag, outside close | `web/ui/floating-menu.js`, `legend-renderer.js`, `settings-renderer.js`, `boot.js`, `style.css` | ✅ Core 4.0 TV UI |
| **76** | **Stage 6 ScoreNodes** — migrate Score/Falcon decision graph into DAG nodes; **do not delete** `market/falcon.go` until then | `core/nodes/`, `market/falcon.go`, `decision/score_types.go` | 🔜 after #67 |
| **77** | ~~**WS dirty routing / TF case-sensitivity**~~ — server sent all TFs to all clients (`clientTF` unused); `.toLowerCase()` on both sides merged `1m`↔`1M` | `server/webserver.go` (`RouteChartTick`/`routeTick`), `web/boot.js`, `web/ws.js` | ✅ Core 4.4 audit + fix |
| **78** | ~~**Frontend chronology gap / no reconnect reconciliation**~~ — `appendTick` glued any forward time jump as one new bar (visual hole); `initLiveWebSocket` had no `onClose`/`onReconnect` | `web/columnar-store.js`, `web/boot.js` | ✅ Core 4.5 Self-Healing |
| **79** | **Self-Healing camera jump** — `loadDashboard()` triggered by gap-heal/reconnect always repaints `viewport:'fresh'`, resets user zoom/scroll. `viewportAnchor` restore path exists but not wired to the self-heal call | `web/boot.js` (`pushLiveTickDelta`, `initLiveWebSocket.onReconnect`) | 🟢 UX polish, not correctness |
| **80** | **`ViewportManager.restore` same 0×0 width risk as #81** — uses `setVisibleLogicalRange` for TF-anchor restore; only `_commitFreshCamera` (cold boot) was hardened in Core 4.10 | `web/ui/viewport-manager.js` | 🟡 same class as fixed bug; not reproduced yet |
| **81** | ~~**Cold boot camera NaN-collapse**~~ — `setVisibleLogicalRange`/`fitContent` divide by DOM container width; 0×0 on cold boot collapsed LWC scale until manual Auto Scale | `web/chart-compositor.js` (`_commitFreshCamera`) | ✅ Core 4.10 |
| **82** | **`prependMonolith` times not normalized via `chartTime`** — asymmetry: `replaceMonolith`/`appendTick` normalize sec/ms (Core 4.5), left-scroll prepend path does not (latent, not triggered — server always sends seconds) | `web/columnar-store.js` (`prependMonolith`) | 🟢 latent parity gap |

### [🔜 OPEN DEBTS — приоритет]

| # | Долг | Файлы | Статус |
|---|------|-------|--------|
| **0** | ~~**Live indicator poisoning (Go)**~~ | `layer2.go`, `indicators/` | ✅ 5.57–5.58 |
| 1 | ~~**Navigator MTF math**~~ | `trendline_navigator.go` | ✅ 5.54 |
| 2 | **Navigator background zones** | `web/trendline_plugin.js` | 🟡 |
| 3 | ~~**Backtest viewport**~~ | `web/ui/viewport-manager.js` | ✅ 20.4C (supersedes `viewport.js`) |
| 4 | **Backend 1M** — UI `1M`→`1w` | `server/`, `main.go` | 🟢 |
| **5** | **Microscope fetch** — `shouldLoad` no longer gated by tick-mute (boot never had it; legacy row removed). Residual: boot `endTimeSec=now`; center-anchored fetch if window miss still optional. | `web/boot.js`, `viewport-manager.js`, `app.legacy.js` | 🟡 mute bug ✅; fetch-around-center optional |
| 6 | **Forward lazy load** | `web/boot.js` | 🟡 scroll prepend only |
| 7 | **SQLite clear в Cache** | `server/webserver.go` | 🟡 |
| 8 | **Qdrant in main** | `main.go`, `vector_db/` | 🔜 |
| 9 | **expose `masterState`** | `server/webserver.go` | 🔜 |
| **10** | ~~**Factors UI panel**~~ | `web/app.js`, `index.html` | ✅ 5.62 → 5.75 compact line |
| **11** | ~~**MTF walk-forward на live**~~ | `master.go`, `mtf_tracker.go` | ✅ 5.68–5.70 |
| **12** | **MTF scoring tune / weight calibration** | `scoring.go`, `matrix.json`, Stats A/B | 🔜 |
| **13** | ~~**Backtest longScore/shortScore per bar**~~ | `backtest.go` | ✅ 5.63 |
| **14** | ~~**RSX/Wozduh HTF в scoring**~~ | `mtf_tracker.go`, `scoring.go` | ✅ 5.69 + 5.72 |
| **15** | ~~**WS full Wozduh / mergeOsc**~~ — tip via DAG plots (Great Purge Stage 5); residual FE header hook optional | `webserver.go`, `web/boot.js` | ✅ bus path; 🟡 optional toolbar hook |
| **16** | ~~**Slippage model**~~ | `backtest.go` | ✅ 5.64 (0.03% default) |
| **17** | **max_drawdown enforcement** | `risk.go`, `master.go` | 🔜 |
| **18** | **fixed_pct stop в UI** | `computePositionStop` | 🔜 |
| **19** | **mergeKlines SSOT** | `kline_merge.go` vs `webserver.go` dup | 🟡 |
| **20** | ~~**Flaky tests**~~ | `rsx_settings_test.go` | ✅ 5.71 fixed |
| **21** | **Stats trade history persistence** | `domain/trade_history.go` | 🔜 SQLite |
| **22** | **Live Trade Inspector** | `web/boot.js` | 🔜 scoring history buffer |
| **23** | **Backtest stop: await partial JSON** | legacy path | 🟡 |
| **24** | **Stats live initial balance sync** | `domain/trade_history.go` | 🟡 uses DefaultSessionCapital |
| **25** | ~~**Frontend monolith / live viewport hacks**~~ | `web/app.js`, `viewport.js` | ✅ Phase 19.5–20 |
| **26** | ~~**Live volume exponential growth**~~ | `web/time-normalizer.js` | ✅ 20.4B LWW |
| **27** | ~~**Chain-reaction history load**~~ | `app.js`, `viewport-manager.js` | ✅ 20.4F guards + no animate |
| **28** | ~~**Backtest black screen (0×0 tab nesting)**~~ | `web/index.html`, `chart-projection.js` | ✅ Phase 28 — HTML `</div>` + Data/View split |
| **29** | **Backtest history bypasses Projection** | backtest path | 🟡 прямой prepend — асимметрия с live Atomic |
| **30** | **trySync: re-mark dirty on paint fail** | `web/ui/chart-projection.js` | 🟡 |
| **31** | **Debug agent logs cleanup** | various | 🟡 `#region agent log` |
| **32** | **Overlay navigators in intent** | `backtest-pipeline.js` | 🟡 |
| **33** | **`coversRange` not wired to `needsBaseReload`** | `backtest-pipeline.js`, `store.js` | 🟡 |
| **34** | **`runBacktest(autoSwitchTab)` dead param** | legacy | 🟢 |
| **35** | **DAG → TradeManager wiring (PAUSED)** — ChartOnly default; re-enable only with `ENGINE_MODE=live` | `market/runtime.go`, `core/runner.go` | ⏸ shadow DAG; Live gated |
| **36** | **TradeIntent wire contract** | `decision/score_types.go`, WS/API | ⏸ |
| **37** | **Execution gate `isClosed` only** | `market/runtime.go` | ⏸ TickLiveCh frozen in ChartOnly |
| **38** | **Risk/settings SSOT parity** | UI matrix vs DAG weights | ⏸ purged matrix in Phase F — revisit with new strategies |
| **39** | **~~ChiefAnalyst bus~~** | — | ❌ Phase F purge; do not revive name Analyst |

---

## Справочник: ключевые файлы

| Область | Пути |
|---------|------|
| Decision contracts | `decision/score_types.go`, `decision/doc.go` |
| Frame / streaming | `market/frame.go`, `streaming.go`, `snapshot.go`, `volatility.go`, `engine_mode.go` |
| Runtime / Boot | `market/runtime.go`, `boot_controller.go`, `ram_history.go`, `live_kline.go` |
| MTF | `market/mtf_tracker.go`, `navigator_defaults.go`, `trendline_navigator.go`, `exchange/htf_provider.go` |
| Execution / Sizing | `execution/` |
| Live / Modes | `market/runtime.go`, `main.go`, `config/config.go` (`ENGINE_MODE`) |
| Ingestion / Archive | `exchange/klines.go` (`FetchClosedRange`), `data/persistence_queue.go`, `data/sqlite_tip.go`, `data/history_db.go`, `market/sqlite_catchup.go`, `market/kline_gap.go` |
| History delivery | `server/history_provider.go`, `server/columnar_history.go`, `server/wire/` (`annotation.go`, `projector.go`) |
| Navigators (DAG) | `market/dag_navigator_series.go`, `market/dag_shadow.go` |
| Annotations (DAG→UI) | `core/nodes/divergence.go`, `server/wire/annotation.go`, `ui_config/rsx_layout.go` (`ann_rsx_div`) |
| Backtest (chart replay) | `market/backtest.go`, `market/backtest_loader.go` |
| Stats / Trades | `domain/trade_history.go`, `server/backtest_run.go`, `server/stats_test.go` |
| API / WS | `server/webserver.go`, `server/chart_cache.go` (legacy JSON history only) |
| Core 2.0 DAG | `core/runner.go`, `core/nodes/`, `core/history.go`, `core/manifest.go`, `market/dag_shadow.go`, `server/wire/history.go`, `ui_config/` |
| Falcon (keep #76) | `market/falcon.go` |
| Strategy placeholder | `strategy/doc.go` only |
| DDR Frontend | `web/series-factory.js`, `web/hydration-orchestrator.js`, `web/chart-compositor.js`, `web/render-scheduler.js` |
| Live klines / replay | `market/live_kline.go`, `market/rsx_pipeline.go`, `market/streaming_replay.go`, `market/streaming_replay_accum.go` |
| Frontend core | `web/boot.js` (epoch + Atomic + tick buffer + Self-Healing reconnect), `web/columnar-store.js` (SSOT store + gap-detect `_intervalSec`), `web/chart-core.js`, `web/render-scheduler.js`, `web/series-factory.js`, `web/time-normalizer.js`, `web/mappers.js`, `web/api.js`, `web/ws.js` (case-sensitive TF gate) |
| Frontend legacy | `web/app.legacy.js`, `web/adapter.legacy.js` (quarantined) |
| Config | `.env` / `ENGINE_MODE`, `TRADING_SYMBOL`, `TRADING_TIMEFRAME`, Binance keys |
| Docs | `MEMORY.md` (this file), `.cursor/rules/jeweler-protocol.mdc` |
| Frontend UI | `web/ui/viewport-manager.js` (11D camera CLOSED), `web/ui/scale-controller.js`, `web/ui/timeframe-controller.js`, `web/ui/toolbar-controller.js`, `web/ui/floating-menu.js`, `web/ui/legend-renderer.js`, `web/ui/settings-renderer.js`, … |
| Frontend style | `web/style.css`, `web/trade_marker_plugin.js`, `web/trendline_plugin.js` |
| Wozduh DAG | `core/nodes/wozduh.go`, `core/slots.go` (`SlotWozduh*`), `ui_config/wozduh_layout.go` |
| Правила AI | `.cursor/rules/senior-quant-architect.mdc`, `jeweler-protocol.mdc` |

**Env:** `TRADING_SYMBOL`, `TRADING_TIMEFRAME`, `READ_ONLY`, `SANDBOX_MODE`, **`ENGINE_MODE`** (`ChartOnly` default | `live`).

**Запуск:** `go run .` — dashboard `:8080`, WS Binance futures, **ChartOnly** delivery by default. `ENGINE_MODE=live` включает ScoreMatrix + `Master.Run`. `make ab-test` — CLI A/B backtest.

**Следующий шаг (Core 5.0 Phases A–G ✅):** **#76 ScoreNodes** + **#67 Live Confirm** (RSX tip-spike после double-commit fix 4.8 — если spike остаётся, re-open #67). Затем #68 osc fixed bounds, #69 MemoryBudget, #80 `ViewportManager.restore` 0×0 risk. Trading — только `ENGINE_MODE=live` (после новых стратегий в `decision/`).
