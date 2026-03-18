package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("Raft Consensus Node")
	fmt.Printf("PID: %d\n", os.Getpid())
	// TODO: Initialize and start Raft node
}
