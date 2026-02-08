# Tradebot Implementation Plan

## Context

Build a Go-based virtual stock trading bot from scratch. The bot monitors S&P 500 stocks via the Alpaca API, evaluates RSI + MACD signals on 1-minute bars, executes simulated paper trades against a $10,000 virtual portfolio, persists state in SQLite, and exposes a web dashboard for monitoring.

---

## Project Structure

```
tradebot/
├── cmd/tradebot/main.go            # Entry point, wiring, graceful shutdown
├── internal/
│   ├── config/config.go            # YAML config loading, env var overrides, defaults
│   ├── market/
│   │   ├── client.go               # Alpaca client wrapper (bars, latest prices)
│   │   └── clock.go                # Market hours checking (open/closed, next open)
│   ├── indicator/
│   │   ├── rsi.go                  # RSI calculation (Wilder's smoothing)
│   │   └── macd.go                 # MACD calculation (fast/slow EMA, signal line)
│   ├── strategy/
│   │   ├── strategy.go             # Strategy interface + Signal type
│   │   └── rsimacd.go              # RSI+MACD combined strategy
│   ├── engine/engine.go            # Main trading loop (poll -> evaluate -> execute)
│   ├── broker/paper.go             # Virtual broker (portfolio, position sizing, trade sim)
│   ├── storage/sqlite.go            # SQLite persistence (trades, snapshots, bars, state)
│   └── web/
│       ├── server.go               # HTTP server setup + routes
│       ├── handlers.go             # JSON API handlers
│       └── static/index.html       # Dashboard (vanilla JS + Chart.js from CDN)
├── config.yaml                     # Default configuration
├── go.mod
└── go.sum
```

---

## Key Dependencies

| Package | Purpose |
|---|---|
| `github.com/alpacahq/alpaca-trade-api-go/v3` | Market data + clock API |
| `modernc.org/sqlite` | Pure-Go SQLite driver (no CGO) |
| `gopkg.in/yaml.v3` | Config file parsing |
| `github.com/google/uuid` | Trade IDs |
| `log/slog` (stdlib) | Structured logging |
| `embed` (stdlib) | Embed dashboard HTML |

---

## Core Interfaces & Types

### Strategy Interface

```go
type Signal int // Hold, Buy, Sell

type Analysis struct {
    Symbol, Reason        string
    Signal                Signal
    RSI, MACD, MACDSignal float64
    Timestamp             time.Time
}

type Strategy interface {
    Evaluate(symbol string, closes []float64) Analysis
}
```

### Storage Interface

```go
type Store interface {
    SaveTrade(trade Trade) error
    GetTrades(since time.Time, limit int) ([]Trade, error)
    SaveSnapshot(snap PortfolioSnapshot) error
    GetSnapshots(since time.Time) ([]PortfolioSnapshot, error)
    SaveBars(symbol string, bars []Bar) error
    GetBars(symbol string, since time.Time) ([]Bar, error)
    SaveState(state BrokerState) error
    LoadState() (*BrokerState, error)
    Close() error
}
```

---

## Data Flow

```
┌─────────────┐    1-min bars    ┌────────────┐   closes[]   ┌──────────┐
│  Alpaca API  │ ──────────────> │   Engine    │ ──────────> │ Strategy │
│  (market/)   │                 │  (engine/)  │ <────────── │ RSI+MACD │
└─────────────┘                  └─────┬───────┘  Analysis    └──────────┘
                                       │
                              Buy/Sell signals
                                       │
                                       ▼
                                ┌──────────────┐  trades   ┌──────────┐
                                │ Paper Broker │ ────────> │  SQLite  │
                                │  (broker/)   │ <──────── │ storage/ │
                                └──────┬───────┘  state    └──────────┘
                                       │
                                  reads state
                                       │
                                       ▼
                                ┌──────────────┐
                                │ Web Dashboard│
                                │   (web/)     │
                                └──────────────┘
```

### Engine Loop (during market hours, every 60s)

1. Fetch 50 minutes of 1-min bars for all 20 watchlist symbols via `GetMultiBars` (single API call)
2. For each symbol: extract closes, call `strategy.Evaluate(symbol, closes)`
3. For Buy/Sell signals: call `broker.ExecuteSignal(analysis, currentPrice)`
4. Every 5th tick: save portfolio snapshot to SQLite for P&L history
5. On `ctx.Done()`: stop cleanly

