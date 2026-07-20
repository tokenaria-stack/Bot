package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"trading_bot/config"
	"trading_bot/data"
	"trading_bot/domain"
	"trading_bot/exchange"
	"trading_bot/market"
	"trading_bot/server"
)

const historyKlinesLimit = 1000

func main() {
	log.Println("=== Trading Bot Initialization ===")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	frames := make(map[string]*market.Frame)
	timeframes := []string{"1m", "3m", "5m", "15m", "30m", "1h", "4h", "1d", "1w", "1M"}

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("[Init] Failed to load config: %v", err)
	}

	tradingTF := cfg.Timeframe
	if _, err := server.ResolveTimeframe(tradingTF); err != nil {
		log.Fatalf("[Init] Invalid TRADING_TIMEFRAME %q: %v", tradingTF, err)
	}
	symbol := exchange.NormalizeFuturesSymbol(cfg.Symbol)
	log.Printf("[Init] Trading pair %s @ %s", symbol, tradingTF)

	if err := data.InitDB(); err != nil {
		log.Fatalf("[Init] Failed to init history DB: %v", err)
	}
	log.Println("[Init] SQLite history cache ready (history.db)")

	restClient, err := exchange.NewBinanceExchange(cfg.BinanceAPIKey, cfg.BinanceSecretKey, false)
	if err != nil {
		log.Fatalf("[Init] Failed to create REST client: %v", err)
	}
	if err := restClient.Ping(); err != nil {
		log.Printf("[Init] WARNING: Binance ping failed (non-fatal): %v", err)
	}
	if !restClient.UsesFuturesClient() {
		log.Fatal("[Init] REST client is not a Binance USD-M Futures client")
	}
	if restClient.IsTestnet() {
		log.Fatal("[Init] Refusing to start: futures testnet is enabled (expected mainnet)")
	}
	log.Printf("[Init] Binance USD-M Futures Mainnet — REST: %s", restClient.RESTKlinesEndpoint())
	log.Printf("[Init] Binance USD-M Futures Mainnet — WS:  %s", exchange.FuturesWSCombinedURL())
	if cfg.ReadOnly {
		log.Println("[Init] Read-only mode: no API keys — public klines/websocket only (trading disabled)")
	}

	engineMode := market.NormalizeEngineMode(cfg.EngineMode)
	market.SetEngineMode(engineMode)
	log.Printf("[Init] EngineMode=%s (ENGINE_MODE)", engineMode)
	if !market.EngineAllowsStrategies() {
		log.Println("[Init] ChartOnly: strategy stack gated (ScoreEngine socket empty until Phase G+)")
	}

	// ── Boot FSM Phase 0: Connecting ──────────────────────────────────────────
	// WS goes up FIRST. Live ticks buffer inside BootController while history
	// loads — REST recovery can never again be "the truth" over missed WS bars.
	// Order Flow sink is nil (debt #44): aggTrade ring returns with TickBarBuilder.
	wsClient := exchange.NewWsClient(symbol, nil)
	boot := market.NewBootController(wsClient.OutCh)
	boot.Begin(ctx)
	go func() {
		if err := wsClient.Start(ctx); err != nil {
			log.Fatalf("[Main] Failed to start WS: %v", err)
		}
	}()

	// ── Boot FSM Phase 1: Loading ─────────────────────────────────────────────
	boot.MarkLoading()
	chaosCfg := market.ChaosConfig{
		AOFastPeriod: 5,
		AOSlowPeriod: 34,
	}

	bootStart := time.Now()
	log.Printf("[Init] Parallel Frame boot: %d timeframes (%d CPUs, SQLite WAL)", len(timeframes), runtime.NumCPU())
	var bootWG sync.WaitGroup
	var bootMu sync.Mutex
	for _, tf := range timeframes {
		bootWG.Add(1)
		go func(tf string) {
			defer bootWG.Done()
			tfStart := time.Now()

			history := market.LoadRAMHistory(restClient, symbol, tf, market.FrameBootKlineLimit)
			source := "sqlite"
			if len(history) == 0 {
				candles, err := restClient.GetKlines(symbol, tf, historyKlinesLimit)
				if err != nil {
					log.Printf("[Init] Frame [%s] failed to load history: %v", tf, err)
				} else {
					history = candlesToKlines(candles)
				}
				source = "rest"
			}

			m := market.NewFrame(history, tf, chaosCfg)
			m.UpdateIndicators()

			bootMu.Lock()
			frames[tf] = m
			bootMu.Unlock()

			log.Printf("[Init] Frame [%s] loaded %d klines from %s (%.2fs)",
				tf, len(history), source, time.Since(tfStart).Seconds())
		}(tf)
	}
	bootWG.Wait()
	log.Printf("[Init] All frames ready in %.2fs (parallel)", time.Since(bootStart).Seconds())

	htfProvider := exchange.NewHTFProvider()
	master := market.NewRuntime(
		frames, restClient, htfProvider,
		cfg.ReadOnly, cfg.SandboxMode, symbol, tradingTF,
	)

	dashboard := server.NewDashboardServer(
		frames, restClient, symbol, htfProvider,
		cfg.ReadOnly, cfg.SandboxMode, tradingTF,
	)
	dashboard.BindMaster(master)
	master.SetNavigatorPanes(market.DefaultLiveNavigatorPanes())

	// Timeline publish gate sockets (Phase C): healing/publishable → browser WS.
	// midSession gate: ignore transport signals until GoLive + StartDataFeed.
	var timelineMidSession atomic.Bool
	master.SetOnTimelineHealing(dashboard.BroadcastTimelineHealing)
	master.SetOnTimelinePublishable(dashboard.BroadcastTimelinePublishable)
	wsClient.SetOnDisconnect(func() {
		if !timelineMidSession.Load() {
			return
		}
		master.OnBinanceDisconnect()
	})
	wsClient.SetOnReconnect(func() {
		if !timelineMidSession.Load() {
			return
		}
		master.OnBinanceReconnect(ctx)
	})

	// Shot 9C/9E: sole SQLite writer — bind before gap-fill / tip catch-up.
	persistQ := data.NewPersistenceQueue(4096)
	persistQ.Start(ctx)
	master.SetPersistenceQueue(persistQ)

	// Shot 9B + Core 4.2: every TF computes Projection; Transport routes only to subscribed clients.
	master.SetOnKlineBar(func(tf string, k exchange.Kline, isClosed bool) {
		frame := frames[tf]
		if frame == nil {
			return
		}
		dashboard.RouteChartTick(
			tf,
			domain.CandleFromKline(k),
			isClosed,
			frame.DAGTickFrame(),
		)
		if isClosed {
			persistQ.Enqueue(symbol, tf, data.Candle{
				OpenTime:  k.OpenTime,
				Open:      k.Open,
				High:      k.High,
				Low:       k.Low,
				Close:     k.Close,
				Volume:    k.Volume,
				CloseTime: k.CloseTime,
			})
		}
	})

	// ── Boot FSM Phase 2: Reconciling ─────────────────────────────────────────
	// Buffered WS ticks replay through the canonical routing path; live bars
	// finalize on top of REST history (WS never loses to a REST snapshot).
	boot.Reconcile(master)

	// ── Boot FSM Phase 3: Live ────────────────────────────────────────────────
	boot.GoLive()
	master.StartDataFeed(ctx, wsClient.OutCh)
	master.StartKlineGapFillLoop(ctx)
	// Shot 9D/9E: SQLite tip self-heals via FetchClosedRange → PersistenceQueue (no Frame touch).
	master.StartSQLiteArchiveCatchUpLoop(ctx)
	master.SeedClosedBarTelemetry()
	// Arm mid-session timeline hooks only after live feed is up (first Dial already done).
	timelineMidSession.Store(true)

	go func() {
		log.Println("[Main] Starting Dashboard server on :8080...")
		if err := dashboard.Start(":8080"); err != nil {
			log.Printf("[Main] Dashboard server stopped: %v", err)
		}
	}()

	if market.EngineAllowsStrategies() {
		go func() {
			log.Println("[Main] Starting Runtime Event Loop (Live)...")
			master.Run(ctx)
		}()
	} else {
		log.Println("[Main] ChartOnly: Runtime.Run not started")
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("=== Shutting down bot... ===")
	cancel()
	time.Sleep(1 * time.Second)
	log.Println("=== Bot gracefully stopped ===")
}

func candlesToKlines(candles []exchange.Candle) []exchange.Kline {
	klines := make([]exchange.Kline, len(candles))
	for i, c := range candles {
		klines[i] = exchange.NormalizeKline(exchange.Kline{
			OpenTime:  c.OpenTime,
			CloseTime: c.CloseTime,
			Open:      c.Open,
			High:      c.High,
			Low:       c.Low,
			Close:     c.Close,
			Volume:    c.Volume,
		})
	}
	return klines
}
