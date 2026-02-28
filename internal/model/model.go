package model

import "time"

// Signal represents a trading signal.
type Signal int

const (
	Hold Signal = iota
	Buy
	Sell
)

func (s Signal) String() string {
	switch s {
	case Buy:
		return "buy"
	case Sell:
		return "sell"
	default:
		return "hold"
	}
}

// Analysis is the output of a strategy evaluation.
type Analysis struct {
	Symbol       string
	Signal       Signal
	RSI          float64
	MACD         float64
	MACDSignal   float64
	EMAFast      float64
	EMASlow      float64
	StrategyType string
	Reason       string
	Timestamp    time.Time
}

// Bar represents a single OHLCV price bar.
type Bar struct {
	Symbol    string
	Timestamp time.Time
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    int64
}

// Trade represents an executed trade.
type Trade struct {
	ID          string
	Symbol      string
	Side        string // "buy" or "sell"
	Qty         int
	Price       float64
	Total       float64
	RealizedPnL *float64 // nil for buy trades; set to realized P&L for sell trades
	Reason      string
	Timestamp   time.Time
}

// Position represents a currently held stock position.
type Position struct {
	Symbol       string
	Qty          int
	AvgCost      float64
	CurrentPrice float64
}

func (p Position) MarketValue() float64 {
	return float64(p.Qty) * p.CurrentPrice
}

func (p Position) UnrealizedPL() float64 {
	return float64(p.Qty) * (p.CurrentPrice - p.AvgCost)
}

// PositionSnapshot is a point-in-time record of a position for JSON storage.
type PositionSnapshot struct {
	Symbol       string  `json:"symbol"`
	Qty          int     `json:"qty"`
	AvgCost      float64 `json:"avg_cost"`
	CurrentPrice float64 `json:"current_price"`
	MarketValue  float64 `json:"market_value"`
	UnrealizedPL float64 `json:"unrealized_pl"`
}

// PortfolioSnapshot is a point-in-time record of the full portfolio.
type PortfolioSnapshot struct {
	ID         int64
	Timestamp  time.Time
	Cash       float64
	TotalValue float64
	Positions  map[string]PositionSnapshot
}

// BrokerState is the serializable state of the paper broker for persistence.
type BrokerState struct {
	Cash      float64             `json:"cash"`
	Positions map[string]Position `json:"positions"`
}
