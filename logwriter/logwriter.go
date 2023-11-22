package logwriter

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/joho/godotenv"
)

// A Kind represents the specific kind of type that a Type represents.
// The zero Kind is not a valid kind.
type Kind uint

const (
	Unknown Kind = iota
	String
	Number
	Integer
	Boolean
	Object
	Array
	DateTime
)

func init() {
	godotenv.Load()
}

// Config provides writer relevant information from dispatcher
type Config struct {
	LogName string
}

// LogWriter interface that can be used ny the logDispatcher to write logs.
type LogWriter interface {
	// Init is called when the dispatcher starts. In case ErrWriterStop is returned, the writer won't be
	// considered while writing log messages.
	Init(config Config) error
	// WriteLogMessages shall write given logMessages, which are sorted by their timestamp. Since every message
	// is premarshalled, their timestamps are provided in a separate array of same length with corresponding indices.
	// In case there is an unrecoverable error detected ErrWriterStop shall be returned to disbale the LogWriter and prevent
	// that WriteLogMessages is called again.
	WriteLogMessages(logMessages []json.RawMessage, timestamps []time.Time) error
	// PropertiesSchemaChanged is called when the schema of log messages changed.
	// E.g. yet unknown property has been added to a log message.
	// PropertiesSchemaChanged is called before the according log message is written with WriteLogMessages().
	// An error can be returned to inform that something went wrong, but it won't be handled somehow.
	PropertiesSchemaChanged(schema map[string]Kind) error
	// Informs the writer that the dispatcher stopped
	Close()
}

// ErrWriterDisable is returned when there is an unrecoverable error detected
// and writing log messages will never succeed. Dispatcher will close and disbale the writer.
var ErrWriterDisable = errors.New("Writer disbaled")