### RSI+MACD Signal Logic

- **BUY**: RSI < 30 (oversold) AND MACD line crosses above signal line
- **SELL**: RSI > 70 (overbought) AND MACD line crosses below signal line
- **HOLD**: All other cases
- Crossover = previous bar MACD was below signal, current bar MACD is at/above (or vice versa for bearish)

### Position Sizing

- Max 10% of portfolio value per position (configurable)
- Max 5 concurrent positions (configurable)
- `qty = floor(maxSpend / currentPrice)`, skip if qty < 1

---

## Web Dashboard

### API Endpoints

| Endpoint | Returns |
|---|---|
| `GET /` | Embedded `index.html` dashboard |
| `GET /api/portfolio` | Cash, positions, total value, unrealized P&L |
| `GET /api/portfolio/history` | Snapshots over time (for P&L chart) |
| `GET /api/trades` | Trade history with reasons |
| `GET /api/watchlist` | Symbols with latest price, RSI, MACD, signal |
| `GET /api/status` | Bot running state, market open/closed, last poll time |

### Dashboard UI

Single HTML file, CSS grid with 4 panels:
- **Portfolio Summary** — cash, total value, P&L
- **P&L Chart** — line chart via Chart.js (CDN)
- **Trade History** — table of recent trades with side, qty, price, reason
- **Watchlist** — symbols with RSI, MACD, current signal, held status

JS polls all API endpoints every 10 seconds. Green/red coloring for P&L values.

---

## Configuration (`config.yaml`)

```yaml
alpaca:
  api_key: ""            # or env APCA_API_KEY_ID
  api_secret: ""         # or env APCA_API_SECRET_KEY
  base_url: "https://paper-api.alpaca.markets"
  feed: "iex"

trading:
  initial_cash: 10000.00
  poll_interval_sec: 60
  max_position_pct: 0.10
  max_positions: 5
  watchlist: [AAPL, MSFT, GOOGL, AMZN, NVDA, META, TSLA, BRK.B, JPM, V,
              UNH, JNJ, XOM, PG, MA, HD, CVX, MRK, ABBV, PEP]

strategy:
  rsi_period: 14
  rsi_oversold: 30.0
  rsi_overbought: 70.0
  macd_fast_period: 12
  macd_slow_period: 26
  macd_signal_period: 9

storage:
  db_path: "tradebot.db"

web:
  addr: ":8080"
```

API keys: env vars take precedence over config file values.

---

## SQLite Schema

```sql
CREATE TABLE trades (
    id         TEXT PRIMARY KEY,
    symbol     TEXT NOT NULL,
    side       TEXT NOT NULL,       -- "buy" or "sell"
    qty        INTEGER NOT NULL,
    price      REAL NOT NULL,
    total      REAL NOT NULL,
    reason     TEXT NOT NULL,
    timestamp  DATETIME NOT NULL
);
CREATE INDEX idx_trades_timestamp ON trades(timestamp);

CREATE TABLE snapshots (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp   DATETIME NOT NULL,
    cash        REAL NOT NULL,
    total_value REAL NOT NULL,
    positions   TEXT NOT NULL        -- JSON-encoded map[string]PositionSnapshot
);
CREATE INDEX idx_snapshots_timestamp ON snapshots(timestamp);

CREATE TABLE bars (
    symbol    TEXT NOT NULL,
    timestamp DATETIME NOT NULL,
    open      REAL NOT NULL,
    high      REAL NOT NULL,
    low       REAL NOT NULL,
    close     REAL NOT NULL,
    volume    INTEGER NOT NULL,
    PRIMARY KEY (symbol, timestamp)
);

CREATE TABLE state (
    key   TEXT PRIMARY KEY,          -- "portfolio"
    value TEXT NOT NULL               -- JSON-encoded BrokerState
);
```

SQLite provides proper SQL queries with indexing, making range queries (trades since X, snapshots between dates) natural. Use `database/sql` with `modernc.org/sqlite` driver (pure Go, no CGO required). Enable WAL mode for concurrent read/write performance.

---

## Concurrency Model

