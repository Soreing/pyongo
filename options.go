package pyongo

import "time"

type Options struct {
	DropAck      bool
	ThreadLimit  int
	ThreadExpiry time.Duration
}
