package transport

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/sourcegraph/jsonrpc2"
)

// decodeRequest builds the *jsonrpc2.Request that Route would receive for the
// given wire message, so these tests exercise jsonrpc2's own decoding of a
// missing params field instead of hand-building the struct.
func decodeRequest(t *testing.T, wire string) *jsonrpc2.Request {
	t.Helper()
	var req jsonrpc2.Request
	if err := json.Unmarshal([]byte(wire), &req); err != nil {
		t.Fatalf("unmarshal %s: %v", wire, err)
	}
	return &req
}

// TestRouteParamslessNotificationReachesHandlerAsNull guards that a peer that
// omits the params field cannot crash the connection. jsonrpc2 leaves
// Request.Params nil for such a message, and dereferencing it panicked inside the
// reader goroutine, which no caller can recover from. Absence is routed as JSON
// null - the payload an explicit "params": null already produced - so handlers
// keep behaving exactly as they do for that case.
func TestRouteParamslessNotificationReachesHandlerAsNull(t *testing.T) {
	for _, tc := range []struct {
		name string
		wire string
	}{
		{"params omitted", `{"jsonrpc":"2.0","method":"window/showMessage"}`},
		{"params null", `{"jsonrpc":"2.0","method":"window/showMessage","params":null}`},
		{"params present", `{"jsonrpc":"2.0","method":"window/showMessage","params":{"type":2}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := NewRouter()
			var got json.RawMessage
			var method string
			r.HandleNotification("window/showMessage", func(_ context.Context, m string, params json.RawMessage) {
				method = m
				got = params
			})

			if _, err := r.Route(context.Background(), decodeRequest(t, tc.wire)); err != nil {
				t.Fatalf("Route() error = %v", err)
			}
			if method != "window/showMessage" {
				t.Fatalf("handler method = %q, want window/showMessage", method)
			}
			want := "null"
			if tc.name == "params present" {
				want = `{"type":2}`
			}
			if string(got) != want {
				t.Fatalf("handler params = %s, want %s", got, want)
			}
		})
	}
}

// TestRouteParamslessRequestAndDefaultHandlerReachHandlerAsNull covers the two
// other places that dereferenced params: a registered request handler and the
// default handler for an unregistered method.
func TestRouteParamslessRequestAndDefaultHandlerReachHandlerAsNull(t *testing.T) {
	r := NewRouter()
	var got json.RawMessage
	r.Handle("workspace/configuration", func(_ context.Context, _ string, params json.RawMessage) (any, error) {
		got = params
		return "configured", nil
	})

	result, err := r.Route(context.Background(), decodeRequest(t, `{"jsonrpc":"2.0","id":1,"method":"workspace/configuration"}`))
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}
	if result != "configured" {
		t.Fatalf("Route() result = %#v, want configured", result)
	}
	if string(got) != "null" {
		t.Fatalf("request handler params = %s, want null", got)
	}

	var defaultGot json.RawMessage
	r.SetDefaultHandler(func(_ context.Context, _ string, params json.RawMessage) (any, error) {
		defaultGot = params
		return nil, nil
	})
	if _, err := r.Route(context.Background(), decodeRequest(t, `{"jsonrpc":"2.0","id":2,"method":"unknown/method"}`)); err != nil {
		t.Fatalf("Route() error = %v", err)
	}
	if string(defaultGot) != "null" {
		t.Fatalf("default handler params = %s, want null", defaultGot)
	}
}

func TestRouteParamslessParamsAreNotShared(t *testing.T) {
	r := NewRouter()
	var calls int
	r.HandleNotification("window/showMessage", func(_ context.Context, _ string, params json.RawMessage) {
		calls++
		if calls == 1 {
			params[0] = 'X'
		}
	})

	for range 2 {
		if _, err := r.Route(context.Background(), decodeRequest(t, `{"jsonrpc":"2.0","method":"window/showMessage"}`)); err != nil {
			t.Fatalf("Route() error = %v", err)
		}
	}
	if calls != 2 {
		t.Fatalf("handler calls = %d, want 2", calls)
	}
}
