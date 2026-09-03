package main

import "sync"

type CommandQueue struct {
	mu       sync.Mutex
	commands []Command
}

func (q *CommandQueue) Enqueue(command Command) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.commands = append(q.commands, command)
}

func (q *CommandQueue) Dequeue() (Command, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.commands) == 0 {
		return Command{}, false
	}
	command := q.commands[0]
	q.commands[0] = Command{} // Clear the reference to the dequeued command
	q.commands = q.commands[1:]
	return command, true
}

func (q *CommandQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.commands)
}

func (q *CommandQueue) IsEmpty() bool {
	return len(q.commands) == 0
}
