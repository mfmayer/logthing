package logwriter

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// AzureMonitor log writer
type azureMonitor struct {
	azWorkspaceID string
	azKey         string
	azLogType     string
	azDomain      string
	azURL         string
	httpClient    *http.Client
	azHMAC        hash.Hash
}

// NewAzureMonitorWriter returns new LogWriter that writes LogMessages to Azure Monitor (Azure Log Analytics Workspace)
// see also: https://docs.microsoft.com/de-de/azure/azure-monitor/platform/data-collector-api
//
// customLogName is the record type in which the logs will be stored (can only contain letters, numbers, and underscore, and may not exceed 100 characters)
//
// The used default URL is: "https://" + workspaceID + "." azureMonitorDomain + "/api/logs?api-version=2016-04-01"
// with the default azureMonitorDomain: "ods.opinsights.azure.com"
//
// The following environemnt variables are used be used to configure the behaviour:
// LOGTHING_LOG_NAME  						- Log name under which log messages are stored (will be used as elasticsearch index or azure custom log type)
// LOGTHING_AZURE_WORKSPACE_ID    - Azure log analytics workspace id
// LOGTHING_AZURE_WORKSPACE_KEY   - Azure log analytics worksoace key
// LOGTHING_AZURE_MONITOR_DOMAIN 	- (optional) to overwrite the default azure monitor domain e.g. in China
func NewAzureMonitorWriter() LogWriter {
	azWorkspaceID := os.Getenv("LOGTHING_AZURE_WORKSPACE_ID")
	azWorkspaceKey := os.Getenv("LOGTHING_AZURE_WORKSPACE_KEY")
	azMonitorDomain := "ods.opinsights.azure.com"
	if amd := os.Getenv("LOGTHING_AZURE_MONITOR_DOMAIN"); amd != "" {
		azMonitorDomain = amd
	}
	writer := &azureMonitor{
		azWorkspaceID: azWorkspaceID,
		azKey:         azWorkspaceKey,
		httpClient:    http.DefaultClient,
		azDomain:      azMonitorDomain,
	}
	return writer
}

// azCreateSignatureString creates azure signature string (not thread safe)
func (am *azureMonitor) azCreateSignatureString(contentLength int) (signature string, msDate string, err error) {
	if am.azHMAC == nil {
		if keyBytes, decodeErr := base64.StdEncoding.DecodeString(am.azKey); decodeErr == nil {
			am.azHMAC = hmac.New(sha256.New, keyBytes)
		} else {
			// disable azure logging
			err = fmt.Errorf("AZURE_MONITOR_KEY invalid: %w", decodeErr)
			return
		}
	}
	dateString := time.Now().UTC().Format(time.RFC1123)
	msDate = strings.Replace(dateString, "UTC", "GMT", -1)
	signatureString := "POST\n" + strconv.Itoa(contentLength) + "\napplication/json\n" + "x-ms-date:" + msDate + "\n/api/logs"
	am.azHMAC.Reset()
	am.azHMAC.Write([]byte(signatureString))
	signature = base64.StdEncoding.EncodeToString(am.azHMAC.Sum(nil))
	return
}

func (am *azureMonitor) Init(config Config) error {
	am.azLogType = config.LogName
	if am.azWorkspaceID == "" {
		return fmt.Errorf("envrionment variable \"LOGTHING_AZURE_WORKSPACE_ID\" must be set")
	}
	if am.azKey == "" {
		return fmt.Errorf("environment variable \"LOGTHING_AZURE_WORKSPACE_KEY\" must be set")
	}
	if am.azLogType == "" {
		return fmt.Errorf("environment varibale \"LOGTHING_LOG_NAME\" must be set")
	}
	if am.azDomain == "" {
		return fmt.Errorf("envrionment variable \"LOGTHING_AZURE_MONITOR_DOMAIN\" mustn't be empty or not set at all")
	}
	am.azURL = "https://" + am.azWorkspaceID + "." + am.azDomain + "/api/logs?api-version=2016-04-01"
	return nil
}

func (am *azureMonitor) Close() {
}

func (am *azureMonitor) PropertiesSchemaChanged(schema map[string]Kind) error {
	return nil
}

func (am *azureMonitor) WriteLogMessages(logMessages []json.RawMessage, timestamps []time.Time) error {
	if len(am.azKey) == 0 || len(am.azWorkspaceID) == 0 {
		return ErrWriterDisable
	}

	postData, _ := json.Marshal(logMessages)
	postDataLength := len(postData)

	signature, msDate, err := am.azCreateSignatureString(postDataLength)
	if err != nil {
		am.azKey = "" // disable azure logging by resetting azKey
		return fmt.Errorf("Creting signature failed: %v: %w", err, ErrWriterDisable)
	}
	authorizationString := "SharedKey " + am.azWorkspaceID + ":" + signature

	req, err := http.NewRequest("POST", am.azURL, bytes.NewReader(postData))
	if err != nil {
		return fmt.Errorf("Creating POST request failed: %v: %w", err, ErrWriterDisable)
	}
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Log-Type", am.azLogType)
	req.Header.Add("Authorization", authorizationString)
	req.Header.Add("x-ms-date", msDate)
	req.Header.Add("time-generated-field", "timestamp")
	req.Header.Add("Content-Type", "application/json")

	resp, err := am.httpClient.Do(req)
	defer func() {
		if resp != nil {
			resp.Body.Close()
		}
	}()
	if err != nil {
		return fmt.Errorf("Sending LogMessage to azure failed: %w", err)
	}
	if resp == nil {
		return fmt.Errorf("Invalid service response")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var body []byte
		resp.Body.Read(body)
		return fmt.Errorf("Sending LogMessage to azure failed (Code: %v): %v", resp.StatusCode, body)
	}
	return nil
}
