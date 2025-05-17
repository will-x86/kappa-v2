package main

import (
	"encoding/json"
	"kappa-v3/pkg/handler"
	"log"
)

func main() {
	// Start the Kappa function with our handler
	handler.Start(handleRequest)
}

func marshalAndPrint(a any) {
	b, _ := json.Marshal(a)
	log.Println(string(b))
}

// handleRequest is where your actual function logic goes
func handleRequest(event handler.Event) handler.Response {
	marshalAndPrint(event)
	greeting := "Hello from your Kappa function!"

	// Extract name from the body if available
	if nameVal, ok := event.Body["name"]; ok {
		name, _ := nameVal.(string)
		greeting = "Hello, " + name + "! Welcome to your Kappa function!"
	}

	// Create response body
	responseBody := map[string]any{
		"message": greeting,
		"input":   event.Body,
	}

	// Return a formatted response
	return handler.NewResponse(200, responseBody, event.RequestID)
}
