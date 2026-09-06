package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type SatelliteRegistry struct {
	mu         sync.RWMutex
	satellites map[string]*Satellite
}

var ErrUnknownSatellite = errors.New("unknown satellite")
var ErrSatelliteAlreadyRegistered = errors.New(
	"satellite already registered",
)

var _ CommandHandler = (*SatelliteRegistry)(nil)

func NewSatelliteRegistry() *SatelliteRegistry {
	return &SatelliteRegistry{
		satellites: make(map[string]*Satellite),
	}
}

func (r *SatelliteRegistry) Register(s *Satellite) error {
	if s == nil {
		return fmt.Errorf("satellite cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.satellites[s.ID()]; ok {
		return fmt.Errorf("%w: satellite with ID %s already registered", ErrSatelliteAlreadyRegistered, s.ID())
	}

	r.satellites[s.id] = s
	return nil
}

func (r *SatelliteRegistry) Lookup(id string) (*Satellite, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.satellites[id]
	if !ok {
		return nil, false
	}
	return s, true
}

func (r *SatelliteRegistry) Handle(ctx context.Context, command Command) error {
	s, ok := r.Lookup(command.SatelliteID)
	if !ok {
		return fmt.Errorf("%w: command satellite ID %s", ErrUnknownSatellite, command.SatelliteID)
	}

	return s.Apply(ctx, command)
}
