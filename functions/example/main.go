package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Event is the input to the function.
type Event struct {
	Data string `json:"data"`
}

// Response is the function output.
type Response struct {
	Result string `json:"result"`
}

func main() {
	// Read event from stdin
	var event Event
	if err := json.NewDecoder(os.Stdin).Decode(&event); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Process event
	response := Response{Result: "Hey, " + event.Data}

	// Write response to stdout
	if err := json.NewEncoder(os.Stdout).Encode(response); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
