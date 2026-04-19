package mcp

import (
	"context"
	"fmt"
	"net/http"

	"github.com/kopecmaciej/vi-sql/internal/build"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog/log"
)

// Server wraps the MCP SDK server and exposes database tools to MCP clients.
type Server struct {
	driver  database.Driver
	server  *mcpsdk.Server
	cfg     config.MCPConfig
	manager *manager.ElementManager
}

// New creates a new MCP server backed by the given driver.
func New(driver database.Driver, cfg config.MCPConfig, mgr *manager.ElementManager) *Server {
	s := &Server{
		driver:  driver,
		cfg:     cfg,
		manager: mgr,
		server: mcpsdk.NewServer(&mcpsdk.Implementation{
			Name:    "vi-sql",
			Version: build.Version,
		}, nil),
	}
	s.registerTools()
	return s
}

// Start begins listening on 127.0.0.1:<port>. Blocks until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf("127.0.0.1:%d", s.cfg.Port)

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