- **Engine goroutine**: writes trades, updates broker state, saves snapshots
- **Web server goroutine**: reads broker state, reads from SQLite
- **Broker**: protected by `sync.RWMutex` (engine writes, web reads)
- **SQLite**: WAL mode enabled for concurrent reads during writes; `database/sql` connection pool handles concurrency
- **Shutdown**: `signal.NotifyContext` for SIGINT/SIGTERM, context propagated to engine + web server

---

## Implementation Order (Parallelizable Tasks)

The dependency graph below shows which tasks can run in parallel. Tasks at the same
level are independent and can be implemented by separate subagents simultaneously.

```
                    ┌──────────────────┐
                    │  Task 1: Scaffold │  (go.mod, config, shared types)
                    └────────┬─────────┘
                             │
            ┌────────────────┼────────────────┐
            ▼                ▼                ▼
   ┌─────────────┐  ┌──────────────┐  ┌──────────────┐
   │ Task 2:     │  │ Task 3:      │  │ Task 4:      │
   │ Indicators  │  │ SQLite Store │  │ Market Client│
   │ + Strategy  │  │              │  │              │
   └──────┬──────┘  └──────┬───────┘  └──────┬───────┘
          │                │                  │
          └────────────────┼──────────────────┘
                           ▼
                  ┌─────────────────┐
                  │ Task 5: Broker  │
                  └────────┬────────┘
                           │
               ┌───────────┴───────────┐
               ▼                       ▼
      ┌─────────────────┐    ┌─────────────────┐
      │ Task 6: Engine  │    │ Task 7: Web     │
      │                 │    │ Dashboard       │
      └────────┬────────┘    └────────┬────────┘
               │                      │
               └──────────┬───────────┘
                          ▼
                 ┌─────────────────┐
                 │ Task 8: Main    │
                 │ + Integration   │
                 └─────────────────┘
```

---

### Task 1: Project Scaffold (must run first)

**Files**: `go.mod`, `config.yaml`, `internal/config/config.go`

- Initialize Go module: `go mod init github.com/rohithbv/tradebot`
- Add all dependencies: `modernc.org/sqlite`, `gopkg.in/yaml.v3`, `github.com/alpacahq/alpaca-trade-api-go/v3`, `github.com/google/uuid`
- Implement `internal/config/config.go`:
  - Full `Config` struct (see Configuration section above)
  - `Load(path string) (*Config, error)` — YAML parsing, env var overrides (`APCA_API_KEY_ID`, `APCA_API_SECRET_KEY`), defaults, validation
- Write default `config.yaml`
- Define shared types used across packages in a `internal/model/model.go` file:
  - `Bar`, `Trade`, `Position`, `PortfolioSnapshot`, `BrokerState`
  - `Signal` enum (Hold/Buy/Sell), `Analysis` struct
  - This avoids circular imports between packages

**Output**: Compiles with `go build ./...`, config loads correctly

---

### Task 2: Indicators + Strategy (parallel with Tasks 3 & 4)

**Depends on**: Task 1 (needs `internal/model` types)
**Files**: `internal/indicator/rsi.go`, `internal/indicator/macd.go`, `internal/strategy/strategy.go`, `internal/strategy/rsimacd.go`, plus `_test.go` files

- `indicator/rsi.go`: `CalcRSI(closes []float64, period int) (RSIResult, error)`
  - Wilder's smoothing method
  - Return error if len(closes) < period+1
- `indicator/macd.go`: `CalcMACD(closes []float64, fast, slow, signal int) (MACDResult, error)`
  - EMA calculation for fast/slow lines, signal line = EMA of MACD line
  - Return full series of MACDPoints for crossover detection
- `strategy/strategy.go`: `Strategy` interface with `Evaluate(symbol string, closes []float64) model.Analysis`
- `strategy/rsimacd.go`: `RSIMACDStrategy` implementing the interface
  - BUY: RSI < 30 AND MACD bullish crossover (prev MACD < Signal, curr MACD >= Signal)
  - SELL: RSI > 70 AND MACD bearish crossover
  - HOLD: all other cases
  - Return `Hold` gracefully if insufficient data
- Unit tests for RSI/MACD with known reference values
- Unit tests for strategy with synthetic price sequences

**Output**: `go test ./internal/indicator/... ./internal/strategy/...` passes

---

### Task 3: SQLite Storage (parallel with Tasks 2 & 4)

