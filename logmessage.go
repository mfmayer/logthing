package logthing

import (
	"fmt"
	"path/filepath"
	"runtime"
	"time"
)

// logMessageStruct type consists of multiple log entries
type logMessageStruct struct {
	timestamp      time.Time
	logMessageType string
	severity       Severity
	trackingID     string
	output         []string
	properties     map[string]interface{}
}

// LogMessage is the interface to log structured information and output log messages
type LogMessage interface {
	// SetSeverity sets log message severity level (only if given severity level is lower than current)
	SetSeverity(severity Severity) LogMessage
	// Severity returns log message severity level
	Severity() Severity
	// SetTrackingID sets log message tracking ID
	SetTrackingID(trackingID string) LogMessage
	// TrackingID returns log message tracking ID
	TrackingID() string
	// SetTimestamp sets log message timestamp
	SetTimestamp(time time.Time) LogMessage
	// Timestamp returns log message timestamp
	Timestamp() time.Time
	// SetProperty allows to add any structured information to the log message that can be marshalled to JSON
	// NOTE: "timestamp", "type", "severtiy", "trackingID", "output" are reserved keys and will be overwritten eventually
	SetProperty(key string, value interface{}) LogMessage
	// Property returns value with given key. If the value isn't found, ok will be false
	Property(key string) (value interface{}, ok bool)
	// Trace appends output data to be printed and implicitly sets appropriate severity level
	Trace(output ...interface{}) LogMessage
	// Tracef appends output data to be printed and implicitly sets appropriate severity level
	Tracef(format string, v ...interface{}) LogMessage
	// Info appends output data to be printed and implicitly sets appropriate severity level
	Info(output ...interface{}) LogMessage
	// Infof appends output data to be printed and implicitly sets appropriate severity level
	Infof(format string, v ...interface{}) LogMessage
	// Notice appends output data to be printed and implicitly sets appropriate severity level
	Notice(output ...interface{}) LogMessage
	// Noticef appends output data to be printed and implicitly sets appropriate severity level
	Noticef(format string, v ...interface{}) LogMessage
	// Warning appends output data to be printed and implicitly sets appropriate severity level
	Warning(output ...interface{}) LogMessage
	// Warningf appends output data to be printed and implicitly sets appropriate severity level
	Warningf(format string, v ...interface{}) LogMessage
	// Error appends output data to be printed and implicitly sets appropriate severity level
	Error(output ...interface{}) LogMessage
	// Errorf appends output data to be printed and implicitly sets appropriate severity level
	Errorf(format string, v ...interface{}) LogMessage
	// Critical appends output data to be printed and implicitly sets appropriate severity level
	Critical(output ...interface{}) LogMessage
	// Criticalf appends output data to be printed and implicitly sets appropriate severity level
	Criticalf(format string, v ...interface{}) LogMessage
	// Alert appends output data to be printed and implicitly sets appropriate severity level
	Alert(output ...interface{}) LogMessage
	// Alertf appends output data to be printed and implicitly sets appropriate severity level
	Alertf(format string, v ...interface{}) LogMessage
	// Emergency appends output data to be printed and implicitly sets appropriate severity level
	Emergency(output ...interface{}) LogMessage
	// Emergencyf appends output data to be printed and implicitly sets appropriate severity level
	Emergencyf(format string, v ...interface{}) LogMessage
	// AppendOutput appends information to be printed and sets given severity level
	AppendOutput(severity Severity, output ...interface{}) LogMessage
	// Log is a convenience function for LogMsg(LogMessage)
	Log() error
}

// NewLogMsg creates new log message and sets the given type
func NewLogMsg(messageType string) LogMessage {
	msg := &logMessageStruct{
		logMessageType: messageType,
		severity:       SeverityTrace,
	}
	return msg
}

// Log is a convenience function for LogMsg(LogMessage)
func (lm *logMessageStruct) Log() error {
	return logMsgWithCalldev(2, lm)
}

// SetSeverity sets log message severity level (only if given severity level is lower than current)
func (lm *logMessageStruct) SetSeverity(severity Severity) LogMessage {
	if lm != nil {
		if severity < lm.severity {
			lm.severity = severity
		}
	}
	return lm
}

// Severity returns log message severity level
func (lm *logMessageStruct) Severity() Severity {
	if lm == nil {
		return SeverityNotApplied
	}
	return lm.severity
}

// SetTrackingID sets log message tracking ID
func (lm *logMessageStruct) SetTrackingID(trackingID string) LogMessage {
	if lm != nil {
		lm.trackingID = trackingID
	}
	return lm
}

// TrackingID returns log message tracking ID
func (lm *logMessageStruct) TrackingID() string {
	if lm == nil {
		return ""
	}
	return lm.trackingID
}

// SetTimestamp sets log message timestamp
func (lm *logMessageStruct) SetTimestamp(timestamp time.Time) LogMessage {
	if lm != nil {
		lm.timestamp = timestamp
	}
	return lm
}

// Timestamp returns log message timestamp
func (lm *logMessageStruct) Timestamp() time.Time {
	if lm == nil {
		return time.Time{}
	}
	return lm.timestamp
}

