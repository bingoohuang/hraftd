package hraftd

import "time"

// Ticker defines a ticker.
type Ticker struct {
	stop           chan bool
	tickerFn       func()
	d              time.Duration
	startInstantly bool
}

// NewTicker creates a new ticker.
func NewTicker(d time.Duration, tickerFn func(), startInstantly bool) *Ticker {
	return &Ticker{stop: make(chan bool, 1), tickerFn: tickerFn, d: d, startInstantly: startInstantly}
}

// StartAsync starts the ticker.
func (j *Ticker) StartAsync() {
	go j.start(nil)
}

// StartAsyncFn starts the ticker with customized ticker functor.
func (j *Ticker) StartAsyncFn(tickerFn func()) {
	go j.start(tickerFn)
}

// start starts the ticker.
func (j *Ticker) start(fn func()) {
	t := time.NewTicker(j.d)
	defer t.Stop()

	if fn != nil {
		j.tickerFn = fn
	}

	if j.tickerFn != nil && j.startInstantly {
		j.tickerFn()
	}

	for {
		select {
		case <-t.C:
			if j.tickerFn != nil {
				j.tickerFn()
			}
		case <-j.stop:
			return
		}
	}
}

// StopAsync stops the ticker.
func (j *Ticker) StopAsync() {
	select {
	case j.stop <- true:
	default:
	}
}