**Depends on**: Task 1 (needs `internal/model` types)
**Files**: `internal/storage/sqlite.go`, `internal/storage/sqlite_test.go`

- `NewSQLiteStore(dbPath string) (*SQLiteStore, error)`:
  - Open DB with `modernc.org/sqlite` driver via `database/sql`
  - Enable WAL mode: `PRAGMA journal_mode=WAL`
  - Run schema creation (CREATE TABLE IF NOT EXISTS for all 4 tables)
- Implement `Store` interface methods:
  - `SaveTrade(trade model.Trade) error` — INSERT into trades
  - `GetTrades(since time.Time, limit int) ([]model.Trade, error)` — SELECT with WHERE/ORDER/LIMIT
  - `SaveSnapshot(snap model.PortfolioSnapshot) error` — INSERT (JSON-encode positions)
  - `GetSnapshots(since time.Time) ([]model.PortfolioSnapshot, error)` — SELECT range
  - `SaveBars(symbol string, bars []model.Bar) error` — batch INSERT OR REPLACE
  - `GetBars(symbol string, since time.Time) ([]model.Bar, error)` — SELECT by symbol + time range
  - `SaveState(state model.BrokerState) error` — UPSERT into state table
  - `LoadState() (*model.BrokerState, error)` — SELECT, return nil if not found
  - `Close() error`
- Unit tests using temp directory + cleanup

**Output**: `go test ./internal/storage/...` passes

---

### Task 4: Market Client (parallel with Tasks 2 & 3)

**Depends on**: Task 1 (needs config types, `internal/model` types)
**Files**: `internal/market/client.go`, `internal/market/clock.go`

- `client.go`:
  - `NewMarketClient(cfg config.AlpacaConfig) *MarketClient`
  - `GetBars(symbols []string, lookback time.Duration) (map[string][]model.Bar, error)`:
    - Wraps `marketdata.Client.GetMultiBars`
    - Converts Alpaca `marketdata.Bar` → `model.Bar`
  - `GetLatestPrices(symbols []string) (map[string]float64, error)`:
    - Wraps `marketdata.Client.GetLatestBars`, extracts close prices
- `clock.go`:
  - `IsMarketOpen() (open bool, nextChange time.Time, err error)`:
    - Wraps `alpaca.Client.GetClock()`
    - Returns whether market is currently open and when it next opens/closes

**Output**: Compiles, functions callable (actual API testing requires keys)

---

### Task 5: Paper Broker (after Tasks 1-4)

**Depends on**: Tasks 1, 3 (needs model types + storage interface)
**Files**: `internal/broker/paper.go`, `internal/broker/paper_test.go`

- `NewPaperBroker(initialCash float64, store storage.Store, cfg config.TradingConfig) *PaperBroker`
  - On creation, attempt `store.LoadState()` to resume from previous run
  - If no saved state, start with `initialCash` and empty positions
- `ExecuteSignal(analysis model.Analysis, currentPrice float64) (*model.Trade, error)`:
  - For BUY: check not already held, check max positions, calculate qty via position sizing, deduct cash, create position, save trade
  - For SELL: check position exists, calculate proceeds, add to cash, remove position, save trade
  - For HOLD: no-op
- `UpdatePrices(prices map[string]float64)`: update current prices on all positions, recompute P&L
- `GetState() model.BrokerState`: return current state (thread-safe read via `sync.RWMutex`)
- `SaveState() error`: persist to storage for restart recovery
- `sync.RWMutex` protecting all state mutations
- Unit tests with a mock store or in-memory SQLite

**Output**: `go test ./internal/broker/...` passes

---

### Task 6: Trading Engine (after Task 5)

**Depends on**: Tasks 2, 3, 4, 5 (needs strategy, storage, market client, broker)
**Files**: `internal/engine/engine.go`

- `New(market *market.MarketClient, broker *broker.PaperBroker, strat strategy.Strategy, store storage.Store, cfg *config.Config) *Engine`
- `Run(ctx context.Context) error`:
  1. Check `market.IsMarketOpen()` — if closed, log next open, sleep (with context select)
  2. Start `time.Ticker` at `poll_interval_sec`
  3. On each tick:
     - Fetch bars: `market.GetBars(watchlist, 50*time.Minute)`
     - For each symbol: extract closes, call `strategy.Evaluate(symbol, closes)`
     - For Buy/Sell signals: `broker.ExecuteSignal(analysis, latestPrice)`
     - Call `broker.UpdatePrices(latestPrices)` for P&L
     - Log all analyses at debug, trades at info level
  4. Every 5th tick: `store.SaveSnapshot(...)` + `broker.SaveState()`
  5. On `ctx.Done()`: save final state, return
