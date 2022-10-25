package pyongo

import (
	"fmt"
)

type HandlerFunc func(ctx *Context)
type HandlerMap map[string]*Handler

type Handler struct {
	fn   HandlerFunc   // Function that handles the event
	mdws []HandlerFunc // Middlewares to call before handler
	hmap HandlerMap    // Map of handlers if this handler is a group
}

// Creates a new handler.
func NewHandler() *Handler {
	return &Handler{
		hmap: HandlerMap{},
		mdws: []HandlerFunc{},
	}
}

// Creates a new handler group with the given name.
func (h *Handler) Group(name string) *Handler {
	newh := NewHandler()
	h.hmap[name] = newh
	return newh
}

// Sets a function as the handler for an event with a name.
func (h *Handler) Set(name string, fn HandlerFunc) {
	h.hmap[name] = &Handler{
		fn:   fn,
		hmap: HandlerMap{},
	}
}

// Attaches a middleware to a handler. Middlewares are executed in the
// order they were added to the handler.
func (h *Handler) Use(fn HandlerFunc) {
	h.mdws = append(h.mdws, fn)
}

// Routes an event with a list of keys through the handler.
// At each layer, middlewares are added to the context, and at the end,
// the handler function is also added. If the keys do not resolve to a handler,
// the function returns an error.
func (h *Handler) Route(ctx *Context, keys []string) error {
	handler := h
	exist := false
	for i := 0; i < len(keys); i++ {
		ctx.AddMiddlewares(handler.mdws)
		if handler, exist = handler.hmap[keys[i]]; !exist {
			return fmt.Errorf("handler not found")
		}
	}
	if handler.fn == nil {
		return fmt.Errorf("handler has no function set")
	} else {
		ctx.AddMiddlewares(append(handler.mdws, handler.fn))
		return nil
	}
}
