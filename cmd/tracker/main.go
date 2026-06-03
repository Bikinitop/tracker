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

	"golang.org/x/time/rate"

	"github.com/bikinitop/tracker/internal/api"
	"github.com/bikinitop/tracker/internal/circuitbreaker"
	"github.com/bikinitop/tracker/internal/config"
	"github.com/bikinitop/tracker/internal/nats"
	"github.com/bikinitop/tracker/internal/ratelimit"
)

// version is the build version, overridden at release build time via
// -ldflags "-X main.version=<tag>". Defaults to "dev" for local builds.
var version = "dev"

type server struct {
	router    http.Handler
	addr      string
	connector natsConnector
	limiter   *ratelimit.IPRateLimiter
}

type natsConnector interface {
	Publish(subject string, data []byte) error
	Close()
}

func newServer(cfg *config.Config, connectFunc func(string) (natsConnector, error)) (*server, error) {
	var publisher api.EventPublisher
	var connector natsConnector
	if cfg.NATSURL != "" {
		var err error
		connector, err = connectFunc(cfg.NATSURL)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to NATS: %w", err)
		}
		publisher = nats.NewClientWrapper(connector)
	}

	// Guard publishing with a circuit breaker so a sick NATS fast-fails (503).
	if publisher != nil && cfg.CBEnabled {
		breaker := circuitbreaker.New(circuitbreaker.Config{
			FailureRatio:   cfg.CBFailureRatio,
			MinRequests:    cfg.CBMinRequests,
			Window:         cfg.CBWindow,
			OpenDuration:   cfg.CBOpenDuration,
			HalfOpenProbes: cfg.CBHalfOpenProbes,
		})
		publisher = api.NewBreakerPublisher(publisher, breaker)
	}

	// Per-IP rate limiting on /track (429).
	var limiter *ratelimit.IPRateLimiter
	var routerOpts []api.RouterOption
	if cfg.RateLimitEnabled {
		limiter = ratelimit.NewIPRateLimiter(rate.Limit(cfg.RateLimitRPS), cfg.RateLimitBurst, cfg.RateLimitIPTTL)
		routerOpts = append(routerOpts, api.WithRateLimiter(limiter, cfg.TrustProxy))
	}

	router := api.NewRouter(publisher, routerOpts...)
	addr := fmt.Sprintf(":%s", cfg.Port)

	return &server{router: router, addr: addr, connector: connector, limiter: limiter}, nil
}

func runWithContext(ctx context.Context, cfg *config.Config, connectFunc func(string) (natsConnector, error)) error {
	srv, err := newServer(cfg, connectFunc)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	if srv.connector != nil {
		defer srv.connector.Close()
	}

	if srv.limiter != nil {
		defer srv.limiter.Stop()
	}

	server := &http.Server{
		Addr:         srv.addr,
		Handler:      srv.router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown handling
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	log.Printf("tracker version %s starting on %s", version, srv.addr)
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
