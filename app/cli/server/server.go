package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/cors"

	"plandex-cli/api"
	"plandex-cli/auth"
	"plandex-cli/lib"
	"plandex-cli/plan_exec"
	"plandex-cli/types"

	shared "plandex-shared"
)

// Config represents the server configuration
type Config struct {
	Server   ServerConfig   `json:"server"`
	Auth     AuthConfig     `json:"auth"`
	CLI      CLIConfig      `json:"cli"`
	Jobs     JobsConfig     `json:"jobs"`
	Webhooks WebhooksConfig `json:"webhooks"`
	Security SecurityConfig `json:"security"`
}

type ServerConfig struct {
	Port         int    `json:"port"`
	Host         string `json:"host"`
	ReadTimeout  string `json:"read_timeout"`
	WriteTimeout string `json:"write_timeout"`
	IdleTimeout  string `json:"idle_timeout"`
}

type AuthConfig struct {
	APIKeys       []string `json:"api_keys"`
	RequireAuth   bool     `json:"require_auth"`
	TokenLifetime string   `json:"token_lifetime"`
}

type CLIConfig struct {
	WorkingDir  string            `json:"working_dir"`
	Environment map[string]string `json:"environment"`
	Timeout     string            `json:"timeout"`
}

type JobsConfig struct {
	MaxConcurrent   int    `json:"max_concurrent"`
	DefaultTTL      string `json:"default_ttl"`
	CleanupInterval string `json:"cleanup_interval"`
	MaxHistorySize  int    `json:"max_history_size"`
}

type WebhooksConfig struct {
	Enabled      bool   `json:"enabled"`
	Secret       string `json:"secret"`
	MaxRetries   int    `json:"max_retries"`
	RetryBackoff string `json:"retry_backoff"`
}

type SecurityConfig struct {
	EnableCORS     bool     `json:"enable_cors"`
	AllowedOrigins []string `json:"allowed_origins"`
	RateLimit      int      `json:"rate_limit"`
	TrustedProxies []string `json:"trusted_proxies"`
}

// Job represents a CLI operation job
type Job struct {
	ID          string                 `json:"job_id"`
	Status      JobStatus              `json:"status"`
	CreatedAt   time.Time              `json:"created_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Result      map[string]interface{} `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	cancel      context.CancelFunc
	mu          sync.RWMutex
}

type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

func (j *Job) SetStatus(status JobStatus) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = status
	now := time.Now()
	if status == JobStatusRunning && j.StartedAt == nil {
		j.StartedAt = &now
	} else if (status == JobStatusCompleted || status == JobStatusFailed || status == JobStatusCancelled) && j.CompletedAt == nil {
		j.CompletedAt = &now
	}
}

func (j *Job) SetResult(result map[string]interface{}) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Result = result
}

func (j *Job) SetError(error string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Error = error
}

func (j *Job) GetStatus() JobStatus {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.Status
}

// JobManager manages concurrent jobs
type JobManager struct {
	jobs      map[string]*Job
	mu        sync.RWMutex
	semaphore chan struct{}
}

func NewJobManager(maxConcurrent int) *JobManager {
	return &JobManager{
		jobs:      make(map[string]*Job),
		semaphore: make(chan struct{}, maxConcurrent),
	}
}

func (jm *JobManager) AddJob(job *Job) {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	jm.jobs[job.ID] = job
}

func (jm *JobManager) GetJob(id string) (*Job, bool) {
	jm.mu.RLock()
	defer jm.mu.RUnlock()
	job, exists := jm.jobs[id]
	return job, exists
}

func (jm *JobManager) ListJobs() map[string]*Job {
	jm.mu.RLock()
	defer jm.mu.RUnlock()
	result := make(map[string]*Job)
	for k, v := range jm.jobs {
		result[k] = v
	}
	return result
}

// APIServer represents the HTTP API server
type APIServer struct {
	config     *Config
	router     *mux.Router
	server     *http.Server
	jobManager *JobManager
	workingDir string
}

// NewServer creates a new API server instance
func NewServer(config *Config) *APIServer {
	jobManager := NewJobManager(config.Jobs.MaxConcurrent)

	return &APIServer{
		config:     config,
		router:     mux.NewRouter(),
		jobManager: jobManager,
		workingDir: config.CLI.WorkingDir,
	}
}

