package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Alpaca   AlpacaConfig  `yaml:"alpaca"`
	Trading  TradingConfig `yaml:"trading"`
	Strategy StrategyConfig `yaml:"strategy"`
	Storage  StorageConfig `yaml:"storage"`
	Web      WebConfig      `yaml:"web"`
	Telegram TelegramConfig `yaml:"telegram"`
}

type AlpacaConfig struct {
	APIKey    string `yaml:"-"`
	APISecret string `yaml:"-"`
	BaseURL   string `yaml:"base_url"`
	Feed      string `yaml:"feed"`
}

type TradingConfig struct {
	InitialCash     float64  `yaml:"initial_cash"`
	PollIntervalSec int      `yaml:"poll_interval_sec"`
	MaxPositionPct  float64  `yaml:"max_position_pct"`
	MaxPositions    int      `yaml:"max_positions"`
	Watchlist       []string `yaml:"watchlist"`
}

type StrategyConfig struct {
	Type             string  `yaml:"type"`
	RSIPeriod        int     `yaml:"rsi_period"`
	RSIOversold      float64 `yaml:"rsi_oversold"`
	RSIOverbought    float64 `yaml:"rsi_overbought"`
	MACDFastPeriod   int     `yaml:"macd_fast_period"`
	MACDSlowPeriod   int     `yaml:"macd_slow_period"`
	MACDSignalPeriod int     `yaml:"macd_signal_period"`
	EMAFastPeriod    int     `yaml:"ema_fast_period"`
	EMASlowPeriod    int     `yaml:"ema_slow_period"`
}

type StorageConfig struct {
	DBPath string `yaml:"db_path"`
}

type WebConfig struct {
	Addr string `yaml:"addr"`
}

type TelegramConfig struct {
	Enabled  bool   `yaml:"enabled"`
	BotToken string `yaml:"-"`
	ChatID   int64  `yaml:"chat_id"`
}

func Load(path string) (*Config, error) {
	cfg := &Config{}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	applyEnvOverrides(cfg)
	applyDefaults(cfg)

	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v, err := envOrFile("APCA_API_KEY_ID"); err != nil {
		slog.Warn("failed to read APCA_API_KEY_ID_FILE", "error", err)
	} else if v != "" {
		cfg.Alpaca.APIKey = v
	}
	if v, err := envOrFile("APCA_API_SECRET_KEY"); err != nil {
		slog.Warn("failed to read APCA_API_SECRET_KEY_FILE", "error", err)
	} else if v != "" {
		cfg.Alpaca.APISecret = v
	}
	if v, err := envOrFile("TELEGRAM_BOT_TOKEN"); err != nil {
		slog.Warn("failed to read TELEGRAM_BOT_TOKEN_FILE", "error", err)
	} else if v != "" {
		cfg.Telegram.BotToken = v
	}
	if v, err := envOrFile("TELEGRAM_CHAT_ID"); err != nil {
		slog.Warn("failed to read TELEGRAM_CHAT_ID_FILE", "error", err)
	} else if v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.Telegram.ChatID = id
		} else {
			slog.Warn("TELEGRAM_CHAT_ID is set but not a valid int64, ignoring", "value", v)
		}
	}
	if v := os.Getenv("TRADEBOT_DB_PATH"); v != "" {
		cfg.Storage.DBPath = v
	}
	if v := os.Getenv("TRADEBOT_WEB_ADDR"); v != "" {
		cfg.Web.Addr = v
	}
	if v := os.Getenv("TRADEBOT_POLL_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Trading.PollIntervalSec = n
		} else {
			slog.Warn("TRADEBOT_POLL_INTERVAL is set but not a valid integer, ignoring", "value", v)
		}
	}
}

