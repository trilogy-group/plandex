package webhooks

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"plandex-cli-api/config"
)

// JobStatusUpdate represents a status update for webhooks
type JobStatusUpdate struct {
	JobID       string                 `json:"job_id"`
	Status      string                 `json:"status"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Output      string                 `json:"output,omitempty"`
	Error       string                 `json:"error,omitempty"`
	ExitCode    *int                   `json:"exit_code,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Sender handles webhook delivery
type Sender struct {
	config     *config.Config
	httpClient *http.Client
}

// NewSender creates a new webhook sender
func NewSender(cfg *config.Config) *Sender {
	return &Sender{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Send sends a webhook notification with retry logic
func (s *Sender) Send(url string, update *JobStatusUpdate) {
	if !s.config.Webhooks.Enabled {
		return
	}

	for attempt := 0; attempt <= s.config.Webhooks.MaxRetries; attempt++ {
		if err := s.sendWebhook(url, update); err != nil {
			log.Printf("Webhook delivery attempt %d failed for job %s: %v",
				attempt+1, update.JobID, err)

			if attempt < s.config.Webhooks.MaxRetries {
				time.Sleep(s.config.Webhooks.RetryBackoff.Duration * time.Duration(attempt+1))
				continue
			}

			log.Printf("Webhook delivery failed permanently for job %s after %d attempts",
				update.JobID, s.config.Webhooks.MaxRetries+1)
		} else {
			log.Printf("Webhook delivered successfully for job %s", update.JobID)
			break
		}
	}
}

// sendWebhook sends a single webhook request
func (s *Sender) sendWebhook(url string, update *JobStatusUpdate) error {
	// Marshal the payload
	payload, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	// Create request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "plandex-cli-api/1.0")

	// Add timestamp
	timestamp := time.Now().Unix()
	req.Header.Set("X-Webhook-Timestamp", fmt.Sprintf("%d", timestamp))

	// Add signature if secret is configured
	if s.config.Webhooks.Secret != "" {
		signature := s.generateSignature(payload, timestamp)
		req.Header.Set("X-Webhook-Signature", signature)
	}

	// Send request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook endpoint returned status %d", resp.StatusCode)
	}

	return nil
}

// generateSignature generates HMAC-SHA256 signature for webhook verification
func (s *Sender) generateSignature(payload []byte, timestamp int64) string {
	message := fmt.Sprintf("%d.%s", timestamp, string(payload))
	mac := hmac.New(sha256.New, []byte(s.config.Webhooks.Secret))
	mac.Write([]byte(message))
	signature := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("sha256=%s", signature)
}

// VerifySignature verifies a webhook signature (for testing purposes)
func (s *Sender) VerifySignature(payload []byte, timestamp int64, signature string) bool {
	if s.config.Webhooks.Secret == "" {
		return true // No secret configured, skip verification
	}

	expectedSignature := s.generateSignature(payload, timestamp)
	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}