// Start starts the API server
func Start(configFile string) {
	config, err := loadConfig(configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	server := NewServer(config)

	if err := server.initialize(); err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}

	log.Printf("Starting Plandex CLI API server on %s:%d", config.Server.Host, config.Server.Port)
	if err := server.start(); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func (s *APIServer) initialize() error {
	// Set working directory if specified
	if s.workingDir != "" {
		if err := os.Chdir(s.workingDir); err != nil {
			return fmt.Errorf("failed to change to working directory %s: %v", s.workingDir, err)
		}
		log.Printf("Changed working directory to: %s", s.workingDir)
	}

	// Set environment variables from config
	for key, value := range s.config.CLI.Environment {
		os.Setenv(key, value)
		log.Printf("Set environment variable: %s=%s", key, value)
	}

	// Verify CLI setup requirements
	if err := s.requireFullCLISetup(); err != nil {
		return fmt.Errorf("CLI setup required: %v", err)
	}

	s.setupRoutes()
	return nil
}

// requireFullCLISetup verifies that CLI is fully configured
func (s *APIServer) requireFullCLISetup() error {
	log.Printf("Debug: Starting CLI setup verification...")

	// Check authentication
	log.Printf("Debug: Checking authentication...")
	auth.MustResolveAuthWithOrg()
	if auth.Current == nil {
		log.Printf("Debug: Authentication failed - auth.Current is nil")
		return fmt.Errorf("not authenticated - run 'plandex auth' first")
	}
	log.Printf("Debug: Authentication OK - User: %s, Org: %s", auth.Current.Email, auth.Current.OrgName)

	// Check project
	log.Printf("Debug: Checking project...")
	lib.MustResolveProject()
	if lib.CurrentProjectId == "" {
		log.Printf("Debug: Project resolution failed - CurrentProjectId is empty")
		return fmt.Errorf("no project found - run 'plandex new' first")
	}
	log.Printf("Debug: Project OK - ProjectId: %s", lib.CurrentProjectId)

	// Check current plan
	log.Printf("Debug: Checking current plan...")
	if lib.CurrentPlanId == "" {
		log.Printf("Debug: Plan resolution failed - CurrentPlanId is empty")
		return fmt.Errorf("no current plan - run 'plandex new' to create a plan")
	}
	log.Printf("Debug: Plan OK - PlanId: %s", lib.CurrentPlanId)

	log.Printf("Debug: CLI setup verification completed successfully")
	return nil
}

func (s *APIServer) setupRoutes() {
	// Middleware
	if s.config.Auth.RequireAuth {
		s.router.Use(s.authMiddleware)
	}

	// Health endpoint (no auth required)
	s.router.HandleFunc("/api/v1/health", s.handleHealth).Methods("GET")

	// Status endpoint
	s.router.HandleFunc("/api/v1/status", s.handleStatus).Methods("GET")

	// Chat endpoint
	s.router.HandleFunc("/api/v1/chat", s.handleChat).Methods("POST")

	// Tell endpoint
	s.router.HandleFunc("/api/v1/tell", s.handleTell).Methods("POST")

	// Plans management
	s.router.HandleFunc("/api/v1/plans", s.handleListPlans).Methods("GET")
	s.router.HandleFunc("/api/v1/plans/current", s.handleCurrentPlan).Methods("GET")

	// Jobs management
	s.router.HandleFunc("/api/v1/jobs", s.handleListJobs).Methods("GET")
	s.router.HandleFunc("/api/v1/jobs/{id}", s.handleGetJob).Methods("GET")
	s.router.HandleFunc("/api/v1/jobs/{id}/cancel", s.handleCancelJob).Methods("POST")

	log.Println("✅ Routes configured")
}

func (s *APIServer) start() error {
	readTimeout, _ := time.ParseDuration(s.config.Server.ReadTimeout)
	writeTimeout, _ := time.ParseDuration(s.config.Server.WriteTimeout)
	idleTimeout, _ := time.ParseDuration(s.config.Server.IdleTimeout)

	s.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port),
		Handler:      s.getCORSHandler(),
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	// Graceful shutdown
	go s.handleShutdown()

	return s.server.ListenAndServe()
}

func (s *APIServer) getCORSHandler() http.Handler {
	if s.config.Security.EnableCORS {
		c := cors.New(cors.Options{
			AllowedOrigins: s.config.Security.AllowedOrigins,
			AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{"*"},
		})
		return c.Handler(s.router)
	}
	return s.router
}

