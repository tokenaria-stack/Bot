.PHONY: ab-test backtest-cli

# A/B scoring matrix backtest (Baseline LTF vs HTF Regime)
ab-test backtest-cli:
	go run ./cmd/backtest

# Same with REST gap-fill when SQLite is sparse (requires .env API keys)
ab-test-rest:
	go run ./cmd/backtest -rest
