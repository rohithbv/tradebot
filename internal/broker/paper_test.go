package broker

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/rohithbv/tradebot/internal/config"
	"github.com/rohithbv/tradebot/internal/model"
	"github.com/rohithbv/tradebot/internal/storage"
)

func setupTest(t *testing.T) (*PaperBroker, *storage.SQLiteStore) {
	t.Helper()

	// Create temp database.
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := storage.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	// Default trading config.
	cfg := config.TradingConfig{
		InitialCash:    10000.0,
		MaxPositionPct: 0.10,
		MaxPositions:   5,
	}

	broker := NewPaperBroker(cfg.InitialCash, store, cfg)

	return broker, store
}

func TestNewBrokerDefaults(t *testing.T) {
	broker, store := setupTest(t)
	defer store.Close()

	state := broker.GetState()

	if state.Cash != 10000.0 {
		t.Errorf("expected cash 10000.0, got %f", state.Cash)
	}

	if len(state.Positions) != 0 {
		t.Errorf("expected 0 positions, got %d", len(state.Positions))
	}

	if broker.TotalValue() != 10000.0 {
		t.Errorf("expected total value 10000.0, got %f", broker.TotalValue())
	}
}

func TestBuySignal(t *testing.T) {
	broker, store := setupTest(t)
	defer store.Close()

	analysis := model.Analysis{
		Symbol:    "AAPL",
		Signal:    model.Buy,
		Reason:    "RSI oversold",
		Timestamp: time.Now(),
	}

	currentPrice := 150.0

	trade, err := broker.ExecuteSignal(analysis, currentPrice)
	if err != nil {
		t.Fatalf("ExecuteSignal failed: %v", err)
	}

	if trade == nil {
		t.Fatal("expected trade, got nil")
	}

	if trade.Symbol != "AAPL" {
		t.Errorf("expected symbol AAPL, got %s", trade.Symbol)
	}

	if trade.Side != "buy" {
		t.Errorf("expected side buy, got %s", trade.Side)
	}

	if trade.Qty != 6 {
		t.Errorf("expected qty 6, got %d", trade.Qty)
	}

	if trade.Price != 150.0 {
		t.Errorf("expected price 150.0, got %f", trade.Price)
	}

	if trade.Total != 900.0 {
		t.Errorf("expected total 900.0, got %f", trade.Total)
	}

	state := broker.GetState()
	if state.Cash != 9100.0 {
		t.Errorf("expected cash 9100.0, got %f", state.Cash)
	}

	if len(state.Positions) != 1 {
		t.Errorf("expected 1 position, got %d", len(state.Positions))
	}

	pos, exists := state.Positions["AAPL"]
	if !exists {
		t.Fatal("position for AAPL not found")
	}

	if pos.Qty != 6 {
		t.Errorf("expected position qty 6, got %d", pos.Qty)
	}

	if pos.AvgCost != 150.0 {
		t.Errorf("expected avg cost 150.0, got %f", pos.AvgCost)
	}
}

func TestSellSignal(t *testing.T) {
	broker, store := setupTest(t)
	defer store.Close()

	// First buy a position.
	buyAnalysis := model.Analysis{
		Symbol:    "AAPL",
		Signal:    model.Buy,
		Reason:    "RSI oversold",
		Timestamp: time.Now(),
	}

	buyTrade, err := broker.ExecuteSignal(buyAnalysis, 150.0)
	if err != nil {
		t.Fatalf("buy failed: %v", err)
	}

	if buyTrade == nil {
		t.Fatal("expected buy trade, got nil")
	}

	// Now sell the position at a higher price.
	sellAnalysis := model.Analysis{
		Symbol:    "AAPL",
		Signal:    model.Sell,
		Reason:    "RSI overbought",
		Timestamp: time.Now(),
	}

	sellTrade, err := broker.ExecuteSignal(sellAnalysis, 160.0)
	if err != nil {
		t.Fatalf("sell failed: %v", err)
	}

	if sellTrade == nil {
		t.Fatal("expected sell trade, got nil")
	}

	if sellTrade.Symbol != "AAPL" {
		t.Errorf("expected symbol AAPL, got %s", sellTrade.Symbol)
	}

	if sellTrade.Side != "sell" {
		t.Errorf("expected side sell, got %s", sellTrade.Side)
	}

	if sellTrade.Qty != 6 {
		t.Errorf("expected qty 6, got %d", sellTrade.Qty)
	}

	if sellTrade.Price != 160.0 {
		t.Errorf("expected price 160.0, got %f", sellTrade.Price)
	}

	if sellTrade.Total != 960.0 {
		t.Errorf("expected total 960.0, got %f", sellTrade.Total)
	}

	state := broker.GetState()
	if state.Cash != 10060.0 {
		t.Errorf("expected cash 10060.0, got %f", state.Cash)
	}

	if len(state.Positions) != 0 {
		t.Errorf("expected 0 positions, got %d", len(state.Positions))
	}
}

