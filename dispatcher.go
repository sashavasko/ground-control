package main

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
)

type CommandHandler interface {
	Handle(context.Context, Command) error
}

type CommandErrorPolicy func(context.Context, Command, error) error

type Dispatcher struct {
	commands    chan Command
	handler     CommandHandler
	errorPolicy CommandErrorPolicy
	running     atomic.Bool
}

type DispatcherOption func(*Dispatcher) error

var ErrDispatcherAlreadyRunning = errors.New("dispatcher is already running")

func NewDispatcher(handler CommandHandler, capacity int, options ...DispatcherOption) (*Dispatcher, error) {
	if handler == nil {
		return nil, fmt.Errorf("handler cannot be nil")
	}
	if capacity < 0 {
		return nil, fmt.Errorf("capacity must not be negative")
	}
	dispatcher := &Dispatcher{
		commands: make(chan Command, capacity),
		handler:  handler,
		errorPolicy: func(ctx context.Context, cmd Command, err error) error {
			return err
		},
	}

	for _, option := range options {
		if option == nil {
			return nil, fmt.Errorf("dispatcher option cannot be nil")
		}
		if err := option(dispatcher); err != nil {
			return nil, err
		}
	}
	return dispatcher, nil
}

func WithCommandErrorPolicy(policy CommandErrorPolicy) DispatcherOption {
	return func(d *Dispatcher) error {
		if policy == nil {
			return errors.New("command error policy cannot be nil")
		}
		d.errorPolicy = policy
		return nil
	}
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
			err := d.handler.Handle(ctx, cmd)
			if err == nil {
				continue
			}
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			if err := d.errorPolicy(ctx, cmd, err); err != nil {
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
