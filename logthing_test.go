package logthing_test

import (
	"fmt"
	"testing"

	"github.com/mfmayer/logthing"
	"github.com/mfmayer/logthing/logwriter"
)

func TestLogthing(t *testing.T) {
	// Init the dispatcher with an Azure Monitoro writer
	if err := logthing.InitDispatcher([]logwriter.LogWriter{
		logwriter.NewAzureMonitorWriter(),
	}); err != nil {
		logthing.Error.Printf("Error init dispatcher: %v", err)
	}

	logMsg := logthing.NewLogMsg("<some_type>")
	logMsg.SetTrackingID("<some_tracking_id>")          // Add a tracking id
	logMsg.SetProperty("foo_bar", 12345)                // set a property
	logMsg.SetProperty("foo", 12345)                    // set a property
	logMsg.SetProperty("bar", []int{5, 6, 7, 8})        // set another property
	logMsg.Infof("Hello %v", []string{"World", "Moon"}) // set an info message
	logthing.Log(logMsg)                                // log the message

	// The calls can be also lined up as in this additional example:
	lm := logthing.NewLogMsg("<another_type>").
		Infof("Hello %v", []string{"Mom", "Dad"}).                                      // add an info message
		SetSProperty("windS", map[string]interface{}{"speed": 10, "directions": 25.5}). // add stringified wind property
		SetProperty("wind", map[string]interface{}{"speed": 10, "directions": 25.5}).   // add non-stringified wind property
		SetProperty("rain", 10).                                                        // add rain property
		Warningf("warning: The weather is %v", []string{"rainy", "stormy"}).            // add a warning message
		Errorf("error: This is an error").
		Info("an info").
		Emergencyf("an emergency").
		Errorf("another error").
		Warningf("another warning")

	lm.Log()

	fmt.Println(lm.OutputWithMaxSeverity(logthing.SeverityError))

	logthing.Close()
}
