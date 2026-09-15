package exchange

import (
	"archive/zip"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const visionFuturesKlineZip = "https://data.binance.vision/data/futures/um/monthly/klines"
const visionSpotKlineZip = "https://data.binance.vision/data/spot/monthly/klines"

// FetchVisionMonthKlines downloads one Vision monthly zip and returns candles by open_time.
// Sterile: does not write SQLite. 404 returns (nil, nil).
func FetchVisionMonthKlines(symbol, interval string, year, month int, spot bool) (map[int64]Candle, error) {
	if month < 1 || month > 12 || year < 2017 {
		return nil, fmt.Errorf("vision month")
	}
	symbol = NormalizeFuturesSymbol(symbol)
	label := fmt.Sprintf("%04d-%02d", year, month)
	base := visionFuturesKlineZip
	if spot {
		base = visionSpotKlineZip
	}
	url := fmt.Sprintf("%s/%s/%s/%s-%s-%s.zip", base, symbol, interval, symbol, interval, label)
	tmp, err := os.MkdirTemp("", "vision-vol-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	zipPath := filepath.Join(tmp, "k.zip")
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vision %s: HTTP %d", url, resp.StatusCode)
	}
	f, err := os.Create(zipPath)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	csvPath, err := extractVisionCSV(zipPath, tmp)
	if err != nil {
		return nil, err
	}
	return parseVisionCSVFile(csvPath)
}

// FetchVisionMonthAuthorities is FetchVisionMonthKlines plus quote/trades from CSV.
func FetchVisionMonthAuthorities(symbol, interval string, year, month int, spot bool) (map[int64]VolumeAuthority, error) {
	if month < 1 || month > 12 || year < 2017 {
		return nil, fmt.Errorf("vision month")
	}
	symbol = NormalizeFuturesSymbol(symbol)
	label := fmt.Sprintf("%04d-%02d", year, month)
	base := visionFuturesKlineZip
	if spot {
		base = visionSpotKlineZip
	}
	url := fmt.Sprintf("%s/%s/%s/%s-%s-%s.zip", base, symbol, interval, symbol, interval, label)
	tmp, err := os.MkdirTemp("", "vision-vol-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	zipPath := filepath.Join(tmp, "k.zip")
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vision %s: HTTP %d", url, resp.StatusCode)
	}
	f, err := os.Create(zipPath)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	csvPath, err := extractVisionCSV(zipPath, tmp)
	if err != nil {
		return nil, err
	}
	return parseVisionAuthorityFile(csvPath)
}

func parseVisionAuthorityFile(path string) (map[int64]VolumeAuthority, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rd := csv.NewReader(f)
	rd.FieldsPerRecord = -1
	rd.ReuseRecord = true
	out := map[int64]VolumeAuthority{}
	first := true
	for {
		rec, err := rd.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(rec) < 7 {
			continue
		}
		if first {
			first = false
			if _, err := strconv.ParseInt(strings.TrimSpace(rec[0]), 10, 64); err != nil {
				continue
			}
		}
		a, err := VolumeAuthorityFromVisionCSV(rec)
		if err != nil {
			return nil, err
		}
		out[a.OpenTime] = a
	}
	return out, nil
}

func extractVisionCSV(zipPath, destDir string) (string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer r.Close()
	var zf *zip.File
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(f.Name), ".csv") {
			zf = f
			break
		}
	}
	if zf == nil {
		return "", fmt.Errorf("no csv in vision zip")
	}
	rc, err := zf.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()
	outPath := filepath.Join(destDir, filepath.Base(zf.Name))
	out, err := os.Create(outPath)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		return "", err
	}
	return outPath, out.Close()
}

func parseVisionCSVFile(path string) (map[int64]Candle, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rd := csv.NewReader(f)
	rd.FieldsPerRecord = -1
	rd.ReuseRecord = true
	out := map[int64]Candle{}
	first := true
	for {
		rec, err := rd.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(rec) < 7 {
			continue
		}
		if first {
			first = false
			if _, err := strconv.ParseInt(strings.TrimSpace(rec[0]), 10, 64); err != nil {
				continue
			}
		}
		c, err := CandleFromVisionCSV(rec)
		if err != nil {
			return nil, err
		}
		out[c.OpenTime] = c
	}
	return out, nil
}
