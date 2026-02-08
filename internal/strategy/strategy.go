package strategy

import "github.com/rohithbv/tradebot/internal/model"

// Strategy evaluates closing prices for a symbol and produces an Analysis.
type Strategy interface {
	Evaluate(symbol string, closes []float64) model.Analysis
}
