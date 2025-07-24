package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"plandex-cli/api"
	"plandex-cli/auth"
	"plandex-cli/fs"
	"plandex-cli/lib"
	"plandex-cli/plan_exec"
	"plandex-cli/term"
	"plandex-cli/types"

	shared "plandex-shared"

	"github.com/fatih/color"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

type Config struct {
	Server struct {
		Port         int    `json:"port"`
		Host         string `json:"host"`
		ReadTimeout  string `json:"read_timeout"`
		WriteTimeout string `json:"write_timeout"`
		IdleTimeout  string `json:"idle_timeout"`
	} `json:"server"`
	Auth struct {
		APIKeys     []string `json:"api_keys"`
		RequireAuth bool     `json:"require_auth"`
	} `json:"auth"`
	CLI struct {
		WorkingDir  string            `json:"working_dir"`
		Environment map[string]string `json:"environment"`
		Timeout     string            `json:"timeout"`
	} `json:"cli"`
	Security struct {
		EnableCORS     bool     `json:"enable_cors"`
		AllowedOrigins []string `json:"allowed_origins"`
		RateLimit      int      `json:"rate_limit"`
		TrustedProxies []string `json:"trusted_proxies"`
	} `json:"security"`
}

type APIServer struct {
	config     *Config
	router     *mux.Router
	server     *http.Server
	workingDir string
}

type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
}

type TellRequest struct {
	Prompt      string `json:"prompt"`
	PlanName    string `json:"plan_name,omitempty"`
	AutoApply   bool   `json:"auto_apply,omitempty"`
	AutoContext bool   `json:"auto_context,omitempty"`
}

type ChatRequest struct {
	Prompt   string `json:"prompt"`
	PlanName string `json:"plan_name,omitempty"`
}

type PlanRequest struct {
	Name string `json:"name,omitempty"`
}

func Start(configFile string) {
	config, err := loadConfig(configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	server := &APIServer{
		config: config,
	}

	if err := server.initialize(); err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}

	server.start()
}

func loadConfig(configFile string) (*Config, error) {
	if configFile == "" {
		configFile = "plandex-api.json"
	}

	// Check if config file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", configFile)
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	// Set defaults
	if config.Server.Port == 0 {
		config.Server.Port = 8080
	}
	if config.Server.Host == "" {
		config.Server.Host = "127.0.0.1"
	}
	if config.Server.ReadTimeout == "" {
		config.Server.ReadTimeout = "30s"
	}
	if config.Server.WriteTimeout == "" {
		config.Server.WriteTimeout = "30s"
	}
	if config.Server.IdleTimeout == "" {
		config.Server.IdleTimeout = "60s"
	}
	if config.CLI.WorkingDir == "" {
		config.CLI.WorkingDir = "."
	}

	return &config, nil
}

func (s *APIServer) initialize() error {
	// Set working directory
	absWorkingDir, err := filepath.Abs(s.config.CLI.WorkingDir)
	if err != nil {
		return fmt.Errorf("failed to resolve working directory: %v", err)
	}
	s.workingDir = absWorkingDir

	// Change to working directory
	if err := os.Chdir(s.workingDir); err != nil {
		return fmt.Errorf("failed to change to working directory: %v", err)
	}

	// Set environment variables
	for key, value := range s.config.CLI.Environment {
		os.Setenv(key, value)
	}

	// Initialize CLI dependencies without interactive prompts
	term.SetIsRepl(false)

	// Require full CLI setup - authentication and project must exist
	if err := s.requireFullCLISetup(); err != nil {
		return fmt.Errorf("CLI setup required: %v", err)
	}

	s.setupRoutes()
	return nil
}

