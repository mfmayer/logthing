// Package logthing implements structured logging and log dispatching functions to write and store logs
// and metrics in cloud servcies like azure monitor (azure log analytics) and elasticsearch.
// By implementing additional LogWriters (see LogWriter interface) it can be extended to log to additional
// log stores.
//
// Besides logging to cloud services, logthing also supports to write logs to stdout and stderr simultaneously.
//
// The following environemnt variables can be used to configure the behaviour:
//
// LOGTHING_LOG_NAME or SERVICE_NAME  - Log name under which log messages are stored (will be used as elasticsearch index or azure custom log type)
//
// LOGTHING_LOG_MAX_SEVERITY          - Messages with severity > LOGTHING_LOG_MAX_SEVERITY won't be logged and are immediately dropped
//
// LOGTHING_WHITELIST_LOG_TYPES       - Messages that match any whitelisted log type (comma separated) are logged independent of their severity
//
// LOGTHING_PRINT_MAX_SEVERITY        - Messages with severity <= LOG_OUTPUT_SEVERITY_MAX are directly printed to stdout / stderr
//
// LOGTHING_PRINT_PROPERTIES          - Message properties that match any give print property (comma separated) are printed with the message output
//
// Note: Severity increases with lower values (SeverityEmergency: 0 ... SeverityTrace: 7)
package logthing

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
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

// SProp stringifies the sutrucuterd property data before dispatching it to the log writers.
type SProp map[string]interface{}

// MarshalJSON creates stringified version of
func (sm SProp) MarshalJSON() (ret []byte, err error) {
	ret, err = json.Marshal(map[string]interface{}(sm))
	if err == nil {
		ret, err = json.Marshal(string(ret))
	}
	return ret, err
}

// Severity to declare log message severities
type Severity uint

var (
	ld            *logDispatcher // default log dispatcher
	loggers       = []**log.Logger{&Emergency, &Alert, &Critical, &Error, &Warning, &Notice, &Info, &Trace}
	logPrefixes   = []string{"EMERG: ", "ALERT: ", "CRIT:  ", "ERROR: ", "WARN:  ", "NOTICE:", "INFO:  ", "TRACE: "}
	severityNames = []string{"Emergency", "Alert", "Critical", "Error", "Warnin", "Notice", "Info", "Trace"}
)

// Trace logger to print Trace messages to stdout
var Trace *log.Logger

// Info logger to print Info messages to stdout
var Info *log.Logger

// Notice logger to print Notice messages to stdout
var Notice *log.Logger

// Warning logger to print Warning messages to stdout
var Warning *log.Logger

// Error logger to print Error messages to stderr
var Error *log.Logger

// Critical logger to print Critical messages to stderr
var Critical *log.Logger

// Alert logger to print Alert messages to stderr
var Alert *log.Logger

// Emergency logger to print Emergency messages to stderr
var Emergency *log.Logger

// ErrNotInitialized is returned when the dispatcher hasn't been initialized
var ErrNotInitialized error = errors.New("Dispatcher not initialized")

// ErrSeverityAboveMax is returned when the message's severity is above the max severity level. See LOGTHING_LOG_MAX_SEVERITY
var ErrSeverityAboveMax error = errors.New("LogMessage severity level above LOGTHING_LOG_MAX_SEVERITY")

// ErrWrongMessageType is returned whe the log message is of wrong type. Ensure that LogMessage has been created by calling NewLogMsg()
var ErrWrongMessageType error = errors.New("LogMessage is of wrong type")

// ErrChannelFull is returned when there is no empty space in the LogMessage queue
var ErrChannelFull error = errors.New("Channel full")

const (
	// SeverityNotApplied Severity level for tracing log entries
	SeverityNotApplied Severity = 8
	// SeverityTrace Severity level for tracing log entries
	SeverityTrace Severity = 7
	// SeverityInfo Severity level for information log entries
	SeverityInfo Severity = 6
	// SeverityNotice Severity level for notice log entries
	SeverityNotice Severity = 5
	// SeverityWarning Severity level for warning log entries
	SeverityWarning Severity = 4
	// SeverityError Severity level for error log entries
	SeverityError Severity = 3
	// SeverityCritical Severity level for critical log entries
	SeverityCritical Severity = 2
	// SeverityAlert Severity level for alert log entries
	SeverityAlert Severity = 1
	// SeverityEmergency Severity level for emergency log entries
	SeverityEmergency Severity = 0
)