func applyDefaults(cfg *Config) {
	if cfg.Alpaca.BaseURL == "" {
		cfg.Alpaca.BaseURL = "https://paper-api.alpaca.markets"
	}
	if cfg.Alpaca.Feed == "" {
		cfg.Alpaca.Feed = "iex"
	}
	if cfg.Trading.InitialCash == 0 {
		cfg.Trading.InitialCash = 10000.00
	}
	if cfg.Trading.PollIntervalSec == 0 {
		cfg.Trading.PollIntervalSec = 60
	}
	if cfg.Trading.MaxPositionPct == 0 {
		cfg.Trading.MaxPositionPct = 0.10
	}
	if cfg.Trading.MaxPositions == 0 {
		cfg.Trading.MaxPositions = 5
	}
	if len(cfg.Trading.Watchlist) == 0 {
		cfg.Trading.Watchlist = []string{
			"AAPL", "MSFT", "GOOGL", "AMZN", "NVDA",
			"META", "TSLA", "BRK.B", "JPM", "V",
			"UNH", "JNJ", "XOM", "PG", "MA",
			"HD", "CVX", "MRK", "ABBV", "PEP",
		}
	}
	if cfg.Strategy.Type == "" {
		cfg.Strategy.Type = "rsi_macd"
	}
	if cfg.Strategy.RSIPeriod == 0 {
		cfg.Strategy.RSIPeriod = 14
	}
	if cfg.Strategy.RSIOversold == 0 {
		cfg.Strategy.RSIOversold = 30.0
	}
	if cfg.Strategy.RSIOverbought == 0 {
		cfg.Strategy.RSIOverbought = 70.0
	}
	if cfg.Strategy.MACDFastPeriod == 0 {
		cfg.Strategy.MACDFastPeriod = 12
	}
	if cfg.Strategy.MACDSlowPeriod == 0 {
		cfg.Strategy.MACDSlowPeriod = 26
	}
	if cfg.Strategy.MACDSignalPeriod == 0 {
		cfg.Strategy.MACDSignalPeriod = 9
	}
	if cfg.Strategy.EMAFastPeriod == 0 {
		cfg.Strategy.EMAFastPeriod = 9
	}
	if cfg.Strategy.EMASlowPeriod == 0 {
		cfg.Strategy.EMASlowPeriod = 21
	}
	if cfg.Storage.DBPath == "" {
		cfg.Storage.DBPath = "tradebot.db"
	}
	if cfg.Web.Addr == "" {
		cfg.Web.Addr = ":8080"
	}
}

func validate(cfg *Config) error {
	if cfg.Alpaca.APIKey == "" {
		return fmt.Errorf("alpaca API key is required (set in config or APCA_API_KEY_ID env var)")
	}
	if cfg.Alpaca.APISecret == "" {
		return fmt.Errorf("alpaca API secret is required (set in config or APCA_API_SECRET_KEY env var)")
	}
	if cfg.Trading.MaxPositionPct <= 0 || cfg.Trading.MaxPositionPct > 1 {
		return fmt.Errorf("max_position_pct must be between 0 and 1")
	}
	if cfg.Trading.MaxPositions < 1 {
		return fmt.Errorf("max_positions must be at least 1")
	}
	if len(cfg.Trading.Watchlist) == 0 {
		return fmt.Errorf("watchlist must not be empty")
	}
	if cfg.Strategy.Type == "ema_crossover" {
		if cfg.Strategy.EMAFastPeriod <= 0 || cfg.Strategy.EMASlowPeriod <= 0 {
			return fmt.Errorf("ema_fast_period and ema_slow_period must be positive")
		}
		if cfg.Strategy.EMAFastPeriod >= cfg.Strategy.EMASlowPeriod {
			return fmt.Errorf("ema_fast_period must be less than ema_slow_period")
		}
	}
	return nil
}

// readSecretFile reads a secret from a file path, trimming whitespace.
func readSecretFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// envOrFile checks for a _FILE env var first (Docker secrets), then falls back
// to the direct env var. Returns the value and any error from reading the file.
func envOrFile(key string) (string, error) {
	if path := os.Getenv(key + "_FILE"); path != "" {
		return readSecretFile(path)
	}
	return os.Getenv(key), nil
}
