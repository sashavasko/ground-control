package main

import (
	"encoding/json"
	"testing"
)

func TestNewCommand(t *testing.T) {
	tests := []struct {
		name        string
		satelliteID string
		sequence    uint64
		payload     []byte
		expectError bool
	}{
		{
			name:        "valid",
			satelliteID: "SAT-1",
			sequence:    1,
			payload:     []byte("CAPTURE"),
			expectError: false,
		},
		{
			name:        "empty satellite ID",
			satelliteID: "",
			sequence:    1,
			payload:     []byte("CAPTURE"),
			expectError: true,
		},
		{
			name:        "empty sequence",
			satelliteID: "SAT-1",
			sequence:    0,
			payload:     []byte("CAPTURE"),
			expectError: true,
		},
		{
			name:        "nil payload",
			satelliteID: "SAT-1",
			sequence:    1,
			payload:     nil,
			expectError: true,
		},
		{
			name:        "empty payload",
			satelliteID: "SAT-1",
			sequence:    1,
			payload:     []byte{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewCommand(tt.satelliteID, tt.sequence, tt.payload)
			if (err != nil) != tt.expectError {
				t.Errorf("NewCommand() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestNewCommandPayloadOwnership(t *testing.T) {
	originalPayload := []byte("CAPTURE")
	command, err := NewCommand("SAT-1", 1, originalPayload)
	if err != nil {
		t.Fatalf("failed to create command: %v", err)
	}

	// Modify the original payload
	originalPayload[0] = 'X'

	// Check if the command's payload has changed
	if string(command.Payload) != "CAPTURE" {
		t.Errorf("command's payload  = %q, want %q", command.Payload, "CAPTURE")
	}
}

func TestUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name        string
		jsonData    string
		expectError bool
	}{
		{
			name:        "valid JSON",
			jsonData:    `{"satelliteId":"SAT-1","sequence":1,"payload":"CAPTURE"}`,
			expectError: false,
		},
		{
			name:        "missing satelliteId",
			jsonData:    `{"sequence":1,"payload":"CAPTURE"}`,
			expectError: true,
		},
		{
			name:        "missing sequence",
			jsonData:    `{"satelliteId":"SAT-1","payload":"CAPTURE"}`,
			expectError: true,
		},
		{
			name:        "missing payload",
			jsonData:    `{"satelliteId":"SAT-1","sequence":1}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var command Command
			err := json.Unmarshal([]byte(tt.jsonData), &command)
			if (err != nil) != tt.expectError {
				t.Errorf("json.Unmarshal() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}
