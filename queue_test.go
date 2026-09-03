package main

import (
	"fmt"
	"sync"
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
	numCommands := 1000

	// Enqueue commands concurrently
	var wg sync.WaitGroup
	for i := 0; i < numCommands; i++ {
		wg.Add(1)
		go func(sequence uint64) {
			defer wg.Done()
			command := mustCommand(t, fmt.Sprintf("SAT-%d", sequence), sequence, fmt.Sprintf("PAYLOAD-%d", sequence))
			queue.Enqueue(command)
		}(uint64(i))
	}
	wg.Wait()

	// Dequeue commands concurrently
	for i := range numCommands {
		wg.Add(1)
		go func(sequence uint64) {
			defer wg.Done()
			queue.Enqueue(mustCommand(t, "SAT-1", sequence, "CAPTURE"))
		}(uint64(i))
	}
	wg.Wait()
	if queue.Len() != numCommands {
		t.Errorf("expected queue length %d, got %d", numCommands, queue.Len())
	}
}
