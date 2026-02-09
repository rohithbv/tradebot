# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
# Build
go build -o tradebot ./cmd/tradebot

# Run (requires Alpaca API credentials)
APCA_API_KEY_ID=xxx APCA_API_SECRET_KEY=xxx ./tradebot -config config.yaml

# Run all tests
go test ./...

# Run a single package's tests
go test ./internal/indicator/

# Run a specific test
go test ./internal/broker/ -run TestExecuteBuy
```

No Makefile, no linter configured, no CGO required. Uses `modernc.org/sqlite` (pure-Go).

## Architecture

Paper trading bot that polls Alpaca for market data, evaluates RSI+MACD signals, executes simulated trades, and serves a web dashboard.

**Data flow per tick:** `Engine.tick()` fetches bars/prices via `MarketClient` -> passes closes to `Strategy.Evaluate()` -> sends signals to `PaperBroker.ExecuteSignal()` -> broker persists trades to `Store` and updates in-memory positions.

**Key interfaces:**
- `strategy.Strategy` — `Evaluate(symbol, closes) Analysis` — only implementation is `RSIMACDStrategy`
- `storage.Store` — persistence interface implemented by `SQLiteStore`
- `web.EngineReader` — read-only engine access for the web layer (decouples web from engine)

**Concurrency model:** Engine writes, web server reads. `PaperBroker` uses `sync.RWMutex`. Engine's `lastAnalyses`/`lastPollTime` are also guarded by `sync.RWMutex`. All public getters return deep copies.

**Config:** YAML file (`config.yaml`) with env var overrides for secrets (`APCA_API_KEY_ID`, `APCA_API_SECRET_KEY`). Defaults are applied for all fields; only the API credentials are required.

**Web server:** Static files are embedded via `//go:embed static` in `internal/web/server.go`. Dashboard is a single `index.html`. API routes are all under `/api/`.

**Broker state persistence:** `PaperBroker` restores cash/positions from SQLite on startup via `Store.LoadState()`. State is saved every 5th tick and on shutdown.
