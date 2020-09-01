package logwriter

import (
	"encoding/json"
	"errors"
)

// LogWriter interface that can be used ny the logDispatcher to write logs.
type LogWriter interface {
	// Init is called when the dispatcher starts. In case ErrWriterStop is returned, the writer won't be
	// considered while writing log messages.
	Init() error
	// WriteLogMessages writes given logMessages. In case there is an unrecoverable error detected
	// ErrWriterStop shall be returned to disbale the LogWriter and prevent that WriteLogMessages is called again.
	WriteLogMessages(logMessages []json.RawMessage) error
	// Informs the writer that the dispatcher stopped
	Close()
}

// ErrWriterDisable is returned when there is an unrecoverable error detected
// and writing log messages will never succeed. Dispatcher will close and disbale the writer.
var ErrWriterDisable = errors.New("Writer disbaled")
