package hraftd

import "time"

// Ticker defines a ticker.
type Ticker struct {
	stop           chan bool
	tickerFn       []func()
	d              time.Duration
	startInstantly bool
}

// NewTicker creates a new ticker.
func NewTicker(d time.Duration, startInstantly bool, tickerFns ...func()) *Ticker {
	fns := make([]func(), 0, len(tickerFns))

	for _, fn := range tickerFns {
		if fn != nil {
			fns = append(fns, fn)
		}
	}

	return &Ticker{
		stop:           make(chan bool, 1),
		d:              d,
		startInstantly: startInstantly,
		tickerFn:       fns,
	}
}

// StartAsync starts the ticker.
// if tickerFns are passed, they will overwrite the previous passed in NewTicker call.
func (j *Ticker) StartAsync(tickerFns ...func()) {
	if len(tickerFns) > 0 {
		j.tickerFn = make([]func(), 0, len(tickerFns))

		for _, fn := range tickerFns {
			if fn != nil {
				j.tickerFn = append(j.tickerFn, fn)
			}
		}
	}

	go j.start()
}

// start starts the ticker.
func (j *Ticker) start() {
	t := time.NewTicker(j.d)
	defer t.Stop()

	if j.startInstantly {
		j.execFns()
	}

	for {
		select {
		case <-t.C:
			j.execFns()
		case <-j.stop:
			return
		}
	}
}

// StopAsync stops the ticker.
func (j *Ticker) StopAsync() {
	close(j.stop)
}

func (j *Ticker) execFns() {
	for _, fn := range j.tickerFn {
		fn()
	}
}