func TestHoldSignal(t *testing.T) {
	broker, store := setupTest(t)
	defer store.Close()

	analysis := model.Analysis{
		Symbol:    "AAPL",
		Signal:    model.Hold,
		Reason:    "no signal",
		Timestamp: time.Now(),
	}

	trade, err := broker.ExecuteSignal(analysis, 150.0)
	if err != nil {
		t.Fatalf("ExecuteSignal failed: %v", err)
	}

	if trade != nil {
		t.Errorf("expected nil trade for hold signal, got %+v", trade)
	}

	state := broker.GetState()
	if state.Cash != 10000.0 {
		t.Errorf("expected cash unchanged at 10000.0, got %f", state.Cash)
	}

	if len(state.Positions) != 0 {
		t.Errorf("expected 0 positions, got %d", len(state.Positions))
	}
}

func TestMaxPositions(t *testing.T) {
	broker, store := setupTest(t)
	defer store.Close()

	// Fill up to max positions (5).
	symbols := []string{"AAPL", "MSFT", "GOOGL", "AMZN", "NVDA"}
	for _, symbol := range symbols {
		analysis := model.Analysis{
			Symbol:    symbol,
			Signal:    model.Buy,
			Reason:    "test",
			Timestamp: time.Now(),
		}

		trade, err := broker.ExecuteSignal(analysis, 100.0)
		if err != nil {
			t.Fatalf("buy %s failed: %v", symbol, err)
		}

		if trade == nil {
			t.Fatalf("expected trade for %s, got nil", symbol)
		}
	}

	state := broker.GetState()
	if len(state.Positions) != 5 {
		t.Fatalf("expected 5 positions, got %d", len(state.Positions))
	}

	// Try to buy a 6th position - should be rejected.
	analysis := model.Analysis{
		Symbol:    "META",
		Signal:    model.Buy,
		Reason:    "test",
		Timestamp: time.Now(),
	}

	trade, err := broker.ExecuteSignal(analysis, 100.0)
	if err != nil {
		t.Fatalf("ExecuteSignal failed: %v", err)
	}

	if trade != nil {
		t.Errorf("expected nil trade (max positions reached), got %+v", trade)
	}

	state = broker.GetState()
	if len(state.Positions) != 5 {
		t.Errorf("expected positions to stay at 5, got %d", len(state.Positions))
	}
}

func TestAlreadyHolding(t *testing.T) {
	broker, store := setupTest(t)
	defer store.Close()

	// Buy AAPL.
	analysis := model.Analysis{
		Symbol:    "AAPL",
		Signal:    model.Buy,
		Reason:    "test",
		Timestamp: time.Now(),
	}

	trade1, err := broker.ExecuteSignal(analysis, 150.0)
	if err != nil {
		t.Fatalf("first buy failed: %v", err)
	}

	if trade1 == nil {
		t.Fatal("expected first trade, got nil")
	}

	// Try to buy AAPL again - should be rejected.
	trade2, err := broker.ExecuteSignal(analysis, 150.0)
	if err != nil {
		t.Fatalf("ExecuteSignal failed: %v", err)
	}

	if trade2 != nil {
		t.Errorf("expected nil trade (already holding), got %+v", trade2)
	}

	state := broker.GetState()
	if len(state.Positions) != 1 {
		t.Errorf("expected 1 position, got %d", len(state.Positions))
	}
}

func TestSellWithoutPosition(t *testing.T) {
	broker, store := setupTest(t)
	defer store.Close()

	// Try to sell without holding the position.
	analysis := model.Analysis{
		Symbol:    "AAPL",
		Signal:    model.Sell,
		Reason:    "test",
		Timestamp: time.Now(),
	}

	trade, err := broker.ExecuteSignal(analysis, 150.0)
	if err != nil {
		t.Fatalf("ExecuteSignal failed: %v", err)
	}

	if trade != nil {
		t.Errorf("expected nil trade (no position to sell), got %+v", trade)
	}

	state := broker.GetState()
	if state.Cash != 10000.0 {
		t.Errorf("expected cash unchanged at 10000.0, got %f", state.Cash)
	}
}

