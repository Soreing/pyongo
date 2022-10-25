package pyongo

import (
	"context"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Mark struct {
	name string
	time time.Time
}

type Context struct {
	ctx context.Context
	eng *Engine
	msg amqp.Delivery

	mu   sync.RWMutex
	keys map[string]any

	mwIndex int
	mwares  []HandlerFunc

	marks  []Mark
	Errors []error
}

func NewContext(
	eng *Engine,
	msg amqp.Delivery,
) *Context {
	return &Context{
		eng:    eng,
		msg:    msg,
		ctx:    context.TODO(),
		mwares: []HandlerFunc{},
		marks:  []Mark{},
	}
}

func (c *Context) GetEngine() *Engine {
	return c.eng
}

func (c *Context) GetMessage() amqp.Delivery {
	return c.msg
}

func (c *Context) Set(key string, value any) {
	c.mu.Lock()
	if c.keys == nil {
		c.keys = make(map[string]any)
	}

	c.keys[key] = value
	c.mu.Unlock()
}

func (c *Context) Get(key string) (value any, exists bool) {
	c.mu.RLock()
	value, exists = c.keys[key]
	c.mu.RUnlock()
	return
}

func (c *Context) MustGet(key string) any {
	if value, exists := c.Get(key); exists {
		return value
	}
	panic("Key \"" + key + "\" does not exist")
}

func (c *Context) Mark(name string) {
	c.marks = append(c.marks, Mark{name, time.Now()})
}

func (c *Context) GetMarks() []Mark {
	return c.marks
}

func (c *Context) Error(err error) error {
	if err == nil {
		return fmt.Errorf("err is nil")
	}

	c.Errors = append(c.Errors, err)
	return nil
}

func (c *Context) AddMiddlewares(mws []HandlerFunc) {
	if len(mws) > 0 {
		c.mwares = append(c.mwares, mws...)
	}
}

func (c *Context) Next() {
	if c.mwIndex < len(c.mwares) {
		idx := c.mwIndex
		c.mwIndex++
		c.mwares[idx](c)
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
