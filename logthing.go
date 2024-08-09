// Package logthing implements structured logging and log dispatching functions to write and store logs
// and metrics in cloud servcies like azure monitor (azure log analytics) and elasticsearch.
// By implementing additional LogWriters (see LogWriter interface) it can be extended to log to additional
// log stores.
//
// Besides logging to cloud services, logthing also supports to write logs to stdout and stderr simultaneously.
//
// The following environemnt variables can be used to configure the behaviour:
// LOGTHING_LOG_NAME  					 - Log name under which log messages are stored (will be used as elasticsearch index or azure custom log type)
// LOGTHING_LOG_MAX_SEVERITY     - Messages with severity > LOGTHING_LOG_MAX_SEVERITY won't be logged or printed at all and are immediately dropped
// LOGTHING_PRINT_MAX_SEVERITY   - Messages with severity <= LOGTHING_PRINT_MAX_SEVERITY are are also printed to stdout / stderr
// LOGTHING_WHITELIST_LOG_TYPES  - Messages that match any whitelisted log type (comma separated) are logged independently of their severity
// LOGTHING_PRINT_PROPERTIES     - Message properties that match any give print property (comma separated) are printed with the message output
//
// Note: Severity increases with lower values (SeverityEmergency: 0 ... SeverityTrace: 7)
package logthing

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/mfmayer/logthing/logwriter"
)

// UTCTime to specially marshal time.Time according to azure monitor needs (which must be UTC and limited to 3 descimals)
type UTCTime time.Time

// MarshalJSON to marshal timestamp to JSON
func (t UTCTime) MarshalJSON() ([]byte, error) {
	timestamp := fmt.Sprintf("\"%s\"", time.Time(t).UTC().Format("2006-01-02T15:04:05.999999Z"))
	return []byte(timestamp), nil
}

var (
	ld          *logDispatcher // default log dispatcher
	loggers     = []**log.Logger{&Emergency, &Alert, &Critical, &Error, &Warning, &Notice, &Info, &Trace}
	logPrefixes = []string{"EMERG: ", "ALERT: ", "CRIT:  ", "ERROR: ", "WARN:  ", "NOTICE:", "INFO:  ", "TRACE: ", "N/A:   "}
	// severityNames = []string{"Emergency", "Alert", "Critical", "Error", "Warnin", "Notice", "Info", "Trace"}
	newLineSpacer = "\n       "
)

var (
	// Trace logger to print Trace messages to stdout
	Trace *log.Logger
	// Info logger to print Info messages to stdout
	Info *log.Logger
	// Notice logger to print Notice messages to stdout
	Notice *log.Logger
	// Warning logger to print Warning messages to stdout
	Warning *log.Logger
	// Error logger to print Error messages to stderr
	Error *log.Logger
	// Critical logger to print Critical messages to stderr
	Critical *log.Logger
	// Alert logger to print Alert messages to stderr
	Alert *log.Logger
	// Emergency logger to print Emergency messages to stderr
	Emergency *log.Logger
)

var (
	// ErrNotInitialized is returned when the dispatcher hasn't been initialized
	ErrNotInitialized error = errors.New("dispatcher not initialized")
	// ErrSeverityAboveMax is returned when the message's severity is above the max severity level. See LOGTHING_LOG_MAX_SEVERITY
	ErrSeverityAboveMax error = errors.New("LogMessage severity level above LOGTHING_LOG_MAX_SEVERITY")
	// ErrWrongMessageType is returned whe the log message is of wrong type. Ensure that LogMessage has been created by calling NewLogMsg()
	ErrWrongMessageType error = errors.New("LogMessage is of wrong type")
	// ErrChannelFull is returned when there is no empty space in the LogMessage queue
	ErrChannelFull error = errors.New("channel full")
)

// func unwrappedErrorStrings(err error) []string {
// 	if err != nil {
// 		errStrings := []string{err.Error()}
// 		spaces := "  "
// 		for wrappedErr := errors.Unwrap(err); wrappedErr != nil; wrappedErr = errors.Unwrap(wrappedErr) {
// 			errStrings = append(errStrings, spaces+wrappedErr.Error())
// 			spaces += "  "
// 		}
// 		return errStrings
// 	}
// 	return []string{}
// }

func stringSetFromSlice(slice []string) (set map[string]struct{}) {
	set = map[string]struct{}{}
	for _, entry := range slice {
		if entry != "" {
			set[entry] = struct{}{}
		}
	}
	return set
}

