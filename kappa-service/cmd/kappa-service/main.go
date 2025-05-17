package main

import (
	"context"
	"encoding/json"
	"fmt"
	"kappa-service/internal/kappa"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/joho/godotenv/autoload"
	"go.uber.org/zap"

	"go.uber.org/zap/zapcore"
)

type KappaFunctionConfig struct {
	Name       string   `json:"name"`
	BinaryPath string   `json:"binaryPath"`
	Image      string   `json:"image"`
	Env        []string `json:"env"`
	Port       int      `json:"port"`
}

type KappaService struct {
	functions map[string]*kappa.KappaFunction
	router    *mux.Router
	server    *http.Server
}

func NewKappaService() *KappaService {
	router := mux.NewRouter()

	service := &KappaService{
		functions: make(map[string]*kappa.KappaFunction),
		router:    router,
	}

	// Set up API routes
	router.HandleFunc("/functions", service.listFunctions).Methods("GET")
	router.HandleFunc("/functions", service.registerFunction).Methods("POST")
	router.HandleFunc("/functions/{name}", service.invokeFunction).Methods("POST")
	router.HandleFunc("/functions/{name}", service.deleteFunction).Methods("DELETE")
	router.HandleFunc("/functions/{name}/logs", service.getFunctionLogs).Methods("GET")

	return service
}

func (s *KappaService) Start(addr string) error {
	s.server = &http.Server{
		Addr:    addr,
		Handler: s.router,
	}

	zap.L().Info("Starting Kappa service", zap.String("address", addr))
	return s.server.ListenAndServe()
}

func (s *KappaService) Shutdown(ctx context.Context) error {
	zap.L().Info("Shutting down Kappa service")

	// Stop all running functions
	for _, fn := range s.functions {
		if fn.IsRunning() {
			if err := fn.Stop(); err != nil {
				zap.L().Warn("Failed to stop function", zap.String("name", fn.Name), zap.Error(err))
			}
		}
	}

	return s.server.Shutdown(ctx)
}

// HTTP handler for registering a new function
func (s *KappaService) registerFunction(w http.ResponseWriter, r *http.Request) {
	var config KappaFunctionConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Validate the configuration
	if config.Name == "" || config.BinaryPath == "" || config.Image == "" {
		http.Error(w, "Missing required fields: name, binaryPath, image", http.StatusBadRequest)
		return
	}

	// Check if the binary exists
	if _, err := os.Stat(config.BinaryPath); os.IsNotExist(err) {
		http.Error(w, fmt.Sprintf("Binary not found: %s", config.BinaryPath), http.StatusBadRequest)
		return
	}

	// If no port specified, assign a default
	if config.Port == 0 {
		config.Port = 8080
	}

	// Create a new kappa function
	fn := kappa.NewKappaFunction(config.Name, config.BinaryPath, config.Image, config.Env, config.Port)

	// Add to the service
	s.functions[config.Name] = fn

	zap.L().Info("Function registered", zap.String("name", config.Name))

	// Return success
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"name":   config.Name,
		"status": "registered",
	})
}

// HTTP handler for invoking a function
func (s *KappaService) invokeFunction(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	// Find the function
	fn, exists := s.functions[name]
	if !exists {
		http.Error(w, fmt.Sprintf("Function not found: %s", name), http.StatusNotFound)
		return
	}

	// Parse the event from the request body
	var event kappa.KappaEvent
	if err := json.NewDecoder(r.Body).Decode(&event.Body); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Copy request info to the event
	event.Path = r.URL.Path
	event.HTTPMethod = r.Method
	event.Headers = make(map[string]string)
	for key, values := range r.Header {
		if len(values) > 0 {
			event.Headers[key] = values[0]
		}
	}

	event.QueryParams = make(map[string]string)
	for key, values := range r.URL.Query() {
		if len(values) > 0 {
			event.QueryParams[key] = values[0]
		}
	}

	// Invoke the function
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	resp, err := fn.Invoke(ctx, event)
	if err != nil {
		http.Error(w, fmt.Sprintf("Function invocation failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Set response headers
	for key, value := range resp.Headers {
		w.Header().Set(key, value)
	}

	// Set status code
	w.WriteHeader(resp.StatusCode)

	// Write response body
	json.NewEncoder(w).Encode(resp.Body)
}

// HTTP handler for listing functions
func (s *KappaService) listFunctions(w http.ResponseWriter, r *http.Request) {
	type functionInfo struct {
		Name      string `json:"name"`
		IsRunning bool   `json:"isRunning"`
	}

	functions := make([]functionInfo, 0, len(s.functions))
	for name, fn := range s.functions {
		functions = append(functions, functionInfo{
			Name:      name,
			IsRunning: fn.IsRunning(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"functions": functions,
	})
}

// HTTP handler for deleting a function
func (s *KappaService) deleteFunction(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	// Find the function
	fn, exists := s.functions[name]
	if !exists {
		http.Error(w, fmt.Sprintf("Function not found: %s", name), http.StatusNotFound)
		return
	}

	// Stop the function if it's running
	if fn.IsRunning() {
		if err := fn.Stop(); err != nil {
			http.Error(w, fmt.Sprintf("Failed to stop function: %v", err), http.StatusInternalServerError)
			return
		}
	}

	// Remove the function from the service
	delete(s.functions, name)

	zap.L().Info("Function deleted", zap.String("name", name))

	// Return success
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"name":   name,
		"status": "deleted",
	})
}

// HTTP handler for getting function logs
func (s *KappaService) getFunctionLogs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	// Find the function
	fn, exists := s.functions[name]
	if !exists {
		http.Error(w, fmt.Sprintf("Function not found: %s", name), http.StatusNotFound)
		return
	}

	// Get the logs
	logs := fn.GetLogs()

	// Return the logs
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"name": name,
		"logs": logs,
	})
}

func main() {
	// Initialize logger
	logger := zap.Must(zap.NewProduction())
	if os.Getenv("APP_ENV") == "development" {
		config := zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		logger = zap.Must(config.Build())
	}
	zap.ReplaceGlobals(logger)

	// Create and start the kappa service
	service := NewKappaService()

	// Handle graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := service.Start(":8000"); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start service", zap.Error(err))
		}
	}()

	logger.Info("Kappa service started", zap.String("address", ":8000"))

	// Wait for shutdown signal
	<-stop

	logger.Info("Shutting down...")

	// Give it some time to complete in-flight requests
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := service.Shutdown(ctx); err != nil {
		logger.Fatal("Server shutdown failed", zap.Error(err))
	}

	logger.Info("Server stopped")
}
