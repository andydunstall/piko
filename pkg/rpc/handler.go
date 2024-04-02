package rpc

import "sync"

// HandlerFunc handles the given request message and returns a response.
type HandlerFunc func(message []byte) []byte

// Handler is responsible for registering RPC request handlers for RPC types.
type Handler struct {
	handlers map[Type]HandlerFunc
	mu       sync.Mutex
}

func NewHandler() *Handler {
	return &Handler{
		handlers: make(map[Type]HandlerFunc),
	}
}

// Register adds a new handler for the given RPC request type.
func (h *Handler) Register(rpcType Type, handler HandlerFunc) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.handlers[rpcType] = handler
}

// Find looks up the handler for the given RPC type.
func (h *Handler) Find(rpcType Type) (HandlerFunc, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	handler, ok := h.handlers[rpcType]
	return handler, ok
}
