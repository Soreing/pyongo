package pyongo

import (
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

type msg struct {
	msg  amqp.Delivery
	done func()
}

type thread struct {
	eng *Engine
	src chan msg
	wg  *sync.WaitGroup
}

// Create a new thread to process messages concurrently.
// Each thread has a reference to the handler engine and waits on a channel to
// get messages to process. Messages get handled by the engine's specifications.
func newThread(eng *Engine) *thread {
	t := &thread{
		eng: eng,
		src: make(chan msg),
		wg:  &sync.WaitGroup{},
	}

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()

		var pm msg
		for active := true; active; {
			if pm, active = <-t.src; active {
				t.eng.handle(pm.msg)
				pm.done()
			}
		}
	}()

	return t
}

// Close the thread and wait for it.
func (t *thread) PoolRelease() {
	close(t.src)
	t.wg.Wait()
}
