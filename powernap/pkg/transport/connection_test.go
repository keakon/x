package transport

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/sourcegraph/jsonrpc2"
)

func TestClosedConnectionReportsTransportError(t *testing.T) {
	local, peer := net.Pipe()
	t.Cleanup(func() { _ = peer.Close() })
	conn, err := NewConnection(context.Background(), local, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		call func() error
	}{
		{"call", func() error { return conn.Call(context.Background(), "sample/request", nil, nil) }},
		{"notify", func() error { return conn.Notify(context.Background(), "sample/notification", nil) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); !errors.Is(err, jsonrpc2.ErrClosed) {
				t.Fatalf("error = %v, want jsonrpc2.ErrClosed in the error chain", err)
			}
		})
	}
}
