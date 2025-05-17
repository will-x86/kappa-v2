package main

import (
	"kappa-v2/pkg/handler"
	"os"
	"runtime"
)

func main() {
	// Start the Kappa function with our handler
	handler.Start(handleRequest)
}

// handleRequest is where your actual function logic goes
func handleRequest(event handler.Event) handler.Response {
	greeting := "Hello from your Kappa function!"
	cores := runtime.NumCPU()
	e := os.Environ()
	m := runtime.MemStats{}
	runtime.ReadMemStats(&m)
	totalRAMMB := m.Sys / (1024 * 1024) // Convert bytes to MB
	// Extract name from the body if available
	if nameVal, ok := event.Body["name"]; ok {
		name, _ := nameVal.(string)
		greeting = "Hello, " + name + "! Welcome to your Kappa function!"
	}
	// Create response body
	responseBody := map[string]any{
		"message":   greeting,
		"input":     event.Body,
		"event":     event,
		"env":       e,
		"mem_total": totalRAMMB,
		"cores":     cores,
	}

	// Return a formatted response
	return handler.NewResponse(200, responseBody, event.RequestID)
}
