package main

type CommandQueue struct {
	commands []Command
}

func (q *CommandQueue) Enqueue(command Command) {
	q.commands = append(q.commands, command)
}

func (q *CommandQueue) Dequeue() (Command, bool) {
	if len(q.commands) == 0 {
		return Command{}, false
	}
	command := q.commands[0]
	q.commands[0] = Command{} // Clear the reference to the dequeued command
	q.commands = q.commands[1:]
	return command, true
}

func (q *CommandQueue) Len() int {
	return len(q.commands)
}

func (q *CommandQueue) IsEmpty() bool {
	return len(q.commands) == 0
}
