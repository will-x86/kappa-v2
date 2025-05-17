package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

// Response is the Kappa function response structure
type Response struct {
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers"`
	Body       any               `json:"body"`
	RequestID  string            `json:"requestId"`
}

// Event is the Kappa function event structure
type Event struct {
	Body        map[string]any    `json:"body"`
	Path        string            `json:"path"`
	HTTPMethod  string            `json:"httpMethod"`
	Headers     map[string]string `json:"headers"`
	QueryParams map[string]string `json:"queryParams"`
	RequestID   string            `json:"requestId"`
}

func main() {
	// Get the port from environment variables (injected by the kappa system)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port
	}

	http.HandleFunc("/2015-03-31/functions/function/invocations", handleInvocation)
	http.HandleFunc("/health", handleHealth)

	// Print startup message
	log.Printf("Kappa function starting on port %s", port)

	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// Handler for the invocation endpoint
func handleInvocation(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Extract request ID from headers or generate a new one
	requestID := r.Header.Get("Kappa-Runtime-Aws-Request-Id")
	if requestID == "" {
		requestID = "req-" + r.Header.Get("X-Request-Id")
	}

	// Log the received request
	log.Printf("REQUEST: %s %s", requestID, r.URL.Path)

	// Parse the incoming event
	var event Event
	err := json.NewDecoder(r.Body).Decode(&event)
	if err != nil {
		log.Printf("Error parsing request body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Invalid request body",
		})
		return
	}

	// Set the request ID if not already in the event
	if event.RequestID == "" {
		event.RequestID = requestID
	}

	// Call the handler function
	response := handleRequest(event)

	// Set the content type to JSON
	w.Header().Set("Content-Type", "application/json")

	// Send the response
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)

	// Log the response
	log.Printf("RESPONSE: %s %d", requestID, response.StatusCode)
}

// Health check endpoint
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// This is where your actual function logic goes
func handleRequest(event Event) Response {
	// You can modify this function to implement your specific logic
	log.Printf("Processing event: %+v", event)

	// Example: Echo back the request body with a greeting
	greeting := "Hello from your Kappa function!"

	// Extract name from the body if available
	var name string
	if nameVal, ok := event.Body["name"]; ok {
		name = fmt.Sprintf("%v", nameVal)
		greeting = fmt.Sprintf("Hello, %s! Welcome to your Kappa function!", name)
	}

	// Create response body
	responseBody := map[string]any{
		"message": greeting,
		"input":   event.Body,
	}

	// Return a formatted response
	return Response{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body:      responseBody,
		RequestID: event.RequestID,
	}
}
