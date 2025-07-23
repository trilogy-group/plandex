package jobs

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"

	"plandex-cli-api/config"
	"plandex-cli-api/executor"
	"plandex-cli-api/webhooks"
)

// Manager handles job lifecycle and execution
type Manager struct {
	config        *config.Config
	jobs          map[string]*Job
	jobsMutex     sync.RWMutex
	running       map[string]context.CancelFunc
	runningMutex  sync.RWMutex
	semaphore     chan struct{}
	webhookSender *webhooks.Sender
	ctx           context.Context
	cancel        context.CancelFunc
	executor      *executor.CLIExecutor
}

// NewManager creates a new job manager
func NewManager(cfg *config.Config) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		config:        cfg,
		jobs:          make(map[string]*Job),
		running:       make(map[string]context.CancelFunc),
		semaphore:     make(chan struct{}, cfg.Jobs.MaxConcurrent),
		webhookSender: webhooks.NewSender(cfg),
		ctx:           ctx,
		cancel:        cancel,
		executor:      executor.NewCLIExecutor(cfg.CLI.WorkingDir, cfg.CLI.ProjectPath, cfg.CLI.APIKeys, cfg.CLI.Environment),
	}

	// Start cleanup routine
	go m.cleanupRoutine()

	return m
}

// CreateJob creates a new job from a request
func (m *Manager) CreateJob(req *JobRequest) (*Job, error) {
	if err := m.validateCommand(req.Command); err != nil {
		return nil, fmt.Errorf("invalid command: %w", err)
	}

	job := &Job{
		ID:         uuid.New().String(),
		Command:    req.Command,
		Args:       req.Args,
		Status:     JobStatusPending,
		CreatedAt:  time.Now(),
		Metadata:   req.Metadata,
		WebhookURL: req.WebhookURL,
		TTL:        m.config.Jobs.DefaultTTL,
	}

	if req.TTL != nil {
		job.TTL = *req.TTL
	}

	m.jobsMutex.Lock()
	m.jobs[job.ID] = job
	m.jobsMutex.Unlock()

	// Start job execution asynchronously
	go m.executeJob(job)

	return job, nil
}

// GetJob retrieves a job by ID
func (m *Manager) GetJob(id string) (*Job, error) {
	m.jobsMutex.RLock()
	job, exists := m.jobs[id]
	m.jobsMutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("job not found: %s", id)
	}

	return job, nil
}

// ListJobs returns all jobs with optional filtering
func (m *Manager) ListJobs(status JobStatus, limit int) ([]*Job, error) {
	m.jobsMutex.RLock()
	defer m.jobsMutex.RUnlock()

	var jobs []*Job
	count := 0

	for _, job := range m.jobs {
		if status != "" && job.Status != status {
			continue
		}

		jobs = append(jobs, job)
		count++

		if limit > 0 && count >= limit {
			break
		}
	}

	return jobs, nil
}

// CancelJob cancels a running job
func (m *Manager) CancelJob(id string) error {
	m.jobsMutex.Lock()
	job, exists := m.jobs[id]
	if !exists {
		m.jobsMutex.Unlock()
		return fmt.Errorf("job not found: %s", id)
	}

	if job.IsComplete() {
		m.jobsMutex.Unlock()
		return fmt.Errorf("job already completed: %s", id)
	}

	job.Status = JobStatusCancelled
	now := time.Now()
	job.CompletedAt = &now
	m.jobsMutex.Unlock()

	// Cancel the running command if it exists
	m.runningMutex.Lock()
	if cancelFn, exists := m.running[id]; exists {
		cancelFn()
		delete(m.running, id)
	}
	m.runningMutex.Unlock()

	// Send webhook notification
	if job.WebhookURL != "" {
		update := &webhooks.JobStatusUpdate{
			JobID:       job.ID,
			Status:      string(job.Status),
			CompletedAt: job.CompletedAt,
			Output:      job.Output,
			Error:       job.Error,
			ExitCode:    job.ExitCode,
			Metadata:    job.Metadata,
		}
		go m.webhookSender.Send(job.WebhookURL, update)
	}

	return nil
}

