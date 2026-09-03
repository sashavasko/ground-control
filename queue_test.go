package main

import "testing"

func TestCommandQueue(t *testing.T) {
	queue := CommandQueue{}
	if !queue.IsEmpty() {
		t.Errorf("expected empty queue")
	}

	_, ok := queue.Dequeue()
	if ok {
		t.Errorf("expected dequeue to return false on empty queue")
	}

	command1, _ := NewCommand("SAT-1", 1, []byte("CAPTURE"))
	command2, _ := NewCommand("SAT-2", 2, []byte("TRANSMIT"))

	queue.Enqueue(command1)

	len := queue.Len()
	if len != 1 {
		t.Errorf("expected queue length 1, got %d", len)
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

	command3, _ := NewCommand("SAT-3", 3, []byte("RECEIVE"))
	queue.Enqueue(command3)
	if queue.Len() != 1 {
		t.Errorf("expected queue length 1, got %d", queue.Len())
	}
}
