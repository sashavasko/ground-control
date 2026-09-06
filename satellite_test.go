package main

import (
	"context"
	"errors"
	"testing"
)

func TestNewSatellite(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		expectError bool
	}{
		{
			name:        "valid",
			id:          "SAT-1",
			expectError: false,
		},
		{
			name:        "empty ID",
			id:          "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			satellite, err := NewSatellite(tt.id)
			if (err != nil) != tt.expectError {
				t.Errorf("NewSatellite() error = %v, expectError %v", err, tt.expectError)
				return
			}
			if !tt.expectError && satellite.ID() != tt.id {
				t.Errorf("NewSatellite() ID = %v, want %v", satellite.ID(), tt.id)
			}
		})
	}
}

func TestSatelliteApply(t *testing.T) {
	satellite, err := NewSatellite("SAT-1")
	if err != nil {
		t.Fatalf("failed to create satellite: %v", err)
	}

	tests := []struct {
		name        string
		command     Command
		expectError error
	}{
		{
			name: "valid command",
			command: Command{
				SatelliteID: "SAT-1",
				Sequence:    1,
				Payload:     []byte("CAPTURE"),
			},
			expectError: nil,
		},
		{
			name: "wrong satellite ID",
			command: Command{
				SatelliteID: "SAT-2",
				Sequence:    2,
				Payload:     []byte("TRANSMIT"),
			},
			expectError: ErrWrongSatellite,
		},
		{
			name: "sequence rejected",
			command: Command{
				SatelliteID: "SAT-1",
				Sequence:    1,
				Payload:     []byte("CAPTURE"),
			},
			expectError: ErrSequenceRejected,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := satellite.Apply(context.Background(), tt.command)
			if !errors.Is(err, tt.expectError) {
				t.Errorf("Satellite.Apply() error = %v, expectError %v", err, tt.expectError)
			}

			if tt.expectError == nil && satellite.LastSequence() != tt.command.Sequence {
				t.Errorf("Satellite.Apply() lastSequence = %v, want %v", satellite.LastSequence(), tt.command.Sequence)
			}
		})
	}
}