func init() {
	initConfig()
	isSystemD := (os.Getenv("INVOCATION_ID") != "")
	for severityLevel := Severity(0); severityLevel < SeverityNotApplied; severityLevel++ {
		writer := os.Stdout
		if severityLevel <= SeverityError {
			writer = os.Stderr
		}
		prefix := logPrefixes[severityLevel]
		flag := log.LstdFlags //log.Lshortfile | log.LstdFlags
		if isSystemD {
			prefix = fmt.Sprintf("<%v>%v", severityLevel, logPrefixes[severityLevel])
			flag = 0
		}
		*loggers[severityLevel] = log.New(writer, prefix, flag)
		if !config.meetsPrintMaxSeverity(severityLevel) {
			(*loggers[severityLevel]).SetOutput(io.Discard)
		}
	}
}

// func getLogPrefix(severity Severity) string {
// 	if severity >= 0 && severity < SeverityNotApplied {
// 		return logPrefixes[severity]
// 	}
// 	return logPrefixes[SeverityNotApplied]
// }

// WithDispatchCallback sets function that is called back before messages are dispatched
func WithDispatchCallback(callback func(LogMsg)) func(*dispatcherOptions) {
	return func(opt *dispatcherOptions) {
		opt.dispatchCallback = callback
	}
}

// WithOverflowCallback sets function that is called back when message queue is overflown and message got dropped
func WithOverflowCallback(callback func(LogMsg, uint64)) func(*dispatcherOptions) {
	return func(opt *dispatcherOptions) {
		opt.overflowCallback = callback
	}
}

// WithDispatchInterval sets interval for how long messages that shall be dispatched are queued before (default 5 seconds)
func WithDispatchInterval(interval time.Duration) func(*dispatcherOptions) {
	return func(opt *dispatcherOptions) {
		opt.dispatchInterval = interval
	}
}

// WithQueueSize sets queue size how many messsages can be buffered within a dispatch interval (default 8192)
func WithQueueSize(size int) func(*dispatcherOptions) {
	return func(opt *dispatcherOptions) {
		opt.queueSize = size
	}
}

// WithSetLogEntryID enables that for every log message an individual "logEntryID" property is set (counter that is atomically incremented)
func WithSetLogEntryID() func(*dispatcherOptions) {
	return func(opt *dispatcherOptions) {
		opt.setEntryID = true
	}
}

// WithSetStaticProperties enables that for every log message all given static properties are set
func WithSetStaticProperties(staticProperties map[string]interface{}) func(*dispatcherOptions) {
	return func(opt *dispatcherOptions) {
		opt.staticProperties = map[string]interface{}{}
		for k, v := range staticProperties {
			opt.staticProperties[k] = v
		}
	}
}

// InitDispatcher to init logthing log message dispatcher with given writers.
// When logthing isn't needed anymore (e.g. when the application exits) Close() must be called.
func InitDispatcher(logWriters []logwriter.LogWriter, opts ...func(*dispatcherOptions)) (err error) {
	if ld != nil {
		ld.close()
	}
	ld, err = newLogDispatcher(logWriters, opts...)
	return
}

// Close to flush all queued messages and close the writers
func Close() {
	if ld != nil {
		ld.close()
	}
}

// Log outputs and sends LogMessage with default dispatcher
// returns:
// ErrNotInitialized when the dispatcher hasn't been initialized
// ErrSeverityAboveMax when the message's severity is above the max severity level. See LOGTHING_LOG_MAX_SEVERITY
// ErrWrongMessageType whe the log message is of wrong type. Ensure that LogMessage has been created by calling NewLogMsg()
// ErrChannelFull when there is no empty space in the LogMessage queue
func Log(msg LogMsg) (err error) {
	return LogMsgWithCalldepth(2, msg)
}

// LogMsgWithCalldepth outputs and sends LogMessage with default dispatcher
// returns:
// ErrNotInitialized when the dispatcher hasn't been initialized
// ErrSeverityAboveMax when the message's severity is above the max severity level. See LOGTHING_LOG_MAX_SEVERITY
// ErrWrongMessageType whe the log message is of wrong type. Ensure that LogMessage has been created by calling NewLogMsg()
// ErrChannelFull when there is no empty space in the LogMessage queue
func LogMsgWithCalldepth(calldepth int, msg LogMsg) (err error) {
	if ld == nil {
		return ErrNotInitialized
	}
	if msg == nil {
		return
	}
	err = ld.log(calldepth+1, msg)
	return
}
