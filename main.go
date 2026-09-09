package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type componentResult struct {
	name string
	err  error
}

func main() {
	if err := run(); err != nil {
		fmt.Printf("round control stopped with error: %v", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	registry := NewSatelliteRegistry()
	satellite, err := NewSatellite("SAT-1")

	if err != nil {
		return fmt.Errorf("error creating satellite: %w", err)
	}

	if err := registry.Register(satellite); err != nil {
		return fmt.Errorf("error registering satellite: %w", err)
	}

	fmt.Println("Satellite registered successfully:", satellite.ID())

	dispatcher, err := NewDispatcher(registry, 10)
	if err != nil {
		return fmt.Errorf("error creating dispatcher: %w", err)
	}

	api, err := NewAPIServer(dispatcher)
	if err != nil {
		return fmt.Errorf("error creating API server: %w", err)
	}

	fmt.Println("API server created successfully:", api)

	server := &http.Server{
		Addr:              ":8080",
		Handler:           api.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	results := make(chan componentResult, 2)

	go func() {
		results <- componentResult{name: "dispatcher", err: dispatcher.Run(ctx)}
	}()

	go func() {
		err := server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}

		results <- componentResult{name: "http server", err: err}
	}()

	completed := 0

	select {
	case <-ctx.Done():
		fmt.Println("Shutdown signal received, shutting down...")
	case result := <-results:
		completed++
		if result.err != nil && !errors.Is(result.err, context.Canceled) {
			fmt.Printf("Component %s exited with error: %v\n", result.name, result.err)
		}
		stop()
	}

	var componentErrors []error

	for completed < 2 {
		result := <-results
		completed++
		if result.err != nil && !errors.Is(result.err, context.Canceled) {
			fmt.Printf("Component %s exited with error: %v\n", result.name, result.err)
			componentErrors = append(componentErrors, fmt.Errorf("%s: %w", result.name, result.err))
		}
	}
	finalErr := errors.Join(componentErrors...)
	return finalErr
}
