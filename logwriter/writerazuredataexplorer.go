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

	"github.com/Azure/azure-kusto-go/azkustodata"
	"github.com/Azure/azure-kusto-go/azkustodata/kql"
	"github.com/Azure/azure-kusto-go/azkustoingest"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
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

func createTable(kc *azkustodata.Client, kustoDB string, table string) error {
	query := kql.New(".create table ").AddTable(table).AddUnsafe(" (timestamp:datetime)") //AddUnsafe(" (TimeGenerated:datetime, ColumnB:int)") //.AddString("(ColumnA:string, ColumnB:int)")
	_, err := kc.Mgmt(context.Background(), kustoDB, query)
	if err != nil {
		return err
	}
	query = kql.New(".alter table ").AddTable(table).AddUnsafe(" policy streamingingestion enable") //AddUnsafe(" (TimeGenerated:datetime, ColumnB:int)") //.AddString("(ColumnA:string, ColumnB:int)")
	_, err = kc.Mgmt(context.Background(), kustoDB, query)
	return err
}

func alterMergeTable(kc *azkustodata.Client, kustoDB string, table string, schema map[string]Kind) error {
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
	client       *azkustodata.Client
	ingestClient *azkustoingest.Streaming
	logName      string
}

func getKustoClient() (client *azkustodata.Client, kcs *azkustodata.ConnectionStringBuilder, err error) {
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
	kcs = azkustodata.NewConnectionStringBuilder(clusterURL)
	kcs.AttachPolicyClientOptions(&policy.ClientOptions{
		Retry: policy.RetryOptions{
			MaxRetries: 8,
		},
	})
	kcs.WithAadAppKey(appID, appKey, authorityID)

	client, err = azkustodata.New(kcs)
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
	var kcs *azkustodata.ConnectionStringBuilder
	de.client, kcs, err = getKustoClient()
	if err != nil {
		return
	}
	de.ingestClient, err = azkustoingest.NewStreaming(kcs)
	if err != nil {
		return fmt.Errorf("cannot create Kusto streaming ingest client: %w", err)
	}
	return
}

func (de *azureDataExplorer) PropertiesSchemaChanged(schema map[string]Kind) error {
	if de.client == nil {
		return fmt.Errorf("invalid client")
	}
	return alterMergeTable(de.client, "logs", de.logName, schema)
}

func (de *azureDataExplorer) WriteLogMessages(logMessages []json.RawMessage, timestamps []time.Time) (err error) {
	if de.client == nil || de.ingestClient == nil {
		return fmt.Errorf("invalid client")
	}
	readers := make([]io.Reader, len(logMessages))
	for i, msg := range logMessages {
		readers[i] = bytes.NewReader(msg)
	}
	reader := io.MultiReader(readers...)

	res, err := de.ingestClient.FromReader(
		context.Background(),
		reader,
		azkustoingest.Database("logs"),
		azkustoingest.Table(de.logName),
		azkustoingest.FileFormat(azkustoingest.MultiJSON),
	)
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
