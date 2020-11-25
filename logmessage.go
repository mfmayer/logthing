package logthing

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Severity to declare log message severities
type Severity uint

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

// logMsg type consists of multiple log entries
type logMsg struct {
	timestamp      UTCTime
	logMessageType string
	severity       Severity
	trackingID     string
	output         []string
	properties     interface{} //map[string]interface{}
}

// LogMsg is the interface to build up a log message with structured data and formatted text.
// Structured data and formatted text will be dispatched to log writers. The formatted text will be also printed to stderr and stdout.
type LogMsg interface {
	//SetType(msgType string) LogMsg                                // sets log message type
	Type() string                                                 // returns log message type
	SetSeverity(severity Severity) LogMsg                         // sets log message severity level (only if given severity level is lower than current)
	Severity() Severity                                           // returns log message severity level
	SetTrackingID(trackingID string) LogMsg                       // sets log message tracking ID
	TrackingID() string                                           // returns log message tracking ID
	SetTimestamp(time time.Time) LogMsg                           // sets log message timestamp
	Timestamp() time.Time                                         // returns log message timestamp
	SetProperty(key string, value interface{}) LogMsg             // sets property value for given key. NOTE: "timestamp", "type", "severtiy", "trackingID" and "output" are reserved keys. They do have separate set functions.
	SetSProperty(key string, value interface{}) LogMsg            // like SetProperty but stringifies the value will be stringified
	Property(key string) interface{}                              // returns value with given key. If the value isn't found, ok will be false.
	Properties() map[string]interface{}                           // returns property map
	Output() []string                                             // returns output data
	Trace(output ...interface{}) LogMsg                           // appends output data to be printed and implicitly sets appropriate severity level
	Tracef(format string, v ...interface{}) LogMsg                // appends output data to be printed and implicitly sets appropriate severity level
	Info(output ...interface{}) LogMsg                            // appends output data to be printed and implicitly sets appropriate severity level
	Infof(format string, v ...interface{}) LogMsg                 // appends output data to be printed and implicitly sets appropriate severity level
	Notice(output ...interface{}) LogMsg                          // appends output data to be printed and implicitly sets appropriate severity level
	Noticef(format string, v ...interface{}) LogMsg               // appends output data to be printed and implicitly sets appropriate severity level
	Warning(output ...interface{}) LogMsg                         // appends output data to be printed and implicitly sets appropriate severity level
	Warningf(format string, v ...interface{}) LogMsg              // appends output data to be printed and implicitly sets appropriate severity level
	Error(output ...interface{}) LogMsg                           // appends output data to be printed and implicitly sets appropriate severity level
	Errorf(format string, v ...interface{}) LogMsg                // appends output data to be printed and implicitly sets appropriate severity level
	Critical(output ...interface{}) LogMsg                        // appends output data to be printed and implicitly sets appropriate severity level
	Criticalf(format string, v ...interface{}) LogMsg             // appends output data to be printed and implicitly sets appropriate severity level
	Alert(output ...interface{}) LogMsg                           // appends output data to be printed and implicitly sets appropriate severity level
	Alertf(format string, v ...interface{}) LogMsg                // appends output data to be printed and implicitly sets appropriate severity level
	Emergency(output ...interface{}) LogMsg                       // appends output data to be printed and implicitly sets appropriate severity level
	Emergencyf(format string, v ...interface{}) LogMsg            // appends output data to be printed and implicitly sets appropriate severity level
	AppendOutput(severity Severity, output ...interface{}) LogMsg // appends information to be printed and sets given severity level
	Log() error                                                   // is a convenience function for Log(LogMessage) / LogMsgWithCalldepth(calldepth, LogMessage)
}

// NewLogMsg creates new log message and sets the given type
func NewLogMsg(messageType string) LogMsg {
	msg := &logMsg{
		logMessageType: messageType,
		severity:       SeverityTrace,
	}
	return msg
}

// Log is a convenience function for LogMsg(LogMessage)
func (lm *logMsg) Log() error {
	return LogMsgWithCalldepth(2, lm)
}

