package logwriter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/Azure/azure-kusto-go/kusto/ingest"
	"github.com/Azure/azure-kusto-go/kusto/kql"
)

var adeKindNames = [...]string{
	Unknown:  "dynamic",
	String:   "string",
	Number:   "dynamic",
	Integer:  "dynamic",
	Boolean:  "bool",
	Object:   "dynamic",
	Array:    "dynamic",
	DateTime: "datetime",
}

func createTable(kc *kusto.Client, kustoDB string, table string) error {
	query := kql.New(".create table ").AddTable(table).AddUnsafe(" (timestamp:datetime)") //AddUnsafe(" (TimeGenerated:datetime, ColumnB:int)") //.AddString("(ColumnA:string, ColumnB:int)")
	_, err := kc.Mgmt(context.Background(), kustoDB, query)
	if err != nil {
		return err
	}
	query = kql.New(".alter table ").AddTable(table).AddUnsafe(" policy streamingingestion enable") //AddUnsafe(" (TimeGenerated:datetime, ColumnB:int)") //.AddString("(ColumnA:string, ColumnB:int)")
	_, err = kc.Mgmt(context.Background(), kustoDB, query)
	return err
}

func alterMergeTable(kc *kusto.Client, kustoDB string, table string, schema map[string]Kind) error {
	var b strings.Builder
	first := true
	for column, columnKind := range schema {
		columnType := adeKindNames[columnKind]
		if first {
			fmt.Fprintf(&b, " (%s:%s", column, columnType)
		} else {
			fmt.Fprintf(&b, ",%s:%s", column, columnType)
		}
		first = false
	}
	fmt.Fprint(&b, ")")
	err := createTable(kc, kustoDB, table)
	if err != nil {
		return err
	}
	query := kql.New(".alter-merge table ").AddTable(table).AddUnsafe(b.String()) //AddUnsafe(" (TimeGenerated:datetime, ColumnB:int)") //.AddString("(ColumnA:string, ColumnB:int)")
	_, err = kc.Mgmt(context.Background(), kustoDB, query)
	return err
}

// AzureMonitor log writer
type azureDataExplorer struct {
	client  *kusto.Client
	logName string
}

func getKustoClient() (client *kusto.Client, err error) {
	clusterURL := os.Getenv("LOGTHING_DATA_EXPLORER_CLUSTER_URL")
	if clusterURL == "" {
		err = fmt.Errorf("missing LOGTHING_DATA_EXPLORER_CLUSTER_URL")
		return
	}
	appID := os.Getenv("LOGTHING_DATA_EXPLORER_APP_ID")
	if appID == "" {
		err = fmt.Errorf("missing LOGTHING_DATA_EXPLORER_APP_ID")
		return
	}
	appKey := os.Getenv("LOGTHING_DATA_EXPLORER_APP_KEY")
	if appKey == "" {
		err = fmt.Errorf("missing LOGTHING_DATA_EXPLORER_APP_KEY")
		return
	}
	authorityID := os.Getenv("LOGTHING_DATA_EXPLORER_AUTHORITY_ID")
	if authorityID == "" {
		err = fmt.Errorf("missing LOGTHING_DATA_EXPLORER_AUTHORITY_ID")
		return
	}
	kcs := kusto.NewConnectionStringBuilder(clusterURL)
	kcs.WithAadAppKey(appID, appKey, authorityID)

	client, err = kusto.New(kcs)
	if err != nil {
		err = fmt.Errorf("cannot create Kusto client: %w", err)
	}
	return
}

func NewAzureDataExplorerWriter() LogWriter {
	return &azureDataExplorer{}
}

func (de *azureDataExplorer) Init(config Config) (err error) {
	de.logName = config.LogName
	de.client, err = getKustoClient()
	if err != nil {
		return
	}
	return
}

func (de *azureDataExplorer) PropertiesSchemaChanged(schema map[string]Kind) error {
	if de.client == nil {
		return fmt.Errorf("invalid client")
	}
	alterMergeTable(de.client, "logs", de.logName, schema)
	return nil
}

func (de *azureDataExplorer) WriteLogMessages(logMessages []json.RawMessage, timestamps []time.Time) (err error) {
	if de.client == nil {
		return fmt.Errorf("invalid client")
	}
	in, err := ingest.NewStreaming(de.client, "logs", de.logName)
	if err != nil {
		return err
	}
	readers := make([]io.Reader, len(logMessages))
	for i, msg := range logMessages {
		readers[i] = bytes.NewReader(msg)
	}
	reader := io.MultiReader(readers...)

	res, err := in.FromReader(context.Background(), reader, ingest.FileFormat(ingest.MultiJSON))
	if err != nil {
		return err
	}
	test := res.Wait(context.Background())
	resErr := <-test
	if resErr != nil {
		return resErr
	}

	return
}

func (de *azureDataExplorer) Close() {
	if de.client == nil {
		return
	}
	de.client.Close()
}
