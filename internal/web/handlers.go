package web

import (
	"encoding/json"
	"io/fs"
	"log/slog"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// registerRoutes sets up all HTTP routes on the given mux.
func (s *Server) registerRoutes(mux *http.ServeMux) {
	// Serve embedded static files (index.html, etc.)
	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		slog.Error("failed to get static sub-fs", "error", err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticSub)))

	// API routes.
	mux.HandleFunc("/api/portfolio", s.handlePortfolio)
	mux.HandleFunc("/api/portfolio/history", s.handlePortfolioHistory)
	mux.HandleFunc("/api/trades", s.handleTrades)
	mux.HandleFunc("/api/watchlist", s.handleWatchlist)
	mux.HandleFunc("/api/status", s.handleStatus)
}

// writeJSON writes v as JSON to the response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to write JSON response", "error", err)
	}
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// --------------------------------------------------------------------------
// GET /api/portfolio
// --------------------------------------------------------------------------

type positionResponse struct {
	Symbol       string  `json:"symbol"`
	Qty          int     `json:"qty"`
	AvgCost      float64 `json:"avg_cost"`
	CurrentPrice float64 `json:"current_price"`
	MarketValue  float64 `json:"market_value"`
	UnrealizedPL float64 `json:"unrealized_pl"`
}

type portfolioResponse struct {
	Cash       float64            `json:"cash"`
	TotalValue float64            `json:"total_value"`
	Positions  []positionResponse `json:"positions"`
}

func (s *Server) handlePortfolio(w http.ResponseWriter, r *http.Request) {
	state := s.broker.GetState()
	totalValue := s.broker.TotalValue()

	positions := make([]positionResponse, 0, len(state.Positions))
	for _, pos := range state.Positions {
		positions = append(positions, positionResponse{
			Symbol:       pos.Symbol,
			Qty:          pos.Qty,
			AvgCost:      pos.AvgCost,
			CurrentPrice: pos.CurrentPrice,
			MarketValue:  pos.MarketValue(),
			UnrealizedPL: pos.UnrealizedPL(),
		})
	}

	// Sort positions by symbol for stable output.
	sort.Slice(positions, func(i, j int) bool {
		return positions[i].Symbol < positions[j].Symbol
	})

	writeJSON(w, http.StatusOK, portfolioResponse{
		Cash:       state.Cash,
		TotalValue: totalValue,
		Positions:  positions,
	})
}

// --------------------------------------------------------------------------
// GET /api/portfolio/history
// --------------------------------------------------------------------------

type snapshotResponse struct {
	Timestamp  time.Time `json:"timestamp"`
	Cash       float64   `json:"cash"`
	TotalValue float64   `json:"total_value"`
}

