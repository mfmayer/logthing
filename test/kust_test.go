package test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/Azure/azure-kusto-go/kusto"
	"github.com/Azure/azure-kusto-go/kusto/ingest"
	"github.com/joho/godotenv"
)

func init() {
	godotenv.Load()
}

func TestKusto(t *testing.T) {
	clusterURL := os.Getenv("DATA_EXPLORER_CLUSTER_URL")
	if clusterURL == "" {
		t.Fatal()
	}
	appID := os.Getenv("DATA_EXPLORER_APP_ID")
	if appID == "" {
		t.Fatal()
	}
	appKey := os.Getenv("DATA_EXPLORER_APP_KEY")
	if appKey == "" {
		t.Fatal()
	}
	authorityID := os.Getenv("DATA_EXPLIRER_AUTHORITY_ID")
	if authorityID == "" {
		t.Fatal()
	}
	kcs := kusto.NewConnectionStringBuilder(clusterURL)
	kcs.WithAadAppKey(appID, appKey, authorityID)

	kustoClient, err := kusto.New(kcs)
	if err != nil {
		t.Fatalf("Couldn't create Kusto client: %s", err)
	}
	defer func(client *kusto.Client) {
		err := client.Close()
		if err != nil {
			t.Fatal(err)
		}
	}(kustoClient)
	in, err := ingest.NewStreaming(kustoClient, "logs", "test")
	if err != nil {
		t.Fatal(err)
	}
	jsonString := `
	{
		"TimeGenerated":"2023-09-01T15:13:58.070526Z",
		"Severity": 1,
		"Type": "blubb",
		"Other": "bla"
	}`
	res, err := in.FromReader(context.Background(), strings.NewReader(jsonString), ingest.FileFormat(ingest.MultiJSON))
	if err != nil {
		t.Fatal(err)
	}
	t.Log(res)

}
