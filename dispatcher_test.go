package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

type recordingHandler struct {
	handled chan<- Command
}

func (h *recordingHandler) Handle(ctx context.Context, command Command) error {
	select {
	case h.handled <- command:
		return nil
	case <-ctx.Done():
		return ctx.Err()

	}
}

var _ CommandHandler = (*recordingHandler)(nil)

func TestDispatcher(t *testing.T) {
	_, err := NewDispatcher(nil, 10)
	if err == nil {
		t.Fatalf("expected error when creating dispatcher with nil handler, but got nil")
	}

	_, err = NewDispatcher(&recordingHandler{}, -1)
	if err == nil {
		t.Fatalf("expected error when creating dispatcher with negative capacity, but got nil")
	}

	_, err = NewDispatcher(&recordingHandler{}, 0)
	if err != nil {
		t.Fatalf("expected dispatcher creation with 0 capacity to succeed, but got error: %v", err)
	}

	_, err = NewDispatcher(&recordingHandler{}, 0, WithCommandErrorPolicy(nil))
	if err == nil {
		t.Fatalf("expected dispatcher creation with nil error policy to fail, but got nil error")
	}

	_, err = NewDispatcher(&recordingHandler{}, 0, nil)
	if err == nil {
		t.Fatalf("expected dispatcher creation with nil option to fail, but got nil error")
	}

	handled := make(chan Command, 1)
	handler := &recordingHandler{
		handled: handled,
	}
	dispatcher, err := NewDispatcher(handler, 10)
	if err != nil {
		t.Fatalf("failed to create dispatcher: %v", err)
	}

	command := mustCommand(t, "SAT-1", 1, "CAPTURE")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runResult := make(chan error, 1)
	go func() {
		runResult <- dispatcher.Run(ctx)
	}()

	if err := dispatcher.Submit(ctx, command); err != nil {
		t.Fatalf("failed to submit command: %v", err)
	}

	select {
	case handledCommand := <-handled:
		if handledCommand.SatelliteID != command.SatelliteID || handledCommand.Sequence != command.Sequence || string(handledCommand.Payload) != string(command.Payload) {
			t.Errorf("expected handled command to be %v, but got %v", command, handledCommand)
		}
	case err := <-runResult:
		t.Fatalf("dispatcher run exited unexpectedly: %v", err)
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for command to be handled")
	}

	cancel()
	select {
	case err := <-runResult:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected dispatcher run to exit with context.Canceled, but got: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("dispatcher did not stop after cancellation")
	}
}

func TestDispatcherSubmitTimeout(t *testing.T) {
	handler := &recordingHandler{
		handled: make(chan Command, 1),
	}

	dispatcher, err := NewDispatcher(handler, 0)
	if err != nil {
		t.Fatalf("NewDispatcher() error = %v", err)
	}

	command := mustCommand(t, "SAT-1", 1, "CAPTURE")

	ctx, cancel := context.WithTimeout(
		context.Background(),
		100*time.Millisecond,
	)
	defer cancel()

	err = dispatcher.Submit(ctx, command)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf(
			"Submit() error = %v, want context.DeadlineExceeded",
			err,
		)
	}
}

func TestDispatcherBackpressure(t *testing.T) {
	handler := &recordingHandler{
		handled: make(chan Command, 1),
	}

	dispatcher, err := NewDispatcher(handler, 1)
	if err != nil {
		t.Fatalf("NewDispatcher() error = %v", err)
	}

	command1 := mustCommand(t, "SAT-1", 1, "CAPTURE")
	command2 := mustCommand(t, "SAT-2", 2, "TRANSMIT")

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := dispatcher.Submit(ctx, command1); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	if err := dispatcher.Submit(ctx, command2); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Submit() error = %v, want context.DeadlineExceeded", err)
	}
}

var errLinkUnavailable = errors.New("satellite link unavailable")

type failingHandler struct {
	err error
}

func (h *failingHandler) Handle(ctx context.Context, command Command) error {
	return h.err
}

func TestDispatcherErrorPropagation(t *testing.T) {
	handler := &failingHandler{err: errLinkUnavailable}
	dispatcher, err := NewDispatcher(handler, 1)
	if err != nil {
		t.Fatalf("NewDispatcher() error = %v", err)
	}

	command := mustCommand(t, "SAT-1", 1, "CAPTURE")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	runResult := make(chan error, 1)
	go func() {
		runResult <- dispatcher.Run(ctx)
	}()

	if err := dispatcher.Submit(ctx, command); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	select {
	case err := <-runResult:
		if !errors.Is(err, errLinkUnavailable) {
			t.Errorf("Run() error = %v, want %v", err, errLinkUnavailable)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for dispatcher to return error")
	}
}

func TestDispatcherErrorPropagationWithPolicy(t *testing.T) {
	handler := &failingHandler{err: errLinkUnavailable}
	dispatcher, err := NewDispatcher(handler, 1, WithCommandErrorPolicy(func(ctx context.Context, c Command, err error) error { return nil }))
	if err != nil {
		t.Fatalf("NewDispatcher() error = %v", err)
	}

	command := mustCommand(t, "SAT-1", 1, "CAPTURE")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	runResult := make(chan error, 1)
	go func() {
		runResult <- dispatcher.Run(ctx)
	}()

	if err := dispatcher.Submit(ctx, command); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	select {
	case err := <-runResult:
		if err != nil {
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("Run() error = %v, want nil", err)
			}
		} else {
			cancel()
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for dispatcher to return error")
	}

}

func TestDispatcherAlreadyRunning(t *testing.T) {
	handler := &recordingHandler{
		handled: make(chan Command, 1),
	}

	dispatcher, err := NewDispatcher(handler, 1)
	if err != nil {
		t.Fatalf("NewDispatcher() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if dispatcher.Ready() {
		t.Errorf("dispatcher is ready before Run() is called")
	}

	runResult := make(chan error, 1)
	go func() {
		runResult <- dispatcher.Run(ctx)
	}()

	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()

	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()

	for !dispatcher.Ready() {
		select {
		case err := <-runResult:
			t.Fatalf("dispatcher exited unexpectedly: %v", err)
		case <-ticker.C:
		case <-deadline.C:
			t.Fatalf("timed out waiting for dispatcher to be ready")
		}
	}

	err = dispatcher.Run(ctx)
	if !errors.Is(err, ErrDispatcherAlreadyRunning) {
		t.Errorf("Run() error = %v, want %v", err, ErrDispatcherAlreadyRunning)
	}

	cancel()
	select {
	case err := <-runResult:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Run() error = %v, want %v", err, context.Canceled)
		}
		if dispatcher.Ready() {
			t.Errorf("dispatcher is still ready, want false")
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for dispatcher to stop")
	}
}
