package pyongo

import (
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

type poolMsg struct {
	msg  amqp.Delivery
	done func()
}

type Thread struct {
	eng *Engine
	src chan poolMsg
	wg  *sync.WaitGroup
}

func NewThread(eng *Engine) *Thread {
	t := &Thread{
		eng: eng,
		src: make(chan poolMsg),
		wg:  &sync.WaitGroup{},
	}

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()

		var pm poolMsg
		for active := true; active; {
			if pm, active = <-t.src; active {
				t.eng.handle(pm.msg)
				pm.done()
			}
		}
	}()

	return t
}

func (t *Thread) PoolRelease() {
	close(t.src)
	t.wg.Wait()
}
