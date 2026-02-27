package indicator

import (
	"errors"
	"math"
)

// EMACrossoverResult holds the aligned fast and slow EMA series.
type EMACrossoverResult struct {
	FastEMA []float64
	SlowEMA []float64
}

// CalcEMACrossover computes fast and slow EMAs and aligns them so both slices
// have the same length and correspond to the same time indices.
//
// Parameters:
//   - closes: slice of closing prices
//   - fastPeriod: period for the fast EMA (e.g. 9)
//   - slowPeriod: period for the slow EMA (e.g. 21)
//
// Returns aligned slices of length len(closes)-slowPeriod+1.
func CalcEMACrossover(closes []float64, fastPeriod, slowPeriod int) (EMACrossoverResult, error) {
	if fastPeriod <= 0 || slowPeriod <= 0 {
		return EMACrossoverResult{}, errors.New("all periods must be positive")
	}
	if fastPeriod >= slowPeriod {
		return EMACrossoverResult{}, errors.New("fast period must be less than slow period")
	}
	if len(closes) < slowPeriod {
		return EMACrossoverResult{}, errors.New("insufficient data: need at least slowPeriod closes")
	}
	for _, v := range closes {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return EMACrossoverResult{}, errors.New("input contains NaN or Inf values")
		}
	}

	fastEMA := calcEMA(closes, fastPeriod)
	slowEMA := calcEMA(closes, slowPeriod)

	// slowEMA has len(closes)-slowPeriod+1 entries, starting at index slowPeriod-1.
	// fastEMA has len(closes)-fastPeriod+1 entries, starting at index fastPeriod-1.
	// Align: trim fastEMA to match slowEMA length (same trailing window).
	offset := slowPeriod - fastPeriod // elements to skip in fastEMA
	aligned := len(slowEMA)

	alignedFast := make([]float64, aligned)
	copy(alignedFast, fastEMA[offset:offset+aligned])

	return EMACrossoverResult{
		FastEMA: alignedFast,
		SlowEMA: slowEMA,
	}, nil
}
