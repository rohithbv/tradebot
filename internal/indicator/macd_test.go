package indicator

import (
	"math"
	"testing"
)

func TestCalcMACD_KnownSequence(t *testing.T) {
	// Generate a sequence long enough for MACD(12,26,9): need at least 26+9-1=34 bars.
	closes := make([]float64, 50)
	for i := range closes {
		// A rising trend with some oscillation.
		closes[i] = 100.0 + float64(i)*0.5 + 2.0*math.Sin(float64(i)*0.5)
	}

	result, err := CalcMACD(closes, 12, 26, 9)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Points) == 0 {
		t.Fatal("expected non-empty MACD points")
	}

	// Verify histogram = MACD - Signal for all points.
	for i, p := range result.Points {
		expected := p.MACD - p.Signal
		if math.Abs(p.Histogram-expected) > 1e-10 {
			t.Errorf("point[%d]: histogram=%.6f, expected MACD-Signal=%.6f", i, p.Histogram, expected)
		}
	}
}

func TestCalcMACD_InsufficientData(t *testing.T) {
	closes := make([]float64, 20)
	for i := range closes {
		closes[i] = float64(i)
	}
	// Need at least 26+9-1=34 for MACD(12,26,9).
	_, err := CalcMACD(closes, 12, 26, 9)
	if err == nil {
		t.Fatal("expected error for insufficient data")
	}
}

func TestCalcMACD_PointsLength(t *testing.T) {
	// With 50 data points and MACD(12,26,9):
	// slow EMA has 50-26+1 = 25 entries (MACD line also 25 entries)
	// signal EMA of MACD line has 25-9+1 = 17 entries
	closes := make([]float64, 50)
	for i := range closes {
		closes[i] = 100.0 + float64(i)
	}

	result, err := CalcMACD(closes, 12, 26, 9)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected length: len(closes) - slow + 1 - signal + 1 = 50 - 26 + 1 - 9 + 1 = 17
	expected := len(closes) - 26 + 1 - 9 + 1
	if len(result.Points) != expected {
		t.Errorf("expected %d points, got %d", expected, len(result.Points))
	}
}

func TestCalcMACD_ExactMinimumData(t *testing.T) {
	// Exact minimum: slow+signal-1 = 26+9-1 = 34 data points.
	n := 34
	closes := make([]float64, n)
	for i := range closes {
		closes[i] = 100.0 + float64(i)*0.1
	}

	result, err := CalcMACD(closes, 12, 26, 9)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should produce exactly 1 point.
	if len(result.Points) != 1 {
		t.Errorf("expected 1 point for exact minimum data, got %d", len(result.Points))
	}
}

func TestCalcMACD_InvalidPeriods(t *testing.T) {
	closes := make([]float64, 100)
	for i := range closes {
		closes[i] = float64(i)
	}

	// fast >= slow should be an error.
	_, err := CalcMACD(closes, 26, 12, 9)
	if err == nil {
		t.Fatal("expected error when fast >= slow")
	}

	// Zero period.
	_, err = CalcMACD(closes, 0, 26, 9)
	if err == nil {
		t.Fatal("expected error for zero period")
	}
}

func TestCalcMACD_RisingTrend(t *testing.T) {
	// In a strong rising trend, the fast EMA should be above the slow EMA,
	// so MACD line should be positive.
	closes := make([]float64, 60)
	for i := range closes {
		closes[i] = 50.0 + float64(i)*2.0
	}

	result, err := CalcMACD(closes, 12, 26, 9)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The last MACD value should be positive in a strong uptrend.
	last := result.Points[len(result.Points)-1]
	if last.MACD <= 0 {
		t.Errorf("expected positive MACD in uptrend, got %.4f", last.MACD)
	}
}
