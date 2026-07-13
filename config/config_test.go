package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"trading_bot/config"
)

func TestLoadConfig_readsBinanceCredentials(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := "BINANCE_API_KEY=test_key\nBINANCE_SECRET_KEY=test_secret\n"
	if err := os.WriteFile(envPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write test .env: %v", err)
	}

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(origWD); err != nil {
			t.Errorf("restore wd: %v", err)
		}
	})

	os.Unsetenv("BINANCE_API_KEY")
	os.Unsetenv("BINANCE_SECRET_KEY")
	os.Unsetenv("BINANCE_TEST_API_KEY")
	os.Unsetenv("BINANCE_TEST_SECRET_KEY")
	os.Unsetenv("ENGINE_MODE")

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.BinanceAPIKey != "test_key" {
		t.Errorf("BinanceAPIKey = %q, want %q", cfg.BinanceAPIKey, "test_key")
	}
	if cfg.BinanceSecretKey != "test_secret" {
		t.Errorf("BinanceSecretKey = %q, want %q", cfg.BinanceSecretKey, "test_secret")
	}
	if cfg.ReadOnly {
		t.Fatal("ReadOnly = true, want false when keys are set")
	}
	if cfg.Symbol != "BTCUSDT" {
		t.Errorf("Symbol = %q, want BTCUSDT default", cfg.Symbol)
	}
	if cfg.Timeframe != "1m" {
		t.Errorf("Timeframe = %q, want 1m default", cfg.Timeframe)
	}
	if cfg.EngineMode != "ChartOnly" {
		t.Errorf("EngineMode = %q, want ChartOnly default", cfg.EngineMode)
	}
}

func TestLoadConfig_allowsEmptyKeysForReadOnly(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("# read-only\n"), 0o600); err != nil {
		t.Fatalf("write test .env: %v", err)
	}

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(origWD); err != nil {
			t.Errorf("restore wd: %v", err)
		}
	})

	os.Unsetenv("BINANCE_API_KEY")
	os.Unsetenv("BINANCE_SECRET_KEY")
	os.Unsetenv("BINANCE_TEST_API_KEY")
	os.Unsetenv("BINANCE_TEST_SECRET_KEY")

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !cfg.ReadOnly {
		t.Fatal("ReadOnly = false, want true for empty keys")
	}
}

func TestLoadConfig_sandboxModeFromEnv(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := "HYPER_SCALP_TEST=true\n"
	if err := os.WriteFile(envPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write test .env: %v", err)
	}

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origWD)
		os.Unsetenv("HYPER_SCALP_TEST")
		os.Unsetenv("SANDBOX_MODE")
	})

	os.Unsetenv("BINANCE_API_KEY")
	os.Unsetenv("BINANCE_SECRET_KEY")
	os.Unsetenv("BINANCE_TEST_API_KEY")
	os.Unsetenv("BINANCE_TEST_SECRET_KEY")

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !cfg.SandboxMode {
		t.Fatal("SandboxMode = false, want true when HYPER_SCALP_TEST=true")
	}
}

func TestLoadConfig_rejectsPartialKeys(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("BINANCE_API_KEY=only_key\n"), 0o600); err != nil {
		t.Fatalf("write test .env: %v", err)
	}

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(origWD); err != nil {
			t.Errorf("restore wd: %v", err)
		}
	})

	os.Unsetenv("BINANCE_API_KEY")
	os.Unsetenv("BINANCE_SECRET_KEY")
	os.Unsetenv("BINANCE_TEST_API_KEY")
	os.Unsetenv("BINANCE_TEST_SECRET_KEY")

	_, err = config.LoadConfig()
	if err == nil {
		t.Fatal("LoadConfig: expected error for partial keys, got nil")
	}
}

func TestLoadConfig_engineModeLiveFromEnv(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("ENGINE_MODE=live\n"), 0o600); err != nil {
		t.Fatalf("write test .env: %v", err)
	}

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origWD)
		os.Unsetenv("ENGINE_MODE")
	})

	os.Unsetenv("BINANCE_API_KEY")
	os.Unsetenv("BINANCE_SECRET_KEY")
	os.Unsetenv("BINANCE_TEST_API_KEY")
	os.Unsetenv("BINANCE_TEST_SECRET_KEY")

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.EngineMode != "live" {
		t.Fatalf("EngineMode = %q, want live from env", cfg.EngineMode)
	}
}
