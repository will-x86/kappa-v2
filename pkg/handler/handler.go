// Package kappa provides a framework for creating serverless functions
// similar to AWS Lambda but for the Kappa platform.
package handler

import (
	"encoding/json"
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

// Handler is a function type that processes a Kappa event and returns a response
type Handler func(Event) Response

// Start initializes the Kappa function server with the provided handler
func Start(handler Handler) {
	// Get the port from environment variables (injected by the kappa system)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port
	}

	// Create a closure around the handler function
	http.HandleFunc("/2015-03-31/functions/function/invocations", createInvocationHandler(handler))
	http.HandleFunc("/health", handleHealth)

	// Print startup message
	log.Printf("Kappa function starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// createInvocationHandler returns an http.HandlerFunc that processes Kappa invocations
func createInvocationHandler(handler Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		response := handler(event)

		// Set the content type to JSON
		w.Header().Set("Content-Type", "application/json")

		// Send the response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)

		// Log the response
		log.Printf("RESPONSE: %s %d", requestID, response.StatusCode)
	}
}

// Health check endpoint
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// NewResponse creates a new Response with default values
func NewResponse(statusCode int, body any, requestID string) Response {
	return Response{
		StatusCode: statusCode,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body:      body,
		RequestID: requestID,
	}
}

// WithHeader adds or updates a header in the Response
func (r Response) WithHeader(key, value string) Response {
	r.Headers[key] = value
	return r
}

// WithStatusCode updates the status code in the Response
func (r Response) WithStatusCode(statusCode int) Response {
	r.StatusCode = statusCode
	return r
}