// Type returns log message type
func (lm *logMsg) Type() string {
	if lm != nil {
		return lm.logMessageType
	}
	return ""
}

// SetSeverity sets log message severity level (only if given severity level is lower than current)
func (lm *logMsg) SetSeverity(severity Severity) LogMsg {
	if lm != nil {
		if severity < lm.severity {
			lm.severity = severity
		}
	}
	return lm
}

// Severity returns log message severity level
func (lm *logMsg) Severity() Severity {
	if lm != nil {
		return lm.severity
	}
	return SeverityNotApplied
}

// SetTrackingID sets log message tracking ID
func (lm *logMsg) SetTrackingID(trackingID string) LogMsg {
	if lm != nil {
		lm.trackingID = trackingID
	}
	return lm
}

// TrackingID returns log message tracking ID
func (lm *logMsg) TrackingID() string {
	if lm != nil {
		return lm.trackingID
	}
	return ""
}

// SetTimestamp sets log message timestamp
func (lm *logMsg) SetTimestamp(timestamp time.Time) LogMsg {
	if lm != nil {
		lm.timestamp = UTCTime(timestamp)
	}
	return lm
}

// Timestamp returns log message timestamp
func (lm *logMsg) Timestamp() time.Time {
	if lm != nil {
		return time.Time(lm.timestamp)
	}
	return time.Time{}
}

// SetProperty allows to add any structured information to the log message that can be marshalled to JSON
// NOTE: keys "timestamp", "type", "severtiy", "trackingID", "output" are reserved keys and will be overwritten eventually
func (lm *logMsg) SetProperty(key string, value interface{}) LogMsg {
	lmp := lm.Properties()
	if lmp != nil {
		lmp[key] = value
	}
	return lm
}

// SProp stringifies the sutrucuterd property data before dispatching it to the log writers.
type sProp struct {
	value interface{}
}

// MarshalJSON creates stringified version of
func (sp sProp) MarshalJSON() (ret []byte, err error) {
	ret, err = json.Marshal(sp.value)
	if err == nil {
		ret, err = json.Marshal(string(ret))
	}
	return ret, err
}

// SetSProperty like SetProperty but will stringify the value
func (lm *logMsg) SetSProperty(key string, value interface{}) LogMsg {
	lmp := lm.Properties()
	if lmp != nil {
		lmp[key] = sProp{value: value}
	}
	return lm
}

// Property returns value with given key. If the value isn't found, nil is returned
func (lm *logMsg) Property(key string) interface{} {
	lmp := lm.Properties()
	if lmp != nil {
		if value, ok := lmp[key]; ok {
			return value
		}
	}
	return nil
}

// Properties returns properties
func (lm *logMsg) Properties() map[string]interface{} {
	if lm != nil {
		lmp, ok := lm.properties.(map[string]interface{})
		if !ok || lmp == nil {
			lmp = map[string]interface{}{}
			lm.properties = lmp
		}
		return lmp
	}
	return nil
}

func (lm *logMsg) Output() []string {
	if lm != nil {
		return lm.output
	}
	return nil
}

// Trace appends output data to be printed and implicitly sets appropriate severity level
func (lm *logMsg) Trace(v ...interface{}) LogMsg {
	return lm.appendOutput(2, SeverityTrace, v...)
}

// Tracef appends output data to be printed and implicitly sets appropriate severity level
func (lm *logMsg) Tracef(format string, v ...interface{}) LogMsg {
	return lm.appendOutput(2, SeverityTrace, fmt.Sprintf(format, v...))
}

// Info appends output data to be printed and implicitly sets appropriate severity level
func (lm *logMsg) Info(v ...interface{}) LogMsg {
	return lm.appendOutput(2, SeverityInfo, v...)
}

// Infof appends output data to be printed and implicitly sets appropriate severity level
func (lm *logMsg) Infof(format string, v ...interface{}) LogMsg {
	return lm.appendOutput(2, SeverityInfo, fmt.Sprintf(format, v...))
}

