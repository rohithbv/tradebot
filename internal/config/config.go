package config

import (
	"fmt"
	"os"
	"strconv"

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
	APIKey    string `yaml:"api_key"`
	APISecret string `yaml:"api_secret"`
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
	BotToken string `yaml:"bot_token"`
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
	if v := os.Getenv("APCA_API_KEY_ID"); v != "" {
		cfg.Alpaca.APIKey = v
	}
	if v := os.Getenv("APCA_API_SECRET_KEY"); v != "" {
		cfg.Alpaca.APISecret = v
	}
	if v := os.Getenv("TELEGRAM_BOT_TOKEN"); v != "" {
		cfg.Telegram.BotToken = v
	}
	if v := os.Getenv("TELEGRAM_CHAT_ID"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			cfg.Telegram.ChatID = id
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
