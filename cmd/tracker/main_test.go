package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/bikinitop/tracker/internal/config"
)

type mockConnector struct {
	closed bool
}

func (m *mockConnector) Publish(subject string, data []byte) error {
	return nil
}

func (m *mockConnector) Close() {
	m.closed = true
}

func mockConnectSuccess(url string) (natsConnector, error) {
	return &mockConnector{}, nil
}

func mockConnectError(url string) (natsConnector, error) {
	return nil, errors.New("connection failed")
}

func TestRunWithContext_Success(t *testing.T) {
	cfg := &config.Config{
		Port:    "0",
		NATSURL: "",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- runWithContext(ctx, cfg, mockConnectSuccess)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runWithContext() returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("runWithContext() did not complete in time")
	}
}

func TestRunWithContext_NewServerError(t *testing.T) {
	cfg := &config.Config{
		Port:    "8080",
		NATSURL: "nats://example.com",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := runWithContext(ctx, cfg, mockConnectError)
	if err == nil {
		t.Fatal("expected error for newServer failure")
	}
}

func TestNewServer_DefaultConfig(t *testing.T) {
	cfg := &config.Config{
		Port:    "8080",
		NATSURL: "",
	}

	srv, err := newServer(cfg, mockConnectSuccess)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if srv.addr != ":8080" {
		t.Errorf("expected addr :8080, got %s", srv.addr)
	}

	if srv.router == nil {
		t.Fatal("expected router to be set")
	}
}

func TestNewServer_WithNATS(t *testing.T) {
	cfg := &config.Config{
		Port:    "8080",
		NATSURL: "nats://localhost:4222",
	}

	mock := &mockConnector{}
	connectFunc := func(url string) (natsConnector, error) {
		return mock, nil
	}

	srv, err := newServer(cfg, connectFunc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if srv.addr != ":8080" {
		t.Errorf("expected addr :8080, got %s", srv.addr)
	}

	if srv.connector != mock {
		t.Error("expected connector to be stored in server")
	}

	if mock.closed {
		t.Error("connector should not be closed by newServer; lifecycle managed by runWithContext")
	}
}

func TestNewServer_NATSConnectError(t *testing.T) {
	cfg := &config.Config{
		Port:    "8080",
		NATSURL: "nats://invalid",
	}

	_, err := newServer(cfg, mockConnectError)
	if err == nil {
		t.Fatal("expected error for NATS connection failure")
	}
}

func TestNewServer_InvalidNATS(t *testing.T) {
	cfg := &config.Config{
		Port:    "8080",
		NATSURL: "invalid-url",
	}

	_, err := newServer(cfg, defaultConnect)
	if err == nil {
		t.Fatal("expected error for invalid NATS URL")
	}
}

func TestNewServer_RouterHandlesRequests(t *testing.T) {
	cfg := &config.Config{
		Port:    "8080",
		NATSURL: "",
	}

	srv, err := newServer(cfg, mockConnectSuccess)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	rec := &recordingResponseWriter{}
	srv.router.ServeHTTP(rec, req)

	if rec.status != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.status)
	}
}

func TestServer_Integration(t *testing.T) {
	cfg := &config.Config{
		Port:    "0",
		NATSURL: "",
	}

	srv, err := newServer(cfg, mockConnectSuccess)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	server := &http.Server{
		Addr:    srv.addr,
		Handler: srv.router,
	}

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			t.Logf("Server stopped: %v", err)
		}
	}()
	defer server.Close()

	baseURL := fmt.Sprintf("http://%s", listener.Addr().String())
	time.Sleep(10 * time.Millisecond)

	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("failed to call health endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

type recordingResponseWriter struct {
	status int
	header http.Header
	body   []byte
}

func (r *recordingResponseWriter) Header() http.Header {
	if r.header == nil {
		r.header = make(http.Header)
	}
	return r.header
}

func (r *recordingResponseWriter) Write(b []byte) (int, error) {
	r.body = append(r.body, b...)
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return len(b), nil
}

func (r *recordingResponseWriter) WriteHeader(status int) {
	r.status = status
}

func TestVersionDefault(t *testing.T) {
	if version != "dev" {
		t.Errorf("expected default version %q, got %q", "dev", version)
	}
}
