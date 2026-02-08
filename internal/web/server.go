package web

import (
	"context"
	"embed"
	"log/slog"
	"net/http"
	"time"

	"github.com/rohithbv/tradebot/internal/broker"
	"github.com/rohithbv/tradebot/internal/config"
	"github.com/rohithbv/tradebot/internal/model"
	"github.com/rohithbv/tradebot/internal/storage"
)

//go:embed static
var staticFS embed.FS

// EngineReader provides read access to engine state for the web layer.
type EngineReader interface {
	GetLastAnalyses() map[string]model.Analysis
	GetLastPollTime() time.Time
	IsRunning() bool
}

// Server is the HTTP server that serves the web dashboard and API endpoints.
type Server struct {
	cfg    config.WebConfig
	broker *broker.PaperBroker
	store  storage.Store
	engine EngineReader
	srv    *http.Server
	start  time.Time
}

// NewServer creates a new web server with the given configuration, broker, and store.
func NewServer(cfg config.WebConfig, brk *broker.PaperBroker, store storage.Store) *Server {
	s := &Server{
		cfg:    cfg,
		broker: brk,
		store:  store,
		start:  time.Now(),
	}
	mux := http.NewServeMux()
	s.registerRoutes(mux)
	s.srv = &http.Server{
		Addr:    cfg.Addr,
		Handler: mux,
	}
	return s
}

// SetEngine sets the engine reader so the web layer can access engine state.
func (s *Server) SetEngine(e EngineReader) {
	s.engine = e
}

// Start begins serving HTTP requests and blocks until ctx is cancelled,
// at which point it performs a graceful shutdown with a 5-second timeout.
func (s *Server) Start(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		slog.Info("web server starting", "addr", s.cfg.Addr)
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	slog.Info("web server shutting down")
	return s.srv.Shutdown(shutdownCtx)
}
