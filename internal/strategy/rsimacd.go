package strategy

import (
	"fmt"
	"time"

	"github.com/rohithbv/tradebot/internal/indicator"
	"github.com/rohithbv/tradebot/internal/model"
)

// RSIMACDStrategy combines RSI and MACD indicators to generate trading signals.
type RSIMACDStrategy struct {
	rsiPeriod     int
	rsiOversold   float64
	rsiOverbought float64
	macdFast      int
	macdSlow      int
	macdSignal    int
}

// NewRSIMACDStrategy creates a new RSIMACDStrategy with the given parameters.
func NewRSIMACDStrategy(rsiPeriod int, rsiOversold, rsiOverbought float64, macdFast, macdSlow, macdSignal int) *RSIMACDStrategy {
	return &RSIMACDStrategy{
		rsiPeriod:     rsiPeriod,
		rsiOversold:   rsiOversold,
		rsiOverbought: rsiOverbought,
		macdFast:      macdFast,
		macdSlow:      macdSlow,
		macdSignal:    macdSignal,
	}
}

// Evaluate analyses the given closing prices and returns an Analysis with a
// Buy, Sell, or Hold signal. If there is insufficient data to compute the
// indicators, it returns a Hold signal gracefully.
func (s *RSIMACDStrategy) Evaluate(symbol string, closes []float64) model.Analysis {
	hold := model.Analysis{
		Symbol:       symbol,
		Signal:       model.Hold,
		StrategyType: "rsi_macd",
		Reason:       "hold",
		Timestamp:    time.Now(),
	}

	// Calculate RSI.
	rsiResult, err := indicator.CalcRSI(closes, s.rsiPeriod)
	if err != nil || len(rsiResult.Values) == 0 {
		hold.Reason = "insufficient data for RSI"
		return hold
	}

	// Calculate MACD.
	macdResult, err := indicator.CalcMACD(closes, s.macdFast, s.macdSlow, s.macdSignal)
	if err != nil || len(macdResult.Points) < 2 {
		hold.Reason = "insufficient data for MACD"
		return hold
	}

	// Use the latest RSI value.
	latestRSI := rsiResult.Values[len(rsiResult.Values)-1]

	// Use the last two MACD points for crossover detection.
	prev := macdResult.Points[len(macdResult.Points)-2]
	curr := macdResult.Points[len(macdResult.Points)-1]

	analysis := model.Analysis{
		Symbol:       symbol,
		RSI:          latestRSI,
		MACD:         curr.MACD,
		MACDSignal:   curr.Signal,
		StrategyType: "rsi_macd",
		Timestamp:    time.Now(),
	}

	// BUY: RSI < oversold AND bullish crossover (previous MACD < Signal, current MACD >= Signal).
	bullishCrossover := prev.MACD < prev.Signal && curr.MACD >= curr.Signal
	if latestRSI < s.rsiOversold && bullishCrossover {
		analysis.Signal = model.Buy
		analysis.Reason = fmt.Sprintf("RSI %.1f < %.1f (oversold) + MACD bullish crossover", latestRSI, s.rsiOversold)
		return analysis
	}

	// SELL: RSI > overbought AND bearish crossover (previous MACD > Signal, current MACD <= Signal).
	bearishCrossover := prev.MACD > prev.Signal && curr.MACD <= curr.Signal
	if latestRSI > s.rsiOverbought && bearishCrossover {
		analysis.Signal = model.Sell
		analysis.Reason = fmt.Sprintf("RSI %.1f > %.1f (overbought) + MACD bearish crossover", latestRSI, s.rsiOverbought)
		return analysis
	}

	// HOLD: all other cases.
	analysis.Signal = model.Hold
	analysis.Reason = "no signal"
	return analysis
}
