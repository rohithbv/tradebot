package engine

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/rohithbv/tradebot/internal/broker"
	"github.com/rohithbv/tradebot/internal/config"
	"github.com/rohithbv/tradebot/internal/market"
	"github.com/rohithbv/tradebot/internal/model"
	"github.com/rohithbv/tradebot/internal/notification"
	"github.com/rohithbv/tradebot/internal/storage"
	"github.com/rohithbv/tradebot/internal/strategy"
)

// Engine orchestrates the trading loop, coordinating market data, strategy evaluation,
// and trade execution. It is safe for concurrent access by the web dashboard.
type Engine struct {
	market   *market.MarketClient
	broker   *broker.PaperBroker
	strategy strategy.Strategy
	store    storage.Store
	cfg      *config.Config
	notifier notification.Notifier

	mu           sync.RWMutex
	lastAnalyses map[string]model.Analysis
	lastPollTime time.Time
	running      bool
	tickCount    int
}

// New creates a new trading engine with the given dependencies.
func New(mkt *market.MarketClient, brk *broker.PaperBroker, strat strategy.Strategy, store storage.Store, cfg *config.Config, notifier notification.Notifier) *Engine {
	return &Engine{
		market:       mkt,
		broker:       brk,
		strategy:     strat,
		store:        store,
		cfg:          cfg,
		notifier:     notifier,
		lastAnalyses: make(map[string]model.Analysis),
	}
}

// Run executes the main trading loop. It checks market hours, polls on a regular interval,
// fetches bars and prices, evaluates strategy signals, executes trades, and persists state.
// It respects context cancellation for graceful shutdown.
func (e *Engine) Run(ctx context.Context) error {
	e.mu.Lock()
	e.running = true
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		e.running = false
		e.mu.Unlock()
	}()

	slog.Info("trading engine starting")

	// Restore last analyses from storage so the watchlist is available
	// immediately (e.g. when the market is closed after a restart).
	if saved, err := e.store.LoadAnalyses(); err != nil {
		slog.Error("failed to load saved analyses", "error", err)
	} else if saved != nil {
		e.mu.Lock()
		e.lastAnalyses = saved
		e.mu.Unlock()
		slog.Info("restored analyses from storage", "count", len(saved))
	}

	// Main loop: check market hours and run trading ticks.
	for {
		select {
		case <-ctx.Done():
			slog.Info("trading engine shutting down")
			if err := e.saveState(); err != nil {
				slog.Error("failed to save final state", "error", err)
			}
			return ctx.Err()
		default:
		}

		// Check if market is open.
		clock, err := e.market.GetClock()
		if err != nil {
			slog.Error("failed to get market clock", "error", err)
			time.Sleep(30 * time.Second)
			continue
		}

		if !clock.IsOpen {
			slog.Info("market is closed, waiting until next open", "next_open", clock.NextChange)
			if err := e.waitUntil(ctx, clock.NextChange); err != nil {
				return err
			}
			continue
		}

		// Market is open, start ticker.
		if err := e.runTradingLoop(ctx); err != nil {
			if err == ctx.Err() {
				slog.Info("trading loop interrupted by context cancellation")
				if err := e.saveState(); err != nil {
					slog.Error("failed to save final state", "error", err)
				}
				return err
			}
			slog.Error("trading loop error", "error", err)
			time.Sleep(30 * time.Second)
			continue
		}
	}
}

// runTradingLoop executes the periodic polling and trading logic while the market is open.
func (e *Engine) runTradingLoop(ctx context.Context) error {
	ticker := time.NewTicker(time.Duration(e.cfg.Trading.PollIntervalSec) * time.Second)
	defer ticker.Stop()

	slog.Info("trading loop started", "poll_interval_sec", e.cfg.Trading.PollIntervalSec)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := e.tick(ctx); err != nil {
				slog.Error("tick failed", "error", err)
			}
		}
	}
}

