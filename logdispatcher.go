package logthing

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mfmayer/logthing/logwriter"
)

type dispatcherOptions struct {
	dispatchInterval time.Duration
	queueSize        int
	dispatchCallback func(msg LogMsg)
	overflowCallback func(droppedMsg LogMsg, overflowCount uint64)
}

// logDispatcher can be created using newLogDispatcher and can be used to write log messages to various cloud logging services
// with appropriate log writers like Azure Monitor (Azure Log Analytics) and/or ElasticSearch.
// By implementing own LogWriter interface additional services can be added.
//
// Must be closed when no longer needed, to ensure that all log messages have been written, user writers are closed and resources are freed
//
// Currently dispatcher is created by the logthing module and can be configuresd via environment variables.
// Therefore only a single one is supported and it's unclear whether it makes sense to have multiple dispatchers.
type logDispatcher struct {
	options         dispatcherOptions
	logMessageCh    chan *logMsg
	logWriters      []logwriter.LogWriter
	done            chan bool
	overflowCounter uint64
}

// NewLogDispatcher returns a new LogDispatcher
func newLogDispatcher(logWriters []logwriter.LogWriter, opts ...func(*dispatcherOptions)) (ld *logDispatcher, err error) {
	options := dispatcherOptions{
		dispatchInterval: 5 * time.Second,
		queueSize:        8192,
	}
	for _, opt := range opts {
		opt(&options)
	}

	ld = &logDispatcher{
		options:      options,
		logMessageCh: make(chan *logMsg, options.queueSize),
		done:         make(chan bool),
	}
	lwConfig := logwriter.Config{
		LogName: config.logName,
	}
	var lwInitErrors []error
	for _, logWriter := range logWriters {
		lwInitError := logWriter.Init(lwConfig)
		if lwInitError == nil {
			ld.logWriters = append(ld.logWriters, logWriter)
		} else {
			lwInitErrors = append(lwInitErrors, lwInitError)
		}
	}
	if len(lwInitErrors) > 0 {
		err = fmt.Errorf("init of writers failed: %v", lwInitErrors)
	}

	go func(ld *logDispatcher) {
		ticker := time.NewTicker(options.dispatchInterval)
		var logMessages []*logMsg
		for {
			select {
			case <-ticker.C:
				ld.writeLogMessages(logMessages)
				logMessages = nil
			case msg, more := <-ld.logMessageCh:
				if msg != nil {
					logMessages = append(logMessages, msg)
				}
				if !more {
					ld.writeLogMessages(logMessages)
					logMessages = nil
					close(ld.done)
					return
				}
			}
		}
	}(ld)
	return
}

// close flushes all logMessages, closes all writers and ends the dispatcher
func (ld *logDispatcher) close() {
	if ld == nil {
		return
	}
	close(ld.logMessageCh)
	<-ld.done // wait until dispatcher finished writing all logMessages

	// Close the writers
	for _, lw := range ld.logWriters {
		if lw != nil {
			lw.Close()
		}
	}
}

// writeLogMessages pre-marshals the log message and forwards it to all registered writers
func (ld *logDispatcher) writeLogMessages(logMessages []*logMsg) {
	if len(logMessages) <= 0 {
		return
	}

	sort.Slice(logMessages, func(i, j int) bool {
		return time.Time(logMessages[i].timestamp).Before(time.Time(logMessages[j].timestamp))
	})

	rawLogMessages := make([]json.RawMessage, len(logMessages))
	timestamps := make([]time.Time, len(logMessages))
	j := 0
	for _, logMessage := range logMessages {
		if rawLogMessage, err := json.Marshal(logMessage.Properties()); err != nil {
			Error.Printf("Error while marshalling log message: %v", err)
		} else {
			rawLogMessages[j] = rawLogMessage
			timestamps[j] = logMessage.Timestamp()
			j++
		}
	}
	rawLogMessages = rawLogMessages[:j]
	timestamps = timestamps[:j]
	for i, lw := range ld.logWriters {
		if lw != nil {
			err := lw.WriteLogMessages(rawLogMessages, timestamps)
			if err != nil {
				Error.Printf("Error while writing log message: %v", err)
				if errors.Is(err, logwriter.ErrWriterDisable) { // if writer returns ErrWriterStop, it is closed and removed from registered writers
					lw.Close()
					ld.logWriters[i] = nil
				}
			}
		}
	}
}

// printLogMsg formats and prints the log message's properties and given output
func printLogMsg(calldepth int, msg *logMsg) {
	if msg == nil {
		return
	}
	output := msg.Output()
	if len(output) > 0 {
		var lg *log.Logger
		if msg.severity < SeverityNotApplied {
			lg = *loggers[msg.severity]
		}
		_, file, line, ok := runtime.Caller(calldepth)
		if !ok {
			file = "???"
			line = 0
		} else {
			file = filepath.Base(file)
		}
		outputProperties := []string{fmt.Sprintf("%v:%v", file, line)}
		for outputProperty := range config.printOutputProperties {
			if outputPropertyValue := msg.Property(outputProperty); outputPropertyValue != nil {
				v := fmt.Sprintf("%v:%v", outputProperty, outputPropertyValue)
				if len(v) > 0 {
					outputProperties = append(outputProperties, v)
				}
			}
		}
		calldepth++
		logString := ""
		logString += strings.Join(output, newLineSpacer)
		if len(outputProperties) > 0 {
			outputPropertiesString := fmt.Sprintf("(%v)", outputProperties)
			if len(output) > 1 {
				lg.Output(calldepth, outputPropertiesString+newLineSpacer+logString)
			} else {
				lg.Output(calldepth, logString+" "+outputPropertiesString)
			}
		} else {
			lg.Output(calldepth, logString)
		}
	}
}

// log prints the log message and queues it to be written
func (ld *logDispatcher) log(calldepth int, logMessage LogMsg) error {
	if ld.options.dispatchCallback != nil {
		ld.options.dispatchCallback(logMessage)
	}

	// msg, ok := logMessage.(*logMsg)
	// if !ok {
	// 	return ErrWrongMessageType
	// }
	msg := logMessage.msgData()

	// Set at least trace severity
	msg.SetSeverity(SeverityTrace)

	// Drop message if severity is greater than configured logSeverity and according logType is not explicitely whitelisted
	whitelisted := config.isWhitelisted(msg.logMessageType)
	if !config.meetsLogMaxSeverity(msg.Severity()) {
		if !whitelisted {
			return ErrSeverityAboveMax
		}
	}

	// Ensure that timestamp is set
	if time.Time(msg.timestamp).IsZero() {
		msg.timestamp = UTCTime(time.Now())
	}

	// Ensure that msg properties are complete, because only the properties will be marshalled and logged
	msg.SetProperty(PropertyType, msg.logMessageType)
	msg.SetProperty(PropertyTimestamp, msg.timestamp)
	msg.SetProperty(PropertySeverity, msg.severity)
	if msg.trackingID != "" {
		msg.SetProperty(PropertyTrackingID, msg.trackingID)
	}

	// Print msg to stdout/stderr
	if whitelisted || config.meetsPrintMaxSeverity(msg.Severity()) {
		printLogMsg(calldepth+1, msg)
	}

	// Also make msg output part of its properties
	msg.SetProperty("output", msg.output)

	select {
	case ld.logMessageCh <- msg:
	default:
		overflowCount := atomic.AddUint64(&ld.overflowCounter, 1)
		if ld.options.overflowCallback != nil {
			ld.options.overflowCallback(msg, overflowCount)
		}
		return ErrChannelFull
	}
	return nil
}
