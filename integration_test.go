package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSubmitToSatellite(t *testing.T) {
	registry := NewRegistry()

	satellite, err := NewSatellite("SAT-1")
	if err != nil {
		t.Fatalf("failed to create satellite: %v", err)
	}

	if err := registry.Register(satellite); err != nil {
		t.Fatalf("failed to register satellite: %v", err)
	}

	dispatcher, err := NewDispatcher(registry, 1)
	if err != nil {
		t.Fatalf("failed to create dispatcher: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	runResult := make(chan error, 1)
	go func() {
		runResult <- dispatcher.Run(ctx)
	}()

	command := mustCommand(t, "SAT-1", 1, "CAPTURE")
	if err := dispatcher.Submit(ctx, command); err != nil {
		t.Fatalf("failed to submit command: %v", err)
	}

	if err := WaitForSequence(ctx, satellite, 1); err != nil {
		t.Fatalf("failed to wait for satellite sequence: %v", err)
	}

	if err := dispatcher.Submit(ctx, mustCommand(t, "SAT-1", 2, "TRANSMIT")); err != nil {
		t.Fatalf("failed to submit command: %v", err)
	}

	if err := WaitForSequence(ctx, satellite, 2); err != nil {
		t.Fatalf("failed to wait for satellite sequence: %v", err)
	}

	cancel()
	if err := <-runResult; !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled error from dispatcher.Run(), but got %v", err)
	}
}

func WaitForSequence(ctx context.Context, s *Satellite, expectedSequence int) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if s.LastSequence() == uint64(expectedSequence) {
			return nil
		}
		select {
		case <-ticker.C:
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
