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
func (j *Ticker) StartAsync() {
	go j.start()
}

// StartAsyncFn starts the ticker with customized ticker functor.
func (j *Ticker) StartAsyncFn(tickerFn func()) {
	if tickerFn != nil {
		j.tickerFn = append(j.tickerFn, tickerFn)
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
	select {
	case j.stop <- true:
	default:
	}
}

func (j *Ticker) execFns() {
	for _, fn := range j.tickerFn {
		fn()
	}
}
