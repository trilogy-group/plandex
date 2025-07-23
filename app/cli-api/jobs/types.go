package jobs

import (
	"time"
)

// JobStatus represents the status of a job
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

// Job represents a CLI command job
type Job struct {
	ID          string                 `json:"id"`
	Command     string                 `json:"command"`
	Args        []string               `json:"args"`
	Status      JobStatus              `json:"status"`
	CreatedAt   time.Time              `json:"created_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Output      string                 `json:"output,omitempty"`
	Error       string                 `json:"error,omitempty"`
	ExitCode    *int                   `json:"exit_code,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	WebhookURL  string                 `json:"webhook_url,omitempty"`
	TTL         time.Duration          `json:"ttl"`
}

// JobRequest represents a request to create a new job
type JobRequest struct {
	Command    string                 `json:"command"`
	Args       []string               `json:"args,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	WebhookURL string                 `json:"webhook_url,omitempty"`
	TTL        *time.Duration         `json:"ttl,omitempty"`
}

// JobResponse represents the response when creating or querying a job
type JobResponse struct {
	ID          string                 `json:"id"`
	Command     string                 `json:"command"`
	Status      JobStatus              `json:"status"`
	CreatedAt   time.Time              `json:"created_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Output      string                 `json:"output,omitempty"`
	Error       string                 `json:"error,omitempty"`
	ExitCode    *int                   `json:"exit_code,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}



// CommandMapping defines how CLI commands map to API endpoints
type CommandMapping struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Args        []CommandArg      `json:"args"`
	Flags       []CommandFlag     `json:"flags"`
	Examples    []CommandExample  `json:"examples"`
}

// CommandArg represents a command argument
type CommandArg struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Type        string `json:"type"`
}

// CommandFlag represents a command flag
type CommandFlag struct {
	Name        string      `json:"name"`
	Short       string      `json:"short,omitempty"`
	Description string      `json:"description"`
	Type        string      `json:"type"`
	Default     interface{} `json:"default,omitempty"`
}

// CommandExample represents an example usage
type CommandExample struct {
	Description string `json:"description"`
	Request     string `json:"request"`
	Response    string `json:"response"`
}

// IsComplete returns true if the job has finished (completed, failed, or cancelled)
func (j *Job) IsComplete() bool {
	return j.Status == JobStatusCompleted || j.Status == JobStatusFailed || j.Status == JobStatusCancelled
}

// ToResponse converts a Job to a JobResponse
func (j *Job) ToResponse() *JobResponse {
	return &JobResponse{
		ID:          j.ID,
		Command:     j.Command,
		Status:      j.Status,
		CreatedAt:   j.CreatedAt,
		StartedAt:   j.StartedAt,
		CompletedAt: j.CompletedAt,
		Output:      j.Output,
		Error:       j.Error,
		ExitCode:    j.ExitCode,
		Metadata:    j.Metadata,
	}
}

 