// tick executes a single trading cycle: fetch data, evaluate signals, execute trades, persist state.
func (e *Engine) tick(ctx context.Context) error {
	slog.Debug("tick starting")

	// Fetch bars for strategy evaluation.
	bars, err := e.market.GetBars(e.cfg.Trading.Watchlist, 50*time.Minute)
	if err != nil {
		return fmt.Errorf("get bars: %w", err)
	}

	// Fetch latest prices.
	latestPrices, err := e.market.GetLatestPrices(e.cfg.Trading.Watchlist)
	if err != nil {
		return fmt.Errorf("get latest prices: %w", err)
	}

	// Update broker with latest prices.
	e.broker.UpdatePrices(latestPrices)

	// Evaluate strategy for each symbol in the watchlist.
	analyses := make(map[string]model.Analysis, len(e.cfg.Trading.Watchlist))
	for _, symbol := range e.cfg.Trading.Watchlist {
		// Extract close prices from bars.
		closes := extractCloses(bars[symbol])
		if len(closes) == 0 {
			slog.Debug("no bars for symbol, skipping", "symbol", symbol)
			continue
		}

		// Evaluate strategy.
		analysis := e.strategy.Evaluate(symbol, closes)
		analyses[symbol] = analysis

		slog.Debug("strategy analysis",
			"symbol", symbol,
			"signal", analysis.Signal.String(),
			"rsi", analysis.RSI,
			"macd", analysis.MACD,
			"macd_signal", analysis.MACDSignal,
			"reason", analysis.Reason,
		)

		// Execute signal if not Hold.
		if analysis.Signal != model.Hold {
			price, ok := latestPrices[symbol]
			if !ok {
				slog.Warn("no price available for signal", "symbol", symbol, "signal", analysis.Signal.String())
				continue
			}

			trade, err := e.broker.ExecuteSignal(analysis, price)
			if err != nil {
				slog.Error("failed to execute signal",
					"symbol", symbol,
					"signal", analysis.Signal.String(),
					"error", err,
				)
				continue
			}

			if trade != nil {
				slog.Info("trade executed",
					"trade_id", trade.ID,
					"symbol", trade.Symbol,
					"side", trade.Side,
					"qty", trade.Qty,
					"price", trade.Price,
					"total", trade.Total,
					"reason", trade.Reason,
				)
				if e.notifier != nil {
					go e.notifier.NotifyTrade(*trade)
				}
			}
		}
	}

	// Store analyses and update poll time.
	e.mu.Lock()
	e.lastAnalyses = analyses
	e.lastPollTime = time.Now()
	e.tickCount++
	tickCount := e.tickCount
	e.mu.Unlock()

	// Every 5th tick, save portfolio snapshot and broker state.
	if tickCount%5 == 0 {
		if err := e.saveState(); err != nil {
			slog.Error("failed to save state", "error", err)
		} else {
			slog.Debug("state saved", "tick", tickCount)
		}
	}

	slog.Debug("tick completed", "tick", tickCount)
	return nil
}

// saveState persists both the portfolio snapshot and broker state.
func (e *Engine) saveState() error {
	// Get broker state for snapshot.
	brokerState := e.broker.GetState()

	// Calculate total value.
	totalValue := brokerState.Cash
	for _, pos := range brokerState.Positions {
		totalValue += pos.MarketValue()
	}

	// Convert positions to snapshots.
	posSnaps := make(map[string]model.PositionSnapshot, len(brokerState.Positions))
	for symbol, pos := range brokerState.Positions {
		posSnaps[symbol] = model.PositionSnapshot{
			Symbol:       pos.Symbol,
			Qty:          pos.Qty,
			AvgCost:      pos.AvgCost,
			CurrentPrice: pos.CurrentPrice,
			MarketValue:  pos.MarketValue(),
			UnrealizedPL: pos.UnrealizedPL(),
		}
	}

	// Save portfolio snapshot.
	snapshot := model.PortfolioSnapshot{
		Timestamp:  time.Now(),
		Cash:       brokerState.Cash,
		TotalValue: totalValue,
		Positions:  posSnaps,
	}
	if err := e.store.SaveSnapshot(snapshot); err != nil {
		return fmt.Errorf("save snapshot: %w", err)
	}

	// Save broker state.
	if err := e.broker.SaveState(); err != nil {
		return fmt.Errorf("save broker state: %w", err)
	}

	// Save latest analyses so watchlist survives restarts.
	e.mu.RLock()
	analysesCopy := make(map[string]model.Analysis, len(e.lastAnalyses))
	for k, v := range e.lastAnalyses {
		analysesCopy[k] = v
	}
	e.mu.RUnlock()
	if err := e.store.SaveAnalyses(analysesCopy); err != nil {
		return fmt.Errorf("save analyses: %w", err)
	}

	return nil
}

// waitUntil sleeps until the target time, respecting context cancellation.
func (e *Engine) waitUntil(ctx context.Context, target time.Time) error {
	duration := time.Until(target)
	if duration <= 0 {
		return nil
	}

	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// GetLastAnalyses returns a deep copy of the most recent strategy analyses.
// Safe for concurrent access by the web dashboard.
func (e *Engine) GetLastAnalyses() map[string]model.Analysis {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Deep copy the analyses map.
	analysesCopy := make(map[string]model.Analysis, len(e.lastAnalyses))
	for k, v := range e.lastAnalyses {
		analysesCopy[k] = v
	}

	return analysesCopy
}

// GetLastPollTime returns the timestamp of the most recent trading tick.
// Safe for concurrent access by the web dashboard.
func (e *Engine) GetLastPollTime() time.Time {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.lastPollTime
}

// IsRunning returns whether the trading engine is currently running.
// Safe for concurrent access by the web dashboard.
func (e *Engine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.running
}

// extractCloses extracts the close prices from a slice of bars.
func extractCloses(bars []model.Bar) []float64 {
	if len(bars) == 0 {
		return nil
	}

	closes := make([]float64, len(bars))
	for i, bar := range bars {
		closes[i] = bar.Close
	}

	return closes
}
