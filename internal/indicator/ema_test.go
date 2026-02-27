package indicator

import (
	"math"
	"testing"
)

func TestCalcEMACrossover_KnownSequence(t *testing.T) {
	// 25 data points, fast=3, slow=5
	closes := make([]float64, 25)
	for i := range closes {
		closes[i] = float64(100 + i)
	}

	res, err := CalcEMACrossover(closes, 3, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Output length should be len(closes)-slowPeriod+1 = 25-5+1 = 21
	if len(res.FastEMA) != 21 {
		t.Errorf("FastEMA length = %d, want 21", len(res.FastEMA))
	}
	if len(res.SlowEMA) != 21 {
		t.Errorf("SlowEMA length = %d, want 21", len(res.SlowEMA))
	}

	// In a rising series, fast EMA should be above slow EMA.
	for i := 1; i < len(res.FastEMA); i++ {
		if res.FastEMA[i] <= res.SlowEMA[i] {
			t.Errorf("at index %d: fast (%.4f) should be > slow (%.4f) in rising series", i, res.FastEMA[i], res.SlowEMA[i])
		}
	}
}

func TestCalcEMACrossover_InsufficientData(t *testing.T) {
	closes := []float64{1, 2, 3}
	_, err := CalcEMACrossover(closes, 3, 5)
	if err == nil {
		t.Fatal("expected error for insufficient data")
	}
}

func TestCalcEMACrossover_InvalidPeriods(t *testing.T) {
	closes := make([]float64, 30)
	for i := range closes {
		closes[i] = float64(i)
	}

	tests := []struct {
		name string
		fast int
		slow int
	}{
		{"fast >= slow", 5, 5},
		{"fast > slow", 10, 5},
		{"zero fast", 0, 5},
		{"zero slow", 5, 0},
		{"negative fast", -1, 5},
		{"negative slow", 5, -1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CalcEMACrossover(closes, tc.fast, tc.slow)
			if err == nil {
				t.Errorf("expected error for fast=%d, slow=%d", tc.fast, tc.slow)
			}
		})
	}
}

func TestCalcEMACrossover_ExactMinimumData(t *testing.T) {
	// Exactly slowPeriod data points → output length 1.
	closes := []float64{10, 11, 12, 13, 14}
	res, err := CalcEMACrossover(closes, 2, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.FastEMA) != 1 || len(res.SlowEMA) != 1 {
		t.Errorf("expected 1 point, got fast=%d slow=%d", len(res.FastEMA), len(res.SlowEMA))
	}
}

func TestCalcEMACrossover_FallingTrend(t *testing.T) {
	closes := make([]float64, 30)
	for i := range closes {
		closes[i] = float64(130 - i)
	}

	res, err := CalcEMACrossover(closes, 5, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// In a falling series, fast EMA should be below slow EMA after initial stabilization.
	n := len(res.FastEMA)
	for i := n / 2; i < n; i++ {
		if res.FastEMA[i] >= res.SlowEMA[i] {
			t.Errorf("at index %d: fast (%.4f) should be < slow (%.4f) in falling series", i, res.FastEMA[i], res.SlowEMA[i])
		}
	}
}

func TestCalcEMACrossover_NaNInput(t *testing.T) {
	closes := []float64{100, 101, math.NaN(), 102, 103, 104}
	_, err := CalcEMACrossover(closes, 2, 3)
	if err == nil {
		t.Error("expected error for NaN input")
	}
}

func TestCalcEMACrossover_InfInput(t *testing.T) {
	closes := []float64{100, 101, math.Inf(1), 102, 103, 104}
	_, err := CalcEMACrossover(closes, 2, 3)
	if err == nil {
		t.Error("expected error for Inf input")
	}
}

func TestCalcEMACrossover_AlignmentCheck(t *testing.T) {
	// Verify both slices are the same length for various inputs.
	sizes := []struct {
		n    int
		fast int
		slow int
	}{
		{50, 9, 21},
		{100, 12, 26},
		{21, 9, 21},
	}
	for _, s := range sizes {
		closes := make([]float64, s.n)
		for i := range closes {
			closes[i] = 100 + float64(i)*0.5 + math.Sin(float64(i))
		}
		res, err := CalcEMACrossover(closes, s.fast, s.slow)
		if err != nil {
			t.Fatalf("n=%d fast=%d slow=%d: unexpected error: %v", s.n, s.fast, s.slow, err)
		}
		if len(res.FastEMA) != len(res.SlowEMA) {
			t.Errorf("n=%d fast=%d slow=%d: length mismatch fast=%d slow=%d",
				s.n, s.fast, s.slow, len(res.FastEMA), len(res.SlowEMA))
		}
		wantLen := s.n - s.slow + 1
		if len(res.FastEMA) != wantLen {
			t.Errorf("n=%d fast=%d slow=%d: got length %d, want %d",
				s.n, s.fast, s.slow, len(res.FastEMA), wantLen)
		}
	}
}