func unwrappedErrorStrings(err error) []string {
	if err != nil {
		errStrings := []string{err.Error()}
		spaces := "  "
		for wrappedErr := errors.Unwrap(err); wrappedErr != nil; wrappedErr = errors.Unwrap(wrappedErr) {
			errStrings = append(errStrings, spaces+wrappedErr.Error())
			spaces += "  "
		}
		return errStrings
	}
	return []string{}
}

func stringSetFromSlice(slice []string) (set map[string]struct{}) {
	set = map[string]struct{}{}
	for _, entry := range slice {
		set[entry] = struct{}{}
	}
	return set
}

type configStruct struct {
	logName               string
	logMaxSeverity        Severity
	whitelistLogTypes     map[string]struct{}
	printMaxSeverity      Severity
	printOutputProperties map[string]struct{}
}

var config configStruct = configStruct{
	logName:               os.Getenv("SERVICE_NAME"),
	logMaxSeverity:        SeverityError,
	whitelistLogTypes:     map[string]struct{}{},
	printMaxSeverity:      SeverityTrace,
	printOutputProperties: map[string]struct{}{},
}

func initConfig() {
	if config.logName == "" {
		if logName := os.Getenv("LOGTHING_LOG_NAME"); logName != "" {
			config.logName = logName
		}
	}
	if logMaxSeverity, err := strconv.Atoi(os.Getenv("LOGTHING_LOG_MAX_SEVERITY")); err == nil {
		config.logMaxSeverity = Severity(logMaxSeverity)
	}
	config.whitelistLogTypes = stringSetFromSlice(strings.Split(os.Getenv("LOGTHING_WHITELIST_LOG_TYPES"), ","))
	if printMaxSeverity, err := strconv.Atoi(os.Getenv("LOGTHING_PRINT_MAX_SEVERITY")); err == nil {
		config.printMaxSeverity = Severity(printMaxSeverity)
	}
	config.printOutputProperties = stringSetFromSlice(strings.Split(os.Getenv("LOGTHING_PRINT_PROPERTIES"), ","))
}

func init() {
	initConfig()
	isSystemD := (os.Getenv("INVOCATION_ID") != "")
	for lvl := Severity(0); lvl < SeverityNotApplied; lvl++ {
		writer := os.Stdout
		if lvl <= SeverityError {
			writer = os.Stderr
		}
		prefix := logPrefixes[lvl]
		flag := log.Lshortfile | log.LstdFlags
		if isSystemD {
			prefix = fmt.Sprintf("<%v>%v", lvl, logPrefixes[lvl])
			flag = log.Lshortfile
		}
		*loggers[lvl] = log.New(writer, prefix, flag)
		if lvl > config.printMaxSeverity {
			(*loggers[lvl]).SetOutput(ioutil.Discard)
		}
	}
}

// InitDispatcher to init logthing log message dispatcher with given writers.
// When logthing isn't needed anymore (e.g. when the application exits) Close() must be called.
func InitDispatcher(logWriters []logwriter.LogWriter) (err error) {
	if ld != nil {
		ld.close()
	}
	ld, err = newLogDispatcher(logWriters)
	return
}

// Close to flush all queued messages and close the writers
func Close() {
	if ld != nil {
		ld.close()
	}
}

// LogMsg outputs and sends LogMessage with default dispatcher
//
// returns:
//
// ErrNotInitialized when the dispatcher hasn't been initialized
//
// ErrSeverityAboveMax when the message's severity is above the max severity level. See LOGTHING_LOG_MAX_SEVERITY
//
// ErrWrongMessageType whe the log message is of wrong type. Ensure that LogMessage has been created by calling NewLogMsg()
//
// ErrChannelFull when there is no empty space in the LogMessage queue
func LogMsg(msg LogMessage) (err error) {
	return logMsgWithCalldev(2, msg)
}

func logMsgWithCalldev(calldepth int, msg LogMessage) (err error) {
	if ld == nil {
		return ErrNotInitialized
	}
	err = ld.log(calldepth+1, msg)
	return
}
