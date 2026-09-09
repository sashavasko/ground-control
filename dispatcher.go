package main

import (
	"context"
	"fmt"
	"sync/atomic"
)

type CommandHandler interface {
	Handle(context.Context, Command) error
}

type Dispatcher struct {
	commands chan Command
	handler  CommandHandler
	running  atomic.Bool
}

var ErrDispatcherAlreadyRunning = fmt.Errorf("dispatcher is already running")

func NewDispatcher(handler CommandHandler, capacity int) (*Dispatcher, error) {
	if handler == nil {
		return nil, fmt.Errorf("handler cannot be nil")
	}
	if capacity < 0 {
		return nil, fmt.Errorf("capacity must not be negative")
	}
	return &Dispatcher{
		commands: make(chan Command, capacity),
		handler:  handler,
	}, nil
}

func (d *Dispatcher) Submit(ctx context.Context, cmd Command) error {
	select {
	case d.commands <- cmd:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (d *Dispatcher) Run(ctx context.Context) error {

	if !d.running.CompareAndSwap(false, true) {
		return ErrDispatcherAlreadyRunning
	}
	defer d.running.Store(false)

	for {
		select {
		case cmd := <-d.commands:
			if err := d.handler.Handle(ctx, cmd); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (d *Dispatcher) Ready() bool {
	return d.running.Load()
}
