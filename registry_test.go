package main

import (
	"context"
	"errors"
	"testing"
)

func TestRegisterAndLookup(t *testing.T) {
	registry := NewSatelliteRegistry()

	satellite1, err := NewSatellite("SAT-1")
	if err != nil {
		t.Fatalf("failed to create satellite: %v", err)
	}

	satellite2, err := NewSatellite("SAT-2")
	if err != nil {
		t.Fatalf("failed to create satellite: %v", err)
	}

	if err := registry.Register(nil); err == nil {
		t.Fatalf("registering nil satellite is not allowed, but got no error")
	}

	if err := registry.Register(satellite1); err != nil {
		t.Fatalf("failed to register satellite 1: %v", err)
	}

	if err := registry.Register(satellite2); err != nil {
		t.Fatalf("failed to register satellite 2: %v", err)
	}

	satellite, ok := registry.Lookup("SAT-1")
	if !ok {
		t.Fatalf("failed to lookup satellite 1: %v", err)
	}
	if satellite.ID() != "SAT-1" {
		t.Errorf("expected satellite ID SAT-1, got %s", satellite.ID())
	}

	lookupSatellite2, ok := registry.Lookup("SAT-2")
	if !ok {
		t.Fatalf("failed to lookup satellite 2: %v", err)
	}
	if lookupSatellite2.ID() != "SAT-2" {
		t.Errorf("expected satellite ID SAT-2, got %s", lookupSatellite2.ID())
	}

	_, ok = registry.Lookup("SAT-3")
	if ok {
		t.Errorf("expected not to find a satellite for SAT-3")
	}
}

func TestRegisterDuplicateSatellite(t *testing.T) {
	registry := NewSatelliteRegistry()

	satellite, err := NewSatellite("SAT-1")
	if err != nil {
		t.Fatalf("failed to create satellite: %v", err)
	}

	if err := registry.Register(satellite); err != nil {
		t.Fatalf("failed to register satellite: %v", err)
	}

	err = registry.Register(satellite)
	if !errors.Is(err, ErrSatelliteAlreadyRegistered) {
		t.Fatalf("expected error when registering duplicate satellite, but got %v", err)
	}
}

func TestHandleUnknownSatellite(t *testing.T) {
	registry := NewSatelliteRegistry()

	command := Command{
		SatelliteID: "SAT-UNKNOWN",
		Sequence:    1,
		Payload:     []byte("CAPTURE"),
	}

	err := registry.Handle(context.Background(), command)
	if !errors.Is(err, ErrUnknownSatellite) {
		t.Fatalf("expected error when handling command for unknown satellite, but got %v", err)
	}
}

func TestCancelledContext(t *testing.T) {
	registry := NewSatelliteRegistry()

	satellite, err := NewSatellite("SAT-1")
	if err != nil {
		t.Fatalf("failed to create satellite: %v", err)
	}

	if err := registry.Register(satellite); err != nil {
		t.Fatalf("failed to register satellite: %v", err)
	}

	command := Command{
		SatelliteID: "SAT-1",
		Sequence:    1,
		Payload:     []byte("CAPTURE"),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel the context immediately

	err = registry.Handle(ctx, command)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled error when handling command with cancelled context, but got %v", err)
	}

	if got := satellite.LastSequence(); got != 0 {
		t.Errorf("expected last sequence to be 0 after cancellation, but got %d", got)
	}
}
