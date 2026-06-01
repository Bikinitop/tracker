package nats

import (
	"fmt"

	"github.com/nats-io/nats.go"
)

// natsConn abstracts the nats.Conn for testability
type natsConn interface {
	Publish(subj string, data []byte) error
	Close()
}

// Connector wraps a real NATS connection
type Connector struct {
	conn natsConn
}

// Connect establishes a connection to NATS server
func Connect(url string) (*Connector, error) {
	conn, err := nats.Connect(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}
	return &Connector{conn: conn}, nil
}

// Publish sends data to a NATS subject
func (c *Connector) Publish(subject string, data []byte) error {
	return c.conn.Publish(subject, data)
}

// Close closes the NATS connection
func (c *Connector) Close() {
	c.conn.Close()
}
