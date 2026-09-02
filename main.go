package main

import "fmt"

func main() {
	command, err := NewCommand("SAT-1", 1, []byte("CAPTURE"))
	if err != nil {
		fmt.Printf("Failed to create command %v \n", err)
	}
	fmt.Printf("created command %s %d \n", command.SatelliteID, command.Sequence)
}