func (s *APIServer) requireFullCLISetup() error {
	// Check for authentication
	if _, err := os.Stat(fs.HomeAuthPath); os.IsNotExist(err) {
		return fmt.Errorf("Plandex CLI authentication not configured. Please run 'plandex auth' first")
	}

	// Try to resolve auth - this must succeed
	auth.MustResolveAuthWithOrg()
	if auth.Current == nil {
		return fmt.Errorf("authentication failed. Please run 'plandex auth' to configure")
	}

	// Check for project setup
	lib.MustResolveProject()
	if lib.CurrentProjectId == "" {
		return fmt.Errorf("no Plandex project found. Please run 'plandex new' to create a project first")
	}

	// Require at least one existing plan
	plans, apiErr := api.Client.ListPlans([]string{lib.CurrentProjectId})
	if apiErr != nil {
		return fmt.Errorf("failed to list plans: %v", apiErr)
	}

	if len(plans) == 0 {
		return fmt.Errorf("no plans exist. Please create a plan using 'plandex new' first")
	}

	// Require a current plan to be set
	if lib.CurrentPlanId == "" {
		return fmt.Errorf("no current plan set. Please select a plan using 'plandex cd <plan-name>' first")
	}

	// Verify the current plan exists and has proper configuration
	var currentPlan *shared.Plan
	for _, plan := range plans {
		if plan.Id == lib.CurrentPlanId {
			currentPlan = plan
			break
		}
	}

	if currentPlan == nil {
		return fmt.Errorf("current plan not found. Please select a valid plan using 'plandex cd <plan-name>' first")
	}

	log.Printf("✅ CLI setup verified - authenticated as %s, project: %s, current plan: %s",
		auth.Current.Email, lib.CurrentProjectId, currentPlan.Name)

	return nil
}

func (s *APIServer) setupRoutes() {
	s.router = mux.NewRouter()

	// Add API key middleware if auth is required
	if s.config.Auth.RequireAuth {
		s.router.Use(s.authMiddleware)
	}

	// Add logging middleware
	s.router.Use(s.loggingMiddleware)

	// API routes
	api := s.router.PathPrefix("/api/v1").Subrouter()

	// Health check
	api.HandleFunc("/health", s.handleHealth).Methods("GET")
	api.HandleFunc("/status", s.handleStatus).Methods("GET")

	// Chat and Tell - core functionality
	api.HandleFunc("/chat", s.handleChat).Methods("POST")
	api.HandleFunc("/tell", s.handleTell).Methods("POST")

	// Plan management - read-only and switch only, no creation
	api.HandleFunc("/plans", s.handlePlans).Methods("GET")
	api.HandleFunc("/plans/current", s.handleCurrentPlan).Methods("GET")
	api.HandleFunc("/plans/current", s.handleSetCurrentPlan).Methods("POST")

	// Remove plan creation endpoint - not allowed via API
	// Remove job management - keep it simple
}

func (s *APIServer) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			s.writeError(w, http.StatusUnauthorized, "API key required")
			return
		}

		// Check if API key is valid
		validKey := false
		for _, key := range s.config.Auth.APIKeys {
			if key == apiKey {
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

func (s *APIServer) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap the ResponseWriter to capture status code
		wrapped := &statusResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		log.Printf("%s %s %d %v", r.Method, r.URL.Path, wrapped.statusCode, time.Since(start))
	})
}

type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (s *APIServer) writeJSON(w http.ResponseWriter, status int, response APIResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}

func (s *APIServer) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, APIResponse{
		Success: false,
		Error:   message,
	})
}

func (s *APIServer) writeSuccess(w http.ResponseWriter, data interface{}, message string) {
	s.writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    data,
		Message: message,
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
		status["org"] = auth.Current.OrgName
	}

	s.writeSuccess(w, status, "Current status")
}

func (s *APIServer) handleChat(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if req.Prompt == "" {
		s.writeError(w, http.StatusBadRequest, "Prompt is required")
		return
	}

	// Switch to requested plan if specified (must be existing plan)
	if req.PlanName != "" {
		if err := s.switchToPlan(req.PlanName); err != nil {
			s.writeError(w, http.StatusBadRequest, fmt.Sprintf("Failed to switch to plan: %v", err))
			return
		}
	}

	// Verify we have a current plan (should always be true after requireFullCLISetup)
	if lib.CurrentPlanId == "" {
		s.writeError(w, http.StatusInternalServerError, "No current plan available. Please select a plan using CLI first.")
		return
	}

	// Execute chat command
	result, err := s.executeChat(req.Prompt)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Chat failed: %v", err))
		return
	}

	s.writeSuccess(w, result, "Chat completed successfully")
}

