package strategy

import (
	"testing"

	"github.com/rohithbv/tradebot/internal/model"
)

func TestEMACrossover_InterfaceCompliance(t *testing.T) {
	var _ Strategy = (*EMACrossoverStrategy)(nil)
}

func TestEMACrossover_BuySignal(t *testing.T) {
	s := NewEMACrossoverStrategy(3, 5)

	// Long falling series so fast EMA is well below slow, then flat + small uptick.
	// The last price triggers a golden cross at the final 2 aligned EMA points.
	closes := []float64{
		100, 100, 100, 100, 100,
		99, 98, 97, 96, 95,
		94, 93, 92, 91, 90,
		89, 88, 87, 86, 85,
		85, 85, 85, 85, 85,
		85, 85, 85, 86, // ends here: last 2 EMA points cross
	}

	a := s.Evaluate("TEST", closes)
	if a.Signal != model.Buy {
		t.Errorf("expected Buy signal, got %v (reason: %s)", a.Signal, a.Reason)
	}
	if a.StrategyType != "ema_crossover" {
		t.Errorf("expected strategy_type ema_crossover, got %s", a.StrategyType)
	}
}

func TestEMACrossover_SellSignal(t *testing.T) {
	s := NewEMACrossoverStrategy(3, 5)

	// Long rising series so fast EMA is well above slow, then flat + small downtick.
	// The last price triggers a death cross at the final 2 aligned EMA points.
	closes := []float64{
		100, 100, 100, 100, 100,
		101, 102, 103, 104, 105,
		106, 107, 108, 109, 110,
		111, 112, 113, 114, 115,
		115, 115, 115, 115, 115,
		115, 115, 115, 114, // ends here: last 2 EMA points cross
	}

	a := s.Evaluate("TEST", closes)
	if a.Signal != model.Sell {
		t.Errorf("expected Sell signal, got %v (reason: %s)", a.Signal, a.Reason)
	}
}

func TestEMACrossover_Hold(t *testing.T) {
	s := NewEMACrossoverStrategy(3, 5)

	// Steadily rising — fast stays above slow, no crossover.
	closes := make([]float64, 30)
	for i := range closes {
		closes[i] = float64(100 + i)
	}

	a := s.Evaluate("TEST", closes)
	if a.Signal != model.Hold {
		t.Errorf("expected Hold signal, got %v (reason: %s)", a.Signal, a.Reason)
	}
}

func TestEMACrossover_InsufficientData(t *testing.T) {
	s := NewEMACrossoverStrategy(9, 21)

	// Not enough data for slow EMA + 2 points.
	closes := make([]float64, 21) // slowPeriod=21 → only 1 aligned point, need 2
	for i := range closes {
		closes[i] = float64(i)
	}

	a := s.Evaluate("TEST", closes)
	if a.Signal != model.Hold {
		t.Errorf("expected Hold for insufficient data, got %v", a.Signal)
	}
	if a.Reason != "insufficient data for EMA crossover" {
		t.Errorf("unexpected reason: %s", a.Reason)
	}
}

func TestEMACrossover_EmptyData(t *testing.T) {
	s := NewEMACrossoverStrategy(3, 5)

	a := s.Evaluate("TEST", nil)
	if a.Signal != model.Hold {
		t.Errorf("expected Hold for empty data, got %v", a.Signal)
	}
}
