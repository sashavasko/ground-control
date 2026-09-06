package main

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type Command struct {
	SatelliteID string
	Sequence    uint64
	Payload     []byte
}

func NewCommand(satelliteID string, sequence uint64, payload []byte) (Command, error) {
	if satelliteID == "" {
		return Command{}, fmt.Errorf("satellite ID is empty")
	}
	if sequence == 0 {
		return Command{}, fmt.Errorf("sequence must be greater than zero")
	}
	if len(payload) == 0 {
		return Command{}, fmt.Errorf("payload is empty")
	}
	ownedPayload := bytes.Clone(payload)
	return Command{
		SatelliteID: satelliteID,
		Sequence:    sequence,
		Payload:     ownedPayload,
	}, nil
}

func (c *Command) UnmarshalJSON(data []byte) error {
	var representation struct {
		SatelliteID string `json:"satelliteId"`
		Sequence    uint64 `json:"sequence"`
		Payload     string `json:"payload"`
	}

	if err := json.Unmarshal(data, &representation); err != nil {
		return fmt.Errorf("decode command: %w", err)
	}

	command, err := NewCommand(
		representation.SatelliteID,
		representation.Sequence,
		[]byte(representation.Payload),
	)
	if err != nil {
		return err
	}

	*c = command
	return nil
}