func TestPositionSizing(t *testing.T) {
	tests := []struct {
		name          string
		price         float64
		expectedQty   int
		expectedTotal float64
	}{
		{
			name:          "high price stock",
			price:         500.0,
			expectedQty:   2,
			expectedTotal: 1000.0,
		},
		{
			name:          "low price stock",
			price:         10.0,
			expectedQty:   100,
			expectedTotal: 1000.0,
		},
		{
			name:          "medium price stock",
			price:         150.0,
			expectedQty:   6,
			expectedTotal: 900.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh broker for each test.
			dbPath := filepath.Join(t.TempDir(), "test.db")
			store, err := storage.NewSQLiteStore(dbPath)
			if err != nil {
				t.Fatalf("create store: %v", err)
			}
			defer store.Close()

			cfg := config.TradingConfig{
				InitialCash:    10000.0,
				MaxPositionPct: 0.10,
				MaxPositions:   5,
			}

			broker := NewPaperBroker(cfg.InitialCash, store, cfg)

			analysis := model.Analysis{
				Symbol:    "TEST",
				Signal:    model.Buy,
				Reason:    "test",
				Timestamp: time.Now(),
			}

			trade, err := broker.ExecuteSignal(analysis, tt.price)
			if err != nil {
				t.Fatalf("ExecuteSignal failed: %v", err)
			}

			if trade == nil {
				t.Fatal("expected trade, got nil")
			}

			if trade.Qty != tt.expectedQty {
				t.Errorf("expected qty %d, got %d", tt.expectedQty, trade.Qty)
			}

			if trade.Total != tt.expectedTotal {
				t.Errorf("expected total %f, got %f", tt.expectedTotal, trade.Total)
			}
		})
	}
}

func TestUpdatePrices(t *testing.T) {
	broker, store := setupTest(t)
	defer store.Close()

	// Buy two positions.
	symbols := []string{"AAPL", "MSFT"}
	for _, symbol := range symbols {
		analysis := model.Analysis{
			Symbol:    symbol,
			Signal:    model.Buy,
			Reason:    "test",
			Timestamp: time.Now(),
		}

		trade, err := broker.ExecuteSignal(analysis, 100.0)
		if err != nil {
			t.Fatalf("buy %s failed: %v", symbol, err)
		}

		if trade == nil {
			t.Fatalf("expected trade for %s, got nil", symbol)
		}
	}

	// Update prices.
	newPrices := map[string]float64{
		"AAPL": 110.0,
		"MSFT": 120.0,
		"GOOGL": 150.0, // Not held, should be ignored.
	}

	broker.UpdatePrices(newPrices)

	state := broker.GetState()

	aaplPos, exists := state.Positions["AAPL"]
	if !exists {
		t.Fatal("AAPL position not found")
	}

	if aaplPos.CurrentPrice != 110.0 {
		t.Errorf("expected AAPL current price 110.0, got %f", aaplPos.CurrentPrice)
	}

	msftPos, exists := state.Positions["MSFT"]
	if !exists {
		t.Fatal("MSFT position not found")
	}

	if msftPos.CurrentPrice != 120.0 {
		t.Errorf("expected MSFT current price 120.0, got %f", msftPos.CurrentPrice)
	}

	// Verify total value accounts for price updates.
	expectedTotal := state.Cash + aaplPos.MarketValue() + msftPos.MarketValue()
	if broker.TotalValue() != expectedTotal {
		t.Errorf("expected total value %f, got %f", expectedTotal, broker.TotalValue())
	}
}

func TestStateRestore(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := storage.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	cfg := config.TradingConfig{
		InitialCash:    10000.0,
		MaxPositionPct: 0.10,
		MaxPositions:   5,
	}

	// Create first broker and execute some trades.
	broker1 := NewPaperBroker(cfg.InitialCash, store, cfg)

	analysis := model.Analysis{
		Symbol:    "AAPL",
		Signal:    model.Buy,
		Reason:    "test",
		Timestamp: time.Now(),
	}

	trade, err := broker1.ExecuteSignal(analysis, 150.0)
	if err != nil {
		t.Fatalf("ExecuteSignal failed: %v", err)
	}

	if trade == nil {
		t.Fatal("expected trade, got nil")
	}

	// Save state.
	if err := broker1.SaveState(); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	state1 := broker1.GetState()

	// Close the store.
	store.Close()

	// Reopen the store and create a new broker - should restore state.
	store2, err := storage.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer store2.Close()

	broker2 := NewPaperBroker(cfg.InitialCash, store2, cfg)

	state2 := broker2.GetState()

	if state2.Cash != state1.Cash {
		t.Errorf("expected restored cash %f, got %f", state1.Cash, state2.Cash)
	}

	if len(state2.Positions) != len(state1.Positions) {
		t.Errorf("expected %d positions, got %d", len(state1.Positions), len(state2.Positions))
	}

	aaplPos, exists := state2.Positions["AAPL"]
	if !exists {
		t.Fatal("AAPL position not restored")
	}

	origPos := state1.Positions["AAPL"]

	if aaplPos.Qty != origPos.Qty {
		t.Errorf("expected qty %d, got %d", origPos.Qty, aaplPos.Qty)
	}

	if aaplPos.AvgCost != origPos.AvgCost {
		t.Errorf("expected avg cost %f, got %f", origPos.AvgCost, aaplPos.AvgCost)
	}
}