// SetProperty allows to add any structured information to the log message that can be marshalled to JSON
// NOTE: keys "timestamp", "type", "severtiy", "trackingID", "output" are reserved keys and will be overwritten eventually
func (lm *logMessageStruct) SetProperty(key string, value interface{}) LogMessage {
	if lm != nil {
		if lm.properties == nil {
			lm.properties = map[string]interface{}{}
		}
		lm.properties[key] = value
	}
	return lm
}

// Property returns value with given key. If the value isn't found, ok will be false
func (lm *logMessageStruct) Property(key string) (value interface{}, ok bool) {
	if lm == nil || lm.properties == nil {
		return
	}
	value, ok = lm.properties[key]
	return
}

// Trace appends output data to be printed and implicitly sets appropriate severity level
func (lm *logMessageStruct) Trace(v ...interface{}) LogMessage {
	return lm.appendOutput(2, SeverityTrace, v...)
}

// Tracef appends output data to be printed and implicitly sets appropriate severity level
func (lm *logMessageStruct) Tracef(format string, v ...interface{}) LogMessage {
	return lm.appendOutput(2, SeverityTrace, fmt.Sprintf(format, v...))
}

// Info appends output data to be printed and implicitly sets appropriate severity level
func (lm *logMessageStruct) Info(v ...interface{}) LogMessage {
	return lm.appendOutput(2, SeverityInfo, v...)
}

// Infof appends output data to be printed and implicitly sets appropriate severity level
func (lm *logMessageStruct) Infof(format string, v ...interface{}) LogMessage {
	return lm.appendOutput(2, SeverityInfo, fmt.Sprintf(format, v...))
}

// Notice appends output data to be printed and implicitly sets appropriate severity level
func (lm *logMessageStruct) Notice(v ...interface{}) LogMessage {
	return lm.appendOutput(2, SeverityNotice, v...)
}

// Noticef appends output data to be printed and implicitly sets appropriate severity level
func (lm *logMessageStruct) Noticef(format string, v ...interface{}) LogMessage {
	return lm.appendOutput(2, SeverityNotice, fmt.Sprintf(format, v...))
}

// Warning appends output data to be printed and implicitly sets appropriate severity level
func (lm *logMessageStruct) Warning(v ...interface{}) LogMessage {
	return lm.appendOutput(2, SeverityWarning, v...)
}

// Warningf appends output data to be printed and implicitly sets appropriate severity level
func (lm *logMessageStruct) Warningf(format string, v ...interface{}) LogMessage {
	return lm.appendOutput(2, SeverityWarning, fmt.Sprintf(format, v...))
}

// Error appends output data to be printed and implicitly sets appropriate severity level
func (lm *logMessageStruct) Error(v ...interface{}) LogMessage {
	return lm.appendOutput(2, SeverityError, v...)
}

// Errorf appends output data to be printed and implicitly sets appropriate severity level
func (lm *logMessageStruct) Errorf(format string, v ...interface{}) LogMessage {
	return lm.appendOutput(2, SeverityError, fmt.Sprintf(format, v...))
}

// Critical appends output data to be printed and implicitly sets appropriate severity level
func (lm *logMessageStruct) Critical(v ...interface{}) LogMessage {
	return lm.appendOutput(2, SeverityCritical, v...)
}

// Criticalf appends output data to be printed and implicitly sets appropriate severity level
func (lm *logMessageStruct) Criticalf(format string, v ...interface{}) LogMessage {
	return lm.appendOutput(2, SeverityCritical, fmt.Sprintf(format, v...))
}

// Alert appends output data to be printed and implicitly sets appropriate severity level
func (lm *logMessageStruct) Alert(v ...interface{}) LogMessage {
	return lm.appendOutput(2, SeverityAlert, v...)
}

// Alertf appends output data to be printed and implicitly sets appropriate severity level
func (lm *logMessageStruct) Alertf(format string, v ...interface{}) LogMessage {
	return lm.appendOutput(2, SeverityAlert, fmt.Sprintf(format, v...))
}

// Emergency appends output data to be printed and implicitly sets appropriate severity level
func (lm *logMessageStruct) Emergency(v ...interface{}) LogMessage {
	return lm.appendOutput(2, SeverityEmergency, v...)
}

// Emergencyf appends output data to be printed and implicitly sets appropriate severity level
func (lm *logMessageStruct) Emergencyf(format string, v ...interface{}) LogMessage {
	return lm.appendOutput(2, SeverityEmergency, fmt.Sprintf(format, v...))
}

// AppendOutput appends information to be printed and sets given severity level
func (lm *logMessageStruct) AppendOutput(severity Severity, output ...interface{}) LogMessage {
	return lm.appendOutput(2, severity, output...)
}

func (lm *logMessageStruct) appendOutput(calldepth int, severity Severity, values ...interface{}) *logMessageStruct {
	if lm != nil && len(values) > 0 {
		var outputStrings []string
		for _, v := range values {
			outputStrings = append(outputStrings, fmt.Sprint(v))
		}
		_, file, line, _ := runtime.Caller(calldepth)
		if len(file) > 0 {
			file = filepath.Base(file)
		}
		lm.SetSeverity(severity)

		if len(outputStrings) == 1 {
			lm.output = append(lm.output, fmt.Sprintf("%v:%v: %v", file, line, outputStrings[0]))
		} else {
			lm.output = append(lm.output, fmt.Sprintf("%v:%v:", file, line))
			lm.output = append(lm.output, outputStrings...)
		}
	}
	return lm
}
