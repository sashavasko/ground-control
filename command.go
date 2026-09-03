package main

import (
	"bytes"
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
