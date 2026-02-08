package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rohithbv/tradebot/internal/model"
	_ "modernc.org/sqlite"
)

// Store defines the persistence interface for the tradebot.
type Store interface {
	SaveTrade(trade model.Trade) error
	GetTrades(since time.Time, limit int) ([]model.Trade, error)
	SaveSnapshot(snap model.PortfolioSnapshot) error
	GetSnapshots(since time.Time) ([]model.PortfolioSnapshot, error)
	SaveBars(symbol string, bars []model.Bar) error
	GetBars(symbol string, since time.Time) ([]model.Bar, error)
	SaveState(state model.BrokerState) error
	LoadState() (*model.BrokerState, error)
	Close() error
}

// SQLiteStore implements Store backed by a SQLite database.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens (or creates) a SQLite database at dbPath, enables WAL
// mode, creates the required tables, and returns a ready-to-use store.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	// Enable WAL mode for better concurrent read performance.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable WAL mode: %w", err)
	}

	if err := createTables(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("create tables: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

func createTables(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS trades (
			id         TEXT PRIMARY KEY,
			symbol     TEXT NOT NULL,
			side       TEXT NOT NULL,
			qty        INTEGER NOT NULL,
			price      REAL NOT NULL,
			total      REAL NOT NULL,
			reason     TEXT NOT NULL,
			timestamp  DATETIME NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_trades_timestamp ON trades(timestamp)`,

		`CREATE TABLE IF NOT EXISTS snapshots (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp   DATETIME NOT NULL,
			cash        REAL NOT NULL,
			total_value REAL NOT NULL,
			positions   TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_snapshots_timestamp ON snapshots(timestamp)`,

		`CREATE TABLE IF NOT EXISTS bars (
			symbol    TEXT NOT NULL,
			timestamp DATETIME NOT NULL,
			open      REAL NOT NULL,
			high      REAL NOT NULL,
			low       REAL NOT NULL,
			close     REAL NOT NULL,
			volume    INTEGER NOT NULL,
			PRIMARY KEY (symbol, timestamp)
		)`,

		`CREATE TABLE IF NOT EXISTS state (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt[:40], err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Trades
// ---------------------------------------------------------------------------

// SaveTrade inserts a single trade record.
func (s *SQLiteStore) SaveTrade(trade model.Trade) error {
	_, err := s.db.Exec(
		`INSERT INTO trades (id, symbol, side, qty, price, total, reason, timestamp)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		trade.ID,
		trade.Symbol,
		trade.Side,
		trade.Qty,
		trade.Price,
		trade.Total,
		trade.Reason,
		trade.Timestamp.UTC(),
	)
	if err != nil {
		return fmt.Errorf("insert trade: %w", err)
	}
	return nil
}

// GetTrades returns up to limit trades with a timestamp >= since, ordered by
// timestamp descending (most recent first).
func (s *SQLiteStore) GetTrades(since time.Time, limit int) ([]model.Trade, error) {
	rows, err := s.db.Query(
		`SELECT id, symbol, side, qty, price, total, reason, timestamp
		 FROM trades
		 WHERE timestamp >= ?
		 ORDER BY timestamp DESC
		 LIMIT ?`,
		since.UTC(), limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query trades: %w", err)
	}
	defer rows.Close()

	var trades []model.Trade
	for rows.Next() {
		var t model.Trade
		if err := rows.Scan(&t.ID, &t.Symbol, &t.Side, &t.Qty, &t.Price, &t.Total, &t.Reason, &t.Timestamp); err != nil {
			return nil, fmt.Errorf("scan trade: %w", err)
		}
		trades = append(trades, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trades: %w", err)
	}
	return trades, nil
}

// ---------------------------------------------------------------------------
// Snapshots
// ---------------------------------------------------------------------------

// SaveSnapshot persists a portfolio snapshot. The Positions map is stored as
// a JSON string.
func (s *SQLiteStore) SaveSnapshot(snap model.PortfolioSnapshot) error {
	posJSON, err := json.Marshal(snap.Positions)
	if err != nil {
		return fmt.Errorf("marshal positions: %w", err)
	}

	_, err = s.db.Exec(
		`INSERT INTO snapshots (timestamp, cash, total_value, positions)
		 VALUES (?, ?, ?, ?)`,
		snap.Timestamp.UTC(),
		snap.Cash,
		snap.TotalValue,
		string(posJSON),
	)
	if err != nil {
		return fmt.Errorf("insert snapshot: %w", err)
	}
	return nil
}

// GetSnapshots returns all snapshots with a timestamp >= since, ordered by
// timestamp ascending.
func (s *SQLiteStore) GetSnapshots(since time.Time) ([]model.PortfolioSnapshot, error) {
	rows, err := s.db.Query(
		`SELECT id, timestamp, cash, total_value, positions
		 FROM snapshots
		 WHERE timestamp >= ?
		 ORDER BY timestamp ASC`,
		since.UTC(),
	)
	if err != nil {
		return nil, fmt.Errorf("query snapshots: %w", err)
	}
	defer rows.Close()

	var snaps []model.PortfolioSnapshot
	for rows.Next() {
		var snap model.PortfolioSnapshot
		var posJSON string
		if err := rows.Scan(&snap.ID, &snap.Timestamp, &snap.Cash, &snap.TotalValue, &posJSON); err != nil {
			return nil, fmt.Errorf("scan snapshot: %w", err)
		}
		if err := json.Unmarshal([]byte(posJSON), &snap.Positions); err != nil {
			return nil, fmt.Errorf("unmarshal positions: %w", err)
		}
		snaps = append(snaps, snap)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate snapshots: %w", err)
	}
	return snaps, nil
}

// ---------------------------------------------------------------------------
// Bars
// ---------------------------------------------------------------------------

// SaveBars batch-inserts (or replaces) OHLCV bars for a given symbol.
func (s *SQLiteStore) SaveBars(symbol string, bars []model.Bar) error {
	if len(bars) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	stmt, err := tx.Prepare(
		`INSERT OR REPLACE INTO bars (symbol, timestamp, open, high, low, close, volume)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return fmt.Errorf("prepare insert bars: %w", err)
	}
	defer stmt.Close()

	for _, b := range bars {
		if _, err := stmt.Exec(symbol, b.Timestamp.UTC(), b.Open, b.High, b.Low, b.Close, b.Volume); err != nil {
			return fmt.Errorf("insert bar %s %v: %w", symbol, b.Timestamp, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit bars tx: %w", err)
	}
	return nil
}

// GetBars returns bars for the given symbol with a timestamp >= since, ordered
// by timestamp ascending (oldest first).
func (s *SQLiteStore) GetBars(symbol string, since time.Time) ([]model.Bar, error) {
	rows, err := s.db.Query(
		`SELECT symbol, timestamp, open, high, low, close, volume
		 FROM bars
		 WHERE symbol = ? AND timestamp >= ?
		 ORDER BY timestamp ASC`,
		symbol, since.UTC(),
	)
	if err != nil {
		return nil, fmt.Errorf("query bars: %w", err)
	}
	defer rows.Close()

	var bars []model.Bar
	for rows.Next() {
		var b model.Bar
		if err := rows.Scan(&b.Symbol, &b.Timestamp, &b.Open, &b.High, &b.Low, &b.Close, &b.Volume); err != nil {
			return nil, fmt.Errorf("scan bar: %w", err)
		}
		bars = append(bars, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate bars: %w", err)
	}
	return bars, nil
}

// ---------------------------------------------------------------------------
// State (BrokerState persistence)
// ---------------------------------------------------------------------------

const stateKey = "portfolio"

// SaveState persists the broker state as a JSON blob under the "portfolio" key.
func (s *SQLiteStore) SaveState(state model.BrokerState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal broker state: %w", err)
	}

	_, err = s.db.Exec(
		`INSERT OR REPLACE INTO state (key, value) VALUES (?, ?)`,
		stateKey, string(data),
	)
	if err != nil {
		return fmt.Errorf("upsert state: %w", err)
	}
	return nil
}

// LoadState loads the broker state from the database. If no state has been
// saved yet, it returns (nil, nil).
func (s *SQLiteStore) LoadState() (*model.BrokerState, error) {
	var raw string
	err := s.db.QueryRow(
		`SELECT value FROM state WHERE key = ?`, stateKey,
	).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query state: %w", err)
	}

	var state model.BrokerState
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return nil, fmt.Errorf("unmarshal broker state: %w", err)
	}
	return &state, nil
}

// Close closes the underlying database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
