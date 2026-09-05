package main

import (
	"context"
	"fmt"
)

type CommandHandler interface {
	Handle(context.Context, Command) error
}

type Dispatcher struct {
	commands chan Command
	handler  CommandHandler
}

func NewDispatcher(handler CommandHandler, capacity int) (*Dispatcher, error) {
	if handler == nil {
		return nil, fmt.Errorf("handler cannot be nil")
	}
	if capacity < 0 {
		return nil, fmt.Errorf("capacity must must not be negative")
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