func (s *APIServer) handleTell(w http.ResponseWriter, r *http.Request) {
	var req TellRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if req.Prompt == "" {
		s.writeError(w, http.StatusBadRequest, "Prompt is required")
		return
	}

	// Switch to requested plan if specified (must be existing plan)
	if req.PlanName != "" {
		if err := s.switchToPlan(req.PlanName); err != nil {
			s.writeError(w, http.StatusBadRequest, fmt.Sprintf("Failed to switch to plan: %v", err))
			return
		}
	}

	// Verify we have a current plan (should always be true after requireFullCLISetup)
	if lib.CurrentPlanId == "" {
		s.writeError(w, http.StatusInternalServerError, "No current plan available. Please select a plan using CLI first.")
		return
	}

	// Execute tell command
	result, err := s.executeTell(req.Prompt, req.AutoApply, req.AutoContext)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Tell failed: %v", err))
		return
	}

	s.writeSuccess(w, result, "Tell completed successfully")
}

func (s *APIServer) handlePlans(w http.ResponseWriter, r *http.Request) {
	// Get plans using the CLI API
	plans, apiErr := api.Client.ListPlans([]string{lib.CurrentProjectId})
	if apiErr != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list plans: %v", apiErr))
		return
	}

	// Convert to API response format
	planList := make([]map[string]interface{}, len(plans))
	for i, plan := range plans {
		planList[i] = map[string]interface{}{
			"id":      plan.Id,
			"name":    plan.Name,
			"current": plan.Id == lib.CurrentPlanId,
		}
	}

	s.writeSuccess(w, planList, "Plans retrieved")
}

func (s *APIServer) handleCurrentPlan(w http.ResponseWriter, r *http.Request) {
	if lib.CurrentPlanId == "" {
		s.writeError(w, http.StatusNotFound, "No current plan")
		return
	}

	s.writeSuccess(w, map[string]string{
		"id":     lib.CurrentPlanId,
		"branch": lib.CurrentBranch,
	}, "Current plan")
}

func (s *APIServer) handleSetCurrentPlan(w http.ResponseWriter, r *http.Request) {
	var req PlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if req.Name == "" {
		s.writeError(w, http.StatusBadRequest, "Plan name is required")
		return
	}

	if err := s.switchToPlan(req.Name); err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Sprintf("Failed to switch to plan: %v", err))
		return
	}

	s.writeSuccess(w, map[string]string{
		"id":   lib.CurrentPlanId,
		"name": req.Name,
	}, "Switched to plan")
}

func (s *APIServer) switchToPlan(planName string) error {
	// Get available plans
	plans, apiErr := api.Client.ListPlans([]string{lib.CurrentProjectId})
	if apiErr != nil {
		return fmt.Errorf("failed to list plans: %v", apiErr)
	}

	// Find the plan by name
	var targetPlan *shared.Plan
	for _, plan := range plans {
		if plan.Name == planName {
			targetPlan = plan
			break
		}
	}

	if targetPlan == nil {
		return fmt.Errorf("plan not found: %s. Available plans: %v", planName, s.getAvailablePlanNames(plans))
	}

	// Switch to the plan
	if err := lib.WriteCurrentPlan(targetPlan.Id); err != nil {
		return fmt.Errorf("failed to set current plan: %v", err)
	}

	// Reload current plan
	lib.MustLoadCurrentPlan()

	return nil
}

func (s *APIServer) getAvailablePlanNames(plans []*shared.Plan) []string {
	names := make([]string, len(plans))
	for i, plan := range plans {
		names[i] = plan.Name
	}
	return names
}

