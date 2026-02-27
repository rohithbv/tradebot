package strategy

import (
	"fmt"
	"time"

	"github.com/rohithbv/tradebot/internal/indicator"
	"github.com/rohithbv/tradebot/internal/model"
)

// EMACrossoverStrategy generates buy/sell signals based on fast/slow EMA crossovers.
type EMACrossoverStrategy struct {
	fastPeriod int
	slowPeriod int
}

// NewEMACrossoverStrategy creates a new EMA crossover strategy.
func NewEMACrossoverStrategy(fastPeriod, slowPeriod int) *EMACrossoverStrategy {
	return &EMACrossoverStrategy{
		fastPeriod: fastPeriod,
		slowPeriod: slowPeriod,
	}
}

// Evaluate computes fast and slow EMAs and checks for crossovers.
// Buy when fast EMA crosses above slow EMA; sell when fast crosses below slow.
func (s *EMACrossoverStrategy) Evaluate(symbol string, closes []float64) model.Analysis {
	hold := model.Analysis{
		Symbol:       symbol,
		Signal:       model.Hold,
		StrategyType: "ema_crossover",
		Reason:       "hold",
		Timestamp:    time.Now(),
	}

	result, err := indicator.CalcEMACrossover(closes, s.fastPeriod, s.slowPeriod)
	if err != nil || len(result.FastEMA) < 2 {
		hold.Reason = "insufficient data for EMA crossover"
		return hold
	}

	n := len(result.FastEMA)
	prevFast := result.FastEMA[n-2]
	prevSlow := result.SlowEMA[n-2]
	currFast := result.FastEMA[n-1]
	currSlow := result.SlowEMA[n-1]

	analysis := model.Analysis{
		Symbol:       symbol,
		EMAFast:      currFast,
		EMASlow:      currSlow,
		StrategyType: "ema_crossover",
		Timestamp:    time.Now(),
	}

	// Golden cross: fast crosses above slow.
	if prevFast <= prevSlow && currFast > currSlow {
		analysis.Signal = model.Buy
		analysis.Reason = fmt.Sprintf("EMA golden cross: fast %.2f > slow %.2f", currFast, currSlow)
		return analysis
	}

	// Death cross: fast crosses below slow.
	if prevFast >= prevSlow && currFast < currSlow {
		analysis.Signal = model.Sell
		analysis.Reason = fmt.Sprintf("EMA death cross: fast %.2f < slow %.2f", currFast, currSlow)
		return analysis
	}

	analysis.Signal = model.Hold
	analysis.Reason = "no crossover"
	return analysis
}
