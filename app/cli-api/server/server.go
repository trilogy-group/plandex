package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/cors"

	"plandex-cli-api/config"
	"plandex-cli-api/jobs"
)

// Server represents the HTTP server
type Server struct {
	config     *config.Config
	jobManager *jobs.Manager
	server     *http.Server
}

// New creates a new server instance
func New(cfg *config.Config) (*Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	jobManager := jobs.NewManager(cfg)

	s := &Server{
		config:     cfg,
		jobManager: jobManager,
	}

	// Setup HTTP server
	s.setupServer()

	return s, nil
}

// setupServer configures the HTTP server and routes
func (s *Server) setupServer() {
	router := mux.NewRouter()

	// Add middleware
	router.Use(s.loggingMiddleware)
	router.Use(s.authMiddleware)

	// API routes
	api := router.PathPrefix("/api/v1").Subrouter()

	// Job management endpoints
	api.HandleFunc("/jobs", s.createJob).Methods("POST")
	api.HandleFunc("/jobs", s.listJobs).Methods("GET")
	api.HandleFunc("/jobs/{id}", s.getJob).Methods("GET")
	api.HandleFunc("/jobs/{id}/cancel", s.cancelJob).Methods("POST")

	// Command documentation endpoint
	api.HandleFunc("/commands", s.listCommands).Methods("GET")
	api.HandleFunc("/commands/{command}", s.getCommand).Methods("GET")

	// Health check
	api.HandleFunc("/health", s.healthCheck).Methods("GET")

	// Setup CORS if enabled
	var handler http.Handler = router
	if s.config.Security.EnableCORS {
		c := cors.New(cors.Options{
			AllowedOrigins: s.config.Security.AllowedOrigins,
			AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{"*"},
		})
		handler = c.Handler(router)
	}

	s.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port),
		Handler:      handler,
		ReadTimeout:  s.config.Server.ReadTimeout.Duration,
		WriteTimeout: s.config.Server.WriteTimeout.Duration,
		IdleTimeout:  s.config.Server.IdleTimeout.Duration,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	log.Printf("Server starting on %s", s.server.Addr)
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() {
	s.jobManager.Shutdown()
	s.server.Close()
}

// Middleware

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
	})
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for health check
		if r.URL.Path == "/api/v1/health" {
			next.ServeHTTP(w, r)
			return
		}

		if !s.config.Auth.RequireAuth {
			next.ServeHTTP(w, r)
			return
		}

		// Check API key
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			http.Error(w, "API key required", http.StatusUnauthorized)
			return
		}

		valid := false
		for _, key := range s.config.Auth.APIKeys {
			if key == apiKey {
				valid = true
				break
			}
		}

		if !valid {
			http.Error(w, "Invalid API key", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Handlers

func (s *Server) createJob(w http.ResponseWriter, r *http.Request) {
	var req jobs.JobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	job, err := s.jobManager.CreateJob(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(job.ToResponse())
}

func (s *Server) getJob(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	job, err := s.jobManager.GetJob(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job.ToResponse())
}

func (s *Server) listJobs(w http.ResponseWriter, r *http.Request) {
	status := jobs.JobStatus(r.URL.Query().Get("status"))
	limitStr := r.URL.Query().Get("limit")

	limit := 0
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			http.Error(w, "Invalid limit parameter", http.StatusBadRequest)
			return
		}
	}

	jobsList, err := s.jobManager.ListJobs(status, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var responses []*jobs.JobResponse
	for _, job := range jobsList {
		responses = append(responses, job.ToResponse())
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responses)
}

func (s *Server) cancelJob(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	if err := s.jobManager.CancelJob(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Job cancelled"))
}

func (s *Server) listCommands(w http.ResponseWriter, r *http.Request) {
	commands := getCommandMappings()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(commands)
}

func (s *Server) getCommand(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	command := vars["command"]

	commands := getCommandMappings()
	for _, cmd := range commands {
		if cmd.Name == command {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(cmd)
			return
		}
	}

	http.Error(w, "Command not found", http.StatusNotFound)
}

func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"version":   "1.0.0",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// getCommandMappings returns the list of available CLI commands and their documentation
func getCommandMappings() []*jobs.CommandMapping {
	return []*jobs.CommandMapping{
		{
			Name:        "new",
			Description: "Start a new plan",
			Args: []jobs.CommandArg{
				{Name: "name", Description: "Name of the new plan", Required: false, Type: "string"},
			},
			Flags: []jobs.CommandFlag{
				{Name: "context-dir", Description: "Base directory to auto-load context from", Type: "string", Default: "."},
			},
			Examples: []jobs.CommandExample{
				{
					Description: "Create a new plan",
					Request:     `{"command": "new", "args": ["my-plan"]}`,
					Response:    `{"id": "123", "command": "new", "status": "completed"}`,
				},
			},
		},
		{
			Name:        "tell",
			Description: "Send a prompt for the current plan",
			Args: []jobs.CommandArg{
				{Name: "prompt", Description: "The prompt to send", Required: true, Type: "string"},
			},
			Examples: []jobs.CommandExample{
				{
					Description: "Send a prompt",
					Request:     `{"command": "tell", "args": ["Add a login form to the homepage"]}`,
					Response:    `{"id": "124", "command": "tell", "status": "running"}`,
				},
			},
		},
		{
			Name:        "load",
			Description: "Load context from various inputs",
			Args: []jobs.CommandArg{
				{Name: "files", Description: "Files or URLs to load", Required: true, Type: "string"},
			},
			Flags: []jobs.CommandFlag{
				{Name: "recursive", Short: "r", Description: "Search directories recursively", Type: "boolean", Default: false},
				{Name: "note", Short: "n", Description: "Add a note to the context", Type: "string"},
			},
			Examples: []jobs.CommandExample{
				{
					Description: "Load a file",
					Request:     `{"command": "load", "args": ["src/main.js"]}`,
					Response:    `{"id": "125", "command": "load", "status": "completed"}`,
				},
			},
		},
		{
			Name:        "plans",
			Description: "List plans",
			Flags: []jobs.CommandFlag{
				{Name: "archived", Short: "a", Description: "List archived plans", Type: "boolean", Default: false},
			},
			Examples: []jobs.CommandExample{
				{
					Description: "List all plans",
					Request:     `{"command": "plans"}`,
					Response:    `{"id": "126", "command": "plans", "status": "completed"}`,
				},
			},
		},
		{
			Name:        "apply",
			Description: "Apply pending plan changes",
			Examples: []jobs.CommandExample{
				{
					Description: "Apply changes",
					Request:     `{"command": "apply"}`,
					Response:    `{"id": "127", "command": "apply", "status": "running"}`,
				},
			},
		},
		{
			Name:        "config",
			Description: "Show plan configuration",
			Examples: []jobs.CommandExample{
				{
					Description: "Show config",
					Request:     `{"command": "config"}`,
					Response:    `{"id": "128", "command": "config", "status": "completed"}`,
				},
			},
		},
	}
}
