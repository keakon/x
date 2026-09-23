package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/sourcegraph/jsonrpc2"
)

// Handler is a function that handles incoming messages.
type Handler func(ctx context.Context, method string, params json.RawMessage) (any, error)

// NotificationHandler is a function that handles incoming notifications.
type NotificationHandler func(ctx context.Context, method string, params json.RawMessage)

// Router routes incoming messages to appropriate handlers.
type Router struct {
	mu                   sync.RWMutex
	handlers             map[string]Handler
	notificationHandlers map[string]NotificationHandler
	defaultHandler       Handler
}

// NewRouter creates a new message router.
func NewRouter() *Router {
	return &Router{
		handlers:             make(map[string]Handler),
		notificationHandlers: make(map[string]NotificationHandler),
	}
}

// Handle registers a handler for a specific method.
func (r *Router) Handle(method string, handler Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[method] = handler
}

// HandleNotification registers a notification handler for a specific method.
func (r *Router) HandleNotification(method string, handler NotificationHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.notificationHandlers[method] = handler
}

// SetDefaultHandler sets the default handler for unregistered methods.
func (r *Router) SetDefaultHandler(handler Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defaultHandler = handler
}

// jsonNullParams is the params payload substituted when a message carries no
// params field at all. jsonrpc2 leaves Request.Params nil in that case (an
// explicit "params": null arrives as a pointer to the null literal instead), so
// dereferencing it panics inside the connection's reader goroutine - and a panic
// there takes the whole process down, since nothing on this path recovers.
// Routing absence as JSON null keeps every handler on the path it already takes
// for an explicit null.
var jsonNullParams = json.RawMessage("null")

// requestParams returns the params of req, substituting JSON null when the
// message omitted the params field.
func requestParams(req *jsonrpc2.Request) json.RawMessage {
	if req.Params == nil {
		return jsonNullParams
	}
	return *req.Params
}

// Route routes a message to the appropriate handler.
func (r *Router) Route(ctx context.Context, req *jsonrpc2.Request) (any, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check if it's a notification (no ID)
	if req.ID == (jsonrpc2.ID{}) {
		if handler, ok := r.notificationHandlers[req.Method]; ok {
			handler(ctx, req.Method, requestParams(req))
		}
		return nil, nil
	}

	// It's a request
	params := requestParams(req)
	if handler, ok := r.handlers[req.Method]; ok {
		return handler(ctx, req.Method, params)
	}

	// Use default handler if available
	if r.defaultHandler != nil {
		return r.defaultHandler(ctx, req.Method, params)
	}

	return nil, fmt.Errorf("no handler for method: %s", req.Method)
}

// MessageType represents the type of a JSON-RPC message.
type MessageType int

const (
	// RequestMessage is a request that expects a response.
	RequestMessage MessageType = iota
	// NotificationMessage is a notification that doesn't expect a response.
	NotificationMessage
	// ResponseMessage is a response to a request.
	ResponseMessage
	// ErrorMessage is an error response.
	ErrorMessage
)

// ParseMessageType determines the type of a JSON-RPC message.
func ParseMessageType(msg *Message) MessageType {
	if msg.Error != nil {
		return ErrorMessage
	}
	if msg.Result != nil {
		return ResponseMessage
	}
	if msg.ID != nil {
		return RequestMessage
	}
	return NotificationMessage
}
