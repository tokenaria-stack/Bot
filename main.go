package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"trading_bot/config"
	"trading_bot/data"
	"trading_bot/domain"
	"trading_bot/exchange"
	"trading_bot/server"
	"trading_bot/strategy"
	// "trading_bot/vector_db" // Раскомментируем, когда будем подключать Qdrant
)

const historyKlinesLimit = 1000

func main() {
	log.Println("=== Trading Bot Initialization ===")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	analysts := make(map[string]*strategy.Marker)
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
	strategy.SetSandboxMode(cfg.SandboxMode)

	loadedMatrix, err := strategy.LoadMatrixConfig(strategy.MatrixConfigPath)
	if err != nil {
		log.Printf("[Init] Failed to load scoring matrix from %s: %v — using defaults", strategy.MatrixConfigPath, err)
		loadedMatrix = strategy.DefaultScoringMatrix()
	}
	strategy.SetScoringMatrix(loadedMatrix)
	log.Printf("[Init] Loaded Scoring Matrix configuration from %s", strategy.MatrixConfigPath)

	log.Println("[Init] Order Flow buffers ready (100k ticks, 1k liquidations)")

	chaosCfg := strategy.ChaosConfig{
		AOFastPeriod: 5,
		AOSlowPeriod: 34,
	}

	bootStart := time.Now()
	log.Printf("[Init] Parallel analyst boot: %d timeframes (%d CPUs, SQLite WAL)", len(timeframes), runtime.NumCPU())
	var bootWG sync.WaitGroup
	var bootMu sync.Mutex
	for _, tf := range timeframes {
		bootWG.Add(1)
		go func(tf string) {
			defer bootWG.Done()
			tfStart := time.Now()

			history := strategy.LoadRAMHistory(restClient, symbol, tf, strategy.AnalystBootKlineLimit)
			source := "sqlite"
			if len(history) == 0 {
				candles, err := restClient.GetKlines(symbol, tf, historyKlinesLimit)
				if err != nil {
					log.Printf("[Init] Analyst [%s] failed to load history: %v", tf, err)
				} else {
					history = candlesToKlines(candles)
				}
				source = "rest"
			}

			m := strategy.NewMarker(history, nil, tf, "btc_patterns", chaosCfg)
			m.UpdateIndicators()

			bootMu.Lock()
			analysts[tf] = m
			bootMu.Unlock()

			elapsed := time.Since(tfStart)
			log.Printf("[Init] Analyst [%s] loaded %d klines from %s (%.2fs)", tf, len(history), source, elapsed.Seconds())
			// #region agent log
			agentBootLog("main.go:boot", "tf boot complete", "boot", map[string]any{
				"tf": tf, "source": source, "klines": len(history), "ms": elapsed.Milliseconds(),
			})
			// #endregion
		}(tf)
	}
	bootWG.Wait()
	bootElapsed := time.Since(bootStart)
	log.Printf("[Init] All analysts ready in %.2fs (parallel)", bootElapsed.Seconds())
	// #region agent log
	agentBootLog("main.go:boot", "parallel boot complete", "boot", map[string]any{
		"totalMs": bootElapsed.Milliseconds(), "cpus": runtime.NumCPU(), "tfs": len(timeframes),
	})
	// #endregion

	orderFlow := domain.NewOrderFlowStore()
	var _ exchange.OrderFlowSink = orderFlow

	signalAnalyst := strategy.NewAnalyst(cfg.SandboxMode)
	htfProvider := exchange.NewHTFProvider()
	master := strategy.NewMasterGeneral(
		analysts, signalAnalyst, restClient, htfProvider, nil,
		cfg.ReadOnly, cfg.SandboxMode, symbol, tradingTF,
	)

	dashboard := server.NewDashboardServer(
		analysts, restClient, symbol, orderFlow, signalAnalyst, htfProvider,
		cfg.ReadOnly, cfg.SandboxMode, tradingTF,
	)
	dashboard.BindMaster(master)
	master.SetNavigatorPanes(strategy.DefaultLiveNavigatorPanes())
	master.StartKlineGapFillLoop(ctx)
	master.SeedClosedBarTelemetry()
	master.SetOnTelemetry(func(tick exchange.WsTick, falcon strategy.FalconSignals, decision strategy.ScoreDecision) {
		analyst := analysts[tradingTF]
		if analyst == nil {
			return
		}
		rsxColor := analyst.JurikRSXColor()
		brainStatus := strategy.TelemetryBrainStatus(decision, signalAnalyst)
		aiStatus := strategy.TelemetryAIStatus(context.Background(), analyst, nil)
		regime := master.ClosedVolatilityRegimeForTelemetry(analyst)

		dashboard.BroadcastTick(
			tradingTF,
			domain.CandleFromKline(tick.Kline),
			falcon,
			rsxColor,
			decision,
			brainStatus, aiStatus,
			regime,
			tick.IsClosed,
			analyst.DAGTickFrame(),
		)
	})
	master.SetOnKlineBar(func(tf string, k exchange.Kline) {
		if tf == tradingTF {
			return
		}
		dashboard.BroadcastPriceBar(tf, domain.CandleFromKline(k))
	})
	master.SetOnTrade(func(event strategy.TradeEvent) {
		dashboard.BroadcastMarker(event.Side, event.Price, event.BarTime, event.Reason, event.Kind)
	})
	master.SetOnClosedTrade(func(trade domain.ClosedTrade, isVirtual bool) {
		dashboard.RecordClosedTrade(trade, isVirtual)
	})

	master.RecoverState()

	wsClient := exchange.NewWsClient(symbol, orderFlow)

	master.StartDataFeed(ctx, wsClient.OutCh)

	go func() {
		log.Println("[Main] Starting Dashboard server on :8080...")
		dashboard.StartMicroBroadcast(ctx)
		if err := dashboard.Start(":8080"); err != nil {
			log.Printf("[Main] Dashboard server stopped: %v", err)
		}
	}()

	go func() {
		log.Println("[Main] Starting WebSocket connection...")
		if err := wsClient.Start(ctx); err != nil {
			log.Fatalf("[Main] Failed to start WS: %v", err)
		}
	}()

	go func() {
		log.Println("[Main] Starting MasterGeneral Event Loop...")
		master.Run(ctx)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("=== Shutting down bot... ===")
	cancel()
	time.Sleep(1 * time.Second)
	log.Println("=== Bot gracefully stopped ===")
}

const agentDebugLogPath = "/Users/ilmaru/trading_bot/.cursor/debug-39f875.log"

func agentBootLog(location, message, hypothesisID string, data map[string]any) {
	payload := map[string]any{
		"sessionId":    "39f875",
		"timestamp":    time.Now().UnixMilli(),
		"location":     location,
		"message":      message,
		"hypothesisId": hypothesisID,
		"data":         data,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return
	}
	f, err := os.OpenFile(agentDebugLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	_, _ = f.Write(append(b, '\n'))
	_ = f.Close()
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
