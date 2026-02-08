package strategy

import (
	"testing"

	"github.com/rohithbv/tradebot/internal/model"
)

func TestRSIMACDStrategy_BuySignal(t *testing.T) {
	// Craft a price series that produces:
	// 1. RSI below oversold (30) -- prices declining heavily then slight uptick
	// 2. MACD bullish crossover -- short-term momentum turning up
	//
	// Strategy: Start with a stable price, then drop sharply to push RSI low,
	// then add a small bounce at the end for bullish crossover.
	strat := NewRSIMACDStrategy(14, 30, 70, 12, 26, 9)

	// We need at least 26+9-1=34 bars. Build 60 bars.
	closes := make([]float64, 60)

	// First 30 bars: stable around 100.
	for i := 0; i < 30; i++ {
		closes[i] = 100.0
	}

	// Bars 30-55: sharp decline to push RSI very low.
	for i := 30; i < 56; i++ {
		closes[i] = closes[i-1] - 1.5
	}

	// Bars 56-59: small bounce to create bullish crossover.
	closes[56] = closes[55] + 0.3
	closes[57] = closes[56] + 0.5
	closes[58] = closes[57] + 0.8
	closes[59] = closes[58] + 1.2

	analysis := strat.Evaluate("TEST", closes)

	if analysis.Signal != model.Buy {
		t.Errorf("expected Buy signal, got %v (reason: %s, RSI: %.2f, MACD: %.4f, Signal: %.4f)",
			analysis.Signal, analysis.Reason, analysis.RSI, analysis.MACD, analysis.MACDSignal)
	}
	if analysis.Symbol != "TEST" {
		t.Errorf("expected symbol TEST, got %s", analysis.Symbol)
	}
}

func TestRSIMACDStrategy_SellSignal(t *testing.T) {
	// Craft a price series that produces:
	// 1. RSI above overbought (70) -- prices rising heavily then slight dip
	// 2. MACD bearish crossover -- short-term momentum turning down
	strat := NewRSIMACDStrategy(14, 30, 70, 12, 26, 9)

	closes := make([]float64, 60)

	// First 30 bars: stable around 50.
	for i := 0; i < 30; i++ {
		closes[i] = 50.0
	}

	// Bars 30-55: sharp rise to push RSI very high.
	for i := 30; i < 56; i++ {
		closes[i] = closes[i-1] + 1.5
	}

	// Bars 56-59: small dip to create bearish crossover.
	closes[56] = closes[55] - 0.3
	closes[57] = closes[56] - 0.5
	closes[58] = closes[57] - 0.8
	closes[59] = closes[58] - 1.2

	analysis := strat.Evaluate("TEST", closes)

	if analysis.Signal != model.Sell {
		t.Errorf("expected Sell signal, got %v (reason: %s, RSI: %.2f, MACD: %.4f, Signal: %.4f)",
			analysis.Signal, analysis.Reason, analysis.RSI, analysis.MACD, analysis.MACDSignal)
	}
}

func TestRSIMACDStrategy_HoldSignal(t *testing.T) {
	// Neutral data: flat prices produce RSI around 50, no crossover.
	strat := NewRSIMACDStrategy(14, 30, 70, 12, 26, 9)

	closes := make([]float64, 60)
	for i := range closes {
		closes[i] = 100.0 + float64(i)*0.01 // very slight uptrend
	}

	analysis := strat.Evaluate("FLAT", closes)

	if analysis.Signal != model.Hold {
		t.Errorf("expected Hold signal, got %v (reason: %s)", analysis.Signal, analysis.Reason)
	}
}

func TestRSIMACDStrategy_InsufficientData(t *testing.T) {
	strat := NewRSIMACDStrategy(14, 30, 70, 12, 26, 9)

	// Too few data points.
	closes := []float64{1, 2, 3, 4, 5}

	analysis := strat.Evaluate("SHORT", closes)

	if analysis.Signal != model.Hold {
		t.Errorf("expected Hold for insufficient data, got %v", analysis.Signal)
	}
	if analysis.Symbol != "SHORT" {
		t.Errorf("expected symbol SHORT, got %s", analysis.Symbol)
	}
}

func TestRSIMACDStrategy_EmptyData(t *testing.T) {
	strat := NewRSIMACDStrategy(14, 30, 70, 12, 26, 9)

	analysis := strat.Evaluate("EMPTY", nil)

	if analysis.Signal != model.Hold {
		t.Errorf("expected Hold for empty data, got %v", analysis.Signal)
	}
}

// Verify the strategy implements the Strategy interface.
var _ Strategy = (*RSIMACDStrategy)(nil)
