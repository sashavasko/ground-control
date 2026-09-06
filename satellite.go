package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type Satellite struct {
	mu           sync.Mutex
	id           string
	lastSequence uint64
}

var ErrWrongSatellite = errors.New("command addressed to another satellite")
var ErrSequenceRejected = errors.New("command sequence rejected")

func NewSatellite(id string) (*Satellite, error) {
	if id == "" {
		return nil, fmt.Errorf("satellite ID cannot be empty")
	}
	return &Satellite{
		id: id,
	}, nil
}

func (s *Satellite) ID() string {
	return s.id
}

func (s *Satellite) Apply(
	ctx context.Context,
	command Command,
) error {
	if command.SatelliteID != s.id {
		return fmt.Errorf(
			"%w: command satellite ID %s, actual satellite ID %s",
			ErrWrongSatellite,
			command.SatelliteID,
			s.id,
		)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}

	if command.Sequence <= s.lastSequence {
		return fmt.Errorf(
			"%w: command sequence %d, last sequence %d",
			ErrSequenceRejected,
			command.Sequence,
			s.lastSequence,
		)
	}

	s.lastSequence = command.Sequence
	return nil
}

func (s *Satellite) LastSequence() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastSequence
}
