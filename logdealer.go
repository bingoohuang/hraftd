package hraftd

// LogDealer defines the structure of log dealer.
type LogDealer struct {
	DealerMap
}

// MakeLogDealer makes a LogDealer
func MakeLogDealer() LogDealer {
	return LogDealer{DealerMap: MakeDealerMap()}
}

// RegisterLogDealer registers the dealer for the command which name is cmdName.
func (l *LogDealer) RegisterLogDealer(cmdName string, dealerFn interface{}) error {
	return l.RegisterJobDealer(cmdName, dealerFn)
}
