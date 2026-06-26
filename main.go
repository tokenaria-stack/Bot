package main

import (
	"context"
	"log"
	"os"
	"os/signal"
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
		log.Fatalf("[Init] Binance Mainnet ping failed: %v", err)
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
	if cfg.SandboxMode {
		log.Println("[Init] ⚠️  SANDBOX PURE STRATEGY — risk vetoes bypassed, RSX+Wozduh scoring, threshold=70")
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

	for _, tf := range timeframes {
		history := []exchange.Kline{}

		candles, err := restClient.GetKlines(symbol, tf, historyKlinesLimit)
		if err != nil {
			log.Printf("[Init] Analyst [%s] failed to load history: %v", tf, err)
		} else {
			history = candlesToKlines(candles)
			log.Printf("[Init] Analyst [%s] loaded %d historical klines", tf, len(history))
		}

		analysts[tf] = strategy.NewMarker(
			history,
			nil,
			tf,
			"btc_patterns",
			chaosCfg,
		)
		analysts[tf].UpdateIndicators()
		log.Printf("[Init] Analyst [%s] created", tf)
	}

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
	master.NotifyKlineGapFillComplete(symbol, tradingTF)
	master.SetOnTelemetry(func(tick exchange.WsTick, falcon strategy.FalconSignals, decision strategy.ScoreDecision) {
		analyst := analysts[tradingTF]
		if analyst == nil {
			return
		}
		rsxColor := analyst.JurikRSXColor()
		rsxSignal := 0.0
		if analyst.HasMinBars(strategy.BacktestMinBars()) {
			rsxSignal = analyst.RSXSignalLine()
		}
		brainStatus := strategy.TelemetryBrainStatus(decision, signalAnalyst)
		aiStatus := strategy.TelemetryAIStatus(context.Background(), analyst, nil)

		dashboard.BroadcastTick(
			tradingTF,
			domain.CandleFromKline(tick.Kline),
			falcon.JurikRSX, rsxSignal,
			falcon.RedLine, falcon.GreenLine, falcon.BlueLine,
			rsxColor,
			decision,
			brainStatus, aiStatus,
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
