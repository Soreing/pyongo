package pyongo

import (
	"fmt"
	"strings"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

type KeyModFunc func(*amqp.Delivery)

type Engine struct {
	process *sync.WaitGroup // Waitgroup to wait on before closing
	submits *sync.WaitGroup // Waitgroup for the number of manually submitted events
	events  *sync.WaitGroup // Waitgroup for currently running handlers for events
	closeCh chan bool       // Close channel for closing the engine

	hndl    *Handler           // Handler root of the engine that routes events
	mods    []KeyModFunc       // Key modifier functions that transform the routing key
	dropFnc func(ctx *Context) // Function to execute when an message gets dropped (no handler)
	discard bool               // If true, messages are read but not handled
	dropAck bool               // If true, dropped messages are acknowledged

	logger *zap.Logger
}

// Creates a new engine with a handler
func New(
	dropAck bool,
	lgr *zap.Logger,
) *Engine {
	return &Engine{
		process: &sync.WaitGroup{},
		submits: &sync.WaitGroup{},
		events:  &sync.WaitGroup{},

		closeCh: make(chan bool),
		hndl:    NewHandler(),
		dropFnc: func(ctx *Context) {},
		discard: false,
		dropAck: dropAck,
		logger:  lgr,
	}
}

// Sets the function to call when a message is dropped
func (e *Engine) SetDropFunction(fnc func(ctx *Context)) {
	e.dropFnc = fnc
}

// Clears the function to call when a message gets dropped
func (e *Engine) ClearDropFunction() {
	e.dropFnc = func(ctx *Context) {}
}

// Adds a key modifier function that transforms the routing key
// Modifier functions are executed in the order they were added
func (e *Engine) AddKeyModifier(fn KeyModFunc) {
	e.mods = append(e.mods, fn)
}

// Manually submits a message to be handled
func (e *Engine) Submit(msg amqp.Delivery) error {
	if e.discard {
		e.logger.Error("failed to submit message")
		return fmt.Errorf("handler is discarding messages")
	}

	e.submits.Add(1)
	go func() {
		e.submits.Done()
		e.handle(msg)
	}()
	return nil
}

// Runs the engine for a channel supplying messages
func (e *Engine) Run(ch chan amqp.Delivery) {
	e.logger.Info("starting handler")
	e.process.Add(1)
	go func() {
		defer e.process.Done()
		e.listen(ch)
	}()
}

// Stops handling messages but keeps reading and discarding them
func (e *Engine) Discard() {
	e.logger.Info("putting the handler into discard mode")
	e.discard = true
}

// Stops reading from the channel and exit the handler
// Waits for all operations to finish before returning
func (e *Engine) Close() {
	e.logger.Info("closing the handler")
	close(e.closeCh)
	e.process.Wait()
	e.submits.Wait()
	e.events.Wait()
}

// Waits for all the events being handled to finish
func (e *Engine) WaitOnEvents() {
	e.logger.Info("waiting on events to finish handling")
	e.submits.Wait()
	e.events.Wait()
	e.logger.Info("events finished handling")
}

// Listens on a channel and handles incoming messages
func (e *Engine) listen(ch chan amqp.Delivery) {
	e.logger.Info("listening to channel")
	var msg amqp.Delivery

	for active := true; active; {
		select {
		case _, active = <-e.closeCh:
			e.logger.Info("closing listener")
		case msg, active = <-ch:
			if active {
				e.events.Add(1)
				go func() {
					defer e.events.Done()
					e.handle(msg)
				}()
			} else {
				e.logger.Warn("channel closed")
			}
		}
	}
	e.logger.Info("stopped listening to channel")
}

// Handles a message. The routing key of the message is ran through the
// modify functions that transform it, then the key is routed through the
// handler to attach middlewares and the handler function to a context.
// If there is a handler function, the context is ran, otherwise the
// message is dropped with acknowledgement and drop function called
func (e *Engine) handle(msg amqp.Delivery) {
	for _, mod := range e.mods {
		mod(&msg)
	}

	keys := strings.Split(msg.RoutingKey, ".")
	pctx := NewContext(e, msg)
	if err := e.hndl.Route(pctx, keys); err == nil {
		pctx.Next()
	} else {
		if e.dropAck {
			msg.Ack(false)
		}
		e.dropFnc(pctx)
	}
}

// ---------- pyongo.Handler masquerade functions---------- //

func (e *Engine) Use(fn HandlerFunc) {
	e.hndl.Use(fn)
}

func (e *Engine) Group(name string) *Handler {
	return e.hndl.Group(name)
}

func (e *Engine) Set(name string, fn HandlerFunc) {
	e.hndl.Set(name, fn)
}
