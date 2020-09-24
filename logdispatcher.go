package logthing

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/mfmayer/logthing/logwriter"
)

// logDispatcher can be created using newLogDispatcher and can be used to write log messages to various cloud logging services
// with appropriate log writers like Azure Monitor (Azure Log Analytics) and/or ElasticSearch.
// By implementing own LogWriter interface additional services can be added.
//
// Must be closed when no longer needed, to ensure that all log messages have been written, user writers are closed and resources are freed
//
// Currently dispatcher is created by the logthing module and can be configuresd via environment variables.
// Therefore only a single one is supported and it's unclear whether it makes sense to have multiple dispatchers.
type logDispatcher struct {
	logMessageCh chan *logMsg
	logWriters   []logwriter.LogWriter
	done         chan bool
}

// NewLogDispatcher returns a new LogDispatcher
func newLogDispatcher(logWriters []logwriter.LogWriter) (ld *logDispatcher, err error) {
	ld = &logDispatcher{
		logMessageCh: make(chan *logMsg, 4096),
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
		err = fmt.Errorf("Init of writers failed: %v", lwInitErrors)
	}

	go func(ld *logDispatcher) {
		ticker := time.NewTicker(5 * time.Second)
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
		if time.Time(logMessages[i].timestamp).Before(time.Time(logMessages[j].timestamp)) {
			return true
		}
		return false
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

// printLogMsg formats and prints the log message
func printLogMsg(calldepth int, msg *logMsg) {
	if len(msg.output) > 0 && config.printSeverity(msg.Severity()) {
		var lg *log.Logger
		if msg.severity < SeverityNotApplied {
			lg = *loggers[msg.severity]
		}
		outputProperties := []string{}
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
		logString += strings.Join(msg.output, "\n                           ")
		if len(outputProperties) > 0 {
			outputPropertiesString := fmt.Sprintf("(%v)", outputProperties)
			if len(msg.output) > 1 {
				lg.Output(calldepth, outputPropertiesString+"\n                           "+logString)
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
	msg, ok := logMessage.(*logMsg)
	if !ok {
		return ErrWrongMessageType
	}

	// Set at least trace severity
	msg.SetSeverity(SeverityTrace)

	// Drop message if severity is greater than configured logSeverity and according logType is not explicitely defined
	if msg.severity > config.logMaxSeverity {
		if len(msg.logMessageType) > 0 {
			if _, found := config.whitelistLogTypes[msg.logMessageType]; !found {
				return ErrSeverityAboveMax
			}
		}
	}

	// Ensure that timestamp is set
	if time.Time(msg.timestamp).IsZero() {
		msg.timestamp = UTCTime(time.Now())
	}

	// Ensure that msg properties are complete, because only the properties will be marshalled and logged
	msg.SetProperty("type", msg.logMessageType)
	msg.SetProperty("timestamp", msg.timestamp)
	msg.SetProperty("severity", msg.severity)
	if msg.trackingID != "" {
		msg.SetProperty("trackingID", msg.trackingID)
	}

	// Print msg to stdout/stderr
	printLogMsg(calldepth+1, msg)

	// Also make msg output part of its properties
	msg.SetProperty("output", msg.output)

	select {
	case ld.logMessageCh <- msg:
	default:
		return ErrChannelFull
	}
	return nil
}
