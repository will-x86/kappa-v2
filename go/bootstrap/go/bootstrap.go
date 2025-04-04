package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type Response struct {
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
}

type Request struct {
	Body        string            `json:"body"`
	Headers     map[string]string `json:"headers"`
	QueryParams map[string]string `json:"queryStringParameters"`
	Path        string            `json:"path"`
	Method      string            `json:"httpMethod"`
}

type HandlerFunc func(Request) (Response, error)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/invoke", handleInvoke)
	fmt.Printf("Bootstrap server listening on port %s\n", port)
	http.ListenAndServe(":"+port, nil)
}

func handleInvoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp, err := Handler(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
