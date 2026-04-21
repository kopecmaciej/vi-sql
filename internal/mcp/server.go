package mcp

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/kopecmaciej/vi-sql/internal/build"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog/log"
)

// Server wraps the MCP SDK server and exposes database tools to MCP clients.
type Server struct {
	driver      database.Driver
	server      *mcpsdk.Server
	cfg         config.MCPConfig
	manager     *manager.ElementManager
	tabRegistry *manager.TabRegistry
	resultMu    sync.RWMutex
	lastResult  *manager.QueryResult
	eventCh     chan manager.EventMsg
}

func New(driver database.Driver, cfg config.MCPConfig, mgr *manager.ElementManager, reg *manager.TabRegistry) *Server {
	s := &Server{
		driver:      driver,
		cfg:         cfg,
		manager:     mgr,
		tabRegistry: reg,
		server: mcpsdk.NewServer(&mcpsdk.Implementation{
			Name:    "vi-sql",
			Version: build.Version,
		}, nil),
	}
	if mgr != nil {
		s.eventCh = mgr.Subscribe("MCPResultStore")
	}
	s.registerTools()
	return s
}

func (s *Server) listenEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-s.eventCh:
			if event.Message.Type != manager.QueryExecuted {
				continue
			}
			qr, ok := event.Message.Data.(manager.QueryResult)
			if !ok {
				continue
			}
			t := time.Now()
			qr.ExecutedAt = &t
			s.resultMu.Lock()
			s.lastResult = &qr
			s.resultMu.Unlock()
		}
	}
}

func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf("127.0.0.1:%d", s.cfg.Port)

	if s.eventCh != nil {
		go s.listenEvents(ctx)
	}

	handler := mcpsdk.NewStreamableHTTPHandler(func(r *http.Request) *mcpsdk.Server {
		return s.server
	}, nil)

	srv := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	go func() {
		<-ctx.Done()
		if err := srv.Shutdown(context.Background()); err != nil {
			log.Error().Err(err).Msg("MCP server shutdown error")
		}
	}()

	log.Info().Str("addr", addr).Msg("MCP server started")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("MCP server: %w", err)
	}
	return nil
}
