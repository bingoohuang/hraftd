package hraftd

import "time"

// Ticker defines a ticker.
type Ticker struct {
	stop     chan bool
	tickerFn func()
	d        time.Duration
}

// NewTicker creates a new ticker.
func NewTicker(d time.Duration, tickerFn func()) *Ticker {
	return &Ticker{stop: make(chan bool, 1), tickerFn: tickerFn, d: d}
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

	for {
		select {
		case <-t.C:
			if fn == nil {
				fn = j.tickerFn
			}

			if fn != nil {
				fn()
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
