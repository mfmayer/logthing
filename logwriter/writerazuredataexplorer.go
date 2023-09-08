package logwriter

import (
	"encoding/json"
	"time"
)

// AzureMonitor log writer
type azureDataExplorer struct {
	logName string
}

func (de *azureDataExplorer) Init(config Config) (err error) {
	de.logName = config.LogName
	return
}

func (de *azureDataExplorer) WriteLogMessages(logMessages []json.RawMessage, timestamps []time.Time) (err error) {
	return
}

func (de *azureDataExplorer) Close() {
	return
}
