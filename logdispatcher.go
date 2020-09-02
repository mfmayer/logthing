package logthing

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
	logMessageCh chan map[string]interface{}
	logWriters   []logwriter.LogWriter
	done         chan bool
}

// NewLogDispatcher returns a new LogDispatcher
func newLogDispatcher(logWriters []logwriter.LogWriter) (ld *logDispatcher, err error) {
	ld = &logDispatcher{
		logMessageCh: make(chan map[string]interface{}, 4096),
		done:         make(chan bool),
	}
	var lwInitErrors []error
	for _, logWriter := range logWriters {
		lwInitError := logWriter.Init()
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
		var logMessages []map[string]interface{}
		for {
			select {
			case <-ticker.C:
				// logger.logElasticSearch(logMessages)
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
func (ld *logDispatcher) writeLogMessages(logMessages []map[string]interface{}) {
	if len(logMessages) <= 0 {
		return
	}
	rawLogMessages := make([]json.RawMessage, len(logMessages))
	j := 0
	for _, logMessage := range logMessages {
		if rawLogMessage, err := json.Marshal(logMessage); err != nil {
			Error.Printf("Error while marshalling log message: %v", err)
		} else {
			rawLogMessages[j] = rawLogMessage
			j++
		}
	}
	rawLogMessages = rawLogMessages[:j]
	for i, lw := range ld.logWriters {
		if lw != nil {
			err := lw.WriteLogMessages(rawLogMessages)
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

// log formats the log message and queues it to be written
func (ld *logDispatcher) log(calldepth int, logMessage LogMessage) error {
	msg, ok := logMessage.(*logMessageStruct)
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

	if time.Time(msg.timestamp).IsZero() {
		msg.timestamp = time.Now()
	}

	if msg.properties == nil {
		msg.properties = map[string]interface{}{}
	}
	msg.properties["type"] = msg.logMessageType
	msg.properties["timestamp"] = UTCTime(msg.timestamp.UTC())
	msg.properties["severity"] = msg.severity
	if msg.trackingID != "" {
		msg.properties["trackingID"] = msg.trackingID
	}
	msg.properties["output"] = msg.output

	// Print logMessage Output using appropriate logger
	if len(msg.output) > 0 {
		var lg *log.Logger
		if msg.severity < SeverityNotApplied {
			lg = *loggers[msg.severity]
		}
		outputProperties := []string{}
		for outputProperty := range config.printOutputProperties {
			if opv, ok := msg.properties[outputProperty]; ok {
				v := fmt.Sprintf("%v:%v", outputProperty, opv)
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

	select {
	case ld.logMessageCh <- msg.properties:
	default:
		return ErrChannelFull
	}
	return nil
}
