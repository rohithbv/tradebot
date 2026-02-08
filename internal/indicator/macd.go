package indicator

import "errors"

// MACDPoint represents a single point of MACD output.
type MACDPoint struct {
	MACD      float64
	Signal    float64
	Histogram float64
}

// MACDResult holds the full MACD computation output.
type MACDResult struct {
	Points []MACDPoint
}

// CalcMACD computes the MACD indicator (Moving Average Convergence Divergence).
//
// Parameters:
//   - closes: slice of closing prices
//   - fast: fast EMA period (e.g. 12)
//   - slow: slow EMA period (e.g. 26)
//   - signal: signal line EMA period (e.g. 9)
//
// The result contains one MACDPoint for each bar starting from index slow+signal-2
// of the input (i.e. where we have enough data for the slow EMA and then the signal EMA).
func CalcMACD(closes []float64, fast, slow, signal int) (MACDResult, error) {
	if fast <= 0 || slow <= 0 || signal <= 0 {
		return MACDResult{}, errors.New("all periods must be positive")
	}
	if fast >= slow {
		return MACDResult{}, errors.New("fast period must be less than slow period")
	}
	minRequired := slow + signal - 1
	if len(closes) < minRequired {
		return MACDResult{}, errors.New("insufficient data: need at least slow+signal-1 closes")
	}

	// Calculate fast and slow EMAs across the full price series.
	fastEMA := calcEMA(closes, fast)
	slowEMA := calcEMA(closes, slow)

	// MACD line = fast EMA - slow EMA, starting from where slow EMA begins.
	// slowEMA has len(closes)-slow+1 entries, starting at index slow-1 of closes.
	// fastEMA has len(closes)-fast+1 entries, starting at index fast-1 of closes.
	// We align them: MACD starts at index slow-1 of closes.
	// In fastEMA terms, index slow-1 of closes = fastEMA[slow-1 - (fast-1)] = fastEMA[slow-fast].
	macdLine := make([]float64, len(slowEMA))
	fastOffset := slow - fast // offset into fastEMA to align with slowEMA[0]
	for i := 0; i < len(slowEMA); i++ {
		macdLine[i] = fastEMA[fastOffset+i] - slowEMA[i]
	}

	// Signal line = EMA of the MACD line with `signal` period.
	signalLine := calcEMA(macdLine, signal)

	// Build result points. signalLine has len(macdLine)-signal+1 entries.
	points := make([]MACDPoint, len(signalLine))
	signalOffset := signal - 1 // offset into macdLine to align with signalLine[0]
	for i := 0; i < len(signalLine); i++ {
		m := macdLine[signalOffset+i]
		s := signalLine[i]
		points[i] = MACDPoint{
			MACD:      m,
			Signal:    s,
			Histogram: m - s,
		}
	}

	return MACDResult{Points: points}, nil
}

// calcEMA computes an Exponential Moving Average over a data series.
// The first EMA value is the SMA of the first `period` values.
// Returns a slice of length len(data)-period+1.
func calcEMA(data []float64, period int) []float64 {
	if len(data) < period {
		return nil
	}

	result := make([]float64, len(data)-period+1)

	// First value is the SMA of the first `period` data points.
	var sum float64
	for i := 0; i < period; i++ {
		sum += data[i]
	}
	result[0] = sum / float64(period)

	// Subsequent values use EMA smoothing.
	multiplier := 2.0 / float64(period+1)
	for i := period; i < len(data); i++ {
		result[i-period+1] = (data[i]-result[i-period])*multiplier + result[i-period]
	}

	return result
}
