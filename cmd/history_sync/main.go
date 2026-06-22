// history_sync downloads Binance Vision monthly kline archives and bulk-imports them into SQLite.
//
// Usage:
//
//	go run ./cmd/history_sync -symbol=BTCUSDT -interval=15m -year=2023
//	go run ./cmd/history_sync -market=spot -symbol=ETHUSDT -interval=1h -year=2024 -month=6
package main

import (
	"archive/zip"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"trading_bot/data"
)

const (
	visionFuturesBaseURL = "https://data.binance.vision/data/futures/um/monthly/klines"
	visionSpotBaseURL    = "https://data.binance.vision/data/spot/monthly/klines"
	batchSize            = 45000
	httpTimeout          = 5 * time.Minute
)

func main() {
	symbol := flag.String("symbol", "BTCUSDT", "Trading pair, e.g. BTCUSDT")
	interval := flag.String("interval", "15m", "Kline interval, e.g. 15m, 1h, 1d")
	year := flag.Int("year", 0, "Calendar year to import (required)")
	month := flag.Int("month", 0, "Month 1-12 (optional; default all months)")
	market := flag.String("market", "futures", "Market type: futures (USDT-M) or spot")
	dbPath := flag.String("db", "history.db", "SQLite database file path")
	flag.Parse()

	if *year <= 0 {
		log.Fatal("flag -year is required (e.g. -year=2023)")
	}
	if *month != 0 && (*month < 1 || *month > 12) {
		log.Fatal("flag -month must be between 1 and 12")
	}

	mkt := strings.ToLower(strings.TrimSpace(*market))
	if mkt != "futures" && mkt != "spot" {
		log.Fatal("flag -market must be futures or spot")
	}

	sym := strings.ToUpper(strings.TrimSpace(*symbol))
	iv := strings.TrimSpace(*interval)

	data.SetDBPath(*dbPath)
	if err := data.InitDB(); err != nil {
		log.Fatalf("init database: %v", err)
	}

	months := monthsToSync(*month)
	var failed int
	for _, m := range months {
		if err := syncMonth(sym, iv, mkt, *year, m); err != nil {
			log.Printf("[ERROR] %s %s %04d-%02d: %v", sym, iv, *year, m, err)
			failed++
			continue
		}
	}

	if failed > 0 {
		os.Exit(1)
	}
}

func monthsToSync(month int) []int {
	if month > 0 {
		return []int{month}
	}
	out := make([]int, 12)
	for i := range out {
		out[i] = i + 1
	}
	return out
}

func visionBaseURL(market string) string {
	if market == "spot" {
		return visionSpotBaseURL
	}
	return visionFuturesBaseURL
}

func syncMonth(symbol, interval, market string, year, month int) error {
	label := fmt.Sprintf("%04d-%02d", year, month)
	base := visionBaseURL(market)
	url := fmt.Sprintf("%s/%s/%s/%s-%s-%s.zip", base, symbol, interval, symbol, interval, label)

	tmpDir, err := os.MkdirTemp("", "history_sync_*")
	if err != nil {
		return fmt.Errorf("temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	zipPath := filepath.Join(tmpDir, fmt.Sprintf("%s-%s-%s.zip", symbol, interval, label))
	if err := downloadFile(url, zipPath); err != nil {
		return err
	}

	csvPath, err := extractCSV(zipPath, tmpDir)
	if err != nil {
		return err
	}

	count, err := importCSV(symbol, interval, csvPath)
	if err != nil {
		return err
	}

	fmt.Printf("[SUCCESS] Imported %d klines for %s %s (%s)\n", count, symbol, interval, label)
	return nil
}

func downloadFile(url, dest string) error {
	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("archive not found (HTTP 404): %s", url)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create %s: %w", dest, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write %s: %w", dest, err)
	}
	return nil
}

func extractCSV(zipPath, destDir string) (string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("open zip %s: %w", zipPath, err)
	}
	defer r.Close()

	var csvName string
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(f.Name), ".csv") {
			csvName = f.Name
			break
		}
	}
	if csvName == "" {
		return "", fmt.Errorf("no CSV file inside %s", zipPath)
	}

	var zf *zip.File
	for _, f := range r.File {
		if f.Name == csvName {
			zf = f
			break
		}
	}
	if zf == nil {
		return "", fmt.Errorf("csv entry missing in zip")
	}

	rc, err := zf.Open()
	if err != nil {
		return "", fmt.Errorf("open csv in zip: %w", err)
	}
	defer rc.Close()

	base := filepath.Base(csvName)
	outPath := filepath.Join(destDir, base)
	out, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("create csv %s: %w", outPath, err)
	}
	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		return "", fmt.Errorf("extract csv: %w", err)
	}
	if err := out.Close(); err != nil {
		return "", err
	}
	return outPath, nil
}

func importCSV(symbol, interval, csvPath string) (int, error) {
	f, err := os.Open(csvPath)
	if err != nil {
		return 0, fmt.Errorf("open csv: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1
	reader.ReuseRecord = true

	batch := make([]data.Candle, 0, batchSize)
	total := 0
	firstRow := true

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if err := data.SaveKlines(symbol, interval, batch); err != nil {
			return err
		}
		total += len(batch)
		batch = batch[:0]
		return nil
	}

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return total, fmt.Errorf("read csv row: %w", err)
		}
		if len(record) < 7 {
			continue
		}
		if firstRow {
			firstRow = false
			if _, err := strconv.ParseInt(strings.TrimSpace(record[0]), 10, 64); err != nil {
				continue // skip optional CSV header (futures archives)
			}
		}

		candle, err := parseVisionCandle(record)
		if err != nil {
			return total, err
		}
		batch = append(batch, candle)
		if len(batch) >= batchSize {
			if err := flush(); err != nil {
				return total, fmt.Errorf("batch insert: %w", err)
			}
		}
	}

	if err := flush(); err != nil {
		return total, fmt.Errorf("final batch insert: %w", err)
	}
	return total, nil
}

func parseVisionCandle(record []string) (data.Candle, error) {
	openTime, err := strconv.ParseInt(strings.TrimSpace(record[0]), 10, 64)
	if err != nil {
		return data.Candle{}, fmt.Errorf("parse open time %q: %w", record[0], err)
	}
	open, err := strconv.ParseFloat(strings.TrimSpace(record[1]), 64)
	if err != nil {
		return data.Candle{}, fmt.Errorf("parse open: %w", err)
	}
	high, err := strconv.ParseFloat(strings.TrimSpace(record[2]), 64)
	if err != nil {
		return data.Candle{}, fmt.Errorf("parse high: %w", err)
	}
	low, err := strconv.ParseFloat(strings.TrimSpace(record[3]), 64)
	if err != nil {
		return data.Candle{}, fmt.Errorf("parse low: %w", err)
	}
	closePrice, err := strconv.ParseFloat(strings.TrimSpace(record[4]), 64)
	if err != nil {
		return data.Candle{}, fmt.Errorf("parse close: %w", err)
	}
	volume, err := strconv.ParseFloat(strings.TrimSpace(record[5]), 64)
	if err != nil {
		return data.Candle{}, fmt.Errorf("parse volume: %w", err)
	}
	closeTime, err := strconv.ParseInt(strings.TrimSpace(record[6]), 10, 64)
	if err != nil {
		return data.Candle{}, fmt.Errorf("parse close time: %w", err)
	}

	return data.Candle{
		OpenTime:  openTime,
		Open:      open,
		High:      high,
		Low:       low,
		Close:     closePrice,
		Volume:    volume,
		CloseTime: closeTime,
	}, nil
}
