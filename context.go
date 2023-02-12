package pyongo

import (
	"context"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Mark struct {
	Tag  string
	Time time.Time
}

type Context struct {
	ctx  context.Context
	msg  amqp.Delivery
	mu   sync.RWMutex
	keys map[string]any

	fnIndex int
	funcs   []func(ctx *Context)

	marks  []Mark
	Errors []error
}

// Creates a new context with an amqp message
func NewContext(
	msg amqp.Delivery,
) *Context {
	return &Context{
		msg:   msg,
		ctx:   context.TODO(),
		funcs: []func(ctx *Context){},
		marks: []Mark{},
	}
}

// Adds functions to the stack.
func (c *Context) addFunctions(mws []func(ctx *Context)) {
	if len(mws) > 0 {
		c.funcs = append(c.funcs, mws...)
	}
}

// Gets the amqp message of the context.
func (c *Context) GetMessage() amqp.Delivery {
	return c.msg
}

// Sets a value in the context's store.
func (c *Context) Set(key string, value any) {
	c.mu.Lock()
	if c.keys == nil {
		c.keys = make(map[string]any)
	}

	c.keys[key] = value
	c.mu.Unlock()
}

// Gets a value from the context's store and if it exists.
func (c *Context) Get(key string) (value any, exists bool) {
	c.mu.RLock()
	value, exists = c.keys[key]
	c.mu.RUnlock()
	return
}

// Gets a value from the context's store, panics if does not exist.
func (c *Context) MustGet(key string) any {
	if value, exists := c.Get(key); exists {
		return value
	}
	panic("key \"" + key + "\" does not exist")
}

// Adds a mark with a tag name and a timestamp on the context.
func (c *Context) Mark(name string) {
	c.marks = append(c.marks, Mark{name, time.Now()})
}

// Gets all the marks in the context.
func (c *Context) GetMarks() []Mark {
	return c.marks
}

// Adds an error to the context.
func (c *Context) Error(err error) error {
	if err == nil {
		return fmt.Errorf("err is nil")
	}

	c.Errors = append(c.Errors, err)
	return nil
}

// Calls the next function in the function stack like middlewares. Panics if
// there are no more functions on the stack.
func (c *Context) Next() {
	if c.fnIndex < len(c.funcs) {
		idx := c.fnIndex
		c.fnIndex++
		c.funcs[idx](c)
	} else {
		panic("function stack empty")
	}
}

// ---------- context.Context implemetation ---------- //

func (c *Context) Deadline() (deadline time.Time, ok bool) {
	return c.ctx.Deadline()
}

func (c *Context) Done() <-chan struct{} {
	return c.ctx.Done()
}

func (c *Context) Err() error {
	return c.ctx.Err()
}

func (c *Context) Value(key any) any {
	if key == 0 {
		return c.ctx
	}
	if keyAsString, ok := key.(string); ok {
		if val, exists := c.Get(keyAsString); exists {
			return val
		}
	}

	return c.ctx.Value(key)
}
