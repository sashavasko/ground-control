package main

import "fmt"

func main(){
    command, err := NewCommand()
    if err == nil {
        fmt.Printf("created command %s %d \n", command.SatelliteID, command.Sequence)
    } else {
        fmt.Printf("Failed to create command %w", err)
    }

}