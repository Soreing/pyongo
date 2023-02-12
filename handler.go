package pyongo

import (
	"errors"
)

type Handler interface {
	Group(name string) Handler
	Set(name string, fn func(ctx *Context))
	Use(fn func(ctx *Context))
}

type handler struct {
	fn   func(ctx *Context)   // Function that handles the event
	mdws []func(ctx *Context) // Middlewares to call before handler
	hmap map[string]*handler  // Map of handlers if this handler is a group
}

// Creates a new handler.
func newHandler() *handler {
	return &handler{
		hmap: map[string]*handler{},
		mdws: []func(ctx *Context){},
	}
}

// Creates a new handler group with the given name.
func (h *handler) Group(name string) Handler {
	newh := newHandler()
	h.hmap[name] = newh
	return newh
}

// Sets a function as the handler for an event with a name.
func (h *handler) Set(name string, fn func(ctx *Context)) {
	h.hmap[name] = &handler{
		fn:   fn,
		hmap: map[string]*handler{},
	}
}

// Attaches a middleware to a handler. Middlewares are executed in the
// order they were added to the handler. Middlewares can not be removed.
func (h *handler) Use(fn func(ctx *Context)) {
	h.mdws = append(h.mdws, fn)
}

// Routes an event with a list of keys through the handler.
// Adds middlewares and the handler to the context. Returns an error if the
// keys do not resolve to a handler.
func (h *handler) route(ctx *Context, keys []string) error {
	handler := h
	for i := 0; i < len(keys); i++ {
		ctx.addFunctions(handler.mdws)
		if keyhandler, exist := handler.hmap[keys[i]]; !exist {
			if allhandler, exist := handler.hmap["*"]; !exist {
				return errors.New("handler not found")
			} else {
				handler = allhandler
			}
		} else {
			handler = keyhandler
		}
	}
	if handler.fn == nil {
		return errors.New("handler has no function set")
	} else {
		ctx.addFunctions(append(handler.mdws, handler.fn))
		return nil
	}
}

// Routes an event through the global handler if one exists
func (h *handler) global(ctx *Context) error {
	if handler, exist := h.hmap["#"]; !exist {
		return errors.New("handler not found")
	} else {
		if handler.fn == nil {
			return errors.New("handler has no function set")
		} else {
			ctx.addFunctions(append(handler.mdws, handler.fn))
			return nil
		}
	}
}
