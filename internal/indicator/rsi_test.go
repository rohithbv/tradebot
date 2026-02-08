package indicator

import (
	"math"
	"testing"
)

func TestCalcRSI_KnownSequence(t *testing.T) {
	// A realistic price sequence with ups and downs.
	closes := []float64{
		44.0, 44.34, 44.09, 43.61, 44.33,
		44.83, 45.10, 45.42, 45.84, 46.08,
		45.89, 46.03, 45.61, 46.28, 46.28,
		46.00, 46.03, 46.41, 46.22, 45.64,
	}
	period := 14

	result, err := CalcRSI(closes, period)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// We should get len(closes) - period = 6 RSI values.
	expectedLen := len(closes) - period
	if len(result.Values) != expectedLen {
		t.Fatalf("expected %d values, got %d", expectedLen, len(result.Values))
	}

	// All RSI values should be in [0, 100].
	for i, v := range result.Values {
		if v < 0 || v > 100 {
			t.Errorf("RSI[%d] = %.4f, expected in [0, 100]", i, v)
		}
	}

	// The first RSI should be roughly in the 60-80 range for this data set
	// (mostly upward movement in the first 14 bars).
	if result.Values[0] < 50 || result.Values[0] > 90 {
		t.Errorf("first RSI = %.4f, expected roughly between 50 and 90", result.Values[0])
	}
}

func TestCalcRSI_InsufficientData(t *testing.T) {
	closes := []float64{1.0, 2.0, 3.0}
	_, err := CalcRSI(closes, 5)
	if err == nil {
		t.Fatal("expected error for insufficient data")
	}
}

func TestCalcRSI_ExactMinimumData(t *testing.T) {
	// period+1 closes should produce exactly 1 RSI value.
	closes := []float64{10, 11, 12, 13, 14, 15}
	result, err := CalcRSI(closes, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Values) != 1 {
		t.Fatalf("expected 1 value, got %d", len(result.Values))
	}
}

func TestCalcRSI_AllGains(t *testing.T) {
	// Monotonically increasing prices should yield RSI approaching 100.
	closes := make([]float64, 30)
	for i := range closes {
		closes[i] = float64(100 + i)
	}

	result, err := CalcRSI(closes, 14)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	last := result.Values[len(result.Values)-1]
	if last < 99.0 {
		t.Errorf("RSI for all-gains = %.4f, expected >= 99", last)
	}
}

func TestCalcRSI_AllLosses(t *testing.T) {
	// Monotonically decreasing prices should yield RSI approaching 0.
	closes := make([]float64, 30)
	for i := range closes {
		closes[i] = float64(200 - i)
	}

	result, err := CalcRSI(closes, 14)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	last := result.Values[len(result.Values)-1]
	if last > 1.0 {
		t.Errorf("RSI for all-losses = %.4f, expected <= 1", last)
	}
}

func TestCalcRSI_ZeroPeriod(t *testing.T) {
	_, err := CalcRSI([]float64{1, 2, 3}, 0)
	if err == nil {
		t.Fatal("expected error for zero period")
	}
}

func TestCalcRSI_NoMovement(t *testing.T) {
	// Flat prices: no gains, no losses -> RSI should be 50.
	closes := make([]float64, 20)
	for i := range closes {
		closes[i] = 100.0
	}

	result, err := CalcRSI(closes, 14)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i, v := range result.Values {
		if math.Abs(v-50) > 0.01 {
			t.Errorf("RSI[%d] = %.4f, expected 50 for flat prices", i, v)
		}
	}
}
