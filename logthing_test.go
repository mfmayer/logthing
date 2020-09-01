package logthing_test

import (
	"testing"

	"github.com/mfmayer/logthing"
	"github.com/mfmayer/logthing/logwriter"
)

func TestLogthing(t *testing.T) {
	if err := logthing.InitDispatcher([]logwriter.LogWriter{
		logwriter.NewAzureMonitorWriter(),
	}); err != nil {
		logthing.Error.Printf("Error init dispatcher: %v", err)
	}

	logthing.NewLogMsg("mytest").
		SetProperty("foo", []int{1, 2, 3, 4, 5}).
		SetProperty("bar", []int{6, 7, 8, 9}).
		SetProperty("foobar", []int{10, 11, 12, 13}).
		Warningf("Hello %v", []string{"Mom", "Dad"}).
		Errorf("Good Morning %v", []string{"Dog", "Cat"}).
		Log()

	msg := logthing.NewLogMsg("mytest").
		SetProperty("foo", []int{1, 2, 3, 4, 5}).
		SetProperty("bar", []int{6, 7, 8, 9}).
		SetProperty("foobar", []int{10, 11, 12, 13}).
		Warningf("Hello %v", []string{"Mom", "Dad"}).
		Errorf("Good Morning %v", []string{"Dog", "Cat"})
	logthing.LogMsg(msg)

	logthing.Close()
}