func (s *APIServer) handleShutdown() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	<-c
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	s.server.Shutdown(ctx)
	log.Println("Server stopped")
}

func (s *APIServer) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for health endpoint
		if r.URL.Path == "/api/v1/health" {
			next.ServeHTTP(w, r)
			return
		}

		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			s.writeError(w, http.StatusUnauthorized, "Missing API key")
			return
		}

		validKey := false
		for _, key := range s.config.Auth.APIKeys {
			if apiKey == key {
				validKey = true
				break
			}
		}

		if !validKey {
			s.writeError(w, http.StatusUnauthorized, "Invalid API key")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.writeSuccess(w, map[string]string{
		"status":      "healthy",
		"version":     "1.0.0",
		"working_dir": s.workingDir,
	}, "Service is healthy")
}

func (s *APIServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"working_dir": s.workingDir,
		"auth":        auth.Current != nil,
		"project_id":  lib.CurrentProjectId,
		"plan_id":     lib.CurrentPlanId,
		"branch":      lib.CurrentBranch,
	}

	if auth.Current != nil {
		status["user"] = auth.Current.Email
		if auth.Current.OrgName != "" {
			status["org"] = auth.Current.OrgName
		}
	}

	s.writeSuccess(w, status, "Current status")
}

type ChatRequest struct {
	Prompt      string `json:"prompt"`
	AutoContext bool   `json:"auto_context,omitempty"`
}

type TellRequest struct {
	Prompt      string `json:"prompt"`
	AutoContext bool   `json:"auto_context,omitempty"`
	AutoApply   bool   `json:"auto_apply,omitempty"`
}

