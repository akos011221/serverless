// This package servers as the central coordinator, handling incoming HTTP requests for function deployment and
// invocation, bridging client interactions (via pkg/cli) with server-side execution (via pkg/orchestrator)
// and metadata storage (via pkg/storage).
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/akos011221/serverless/pkg/orchestrator"
	"github.com/akos011221/serverless/pkg/storage"
	"github.com/sirupsen/logrus"
)

// Server manages the HTTP interface for the platform.
// It handles function registration and invocation, delegation to storage and orcehstrator.
type Server struct {
	store        *storage.Store
	orchestrator *orchestrator.Orchestrator
	log          *logrus.Logger
}

// NewServer initializes the server with its dependencies.
func NewServer(store *storage.Store, log *logrus.Logger) (*Server, error) {
	// Initizalize the orchestrator - which is the Docker container
	// manager.
	orch, err := orchestrator.NewOrchestrator(log)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize orchestrator: %v", err)
	}

	return &Server{
		store:        store,
		orchestrator: orch,
		log:          log,
	}, nil
}

// Run starts the HTTP server, listening for function deployment and invocation requests.
func (s *Server) Run(ctx context.Context, addr string) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/functions", s.handleDeploy)
	mux.HandleFunc("/invoke/", s.handleInvoke)

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	// Server is running in goroutine so we can handle
	// signals, like shutdown in the main thread
	serverErr := make(chan error, 1)
	go func() {
		s.log.WithField("addr", addr).Info("Starting HTTP server")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- fmt.Errorf("server failed: %v", err)
		}
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		s.log.Info("Shutting down server")
		// Graceful shutdown
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			s.log.WithError(err).Warn("Server shutdown failed")
			return fmt.Errorf("server shutdown failed: %v", err)
		}
		return nil
	case err := <-serverErr:
		return err
	}
}

// handeDeploy processes function deployment requests (POST /functions).
func (s *Server) handleDeploy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.log.WithField("method", r.Method).Warn("Invalid method for deploy")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var metadata struct {
		Name    string `json:"name"`
		Image   string `json:"image"`
		Runtime string `json:"runtime"`
	}
	if err := json.NewDecoder(r.Body).Decode(&metadata); err != nil {
		s.log.WithError(err).Warn("Invalid deploy request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validation
	if metadata.Name == "" || metadata.Image == "" || metadata.Runtime == "" {
		s.log.Warn("Missing required metadata fields")
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	// Store the function in the database
	if err := s.store.CreateFunction(metadata.Name, metadata.Image, metadata.Runtime); err != nil {
		s.log.WithError(err).WithField("function", metadata.Name).Error("Failed to store function")
		http.Error(w, "Failed to store function", http.StatusInternalServerError)
		return
	}

	// Log success
	s.log.WithField("function", metadata.Name).Info("Function deployed successfully")
	// Return 200 OK
	w.WriteHeader(http.StatusOK)
}

// handleInvoke processes function invocation requests (POST /invoke{name}).
func (s *Server) handleInvoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.log.WithField("method", r.Method).Warn("Invalid method for invoke")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get function name from the URL path (/invoke/{name})
	functionName := strings.TrimPrefix(r.URL.Path, "/invoke/")
	if functionName == "" {
		s.log.Warn("Missing function name in invoke request")
		http.Error(w, "Function name required", http.StatusBadRequest)
		return
	}

	// Retrieve function metadata from storage
	function, err := s.store.GetFunction(functionName)
	if err != nil {
		s.log.WithError(err).WithField("function", functionName).Warn("Function not found")
		http.Error(w, "Function not found", http.StatusNotFound)
		return
	}

	// Read the event payload from the request body
	event, err := io.ReadAll(r.Body)
	if err != nil {
		s.log.WithError(err).Warn("Failed to read invoke event")
		http.Error(w, "Failed to read event", http.StatusBadRequest)
		return
	}

	// Execute the function via the orchestrator
	result, err := s.orchestrator.Execute(context.Background(), function, event)
	if err != nil {
		s.log.WithError(err).WithField("function", functionName).Error("Function execution failed")
		http.Error(w, fmt.Sprintf("Function execution failed: %v", err), http.StatusInternalServerError)
		return
	}

	fmt.Println("got result from orchestrator")

	// Set response headers and write the function's output
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(result); err != nil {
		s.log.WithError(err).Warn("Failed to write response")
		// At this point no HTTP error sent, because the headers are
		// already written
	}
}
