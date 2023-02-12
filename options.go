package pyongo

import (
	"time"

	"go.uber.org/zap"
)

type Options struct {
	threads  *int
	idleTime *time.Duration
	buffSize *int
	logger   Logger
}

type options struct {
	threads  int
	idleTime time.Duration
	buffSize int
	logger   Logger
}

// Sets the maximum concurrent threads processing messages
func ConcurrencyOption(
	threads int,
) *Options {
	return &Options{
		threads: &threads,
	}
}

// Sets the maximum idle time for a thread before it's cleaned up
func ThreadIdleTimeOption(
	idle time.Duration,
) *Options {
	return &Options{
		idleTime: &idle,
	}
}

// Sets the maximum buffered messages
func BufferSizeOption(
	size int,
) *Options {
	return &Options{
		buffSize: &size,
	}
}

// Sets a logger for the engine
func LoggerOption(
	logger *zap.Logger,
) *Options {
	return &Options{
		logger: logger,
	}
}

// Sets a logger for the engine
func DefaultLoggerOption() *Options {
	lgr, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}

	return &Options{
		logger: lgr,
	}
}

// Adds options to default options
func makeOptions(opts ...*Options) *options {
	opt := &options{
		threads:  1,
		idleTime: time.Minute,
		buffSize: 100,
		logger:   newDisabledLogger(),
	}

	for i := range opts {
		if opts[i].threads != nil {
			opt.threads = *opts[i].threads
		}
		if opts[i].idleTime != nil {
			opt.idleTime = *opts[i].idleTime
		}
		if opts[i].buffSize != nil {
			opt.buffSize = *opts[i].buffSize
		}
		if opts[i].logger != nil {
			opt.logger = opts[i].logger
		}
	}
	return opt
}
