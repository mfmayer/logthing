package logthing

import (
	"os"
	"strconv"
	"strings"
)

type configStruct struct {
	logName               string
	logMaxSeverity        Severity
	whitelistLogTypes     map[string]struct{}
	printMaxSeverity      Severity
	printOutputProperties map[string]struct{}
}

var config configStruct = configStruct{
	logName:               os.Getenv("LOGTHING_LOG_NAME"),
	logMaxSeverity:        SeverityError,
	whitelistLogTypes:     map[string]struct{}{},
	printMaxSeverity:      SeverityTrace,
	printOutputProperties: map[string]struct{}{},
}

func (c configStruct) meetsPrintSeverity(severity Severity) bool {
	return severity <= config.printMaxSeverity && config.printMaxSeverity != SeverityNotApplied
}

func initConfig() {
	if config.logName == "" {
		config.logName = os.Getenv("SERVICE_NAME")
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

// ConfigLogName returns configured log name (LOGTHING_LOG_NAME or SERVICE_NAME)
func ConfigLogName() string {
	return config.logName
}

// ConfigLogMaxSeverity returns configured max severity for which log messages will be written (LOGTHING_LOG_MAX_SEVERITY)
func ConfigLogMaxSeverity() Severity {
	return config.logMaxSeverity
}

// ConfigPrintMaxSeverity returns configure max severity for which log messages will be printed to stdout/stderr (LOGTHING_PRINT_MAX_SEVERITY)
func ConfigPrintMaxSeverity() Severity {
	return config.printMaxSeverity
}
