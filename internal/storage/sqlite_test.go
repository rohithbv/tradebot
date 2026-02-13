package storage

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/rohithbv/tradebot/internal/model"
)

// newTestStore creates a fresh SQLiteStore backed by a temporary database.
func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

// ---------------------------------------------------------------------------
// Trades
// ---------------------------------------------------------------------------

func TestSaveAndGetTrades(t *testing.T) {
	store := newTestStore(t)

	now := time.Now().UTC().Truncate(time.Second)
	trade := model.Trade{
		ID:        "t-001",
		Symbol:    "AAPL",
		Side:      "buy",
		Qty:       10,
		Price:     150.25,
		Total:     1502.50,
		Reason:    "RSI oversold",
		Timestamp: now,
	}

	if err := store.SaveTrade(trade); err != nil {
		t.Fatalf("SaveTrade: %v", err)
	}

	trades, err := store.GetTrades(now.Add(-time.Hour), 10, 0)
	if err != nil {
		t.Fatalf("GetTrades: %v", err)
	}
	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}

	got := trades[0]
	if got.ID != trade.ID {
		t.Errorf("ID: got %q, want %q", got.ID, trade.ID)
	}
	if got.Symbol != trade.Symbol {
		t.Errorf("Symbol: got %q, want %q", got.Symbol, trade.Symbol)
	}
	if got.Side != trade.Side {
		t.Errorf("Side: got %q, want %q", got.Side, trade.Side)
	}
	if got.Qty != trade.Qty {
		t.Errorf("Qty: got %d, want %d", got.Qty, trade.Qty)
	}
	if got.Price != trade.Price {
		t.Errorf("Price: got %f, want %f", got.Price, trade.Price)
	}
	if got.Total != trade.Total {
		t.Errorf("Total: got %f, want %f", got.Total, trade.Total)
	}
	if got.Reason != trade.Reason {
		t.Errorf("Reason: got %q, want %q", got.Reason, trade.Reason)
	}
	if !got.Timestamp.Equal(trade.Timestamp) {
		t.Errorf("Timestamp: got %v, want %v", got.Timestamp, trade.Timestamp)
	}
}

func TestGetTradesSince(t *testing.T) {
	store := newTestStore(t)

	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	trades := []model.Trade{
		{ID: "t-1", Symbol: "AAPL", Side: "buy", Qty: 5, Price: 100, Total: 500, Reason: "test", Timestamp: base},
		{ID: "t-2", Symbol: "GOOG", Side: "sell", Qty: 3, Price: 200, Total: 600, Reason: "test", Timestamp: base.Add(1 * time.Hour)},
		{ID: "t-3", Symbol: "MSFT", Side: "buy", Qty: 7, Price: 300, Total: 2100, Reason: "test", Timestamp: base.Add(2 * time.Hour)},
	}

	for _, tr := range trades {
		if err := store.SaveTrade(tr); err != nil {
			t.Fatalf("SaveTrade %s: %v", tr.ID, err)
		}
	}

	// Filter: only trades at or after base + 1h should return t-2 and t-3.
	got, err := store.GetTrades(base.Add(1*time.Hour), 10, 0)
	if err != nil {
		t.Fatalf("GetTrades: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 trades, got %d", len(got))
	}
	// Results are ordered DESC, so newest first.
	if got[0].ID != "t-3" {
		t.Errorf("first trade ID: got %q, want %q", got[0].ID, "t-3")
	}
	if got[1].ID != "t-2" {
		t.Errorf("second trade ID: got %q, want %q", got[1].ID, "t-2")
	}

	// Test limit.
	got, err = store.GetTrades(base, 2, 0)
	if err != nil {
		t.Fatalf("GetTrades with limit: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 trades (limit), got %d", len(got))
	}
}

// ---------------------------------------------------------------------------
// Snapshots
// ---------------------------------------------------------------------------

