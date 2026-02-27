package strategy

import (
	"fmt"

	"github.com/rohithbv/tradebot/internal/config"
)

// New creates a Strategy based on the config type.
// Defaults to "rsi_macd" if type is empty.
func New(cfg config.StrategyConfig) (Strategy, error) {
	switch cfg.Type {
	case "", "rsi_macd":
		return NewRSIMACDStrategy(
			cfg.RSIPeriod,
			cfg.RSIOversold,
			cfg.RSIOverbought,
			cfg.MACDFastPeriod,
			cfg.MACDSlowPeriod,
			cfg.MACDSignalPeriod,
		), nil
	case "ema_crossover":
		return NewEMACrossoverStrategy(
			cfg.EMAFastPeriod,
			cfg.EMASlowPeriod,
		), nil
	default:
		return nil, fmt.Errorf("unknown strategy type: %q", cfg.Type)
	}
}
