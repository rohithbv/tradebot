package indicator

import (
	"errors"
	"math"
)

// RSIResult holds the computed RSI values. The Values slice contains one RSI
// value for each bar starting from index `period` of the input closes.
type RSIResult struct {
	Values []float64
}

// CalcRSI calculates the Relative Strength Index using Wilder's smoothing method.
// It requires at least period+1 closing prices to produce the first RSI value.
func CalcRSI(closes []float64, period int) (RSIResult, error) {
	if period <= 0 {
		return RSIResult{}, errors.New("period must be positive")
	}
	if len(closes) < period+1 {
		return RSIResult{}, errors.New("insufficient data: need at least period+1 closes")
	}

	// Calculate price changes.
	changes := make([]float64, len(closes)-1)
	for i := 1; i < len(closes); i++ {
		changes[i-1] = closes[i] - closes[i-1]
	}

	// Separate gains and losses for the initial period.
	var gainSum, lossSum float64
	for i := 0; i < period; i++ {
		if changes[i] > 0 {
			gainSum += changes[i]
		} else {
			lossSum += math.Abs(changes[i])
		}
	}

	avgGain := gainSum / float64(period)
	avgLoss := lossSum / float64(period)

	// Compute RSI values.
	n := len(closes) - period
	values := make([]float64, n)
	values[0] = rsiFromAvg(avgGain, avgLoss)

	// Apply Wilder's smoothing for subsequent values.
	for i := period; i < len(changes); i++ {
		gain := 0.0
		loss := 0.0
		if changes[i] > 0 {
			gain = changes[i]
		} else {
			loss = math.Abs(changes[i])
		}
		avgGain = (avgGain*float64(period-1) + gain) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + loss) / float64(period)
		values[i-period+1] = rsiFromAvg(avgGain, avgLoss)
	}

	return RSIResult{Values: values}, nil
}

// rsiFromAvg computes RSI from average gain and average loss.
func rsiFromAvg(avgGain, avgLoss float64) float64 {
	if avgLoss == 0 {
		if avgGain == 0 {
			return 50 // no movement
		}
		return 100
	}
	rs := avgGain / avgLoss
	return 100 - (100 / (1 + rs))
}
