package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"trading_bot/config"
	"trading_bot/exchange"
	"trading_bot/strategy"
)

type abRow struct {
	label string
	run   *strategy.BacktestRunResult
	err   error
}

func main() {
	os.Exit(run())
}

func run() int {
	var (
		symbol   = flag.String("symbol", "BTCUSDT", "futures symbol")
		interval = flag.String("interval", "15m", "binance kline interval")
		months   = flag.Int("months", 6, "lookback months when -start/-end omitted")
		start    = flag.String("start", "", "start date YYYY-MM-DD (UTC)")
		end      = flag.String("end", "", "end date YYYY-MM-DD (UTC)")
		matrix   = flag.String("matrix", strategy.MatrixConfigPath, "scoring matrix JSON path")
		useREST  = flag.Bool("rest", false, "allow REST gap-fill when SQLite is sparse")
	)
	flag.Parse()

	baseMatrix, err := loadBaseMatrix(*matrix)
	if err != nil {
		log.Printf("[ab-test] matrix load: %v — using in-memory defaults", err)
		baseMatrix = strategy.GetScoringMatrix()
	}

	startMs, endMs, err := resolveRange(*start, *end, *months)
	if err != nil {
		log.Printf("[ab-test] date range: %v", err)
		return 1
	}

	var rest *exchange.BinanceExchange
	if *useREST {
		cfg, cfgErr := config.LoadConfig()
		if cfgErr != nil {
			log.Printf("[ab-test] config: %v", cfgErr)
			return 1
		}
		rest, err = exchange.NewBinanceExchange(cfg.BinanceAPIKey, cfg.BinanceSecretKey, false)
		if err != nil {
			log.Printf("[ab-test] rest client: %v", err)
			return 1
		}
	}

	log.Printf("[ab-test] loading %s %s [%s .. %s]",
		*symbol, *interval,
		time.UnixMilli(startMs).UTC().Format("2006-01-02"),
		time.UnixMilli(endMs).UTC().Format("2006-01-02"))

	candles, effectiveStart, err := strategy.LoadBacktestCandles(strategy.LoadBacktestCandlesOpts{
		Symbol:   *symbol,
		Interval: *interval,
		StartMs:  startMs,
		EndMs:    endMs,
		Rest:     rest,
	})
	if err != nil {
		log.Printf("[ab-test] candles: %v", err)
		return 1
	}
	if effectiveStart != startMs {
		log.Printf("[ab-test] padded start → %s (%d bars)",
			time.UnixMilli(effectiveStart).UTC().Format("2006-01-02"), len(candles))
	} else {
		log.Printf("[ab-test] loaded %d bars from SQLite/REST", len(candles))
	}

	htf := exchange.NewHTFProvider()
	analyst := strategy.NewAnalyst(false)
	specs := strategy.DefaultABRunSpecs(baseMatrix)

	rows := make([]abRow, 0, len(specs))

	for _, spec := range specs {
		matrixCopy := spec.Matrix
		cfg := strategy.BuildBacktestEngineConfig(
			*symbol, *interval,
			&matrixCopy,
			spec.Navigators,
			spec.MtfOptions,
			htf,
			analyst,
			strategy.DefaultBacktestSlippagePct,
		)
		log.Printf("[ab-test] running %s (HTF=%v WozduhCross=%v HTFOsc=%v periods=%v)",
			spec.Label,
			cfg.HTF != nil,
			cfg.Matrix != nil && cfg.Matrix.UseWozduhCross,
			cfg.Matrix != nil && cfg.Matrix.UseHTFOscillators,
			navigatorPeriods(cfg.Navigators))

		run, runErr := strategy.RunBacktestSimulation(context.Background(), cfg, candles)
		rows = append(rows, abRow{label: spec.Label, run: run, err: runErr})
		if runErr != nil {
			log.Printf("[ab-test] %s failed: %v", spec.Label, runErr)
		}
	}

	printReport(*symbol, *interval, candles, rows)
	for _, r := range rows {
		if r.err != nil {
			return 1
		}
	}
	return 0
}

func loadBaseMatrix(path string) (strategy.ScoringMatrix, error) {
	m, err := strategy.LoadMatrixConfig(path)
	if err != nil {
		return strategy.ScoringMatrix{}, err
	}
	strategy.SetScoringMatrix(m)
	return m, nil
}

func resolveRange(startDate, endDate string, months int) (int64, int64, error) {
	if startDate != "" || endDate != "" {
		if startDate == "" || endDate == "" {
			return 0, 0, fmt.Errorf("both -start and -end are required together")
		}
		return strategy.ParseBacktestDateRange(startDate, endDate)
	}
	endMs := time.Now().UTC().UnixMilli()
	if months <= 0 {
		months = 6
	}
	startMs := time.Now().UTC().AddDate(0, -months, 0).UnixMilli()
	return startMs, endMs, nil
}

func navigatorPeriods(navs map[string]strategy.NavigatorUISettings) []string {
	ui, ok := navs["price"]
	if !ok {
		return nil
	}
	return append([]string(nil), ui.Periods...)
}

func printReport(symbol, interval string, candles []exchange.Candle, rows []abRow) {
	fmt.Println()
	fmt.Println("══════════════════════════════════════════════════════════════════════════════")
	fmt.Printf("  Quant A/B Backtest — %s %s — %d bars\n", symbol, interval, len(candles))
	if len(candles) > 0 {
		fmt.Printf("  Window: %s → %s (UTC)\n",
			time.UnixMilli(candles[0].OpenTime).UTC().Format("2006-01-02"),
			time.UnixMilli(candles[len(candles)-1].OpenTime).UTC().Format("2006-01-02"))
	}
	fmt.Println("══════════════════════════════════════════════════════════════════════════════")
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "CONFIG\tTRADES\tWIN%\tNET PNL%\tMAX DD%\tPROFIT FACTOR\tSTATUS")
	_, _ = fmt.Fprintln(w, strings.Repeat("─", 72))

	for _, r := range rows {
		if r.err != nil {
			_, _ = fmt.Fprintf(w, "%s\t-\t-\t-\t-\t-\tERROR: %v\n", r.label, r.err)
			continue
		}
		if r.run == nil {
			_, _ = fmt.Fprintf(w, "%s\t-\t-\t-\t-\t-\tNO RESULT\n", r.label)
			continue
		}
		_, _ = fmt.Fprintf(w, "%s\t%d\t%.1f\t%.2f\t%.2f\t%.2f\tOK\n",
			r.label,
			r.run.TotalTrades,
			r.run.WinRate,
			r.run.NetProfit,
			r.run.MaxDrawdown,
			r.run.ProfitFactor,
		)
	}
	_ = w.Flush()
	fmt.Println()
}
