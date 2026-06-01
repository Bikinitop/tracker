package nats

import (
	"errors"
	"testing"

	"github.com/nats-io/nats-server/v2/test"
)

type mockNatsConn struct {
	published []publishCall
	closed    bool
}

type publishCall struct {
	subject string
	data    []byte
}

func (m *mockNatsConn) Publish(subj string, data []byte) error {
	m.published = append(m.published, publishCall{subject: subj, data: data})
	return nil
}

func (m *mockNatsConn) Close() {
	m.closed = true
}

func TestConnector_Publish(t *testing.T) {
	mock := &mockNatsConn{}
	c := &Connector{conn: mock}

	err := c.Publish("test.subject", []byte("hello"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.published) != 1 {
		t.Fatalf("expected 1 publish, got %d", len(mock.published))
	}

	if mock.published[0].subject != "test.subject" {
		t.Errorf("expected subject test.subject, got %s", mock.published[0].subject)
	}

	if string(mock.published[0].data) != "hello" {
		t.Errorf("expected data hello, got %s", string(mock.published[0].data))
	}
}

func TestConnector_Publish_Error(t *testing.T) {
	failingConn := &failingNatsConn{}
	c := &Connector{conn: failingConn}

	err := c.Publish("test.subject", []byte("hello"))
	if err == nil {
		t.Fatal("expected error for failed publish")
	}
}

func TestConnector_Close(t *testing.T) {
	mock := &mockNatsConn{}
	c := &Connector{conn: mock}

	c.Close()

	if !mock.closed {
		t.Error("expected connection to be closed")
	}
}

type failingNatsConn struct{}

func (f *failingNatsConn) Publish(subj string, data []byte) error {
	return errors.New("publish failed")
}

func (f *failingNatsConn) Close() {}

func TestConnect_InvalidURL(t *testing.T) {
	_, err := Connect("invalid-url")
	if err == nil {
		t.Fatal("expected error for invalid NATS URL")
	}
}

func TestConnect_Success(t *testing.T) {
	s := test.RunRandClientPortServer()
	defer s.Shutdown()

	connector, err := Connect(s.ClientURL())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if connector == nil {
		t.Fatal("expected connector to be non-nil")
	}

	connector.Close()
}
