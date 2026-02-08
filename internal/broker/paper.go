package broker

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rohithbv/tradebot/internal/config"
	"github.com/rohithbv/tradebot/internal/model"
	"github.com/rohithbv/tradebot/internal/storage"
)

// PaperBroker implements a thread-safe paper trading broker.
// The engine writes (ExecuteSignal, UpdatePrices) while the web server reads (GetState, TotalValue).
type PaperBroker struct {
	mu        sync.RWMutex
	cash      float64
	positions map[string]model.Position
	store     storage.Store
	cfg       config.TradingConfig
}

// NewPaperBroker creates a paper broker, attempting to restore state from store.
// If no saved state exists, it starts with initialCash and empty positions.
func NewPaperBroker(initialCash float64, store storage.Store, cfg config.TradingConfig) *PaperBroker {
	b := &PaperBroker{
		cash:      initialCash,
		positions: make(map[string]model.Position),
		store:     store,
		cfg:       cfg,
	}

	// Attempt to restore from saved state.
	if state, err := store.LoadState(); err == nil && state != nil {
		b.cash = state.Cash
		if state.Positions != nil {
			b.positions = state.Positions
		}
	}

	return b
}

// ExecuteSignal processes a trading signal and returns a trade if executed.
// For BUY: checks constraints, calculates position size, deducts cash, creates position.
// For SELL: checks position exists, calculates proceeds, adds cash, removes position.
// For HOLD: returns nil, nil.
func (b *PaperBroker) ExecuteSignal(analysis model.Analysis, currentPrice float64) (*model.Trade, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch analysis.Signal {
	case model.Buy:
		return b.executeBuy(analysis, currentPrice)
	case model.Sell:
		return b.executeSell(analysis, currentPrice)
	case model.Hold:
		return nil, nil
	default:
		return nil, nil
	}
}

// executeBuy handles buy logic (must be called with lock held).
func (b *PaperBroker) executeBuy(analysis model.Analysis, currentPrice float64) (*model.Trade, error) {
	// Check if already holding this symbol.
	if _, exists := b.positions[analysis.Symbol]; exists {
		return nil, nil
	}

	// Check max positions constraint.
	if len(b.positions) >= b.cfg.MaxPositions {
		return nil, nil
	}

	// Calculate total portfolio value.
	totalValue := b.cash
	for _, pos := range b.positions {
		totalValue += pos.MarketValue()
	}

	// Calculate max spend for this position.
	maxSpend := b.cfg.MaxPositionPct * totalValue

	// Calculate quantity.
	qty := int(math.Floor(maxSpend / currentPrice))
	if qty < 1 {
		return nil, nil
	}

	// Calculate actual cost.
	cost := float64(qty) * currentPrice

	// Check if we have enough cash.
	if cost > b.cash {
		return nil, nil
	}

	// Execute the trade.
	b.cash -= cost
	b.positions[analysis.Symbol] = model.Position{
		Symbol:       analysis.Symbol,
		Qty:          qty,
		AvgCost:      currentPrice,
		CurrentPrice: currentPrice,
	}

	// Create trade record.
	trade := &model.Trade{
		ID:        uuid.New().String(),
		Symbol:    analysis.Symbol,
		Side:      "buy",
		Qty:       qty,
		Price:     currentPrice,
		Total:     cost,
		Reason:    analysis.Reason,
		Timestamp: time.Now(),
	}

	// Persist trade.
	if err := b.store.SaveTrade(*trade); err != nil {
		return nil, fmt.Errorf("save buy trade: %w", err)
	}

	return trade, nil
}

// executeSell handles sell logic (must be called with lock held).
func (b *PaperBroker) executeSell(analysis model.Analysis, currentPrice float64) (*model.Trade, error) {
	// Check if we have a position to sell.
	pos, exists := b.positions[analysis.Symbol]
	if !exists {
		return nil, nil
	}

	// Calculate proceeds.
	proceeds := float64(pos.Qty) * currentPrice

	// Execute the trade.
	b.cash += proceeds
	delete(b.positions, analysis.Symbol)

	// Create trade record.
	trade := &model.Trade{
		ID:        uuid.New().String(),
		Symbol:    analysis.Symbol,
		Side:      "sell",
		Qty:       pos.Qty,
		Price:     currentPrice,
		Total:     proceeds,
		Reason:    analysis.Reason,
		Timestamp: time.Now(),
	}

	// Persist trade.
	if err := b.store.SaveTrade(*trade); err != nil {
		return nil, fmt.Errorf("save sell trade: %w", err)
	}

	return trade, nil
}

// UpdatePrices updates the current price for all held positions.
func (b *PaperBroker) UpdatePrices(prices map[string]float64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for symbol, pos := range b.positions {
		if price, ok := prices[symbol]; ok {
			pos.CurrentPrice = price
			b.positions[symbol] = pos
		}
	}
}

// GetState returns a deep copy of the current broker state.
// Safe for concurrent reads while the engine is running.
func (b *PaperBroker) GetState() model.BrokerState {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Deep copy positions map.
	posCopy := make(map[string]model.Position, len(b.positions))
	for k, v := range b.positions {
		posCopy[k] = v
	}

	return model.BrokerState{
		Cash:      b.cash,
		Positions: posCopy,
	}
}

// SaveState persists the current broker state to the store.
func (b *PaperBroker) SaveState() error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	state := model.BrokerState{
		Cash:      b.cash,
		Positions: b.positions,
	}

	if err := b.store.SaveState(state); err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	return nil
}

// TotalValue returns the total portfolio value (cash + market value of all positions).
func (b *PaperBroker) TotalValue() float64 {
	b.mu.RLock()
	defer b.mu.RUnlock()

	total := b.cash
	for _, pos := range b.positions {
		total += pos.MarketValue()
	}

	return total
}
