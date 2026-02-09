# tradebot

A virtual stock trading bot written in Go. Uses the Alpaca API for market data, evaluates RSI + MACD signals, executes paper trades, persists state in SQLite, and serves a web dashboard.

## Prerequisites

- Go 1.24+
- [Alpaca](https://alpaca.markets/) paper trading account (free)

## Setup

1. Copy the example env file and add your Alpaca credentials:

```bash
cp env.example .env
# Edit .env with your APCA_API_KEY_ID and APCA_API_SECRET_KEY
```

2. Build and run:

```bash
go build -o tradebot ./cmd/tradebot
source .env && ./tradebot
```

The bot loads configuration from `config.yaml` by default. Use `-config path/to/config.yaml` to specify a different file.

## Configuration

All settings are in `config.yaml`. API credentials can also be set via environment variables (`APCA_API_KEY_ID`, `APCA_API_SECRET_KEY`), which override the config file values.

| Section    | Key                | Default  | Description                           |
|------------|--------------------|----------|---------------------------------------|
| `alpaca`   | `base_url`         | paper-api.alpaca.markets | Alpaca API base URL      |
| `alpaca`   | `feed`             | `iex`    | Market data feed (`iex` or `sip`)     |
| `trading`  | `initial_cash`     | 10000    | Starting paper trading balance        |
| `trading`  | `poll_interval_sec`| 60       | Seconds between trading ticks         |
| `trading`  | `max_position_pct` | 0.10     | Max portfolio % per position          |
| `trading`  | `max_positions`    | 5        | Max concurrent positions              |
| `trading`  | `watchlist`        | 20 stocks| Symbols to monitor                    |
| `strategy` | `rsi_period`       | 14       | RSI lookback period                   |
| `strategy` | `rsi_oversold`     | 30       | RSI buy threshold                     |
| `strategy` | `rsi_overbought`   | 70       | RSI sell threshold                    |
| `storage`  | `db_path`          | tradebot.db | SQLite database file               |
| `web`      | `addr`             | :8080    | Dashboard listen address              |

## Web Dashboard

Once running, open `http://localhost:8080` to view the dashboard. The following API endpoints are available:

- `GET /api/portfolio` - Current cash, positions, and total value
- `GET /api/portfolio/history?since=RFC3339` - Portfolio value over time
- `GET /api/trades?since=RFC3339&limit=N` - Recent trade history
- `GET /api/watchlist` - Watchlist with latest RSI/MACD signals
- `GET /api/status` - Engine running state and uptime

## How It Works

The trading engine runs a loop that:

1. Checks if the market is open via the Alpaca trading clock
2. Fetches 1-minute OHLCV bars and latest prices for all watchlist symbols
3. Computes RSI (Wilder's smoothing) and MACD (12/26/9 EMA) indicators
4. Generates buy/sell/hold signals based on RSI oversold/overbought levels and MACD crossovers
5. Executes paper trades through the broker, respecting position size and count limits
6. Persists trades immediately; saves portfolio snapshots every 5th tick

Broker state (cash + positions) is persisted to SQLite and restored on restart, so the bot picks up where it left off.

## Project Structure

```
cmd/tradebot/       Entry point
internal/
  config/           YAML config loading with env var overrides
  model/            Shared types (Trade, Position, Bar, Analysis, etc.)
  indicator/        RSI and MACD calculations
  strategy/         Strategy interface + RSI+MACD implementation
  storage/          SQLite Store interface + implementation (WAL mode)
  market/           Alpaca API client wrapper
  broker/           Paper broker (thread-safe, persists state)
  engine/           Trading loop orchestration
  web/              HTTP server + embedded dashboard
```

## Testing

```bash
go test ./...
```

## License

See [LICENSE](LICENSE).