func (s *APIServer) executeChat(prompt string) (interface{}, error) {
	// Execute chat using the CLI's plan_exec.TellPlan with chat-only flag
	tellFlags := types.TellFlags{
		IsChatOnly:  true,
		AutoContext: false,
		ExecEnabled: false,
	}

	// Use plan_exec.TellPlan which is the same function the CLI uses
	plan_exec.TellPlan(plan_exec.ExecParams{
		CurrentPlanId: lib.CurrentPlanId,
		CurrentBranch: lib.CurrentBranch,
		AuthVars:      lib.MustVerifyAuthVars(auth.Current.IntegratedModelsMode),
		CheckOutdatedContext: func(maybeContexts []*shared.Context, projectPaths *types.ProjectPaths) (bool, bool, error) {
			// Auto-confirm for API mode
			return lib.CheckOutdatedContextWithOutput(true, true, maybeContexts, projectPaths)
		},
	}, prompt, tellFlags)

	return map[string]interface{}{
		"prompt":   prompt,
		"response": "Chat executed successfully",
		"mode":     "chat",
		"plan_id":  lib.CurrentPlanId,
	}, nil
}

func (s *APIServer) executeTell(prompt string, autoApply, autoContext bool) (interface{}, error) {
	// Execute tell using the CLI's plan_exec.TellPlan
	tellFlags := types.TellFlags{
		IsChatOnly:      false,
		AutoContext:     autoContext,
		ExecEnabled:     true,
		AutoApply:       autoApply,
		SkipChangesMenu: true, // Skip interactive menus in API mode
	}

	// Use plan_exec.TellPlan which is the same function the CLI uses
	plan_exec.TellPlan(plan_exec.ExecParams{
		CurrentPlanId: lib.CurrentPlanId,
		CurrentBranch: lib.CurrentBranch,
		AuthVars:      lib.MustVerifyAuthVars(auth.Current.IntegratedModelsMode),
		CheckOutdatedContext: func(maybeContexts []*shared.Context, projectPaths *types.ProjectPaths) (bool, bool, error) {
			// Auto-confirm for API mode
			return lib.CheckOutdatedContextWithOutput(true, true, maybeContexts, projectPaths)
		},
	}, prompt, tellFlags)

	if autoApply {
		applyFlags := types.ApplyFlags{
			AutoConfirm: true,
			AutoCommit:  true,
			NoCommit:    false,
			AutoExec:    true,
			NoExec:      false,
		}

		lib.MustApplyPlan(lib.ApplyPlanParams{
			PlanId:     lib.CurrentPlanId,
			Branch:     lib.CurrentBranch,
			ApplyFlags: applyFlags,
			TellFlags:  tellFlags,
			OnExecFail: plan_exec.GetOnApplyExecFail(applyFlags, tellFlags),
		})
	}

	return map[string]interface{}{
		"prompt":       prompt,
		"response":     "Tell executed successfully",
		"mode":         "tell",
		"auto_apply":   autoApply,
		"auto_context": autoContext,
		"plan_id":      lib.CurrentPlanId,
	}, nil
}

func (s *APIServer) start() {
	// Parse timeouts
	readTimeout, _ := time.ParseDuration(s.config.Server.ReadTimeout)
	writeTimeout, _ := time.ParseDuration(s.config.Server.WriteTimeout)
	idleTimeout, _ := time.ParseDuration(s.config.Server.IdleTimeout)

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)

	var handler http.Handler = s.router

	// Add CORS if enabled
	if s.config.Security.EnableCORS {
		c := cors.New(cors.Options{
			AllowedOrigins: s.config.Security.AllowedOrigins,
			AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{"*"},
		})
		handler = c.Handler(s.router)
	}

	s.server = &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	// Start server in a goroutine
	go func() {
		color.New(color.Bold, color.FgGreen).Printf("🚀 Plandex CLI API server starting on %s\n", addr)
		color.New(color.FgCyan).Printf("📂 Working directory: %s\n", s.workingDir)
		color.New(color.FgCyan).Printf("🔍 Health check: http://%s/api/v1/health\n", addr)

		if s.config.Auth.RequireAuth && len(s.config.Auth.APIKeys) > 0 {
			color.New(color.FgYellow).Printf("🔑 API Key required (configured: %d keys)\n", len(s.config.Auth.APIKeys))
		}

		fmt.Println()

		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	color.New(color.FgYellow).Println("\n🛑 Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	color.New(color.FgGreen).Println("✅ Server stopped")
}