// executeJob executes a job
func (m *Manager) executeJob(job *Job) {
	// Acquire semaphore to limit concurrent jobs
	select {
	case m.semaphore <- struct{}{}:
		defer func() { <-m.semaphore }()
	case <-m.ctx.Done():
		return
	}

	// Update job status to running
	m.jobsMutex.Lock()
	if job.Status == JobStatusCancelled {
		m.jobsMutex.Unlock()
		return
	}
	job.Status = JobStatusRunning
	now := time.Now()
	job.StartedAt = &now
	m.jobsMutex.Unlock()

	// Send webhook notification for job start
	if job.WebhookURL != "" {
		update := &webhooks.JobStatusUpdate{
			JobID:       job.ID,
			Status:      string(job.Status),
			CompletedAt: job.CompletedAt,
			Output:      job.Output,
			Error:       job.Error,
			ExitCode:    job.ExitCode,
			Metadata:    job.Metadata,
		}
		go m.webhookSender.Send(job.WebhookURL, update)
	}

	// Create a context for this job execution that can be cancelled
	jobCtx, cancelFn := context.WithCancel(m.ctx)
	defer cancelFn()

	// Track running command cancellation function
	m.runningMutex.Lock()
	m.running[job.ID] = cancelFn
	m.runningMutex.Unlock()

	// Execute command using CLI executor
	result, err := m.executor.Execute(jobCtx, job.Command, job.Args)

	// Clean up running command tracking
	m.runningMutex.Lock()
	delete(m.running, job.ID)
	m.runningMutex.Unlock()

	var output string
	if result != nil {
		output = result.Output
		if result.Error != "" {
			if output != "" {
				output += "\n"
			}
			output += result.Error
		}
	}

	// Update job with results
	m.jobsMutex.Lock()
	completedAt := time.Now()
	job.CompletedAt = &completedAt
	job.Output = string(output)

	if err != nil || (result != nil && result.ExitCode != 0) {
		job.Status = JobStatusFailed
		if err != nil {
			job.Error = err.Error()
		}
		if result != nil {
			job.ExitCode = &result.ExitCode
		} else {
			exitCode := 1
			job.ExitCode = &exitCode
		}
	} else {
		job.Status = JobStatusCompleted
		exitCode := 0
		job.ExitCode = &exitCode
	}
	m.jobsMutex.Unlock()

	// Send final webhook notification
	if job.WebhookURL != "" {
		update := &webhooks.JobStatusUpdate{
			JobID:       job.ID,
			Status:      string(job.Status),
			CompletedAt: job.CompletedAt,
			Output:      job.Output,
			Error:       job.Error,
			ExitCode:    job.ExitCode,
			Metadata:    job.Metadata,
		}
		go m.webhookSender.Send(job.WebhookURL, update)
	}
}

// validateCommand validates that a command is allowed
func (m *Manager) validateCommand(command string) error {
	allowedCommands := []string{
		"new", "tell", "chat", "continue", "load", "ls", "plans", "cd",
		"apply", "build", "log", "convo", "diff", "current", "config",
		"models", "set-config", "set-model", "branches", "checkout",
		"archive", "unarchive", "usage", "version", "debug", "stop",
		"rewind", "reject", "clear", "summary", "delete-plan",
	}

	for _, allowed := range allowedCommands {
		if command == allowed {
			return nil
		}
	}

	return fmt.Errorf("command not allowed: %s", command)
}

// cleanupRoutine periodically cleans up expired jobs
func (m *Manager) cleanupRoutine() {
	ticker := time.NewTicker(m.config.Jobs.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanupExpiredJobs()
		case <-m.ctx.Done():
			return
		}
	}
}

// cleanupExpiredJobs removes expired jobs
func (m *Manager) cleanupExpiredJobs() {
	m.jobsMutex.Lock()
	defer m.jobsMutex.Unlock()

	now := time.Now()
	toDelete := []string{}

	for id, job := range m.jobs {
		if job.CreatedAt.Add(job.TTL).Before(now) {
			toDelete = append(toDelete, id)
		}
	}

	// Keep history size manageable
	if len(m.jobs) > m.config.Jobs.MaxHistorySize {
		// Sort jobs by creation time and remove oldest
		var sortedJobs []*Job
		for _, job := range m.jobs {
			sortedJobs = append(sortedJobs, job)
		}

		// Simple bubble sort by creation time (oldest first)
		for i := 0; i < len(sortedJobs)-1; i++ {
			for j := 0; j < len(sortedJobs)-i-1; j++ {
				if sortedJobs[j].CreatedAt.After(sortedJobs[j+1].CreatedAt) {
					sortedJobs[j], sortedJobs[j+1] = sortedJobs[j+1], sortedJobs[j]
				}
			}
		}

		excessCount := len(m.jobs) - m.config.Jobs.MaxHistorySize
		for i := 0; i < excessCount; i++ {
			toDelete = append(toDelete, sortedJobs[i].ID)
		}
	}

	for _, id := range toDelete {
		delete(m.jobs, id)
	}

	if len(toDelete) > 0 {
		log.Printf("Cleaned up %d expired/excess jobs", len(toDelete))
	}
}

// Shutdown gracefully shuts down the manager
func (m *Manager) Shutdown() {
	m.cancel()

	// Cancel all running jobs
	m.runningMutex.Lock()
	for id, cancelFn := range m.running {
		cancelFn()

		// Update job status
		m.jobsMutex.Lock()
		if job, exists := m.jobs[id]; exists {
			job.Status = JobStatusCancelled
			now := time.Now()
			job.CompletedAt = &now
		}
		m.jobsMutex.Unlock()
	}
	m.runningMutex.Unlock()
}