func TestSaveAndGetSnapshots(t *testing.T) {
	store := newTestStore(t)

	now := time.Now().UTC().Truncate(time.Second)
	snap := model.PortfolioSnapshot{
		Timestamp:  now,
		Cash:       50000.0,
		TotalValue: 75000.0,
		Positions: map[string]model.PositionSnapshot{
			"AAPL": {
				Symbol:       "AAPL",
				Qty:          10,
				AvgCost:      145.0,
				CurrentPrice: 150.0,
				MarketValue:  1500.0,
				UnrealizedPL: 50.0,
			},
			"GOOG": {
				Symbol:       "GOOG",
				Qty:          5,
				AvgCost:      2800.0,
				CurrentPrice: 2850.0,
				MarketValue:  14250.0,
				UnrealizedPL: 250.0,
			},
		},
	}

	if err := store.SaveSnapshot(snap); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}

	snaps, err := store.GetSnapshots(now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("GetSnapshots: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}

	got := snaps[0]
	if got.Cash != snap.Cash {
		t.Errorf("Cash: got %f, want %f", got.Cash, snap.Cash)
	}
	if got.TotalValue != snap.TotalValue {
		t.Errorf("TotalValue: got %f, want %f", got.TotalValue, snap.TotalValue)
	}
	if !got.Timestamp.Equal(snap.Timestamp) {
		t.Errorf("Timestamp: got %v, want %v", got.Timestamp, snap.Timestamp)
	}
	if got.ID == 0 {
		t.Error("expected non-zero snapshot ID from AUTOINCREMENT")
	}

	// Verify positions round-tripped through JSON correctly.
	if len(got.Positions) != 2 {
		t.Fatalf("expected 2 positions, got %d", len(got.Positions))
	}
	aapl, ok := got.Positions["AAPL"]
	if !ok {
		t.Fatal("missing AAPL position")
	}
	if aapl.Qty != 10 {
		t.Errorf("AAPL Qty: got %d, want 10", aapl.Qty)
	}
	if aapl.AvgCost != 145.0 {
		t.Errorf("AAPL AvgCost: got %f, want 145.0", aapl.AvgCost)
	}
	if aapl.CurrentPrice != 150.0 {
		t.Errorf("AAPL CurrentPrice: got %f, want 150.0", aapl.CurrentPrice)
	}
	if aapl.MarketValue != 1500.0 {
		t.Errorf("AAPL MarketValue: got %f, want 1500.0", aapl.MarketValue)
	}
	if aapl.UnrealizedPL != 50.0 {
		t.Errorf("AAPL UnrealizedPL: got %f, want 50.0", aapl.UnrealizedPL)
	}

	goog, ok := got.Positions["GOOG"]
	if !ok {
		t.Fatal("missing GOOG position")
	}
	if goog.Qty != 5 {
		t.Errorf("GOOG Qty: got %d, want 5", goog.Qty)
	}
}

// ---------------------------------------------------------------------------
// Bars
// ---------------------------------------------------------------------------

func TestSaveAndGetBars(t *testing.T) {
	store := newTestStore(t)

	base := time.Date(2025, 6, 1, 9, 30, 0, 0, time.UTC)
	bars := []model.Bar{
		{Symbol: "AAPL", Timestamp: base, Open: 150, High: 152, Low: 149, Close: 151, Volume: 1000},
		{Symbol: "AAPL", Timestamp: base.Add(1 * time.Minute), Open: 151, High: 153, Low: 150, Close: 152, Volume: 1100},
		{Symbol: "AAPL", Timestamp: base.Add(2 * time.Minute), Open: 152, High: 154, Low: 151, Close: 153, Volume: 1200},
	}

	if err := store.SaveBars("AAPL", bars); err != nil {
		t.Fatalf("SaveBars: %v", err)
	}

	// Also save a bar for a different symbol to verify filtering.
	otherBars := []model.Bar{
		{Symbol: "GOOG", Timestamp: base, Open: 2800, High: 2810, Low: 2790, Close: 2805, Volume: 500},
	}
	if err := store.SaveBars("GOOG", otherBars); err != nil {
		t.Fatalf("SaveBars GOOG: %v", err)
	}

	// Retrieve only AAPL bars.
	got, err := store.GetBars("AAPL", base)
	if err != nil {
		t.Fatalf("GetBars: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 AAPL bars, got %d", len(got))
	}

	// Bars should be ordered ASC by timestamp.
	if !got[0].Timestamp.Equal(base) {
		t.Errorf("first bar timestamp: got %v, want %v", got[0].Timestamp, base)
	}
	if got[0].Open != 150 {
		t.Errorf("first bar Open: got %f, want 150", got[0].Open)
	}
	if got[0].Volume != 1000 {
		t.Errorf("first bar Volume: got %d, want 1000", got[0].Volume)
	}
	if got[2].Close != 153 {
		t.Errorf("third bar Close: got %f, want 153", got[2].Close)
	}

	// Filter by time: only bars at or after base+1m.
	got, err = store.GetBars("AAPL", base.Add(1*time.Minute))
	if err != nil {
		t.Fatalf("GetBars since: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 bars after filter, got %d", len(got))
	}

	// Verify GOOG bars are separate.
	googBars, err := store.GetBars("GOOG", base)
	if err != nil {
		t.Fatalf("GetBars GOOG: %v", err)
	}
	if len(googBars) != 1 {
		t.Fatalf("expected 1 GOOG bar, got %d", len(googBars))
	}
}

func TestSaveBarsReplace(t *testing.T) {
	store := newTestStore(t)

	ts := time.Date(2025, 6, 1, 9, 30, 0, 0, time.UTC)
	bars := []model.Bar{
		{Symbol: "AAPL", Timestamp: ts, Open: 150, High: 152, Low: 149, Close: 151, Volume: 1000},
	}
	if err := store.SaveBars("AAPL", bars); err != nil {
		t.Fatalf("SaveBars: %v", err)
	}

	// Save again with updated close price -- should replace, not error.
	bars[0].Close = 155
	if err := store.SaveBars("AAPL", bars); err != nil {
		t.Fatalf("SaveBars replace: %v", err)
	}

	got, err := store.GetBars("AAPL", ts)
	if err != nil {
		t.Fatalf("GetBars: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 bar after replace, got %d", len(got))
	}
	if got[0].Close != 155 {
		t.Errorf("Close after replace: got %f, want 155", got[0].Close)
	}
}

func TestSaveBarsEmpty(t *testing.T) {
	store := newTestStore(t)
	// Saving an empty slice should be a no-op.
	if err := store.SaveBars("AAPL", nil); err != nil {
		t.Fatalf("SaveBars empty: %v", err)
	}
}

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------

func TestSaveAndLoadState(t *testing.T) {
	store := newTestStore(t)

	state := model.BrokerState{
		Cash: 100000.0,
		Positions: map[string]model.Position{
			"AAPL": {Symbol: "AAPL", Qty: 10, AvgCost: 145.0, CurrentPrice: 150.0},
			"MSFT": {Symbol: "MSFT", Qty: 20, AvgCost: 300.0, CurrentPrice: 310.0},
		},
	}

	if err := store.SaveState(state); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	got, err := store.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if got == nil {
		t.Fatal("LoadState returned nil")
	}
	if got.Cash != state.Cash {
		t.Errorf("Cash: got %f, want %f", got.Cash, state.Cash)
	}
	if len(got.Positions) != 2 {
		t.Fatalf("expected 2 positions, got %d", len(got.Positions))
	}

	aapl, ok := got.Positions["AAPL"]
	if !ok {
		t.Fatal("missing AAPL position")
	}
	if aapl.Qty != 10 {
		t.Errorf("AAPL Qty: got %d, want 10", aapl.Qty)
	}
	if aapl.AvgCost != 145.0 {
		t.Errorf("AAPL AvgCost: got %f, want 145.0", aapl.AvgCost)
	}
	if aapl.CurrentPrice != 150.0 {
		t.Errorf("AAPL CurrentPrice: got %f, want 150.0", aapl.CurrentPrice)
	}

	msft, ok := got.Positions["MSFT"]
	if !ok {
		t.Fatal("missing MSFT position")
	}
	if msft.Qty != 20 {
		t.Errorf("MSFT Qty: got %d, want 20", msft.Qty)
	}

	// Update state and verify it overwrites.
	state.Cash = 90000.0
	if err := store.SaveState(state); err != nil {
		t.Fatalf("SaveState update: %v", err)
	}
	got2, err := store.LoadState()
	if err != nil {
		t.Fatalf("LoadState after update: %v", err)
	}
	if got2.Cash != 90000.0 {
		t.Errorf("Cash after update: got %f, want 90000.0", got2.Cash)
	}
}

func TestLoadStateEmpty(t *testing.T) {
	store := newTestStore(t)

	got, err := store.LoadState()
	if err != nil {
		t.Fatalf("LoadState on empty DB: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil state from empty DB, got %+v", got)
	}
}

// ---------------------------------------------------------------------------
// Watchlist
// ---------------------------------------------------------------------------

func TestSaveAndLoadWatchlist(t *testing.T) {
	store := newTestStore(t)

	symbols := []string{"AAPL", "GOOG", "MSFT"}
	if err := store.SaveWatchlist(symbols); err != nil {
		t.Fatalf("SaveWatchlist: %v", err)
	}

	got, err := store.LoadWatchlist()
	if err != nil {
		t.Fatalf("LoadWatchlist: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 symbols, got %d", len(got))
	}
	for i, s := range symbols {
		if got[i] != s {
			t.Errorf("symbol[%d]: got %q, want %q", i, got[i], s)
		}
	}

	// Update and verify overwrite.
	updated := []string{"AAPL", "TSLA"}
	if err := store.SaveWatchlist(updated); err != nil {
		t.Fatalf("SaveWatchlist update: %v", err)
	}
	got2, err := store.LoadWatchlist()
	if err != nil {
		t.Fatalf("LoadWatchlist after update: %v", err)
	}
	if len(got2) != 2 {
		t.Fatalf("expected 2 symbols after update, got %d", len(got2))
	}
	if got2[0] != "AAPL" || got2[1] != "TSLA" {
		t.Errorf("unexpected symbols after update: %v", got2)
	}
}

func TestLoadWatchlistEmpty(t *testing.T) {
	store := newTestStore(t)

	got, err := store.LoadWatchlist()
	if err != nil {
		t.Fatalf("LoadWatchlist on empty DB: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil watchlist from empty DB, got %v", got)
	}
}
