package pyongo

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/Soreing/grasp"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

const (
	state_created = "created"
	state_started = "started"
	state_closed  = "closed"
)

type IEngine interface {
	Handler
	UseModifier(fn func(*amqp.Delivery))
	AddSource(src <-chan amqp.Delivery) error
	Start() error
	Close() error
	Submit(
		ctx context.Context,
		msg amqp.Delivery,
	) error
}

type Engine struct {
	sources *sync.WaitGroup // Sources being listened to
	events  *sync.WaitGroup // Deliveries remaining to be handled
	lock    *sync.RWMutex   // Lock to prevent access while closing
	state   string          // State of the handler engine

	readSrc chan bool          // Condition channel to read from sources
	msgBuff chan amqp.Delivery // Buffered delivery collection channel
	pool    grasp.Pool         // Handler goroutine pool

	hndl *handler               // Handler root of the engine that routes events
	mods []func(*amqp.Delivery) // Modifier functions that transform the delivery

	logger Logger
}

func New(opts ...*Options) *Engine {
	opt := makeOptions(opts...)

	eng := &Engine{
		sources: &sync.WaitGroup{},
		events:  &sync.WaitGroup{},
		lock:    &sync.RWMutex{},
		state:   state_created,

		readSrc: make(chan bool),
		msgBuff: make(chan amqp.Delivery, opt.buffSize),

		hndl:   newHandler(),
		mods:   []func(*amqp.Delivery){},
		logger: opt.logger,
	}

	eng.pool = grasp.NewPool(opt.threads, opt.idleTime,
		func() grasp.Poolable {
			return newThread(eng)
		},
	)
	return eng
}

// Adds a modifier function that transforms the delivery. Modifiers are
// executed in the order they were added and can not be removed.
func (e *Engine) UseModifier(fn func(*amqp.Delivery)) {
	e.mods = append(e.mods, fn)
}

// Adds a listener to a channel, extracting deliveries and buffering them
// to be routed and processed by handlers. Stops listening to the channel if
// the cannel is closed or the handler is closed.
func (e *Engine) AddSource(src <-chan amqp.Delivery) error {
	e.lock.Lock()
	defer e.lock.Unlock()

	if e.state == state_closed {
		err := errors.New("handler is closed")
		e.logger.Error("failed to add source", zap.Error(err))
		return err
	}

	e.sources.Add(1)
	go func() {
		defer e.sources.Done()
		var msg amqp.Delivery
		for active := true; active; {
			select {
			case _, active = <-e.readSrc:
				/* handler closed */
			case msg, active = <-src:
				/* message received from the channel */
				if active {
					e.events.Add(1)
					select {
					case e.msgBuff <- msg:
						/* message is submitted immediately */
					default:
						/* message buffer is full */
						if e.logger != nil {
							e.logger.Warn("message buffer is full")
						}
						e.msgBuff <- msg
					}
				}
			}
		}
	}()

	return nil
}

// Manually submits a delivery to the buffered channel to be routed and
// processed by handlers. If the buffer is full, blocks until there is space
// or the context is canceled
func (e *Engine) Submit(
	ctx context.Context,
	msg amqp.Delivery,
) error {
	e.lock.RLock()
	defer e.lock.RUnlock()

	if e.state == state_closed {
		err := errors.New("handler is closed")
		e.logger.Error("failed to add source", zap.Error(err))
		return err
	}

	e.events.Add(1)

	select {
	case e.msgBuff <- msg:
		/* message is submitted immediately */
	default:
		/* message buffer is full */
		if e.logger != nil {
			e.logger.Warn("message buffer is full")
		}
		select {
		case e.msgBuff <- msg:
			/* message submitted */
		case <-ctx.Done():
			e.events.Done()
			return errors.New("context cancaled")
		}
	}
	return nil
}

// Starts processing deliveries from the buffered channel. Deliveries are
// processed concurrently by threads acquired from a pool.
func (e *Engine) Start() error {
	e.logger.Info("attempting to start handler")
	e.lock.Lock()
	defer e.lock.Unlock()

	e.logger.Info("starting handler")
	if e.state != state_created {
		err := errors.New("invalid state")
		return err
	}

	e.state = state_started
	go func() {
		e.logger.Info("started consuming deliveries")
		for delv := range e.msgBuff {
			val, done, err := e.pool.Acquire(context.TODO())
			if err != nil {
				panic("failed to acquire thread")
			} else if thr, ok := val.(*thread); !ok {
				panic("resource is not a thread")
			} else {
				thr.src <- msg{delv, done}
			}
		}
		e.logger.Info("stopped consuming deliveries")
	}()
	return nil
}

// Closes the handler engine by detaching source channels to stop incoming
// deliveries, waiting for deliveries to be handled and closing the handlers.
func (e *Engine) Close() error {
	e.logger.Info("attempting to close handler")
	e.lock.Lock()
	defer e.lock.Unlock()

	e.logger.Info("closing handler")
	e.state = state_closed

	e.logger.Info("detaching source channels")
	close(e.readSrc)
	e.sources.Wait()

	e.logger.Info("waiting for events")
	e.events.Wait()

	e.logger.Info("cleaning up resources")
	close(e.msgBuff)
	e.pool.Close()

	e.logger.Info("handler closed")
	return nil
}

func (e *Engine) consume() {
	e.logger.Info("started consuming deliveries")

	for delv := range e.msgBuff {
		val, done, err := e.pool.Acquire(context.TODO())
		if err != nil {
			panic("failed to acquire thread")
		} else if thr, ok := val.(*thread); !ok {
			panic("resource is not a thread")
		} else {
			thr.src <- msg{delv, done}
		}
	}

	e.logger.Info("stopped consuming deliveries")
}

// Handles a message. The routing key of the message is ran through the
// modify functions that transform it, then the key is routed through the
// handler to attach middlewares and the handler function to a context.
// If there is a handler function, the context is ran, otherwise the
// message is dropped with acknowledgement and drop function called
func (e *Engine) handle(msg amqp.Delivery) {
	defer e.events.Done()
	for _, mod := range e.mods {
		mod(&msg)
	}

	keys := strings.Split(msg.RoutingKey, ".")
	pctx := NewContext(msg)
	if err := e.hndl.route(pctx, keys); err != nil {
		pctx = NewContext(msg)
		if err := e.hndl.global(pctx); err != nil {
			e.logger.Warn(
				"no handler found",
				zap.String("key", msg.RoutingKey),
				zap.Error(err))
			msg.Ack(false)
			return
		}
	}
	pctx.Next()
}

// ---------- pyongo.Handler masquerade functions---------- //

func (e *Engine) Use(fn func(ctx *Context)) {
	e.hndl.Use(fn)
}

func (e *Engine) Group(name string) Handler {
	return e.hndl.Group(name)
}

func (e *Engine) Set(name string, fn func(ctx *Context)) {
	e.hndl.Set(name, fn)
}
