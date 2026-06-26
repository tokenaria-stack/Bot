package config

import (
	"errors"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds application settings loaded from the environment.
type Config struct {
	BinanceAPIKey    string
	BinanceSecretKey string
	ReadOnly         bool
	SandboxMode      bool
	Symbol           string
	Timeframe        string
}

// LoadConfig loads .env (optional) and reads Binance USD-M Futures credentials.
// Empty API keys enable read-only mode (public klines + websocket only).
func LoadConfig() (*Config, error) {
	_ = godotenv.Load()

	apiKey := firstNonEmpty(
		os.Getenv("BINANCE_API_KEY"),
		os.Getenv("BINANCE_TEST_API_KEY"),
	)
	secretKey := firstNonEmpty(
		os.Getenv("BINANCE_SECRET_KEY"),
		os.Getenv("BINANCE_TEST_SECRET_KEY"),
	)

	readOnly := apiKey == "" || secretKey == ""
	if apiKey != "" && secretKey == "" {
		return nil, errors.New("BINANCE_SECRET_KEY is not set but BINANCE_API_KEY is")
	}
	if secretKey != "" && apiKey == "" {
		return nil, errors.New("BINANCE_API_KEY is not set but BINANCE_SECRET_KEY is")
	}

	symbol := strings.TrimSpace(os.Getenv("TRADING_SYMBOL"))
	if symbol == "" {
		symbol = "BTCUSDT"
	}
	timeframe := strings.TrimSpace(os.Getenv("TRADING_TIMEFRAME"))
	if timeframe == "" {
		timeframe = "1m"
	}

	return &Config{
		BinanceAPIKey:    apiKey,
		BinanceSecretKey: secretKey,
		ReadOnly:         readOnly,
		SandboxMode:      envTruthy("HYPER_SCALP_TEST", "SANDBOX_MODE"),
		Symbol:           symbol,
		Timeframe:        timeframe,
	}, nil
}

func envTruthy(keys ...string) bool {
	for _, key := range keys {
		switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
		case "1", "true", "yes", "on":
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
