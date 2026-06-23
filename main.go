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
	"trading_bot/execution"
	"trading_bot/exchange"
	"trading_bot/server"
	"trading_bot/strategy"
	// "trading_bot/vector_db" // Раскомментируем, когда будем подключать Qdrant
)

const historyKlinesLimit = 1000

const (
	Symbol = "BTCUSDT"
)

func main() {
	log.Println("=== Trading Bot Initialization ===")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	analysts := make(map[string]*strategy.ChiefAnalyst)
	timeframes := []string{"1m", "3m", "15m", "1h", "4h"}

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("[Init] Failed to load config: %v", err)
	}

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

		candles, err := restClient.GetKlines(Symbol, tf, historyKlinesLimit)
		if err != nil {
			log.Printf("[Init] Analyst [%s] failed to load history: %v", tf, err)
		} else {
			history = candlesToKlines(candles)
			log.Printf("[Init] Analyst [%s] loaded %d historical klines", tf, len(history))
		}

		analysts[tf] = strategy.NewChiefAnalyst(
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

	rm := execution.NewRiskManager(1.0, 10)
	entryRisk := strategy.NewRiskManager(strategy.DefaultScalpFeeRate, nil, cfg.SandboxMode)
	master := strategy.NewMasterGeneral(
		analysts, entryRisk, rm, restClient, nil,
		cfg.ReadOnly, cfg.SandboxMode, strategy.DefaultHuntTimeframe,
	)

	dashboard := server.NewDashboardServer(analysts, restClient, Symbol, orderFlow, entryRisk, cfg.ReadOnly, cfg.SandboxMode)
	master.SetOnTick(func(k exchange.Kline, jurik, red, green, blue float64) {
		longScore := 0
		shortScore := 0
		brainStatus := "Analyzing..."
		aiStatus := "Offline"

		rsxColor := strategy.RSXColorNeutral
		rsxSignal := 0.0
		if analyst := analysts["1m"]; analyst != nil {
			rsxColor = analyst.JurikRSXColor()
			if report, err := analyst.GenerateMarketReport(); err == nil {
				rsxSignal = report.RSXSignal
				telemetry := strategy.EvaluateScalpSignal(context.Background(), *report, strategy.DefaultScalpFeeRate, nil)
				longScore = telemetry.LongScore
				shortScore = telemetry.ShortScore
				brainStatus = strategy.TelemetryBrainStatus(telemetry, *report, entryRisk)
				aiStatus = strategy.TelemetryAIStatus(context.Background(), *report, nil)
			}
		}

		dashboard.BroadcastTick(
			domain.CandleFromKline(k),
			jurik, rsxSignal, red, green, blue,
			rsxColor,
			longScore, shortScore,
			brainStatus, aiStatus,
		)
	})
	master.SetOnKlineBar(func(tf string, k exchange.Kline) {
		if tf == "1m" {
			return
		}
		dashboard.BroadcastPriceBar(tf, domain.CandleFromKline(k))
	})
	master.SetOnTrade(func(event strategy.TradeEvent) {
		dashboard.BroadcastMarker(event.Side, event.Price, event.BarTime, event.Reason, event.Kind)
	})

	master.RecoverState()

	wsClient := exchange.NewWsClient(Symbol, orderFlow)

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
		klines[i] = exchange.Kline{
			OpenTime: c.OpenTime,
			Open:     c.Open,
			High:     c.High,
			Low:      c.Low,
			Close:    c.Close,
			Volume:   c.Volume,
		}
	}
	return klines
}