func (s *Server) handlePortfolioHistory(w http.ResponseWriter, r *http.Request) {
	since := time.Now().Add(-24 * time.Hour)
	if v := r.URL.Query().Get("since"); v != "" {
		parsed, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid since parameter: expected RFC3339 format")
			return
		}
		since = parsed
	}

	snapshots, err := s.store.GetSnapshots(since)
	if err != nil {
		slog.Error("failed to get snapshots", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get portfolio history")
		return
	}

	resp := make([]snapshotResponse, 0, len(snapshots))
	for _, snap := range snapshots {
		resp = append(resp, snapshotResponse{
			Timestamp:  snap.Timestamp,
			Cash:       snap.Cash,
			TotalValue: snap.TotalValue,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

// --------------------------------------------------------------------------
// GET /api/trades
// --------------------------------------------------------------------------

type tradeResponse struct {
	ID        string    `json:"id"`
	Symbol    string    `json:"symbol"`
	Side      string    `json:"side"`
	Qty       int       `json:"qty"`
	Price     float64   `json:"price"`
	Total     float64   `json:"total"`
	Reason    string    `json:"reason"`
	Timestamp time.Time `json:"timestamp"`
}

type paginatedTradesResponse struct {
	Trades     []tradeResponse `json:"trades"`
	Total      int             `json:"total"`
	Page       int             `json:"page"`
	PerPage    int             `json:"per_page"`
	TotalPages int             `json:"total_pages"`
}

func (s *Server) handleTrades(w http.ResponseWriter, r *http.Request) {
	var since time.Time
	if v := r.URL.Query().Get("since"); v != "" {
		parsed, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid since parameter: expected RFC3339 format")
			return
		}
		since = parsed
	}

	page := 1
	if v := r.URL.Query().Get("page"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil || parsed < 1 {
			writeError(w, http.StatusBadRequest, "invalid page parameter: expected positive integer")
			return
		}
		page = parsed
	}

	perPage := 10
	if v := r.URL.Query().Get("per_page"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil || parsed < 1 {
			writeError(w, http.StatusBadRequest, "invalid per_page parameter: expected positive integer")
			return
		}
		perPage = parsed
	}

	total, err := s.store.GetTradeCount(since)
	if err != nil {
		slog.Error("failed to count trades", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get trades")
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(perPage)))
	if totalPages == 0 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}

	offset := (page - 1) * perPage
	trades, err := s.store.GetTrades(since, perPage, offset)
	if err != nil {
		slog.Error("failed to get trades", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to get trades")
		return
	}

	items := make([]tradeResponse, 0, len(trades))
	for _, t := range trades {
		items = append(items, tradeResponse{
			ID:        t.ID,
			Symbol:    t.Symbol,
			Side:      t.Side,
			Qty:       t.Qty,
			Price:     t.Price,
			Total:     t.Total,
			Reason:    t.Reason,
			Timestamp: t.Timestamp,
		})
	}

	writeJSON(w, http.StatusOK, paginatedTradesResponse{
		Trades:     items,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	})
}

// --------------------------------------------------------------------------
// GET /api/watchlist
// --------------------------------------------------------------------------

type watchlistItem struct {
	Symbol     string  `json:"symbol"`
	RSI        float64 `json:"rsi"`
	MACD       float64 `json:"macd"`
	MACDSignal float64 `json:"macd_signal"`
	Signal     string  `json:"signal"`
	Held       bool    `json:"held"`
}

func (s *Server) handleWatchlist(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleWatchlistGet(w, r)
	case http.MethodPost:
		s.handleWatchlistPost(w, r)
	case http.MethodDelete:
		s.handleWatchlistDelete(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleWatchlistGet(w http.ResponseWriter, r *http.Request) {
	if s.engine == nil {
		writeJSON(w, http.StatusOK, []watchlistItem{})
		return
	}

	analyses := s.engine.GetLastAnalyses()
	state := s.broker.GetState()
	watchlist := s.engine.GetWatchlist()

	// Build items from watchlist so newly added symbols appear immediately.
	items := make([]watchlistItem, 0, len(watchlist))
	for _, sym := range watchlist {
		item := watchlistItem{Symbol: sym}
		if a, ok := analyses[sym]; ok {
			item.RSI = a.RSI
			item.MACD = a.MACD
			item.MACDSignal = a.MACDSignal
			item.Signal = a.Signal.String()
		} else {
			item.Signal = "hold"
		}
		_, item.Held = state.Positions[sym]
		items = append(items, item)
	}

	// Sort by symbol for stable output.
	sort.Slice(items, func(i, j int) bool {
		return items[i].Symbol < items[j].Symbol
	})

	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleWatchlistPost(w http.ResponseWriter, r *http.Request) {
	if s.engine == nil {
		writeError(w, http.StatusServiceUnavailable, "engine not ready")
		return
	}

	var req struct {
		Symbol string `json:"symbol"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	symbol := strings.ToUpper(strings.TrimSpace(req.Symbol))
	if symbol == "" {
		writeError(w, http.StatusBadRequest, "symbol is required")
		return
	}

	// Validate against Alpaca API.
	if s.market != nil {
		if err := s.market.ValidateSymbol(symbol); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	if err := s.engine.AddSymbol(symbol); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "added", "symbol": symbol})
}

func (s *Server) handleWatchlistDelete(w http.ResponseWriter, r *http.Request) {
	if s.engine == nil {
		writeError(w, http.StatusServiceUnavailable, "engine not ready")
		return
	}

	symbol := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("symbol")))
	if symbol == "" {
		writeError(w, http.StatusBadRequest, "symbol query parameter is required")
		return
	}

	if err := s.engine.RemoveSymbol(symbol); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "removed", "symbol": symbol})
}

// --------------------------------------------------------------------------
// GET /api/status
// --------------------------------------------------------------------------

type statusResponse struct {
	Running      bool       `json:"running"`
	MarketOpen   *bool      `json:"market_open"`
	LastPollTime *time.Time `json:"last_poll_time"`
	Uptime       string     `json:"uptime"`
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	resp := statusResponse{
		Running: false,
		Uptime:  time.Since(s.start).Round(time.Second).String(),
	}

	if s.engine != nil {
		resp.Running = s.engine.IsRunning()
		lpt := s.engine.GetLastPollTime()
		if !lpt.IsZero() {
			resp.LastPollTime = &lpt
		}
	}

	writeJSON(w, http.StatusOK, resp)
}