// Notice appends output data to be printed and implicitly sets appropriate severity level
func (lm *logMsg) Notice(v ...interface{}) LogMsg {
	return lm.appendOutput(2, SeverityNotice, v...)
}

// Noticef appends output data to be printed and implicitly sets appropriate severity level
func (lm *logMsg) Noticef(format string, v ...interface{}) LogMsg {
	return lm.appendOutput(2, SeverityNotice, fmt.Sprintf(format, v...))
}

// Warning appends output data to be printed and implicitly sets appropriate severity level
func (lm *logMsg) Warning(v ...interface{}) LogMsg {
	return lm.appendOutput(2, SeverityWarning, v...)
}

// Warningf appends output data to be printed and implicitly sets appropriate severity level
func (lm *logMsg) Warningf(format string, v ...interface{}) LogMsg {
	return lm.appendOutput(2, SeverityWarning, fmt.Sprintf(format, v...))
}

// Error appends output data to be printed and implicitly sets appropriate severity level
func (lm *logMsg) Error(v ...interface{}) LogMsg {
	return lm.appendOutput(2, SeverityError, v...)
}

// Errorf appends output data to be printed and implicitly sets appropriate severity level
func (lm *logMsg) Errorf(format string, v ...interface{}) LogMsg {
	return lm.appendOutput(2, SeverityError, fmt.Sprintf(format, v...))
}

// Critical appends output data to be printed and implicitly sets appropriate severity level
func (lm *logMsg) Critical(v ...interface{}) LogMsg {
	return lm.appendOutput(2, SeverityCritical, v...)
}

// Criticalf appends output data to be printed and implicitly sets appropriate severity level
func (lm *logMsg) Criticalf(format string, v ...interface{}) LogMsg {
	return lm.appendOutput(2, SeverityCritical, fmt.Sprintf(format, v...))
}

// Alert appends output data to be printed and implicitly sets appropriate severity level
func (lm *logMsg) Alert(v ...interface{}) LogMsg {
	return lm.appendOutput(2, SeverityAlert, v...)
}

// Alertf appends output data to be printed and implicitly sets appropriate severity level
func (lm *logMsg) Alertf(format string, v ...interface{}) LogMsg {
	return lm.appendOutput(2, SeverityAlert, fmt.Sprintf(format, v...))
}

// Emergency appends output data to be printed and implicitly sets appropriate severity level
func (lm *logMsg) Emergency(v ...interface{}) LogMsg {
	return lm.appendOutput(2, SeverityEmergency, v...)
}

// Emergencyf appends output data to be printed and implicitly sets appropriate severity level
func (lm *logMsg) Emergencyf(format string, v ...interface{}) LogMsg {
	return lm.appendOutput(2, SeverityEmergency, fmt.Sprintf(format, v...))
}

// AppendOutput appends information to be printed and sets given severity level
func (lm *logMsg) AppendOutput(severity Severity, output ...interface{}) LogMsg {
	return lm.appendOutput(2, severity, output...)
}

func (lm *logMsg) appendOutput(calldepth int, severity Severity, values ...interface{}) *logMsg {
	if lm == nil || len(values) <= 0 {
		return lm
	}
	lm.SetSeverity(severity)
	if len(values) <= 0 {
		return lm
	}
	if !config.meetsLogMaxSeverity(severity) && !config.isWhitelisted(lm.logMessageType) {
		return lm
	}
	_, file, line, ok := runtime.Caller(calldepth)
	if !ok {
		file = "???"
		line = 0
	} else {
		file = filepath.Base(file)
	}
	outputLines := []string{}
	for _, value := range values {
		lines := strings.Split(fmt.Sprint(value), "\n")
		outputLines = append(outputLines, lines...)
	}
	if len(outputLines) == 1 {
		lm.output = append(lm.output, fmt.Sprintf("[%v:%v]: %v", file, line, outputLines[0]))
	} else {
		lm.output = append(lm.output, fmt.Sprintf("[%v:%v]:", file, line))
		for _, outputLine := range outputLines {
			lm.output = append(lm.output, "  "+outputLine)
		}
	}
	return lm
}
