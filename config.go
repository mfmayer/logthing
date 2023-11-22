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
	whitelistProperties   map[string]struct{}
	printMaxSeverity      Severity
	printOutputProperties map[string]struct{}
}

var config configStruct = configStruct{
	logName:               os.Getenv("LOGTHING_LOG_NAME"),
	logMaxSeverity:        SeverityTrace,
	whitelistLogTypes:     map[string]struct{}{},
	whitelistProperties:   map[string]struct{}{},
	printMaxSeverity:      SeverityError,
	printOutputProperties: map[string]struct{}{},
}

func (c configStruct) meetsPrintMaxSeverity(severity Severity) bool {
	return severity <= config.printMaxSeverity && config.printMaxSeverity != SeverityNotApplied
}

func (c configStruct) meetsLogMaxSeverity(severity Severity) bool {
	return severity <= config.logMaxSeverity && config.logMaxSeverity != SeverityNotApplied
}

func (c configStruct) isWhitelistedProperty(key string) bool {
	if len(c.whitelistProperties) == 0 {
		return true
	}
	if key == PropertyOutput || key == PropertyTimestamp || key == PropertyType || key == PropertySeverity || key == PropertyWhitelist {
		return true
	}
	if _, ok := c.whitelistProperties[key]; ok {
		return true
	}
	return false
}

func (c configStruct) isWhitelisted(logType string) bool {
	whitelisted := false
	if len(logType) > 0 {
		_, whitelisted = c.whitelistLogTypes[logType]
	}
	return whitelisted
}

func initConfig() {
	if config.logName == "" {
		config.logName = "default"
	}
	if logMaxSeverity, err := strconv.Atoi(os.Getenv("LOGTHING_LOG_MAX_SEVERITY")); err == nil {
		config.logMaxSeverity = Severity(logMaxSeverity)
	}
	if printMaxSeverity, err := strconv.Atoi(os.Getenv("LOGTHING_PRINT_MAX_SEVERITY")); err == nil {
		config.printMaxSeverity = Severity(printMaxSeverity)
	}
	config.whitelistProperties = stringSetFromSlice(strings.Split(os.Getenv("LOGTHING_WHITELIST_PROPERTIES"), ","))
	config.whitelistLogTypes = stringSetFromSlice(strings.Split(os.Getenv("LOGTHING_WHITELIST_LOG_TYPES"), ","))
	config.printOutputProperties = stringSetFromSlice(strings.Split(os.Getenv("LOGTHING_PRINT_PROPERTIES"), ","))
}

// ConfigLogName returns configured log name (LOGTHING_LOG_NAME)
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

// ConfigWhiteListLogTypes returns list of whitelisted log types (LOGTHING_WHITELIST_LOG_TYPES)
func ConfigWhiteListLogTypes() []string {
	types := []string{}
	for k := range config.whitelistLogTypes {
		types = append(types, k)
	}
	return types
}

// ConfigPrintOutputProperties returns list of properties that are added to stdout/stderr output of log messages (LOGTHING_PRINT_PROPERTIES)
func ConfigPrintOutputProperties() []string {
	types := []string{}
	for k := range config.printOutputProperties {
		types = append(types, k)
	}
	return types
}