type JobResponse struct {
	JobID     string    `json:"job_id"`
	Status    JobStatus `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *APIServer) handleChat(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if strings.TrimSpace(req.Prompt) == "" {
		s.writeError(w, http.StatusBadRequest, "Prompt cannot be empty")
		return
	}

	// Create job
	job := &Job{
		ID:        generateJobID(),
		Status:    JobStatusPending,
		CreatedAt: time.Now(),
	}

	s.jobManager.AddJob(job)

	// Execute chat asynchronously
	go s.executeJobAsync(job, req.Prompt, true, req.AutoContext, false)

	s.writeSuccess(w, JobResponse{
		JobID:     job.ID,
		Status:    job.Status,
		CreatedAt: job.CreatedAt,
	}, "Chat job created successfully")
}

func (s *APIServer) handleTell(w http.ResponseWriter, r *http.Request) {
	var req TellRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if strings.TrimSpace(req.Prompt) == "" {
		s.writeError(w, http.StatusBadRequest, "Prompt cannot be empty")
		return
	}

	// Create job
	job := &Job{
		ID:        generateJobID(),
		Status:    JobStatusPending,
		CreatedAt: time.Now(),
	}

	s.jobManager.AddJob(job)

	// Execute tell asynchronously
	go s.executeJobAsync(job, req.Prompt, false, req.AutoContext, req.AutoApply)

	s.writeSuccess(w, JobResponse{
		JobID:     job.ID,
		Status:    job.Status,
		CreatedAt: job.CreatedAt,
	}, "Tell job created successfully")
}

// executeJobAsync runs the Plandex command directly using plan_exec.TellPlan
func (s *APIServer) executeJobAsync(job *Job, prompt string, isChatOnly, autoContext, autoApply bool) {
	// Create context for cancellation
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	job.cancel = cancel

	// Execute the Plandex function directly
	go func() {
		defer cancel() // Move cancel to inside the goroutine
		defer func() {
			if r := recover(); r != nil {
				job.SetError(fmt.Sprintf("Job panicked: %v", r))
				job.SetStatus(JobStatusFailed)
			}
		}()

		job.SetStatus(JobStatusRunning)

		// Call plan_exec.TellPlan directly instead of using subprocess
		result, err := s.executePlandexFunction(ctx, prompt, isChatOnly, autoContext, autoApply)

		if err != nil {
			job.SetError(fmt.Sprintf("Plandex execution failed: %v", err))
			job.SetStatus(JobStatusFailed)
			return
		}

		job.SetResult(result)
		job.SetStatus(JobStatusCompleted)
	}()
}

// executePlandexFunction calls plan_exec.TellPlan directly
func (s *APIServer) executePlandexFunction(ctx context.Context, prompt string, isChatOnly, autoContext, autoApply bool) (map[string]interface{}, error) {
	// Set environment variables to disable TTY/UI components
	os.Setenv("PLANDEX_DISABLE_TUI", "1")
	os.Setenv("PLANDEX_HEADLESS", "1") 
	os.Setenv("PLANDEX_NON_INTERACTIVE", "1")
	os.Setenv("CI", "true")
	os.Setenv("TERM", "dumb")
	os.Setenv("NO_COLOR", "1")
	
	log.Printf("Debug: Starting direct function execution")
	
	// Prepare execution parameters
	authVars := lib.MustVerifyAuthVarsSilent(auth.Current.IntegratedModelsMode)

	params := plan_exec.ExecParams{
		CurrentPlanId: lib.CurrentPlanId,
		CurrentBranch: lib.CurrentBranch,
		AuthVars:      authVars,
		CheckOutdatedContext: func(maybeContexts []*shared.Context, projectPaths *types.ProjectPaths) (bool, bool, error) {
			// For API mode, auto-handle outdated context
			auto := autoContext || autoApply
			return lib.CheckOutdatedContextWithOutput(auto, auto, maybeContexts, projectPaths)
		},
	}

	// Configure tell flags
	flags := types.TellFlags{
		IsChatOnly:      isChatOnly,
		AutoContext:     autoContext,
		AutoApply:       autoApply,
		SkipChangesMenu: true,        // Always skip interactive menus in API mode
		ExecEnabled:     !isChatOnly, // Enable execution for tell, disable for chat
		TellBg:          true,        // Run in background mode to avoid streaming UI
	}

	log.Printf("Debug: Calling TellPlan directly with flags: %+v", flags)

	// Call TellPlan directly
	plan_exec.TellPlan(params, prompt, flags)

	log.Printf("Debug: TellPlan completed successfully")

	// Build result response
	result := map[string]interface{}{
		"prompt":       prompt,
		"is_chat_only": isChatOnly,
		"auto_context": autoContext,
		"auto_apply":   autoApply,
		"plan_id":      lib.CurrentPlanId,
		"branch":       lib.CurrentBranch,
		"status":       "completed",
	}

	return result, nil
}

func (s *APIServer) handleListPlans(w http.ResponseWriter, r *http.Request) {
	plans, apiErr := api.Client.ListPlans([]string{lib.CurrentProjectId})
	if apiErr != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list plans: %v", apiErr.Msg))
		return
	}

	s.writeSuccess(w, plans, "Plans retrieved successfully")
}

func (s *APIServer) handleCurrentPlan(w http.ResponseWriter, r *http.Request) {
	if lib.CurrentPlanId == "" {
		s.writeError(w, http.StatusNotFound, "No current plan")
		return
	}

	plan, apiErr := api.Client.GetPlan(lib.CurrentPlanId)
	if apiErr != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get current plan: %v", apiErr.Msg))
		return
	}

	result := map[string]interface{}{
		"plan":   plan,
		"branch": lib.CurrentBranch,
	}

	s.writeSuccess(w, result, "Current plan retrieved successfully")
}

func (s *APIServer) handleListJobs(w http.ResponseWriter, r *http.Request) {
	jobs := s.jobManager.ListJobs()
	s.writeSuccess(w, jobs, "Jobs retrieved successfully")
}

func (s *APIServer) handleGetJob(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID := vars["id"]

	job, exists := s.jobManager.GetJob(jobID)
	if !exists {
		s.writeError(w, http.StatusNotFound, "Job not found")
		return
	}

	s.writeSuccess(w, job, "Job retrieved")
}

func (s *APIServer) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID := vars["id"]

	job, exists := s.jobManager.GetJob(jobID)
	if !exists {
		s.writeError(w, http.StatusNotFound, "Job not found")
		return
	}

	if job.cancel != nil {
		job.cancel()
	}
	job.SetStatus(JobStatusCancelled)

	s.writeSuccess(w, map[string]interface{}{"job_id": jobID}, "Job cancelled")
}

// Response helpers
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
}

func (s *APIServer) writeSuccess(w http.ResponseWriter, data interface{}, message string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(APIResponse{
		Success: true,
		Data:    data,
		Message: message,
	})
}

func (s *APIServer) writeError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(APIResponse{
		Success: false,
		Error:   message,
	})
}

// Utility functions
func loadConfig(configFile string) (*Config, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	return &config, nil
}

func generateJobID() string {
	// Simple job ID generation
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