- Expose `GetLastAnalyses() map[string]model.Analysis` for the web dashboard's watchlist endpoint
- Use `log/slog` structured logging

**Output**: Compiles, engine loop logic correct (full test requires API keys + market hours)

---

### Task 7: Web Dashboard (after Task 5, parallel with Task 6)

**Depends on**: Tasks 3, 5 (needs storage + broker for reading state)
**Files**: `internal/web/server.go`, `internal/web/handlers.go`, `internal/web/static/index.html`

- `server.go`:
  - `NewServer(cfg config.WebConfig, broker *broker.PaperBroker, store storage.Store) *Server`
  - Uses `embed.FS` for static files
  - `Start(ctx context.Context) error`: listens on `cfg.Addr`, graceful shutdown on context cancel
- `handlers.go` — 6 endpoints:
  - `GET /` → serve embedded `index.html`
  - `GET /api/portfolio` → `broker.GetState()` → JSON
  - `GET /api/portfolio/history` → `store.GetSnapshots(since)` → JSON
  - `GET /api/trades` → `store.GetTrades(since, limit)` → JSON
  - `GET /api/watchlist` → reads latest analyses from engine (via a shared reference) → JSON
  - `GET /api/status` → bot status, market state, last poll time → JSON
- `static/index.html` — single-file dashboard:
  - Chart.js from CDN for P&L line chart
  - CSS grid: Portfolio Summary | P&L Chart | Trade History | Watchlist
  - Vanilla JS polls all `/api/*` endpoints every 10s
  - Green/red P&L coloring
  - Keep under 300 lines total

**Output**: Dashboard loads at `localhost:8080`, API endpoints return valid JSON

---

### Task 8: Main Entry Point + Integration (after Tasks 6 & 7)

**Depends on**: All previous tasks
**Files**: `cmd/tradebot/main.go`

- Parse `-config` flag (default: `config.yaml`)
- Load config via `config.Load(path)`
- Set up `log/slog` with JSON handler
- Create all components in order: market client → store → broker → strategy → engine → web server
- Wire engine's analyses into web server for `/api/watchlist` and `/api/status`
- `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)`
- Start engine in goroutine, web server in goroutine
- `<-ctx.Done()` → shutdown web server with timeout → engine stops via context → close store
- Verify end-to-end: build, run, check logs + dashboard

**Output**: `go build ./cmd/tradebot` succeeds, bot runs end-to-end

---

## Edge Cases to Handle

- **Insufficient data at market open**: Return `Hold` if fewer bars than needed for slowest indicator (35+ for MACD)
- **Market closed**: Sleep until next open time from `GetClock()`, re-check every minute
- **Already holding a symbol**: Skip duplicate buy signals
- **No position to sell**: Skip sell signals for symbols not in portfolio
- **API rate limits**: Single `GetMultiBars` call per tick (1/min) is well within Alpaca's 200/min free tier
- **Restart recovery**: Load `BrokerState` from SQLite `state` table on startup

---

## Verification

1. **Unit tests**: Run `go test ./internal/indicator/...` — verify RSI and MACD match known reference values
2. **Unit tests**: Run `go test ./internal/strategy/...` — verify buy/sell/hold signals for crafted price series
3. **Unit tests**: Run `go test ./internal/storage/...` — verify SQLite read/write round-trips
4. **Build**: Run `go build ./cmd/tradebot` — verify clean compilation
5. **Run**: Set Alpaca API keys, run `./tradebot`, verify:
   - Logs show market clock check on startup
   - During market hours: bars fetched, indicators computed, signals logged
   - Trades appear in SQLite and via `/api/trades`
   - Dashboard at `http://localhost:8080` loads and displays data
   - Ctrl+C triggers graceful shutdown, state is persisted
6. **Off-hours test**: Run outside market hours, verify bot sleeps and logs next open time
