package logthing

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/mfmayer/logthing/logwriter"
)

type lifecycleWriter struct {
	messages   []json.RawMessage
	closeCount int
}

func (w *lifecycleWriter) Init(logwriter.Config) error {
	return nil
}

func (w *lifecycleWriter) WriteLogMessages(messages []json.RawMessage, _ []time.Time) error {
	w.messages = append(w.messages, messages...)
	return nil
}

func (w *lifecycleWriter) PropertiesSchemaChanged(map[string]logwriter.Kind) error {
	return nil
}

func (w *lifecycleWriter) Close() {
	w.closeCount++
}

func assertWriterClosed(t *testing.T, writer *lifecycleWriter) {
	t.Helper()
	if len(writer.messages) != 1 {
		t.Errorf("flushed messages = %d, want 1", len(writer.messages))
	}
	if writer.closeCount != 1 {
		t.Errorf("writer close count = %d, want 1", writer.closeCount)
	}
}

func TestDispatcherCloseIdempotent(t *testing.T) {
	writer := &lifecycleWriter{}
	dispatcher, err := newLogDispatcher([]logwriter.LogWriter{writer}, WithDispatchInterval(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	defer dispatcher.close()

	if err := dispatcher.log(1, NewLogMsg("lifecycle", WithWhitelistFlag())); err != nil {
		t.Fatal(err)
	}

	// All callers must wait for the flush and writer shutdown to finish.
	var callers sync.WaitGroup
	for i := 0; i < 8; i++ {
		callers.Add(1)
		go func() {
			defer callers.Done()
			dispatcher.close()
			assertWriterClosed(t, writer)
		}()
	}
	callers.Wait()
	dispatcher.close()
	assertWriterClosed(t, writer)
}

func TestDispatcherReinitialization(t *testing.T) {
	for _, tc := range []struct {
		name            string
		closeBeforeInit bool
	}{
		{name: "after close", closeBeforeInit: true},
		{name: "while active"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			Close()
			Close()
			t.Cleanup(Close)

			first := &lifecycleWriter{}
			if err := InitDispatcher([]logwriter.LogWriter{first}, WithDispatchInterval(time.Hour)); err != nil {
				t.Fatal(err)
			}
			if err := Log(NewLogMsg("first", WithWhitelistFlag())); err != nil {
				t.Fatal(err)
			}

			if tc.closeBeforeInit {
				Close()
				Close()
				if err := Log(NewLogMsg("after close", WithWhitelistFlag())); !errors.Is(err, ErrNotInitialized) {
					t.Fatalf("Log after Close = %v, want ErrNotInitialized", err)
				}
			}

			second := &lifecycleWriter{}
			if err := InitDispatcher([]logwriter.LogWriter{second}, WithDispatchInterval(time.Hour)); err != nil {
				t.Fatal(err)
			}
			assertWriterClosed(t, first)

			if err := Log(NewLogMsg("second", WithWhitelistFlag())); err != nil {
				t.Fatal(err)
			}
			Close()
			Close()
			assertWriterClosed(t, second)
			if err := Log(NewLogMsg("after close", WithWhitelistFlag())); !errors.Is(err, ErrNotInitialized) {
				t.Fatalf("Log after Close = %v, want ErrNotInitialized", err)
			}
		})
	}
}
