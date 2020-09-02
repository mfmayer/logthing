# logthing

[![license](http://img.shields.io/badge/license-MIT-red.svg?style=flat)](https://raw.githubusercontent.com/rs/zerolog/master/LICENSE)

logthing is just another logging module that implements structured json logging. It's focus lies on providing dispatching functionality to write logs and metrics to cloud servcies like Azure Monitor (Azure Log Analytics workspaces) and ElasticSearch. By implementing the LogWriters interface it can be extended to log to additional log stores.

The following log writers are part of the logwriter package:

* Azure Monitor - to log into Azure Log Analytics Workspaces
* ElasticSearch - to log into an ElasticSearch database (In Progress)

## Getting Started

Before trying the code, ensure that relevant environment variables are set (see Configuraiton).

```go
package main

import (
	"github.com/mfmayer/logthing"
	"github.com/mfmayer/logthing/logwriter"
)

func main() {
	// Init the dispatcher with an Azure Monitoro writer
	if err := logthing.InitDispatcher([]logwriter.LogWriter{
		logwriter.NewAzureMonitorWriter(),
	}); err != nil {
		logthing.Error.Printf("Error init dispatcher: %v", err)
	}

	logMsg := logthing.NewLogMsg("<some_type>")         // Create & log a message with a trackingID, a foo & bar property and a info message
	logMsg.SetTrackingID("<some_tracking_id>")          // Add a tracking id
	logMsg.SetProperty("foo", 12345)                    // set a property
	logMsg.SetProperty("bar", []int{5, 6, 7, 8})        // set another property
	logMsg.Infof("Hello %v", []string{"World", "Moon"}) // set an info message
	logthing.LogMsg(logMsg)                             // log the message

	// The calls can be also lined up as in this additional example:
	logthing.NewLogMsg("<another_type>"). // create the message
						Infof("Hello %v", []string{"Mom", "Dad"}).                                       // add an info message
						SetProperty("windStringified", logthing.SProp{"speed": 10, "directions": 25.5}). // add a wind property (stringified)
						SetProperty("wind", map[string]interface{}{"speed": 10, "directions": 25.5}).    // add the same wind property (non-stringified)
						SetProperty("rain", 10).                                                         // add arain property
						Warningf("The weather is %v", []string{"rainy", "stormy"}).                      // and add a warning message (the severity level is automatically adjusted to the lowest level)
						Log()                                                                               // log the message

	logthing.Close()
}

/* Outputs:
INFO:  2020/09/02 20:47:14 logthing_test.go:23: logthing_test.go:22: Hello [World Moon] ([foo:12345 trackingID:<some_tracking_id>])
WARN:  2020/09/02 20:47:14 logthing_test.go:32: logthing_test.go:27: Hello [Mom Dad]
                           logthing_test.go:31: The weather is [rainy stormy]
*/
```

### Configuration

Since logthing is meant for service logging and also credentials must be somehow given to logthing, most of the configuration happens via environment variables:

| Environment Variable              | Description                                                                                                 |
| --------------------------------- | ----------------------------------------------------------------------------------------------------------- |
| LOGTHING_LOG_NAME or SERVICE_NAME | Log name under which log messages are stored (will be used as elasticsearch index or azure custom log type) |
| LOGTHING_LOG_MAX_SEVERITY         | Messages with severity > LOGTHING_LOG_MAX_SEVERITY won't be logged and are immediately dropped              |
| LOGTHING_WHITELIST_LOG_TYPES      | Messages that match any whitelisted log type (comma separated) are logged independent of their severity     |
| LOGTHING_PRINT_MAX_SEVERITY       | Messages with severity <= LOG_OUTPUT_SEVERITY_MAX are directly printed to stdout / stderr                   |
| LOGTHING_PRINT_PROPERTIES         | Message properties that match any give print property (comma separated) are printed with the message output |

#### Azure Montior

For the Azure Monitor writer additional environment variables must be set (for details how the used API is working see: https://docs.microsoft.com/de-de/azure/azure-monitor/platform/data-collector-api):

| Environment Variable          | Description                                   |
| ----------------------------- | --------------------------------------------- |
| LOGTHING_AZURE_WORKSPACE_ID   | Azure log analytics workspace id              |
| LOGTHING_AZURE_WORKSPACE_KEY  | Azure log analytics worksoace key             |
| LOGTHING_AZURE_MONITOR_DOMAIN | To overwrite the default azure monitor domain |

#### ElasticSearch

For ElasticSearch the following environment variables are needed:

| Environment Variable | Description                                      |
| -------------------- | ------------------------------------------------ |
| ELASTICSEARCH_URL    | The URL under which the database can be accessed |
| ELASTICSEARCH_USER   | ElasticSearch Username                           |
| ELASTICSEARCH_PWD    | ElasticSearch Password                           |

