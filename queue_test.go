package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

func mustCommand(t *testing.T, satelliteID string, sequence uint64, payload string) Command {
	t.Helper()
	command, err := NewCommand(satelliteID, sequence, []byte(payload))
	if err != nil {
		t.Fatalf("failed to create command: %v", err)
	}
	return command
}

func TestCommandQueue(t *testing.T) {
	queue := CommandQueue{}
	if !queue.IsEmpty() {
		t.Errorf("expected empty queue")
	}

	_, ok := queue.Dequeue()
	if ok {
		t.Errorf("expected dequeue to return false on empty queue")
	}

	command1 := mustCommand(t, "SAT-1", 1, "CAPTURE")
	command2 := mustCommand(t, "SAT-2", 2, "TRANSMIT")

	queue.Enqueue(command1)

	queueLen := queue.Len()
	if queueLen != 1 {
		t.Errorf("expected queue length 1, got %d", queueLen)
	}

	queue.Enqueue(command2)

	if queue.Len() != 2 {
		t.Errorf("expected queue length 2, got %d", queue.Len())
	}

	dequeuedCommand, ok := queue.Dequeue()
	if !ok || dequeuedCommand.SatelliteID != command1.SatelliteID || dequeuedCommand.Sequence != command1.Sequence {
		t.Errorf("expected to dequeue command1, got %v", dequeuedCommand)
	}

	if queue.Len() != 1 {
		t.Errorf("expected queue length 1, got %d", queue.Len())
	}

	dequeuedCommand, ok = queue.Dequeue()
	if !ok || dequeuedCommand.SatelliteID != command2.SatelliteID || dequeuedCommand.Sequence != command2.Sequence {
		t.Errorf("expected to dequeue command2, got %v", dequeuedCommand)
	}
	if !queue.IsEmpty() {
		t.Errorf("expected empty queue")
	}

	command3 := mustCommand(t, "SAT-3", 3, "RECEIVE")
	queue.Enqueue(command3)
	if queue.Len() != 1 {
		t.Errorf("expected queue length 1, got %d", queue.Len())
	}
}

func TestCommandQueueConcurrency(t *testing.T) {
	queue := CommandQueue{}
	numCommands := 100

	commands := make([]Command, numCommands)
	for i := range numCommands {
		commands[i] = mustCommand(t, fmt.Sprintf("SAT-%d", i), uint64(i+1), fmt.Sprintf("PAYLOAD-%d", i))
	}

	// Enqueue commands concurrently
	var wg sync.WaitGroup
	for i := range numCommands {
		wg.Add(1)
		go func(sequence uint64) {
			defer wg.Done()
			command := commands[sequence]
			queue.Enqueue(command)
		}(uint64(i))
	}
	wg.Wait()

	if queue.Len() != numCommands {
		t.Errorf("expected queue length %d, got %d", numCommands, queue.Len())
	}

	var dequeued atomic.Int64

	// Dequeue commands concurrently
	for range numCommands {
		wg.Add(1)
		go func() {
			defer wg.Done()
			command, ok := queue.Dequeue()
			if ok {
				if command.Sequence == 0 {
					t.Errorf("dequeued command has invalid sequence number")
				}
				dequeued.Add(1)
			} else {
				t.Errorf("expected to dequeue command")
			}
		}()
	}
	wg.Wait()

	if dequeued.Load() != int64(numCommands) {
		t.Errorf("expected to dequeue %d commands, got %d", numCommands, dequeued.Load())
	}

	if queue.Len() != 0 {
		t.Errorf("expected queue length 0, got %d", queue.Len())
	}
}
