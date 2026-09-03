package main

import "fmt"

func main() {
	command, err := NewCommand("SAT-1", 1, []byte("CAPTURE"))
	if err != nil {
		fmt.Printf("failed to create command: %v\n", err)
		return
	}
	fmt.Printf("created command %s %d \n", command.SatelliteID, command.Sequence)
}
