package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bikinitop/tracker/internal/api"
	"github.com/bikinitop/tracker/internal/config"
	"github.com/bikinitop/tracker/internal/nats"
)

type server struct {
	router http.Handler
	addr   string
}

type natsConnector interface {
	Publish(subject string, data []byte) error
	Close()
}

func newServer(cfg *config.Config, connectFunc func(string) (natsConnector, error)) (*server, error) {
	var publisher api.EventPublisher
	if cfg.NATSURL != "" {
		connector, err := connectFunc(cfg.NATSURL)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to NATS: %w", err)
		}
		defer connector.Close()
		publisher = nats.NewClientWrapper(connector)
	}

	router := api.NewRouter(publisher)
	addr := fmt.Sprintf(":%s", cfg.Port)

	return &server{router: router, addr: addr}, nil
}

func runWithContext(ctx context.Context, cfg *config.Config, connectFunc func(string) (natsConnector, error)) error {
	srv, err := newServer(cfg, connectFunc)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	server := &http.Server{
		Addr:    srv.addr,
		Handler: srv.router,
	}

	// Graceful shutdown handling
	go func() {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			server.Shutdown(shutdownCtx)
		}
	}()

	log.Printf("Starting tracker server on %s", srv.addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server failed: %w", err)
	}

	return nil
}

func defaultConnect(url string) (natsConnector, error) {
	return nats.Connect(url)
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runWithContext(ctx, config.Load(), defaultConnect)
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
