package main

import "fmt"

type Command struct {
	SatelliteID string
	Sequence    uint64
	Payload     []byte
}

func NewCommand(satelliteID string, sequence uint64, payload []byte) (Command, error) {
	if satelliteID == "" {
		return Command{}, fmt.Errorf("Satellite ID is empty")
	}
	if sequence == 0 {
		return Command{}, fmt.Errorf("sequence is empty")
	}
	if len(payload) == 0 {
		return Command{}, fmt.Errorf("payload is empty")
	}
	return Command{
		SatelliteID: satelliteID,
		Sequence:    sequence,
		Payload:     payload,
	}, nil
}
