package main

import (
	"context"
	"testing"
	"time"
)

func TestDispatcher(t *testing.T) {
	_, err := NewDispatcher(nil, 10)
	if err == nil {
		t.Fatalf("Expected error when creating dispatcher with nil handler, but got nil")
	}

	_, err = NewDispatcher(&recordingHandler{}, -1)
	if err == nil {
		t.Fatalf("Expected error when creating dispatcher with negative capacity, but got nil")
	}

	_, err = NewDispatcher(&recordingHandler{}, 0)
	if err != nil {
		t.Fatalf("Expected dispatcher creation with 0 capacity to succeed, but got error: %v", err)
	}

	handler := &recordingHandler{
		handled: make(chan Command, 1),
	}
	dispatcher, err := NewDispatcher(handler, 10)
	if err != nil {
		t.Fatalf("Failed to create dispatcher: %v", err)
	}

	command := Command{SatelliteID: "SAT-1", Sequence: 1, Payload: []byte("CAPTURE")}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runResult := make(chan error, 1)
	go func() {
		runResult <- dispatcher.Run(ctx)
	}()

	if err := dispatcher.Submit(ctx, command); err != nil {
		t.Fatalf("Failed to submit command: %v", err)
	}

	select {
	case handledCommand := <-handler.handled:
		if handledCommand.SatelliteID != command.SatelliteID || handledCommand.Sequence != command.Sequence || string(handledCommand.Payload) != string(command.Payload) {
			t.Errorf("Expected handled command to be %v, but got %v", command, handledCommand)
		}
	case err := <-runResult:
		t.Fatalf("Dispatcher run exited unexpectedly: %v", err)
	}

	cancel()
	select {
	case err := <-runResult:
		if err != context.Canceled {
			t.Errorf("Expected dispatcher run to exit with context.Canceled, but got: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("Expected dispatcher run to exit with context.Canceled, but got: %v", err)
	}
}
