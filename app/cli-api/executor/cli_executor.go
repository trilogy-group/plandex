package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type CLIExecutor struct {
	workingDir    string
	projectPath   string
	plandexBinary string
	timeout       time.Duration
	apiKeys       map[string]string
	environment   map[string]string
}

func NewCLIExecutor(workingDir, projectPath string, apiKeys map[string]string, environment map[string]string) *CLIExecutor {
	return &CLIExecutor{
		workingDir:    workingDir,
		projectPath:   projectPath,
		plandexBinary: "plandex",
		timeout:       10 * time.Minute,
		apiKeys:       apiKeys,
		environment:   environment,
	}
}

type ExecuteResult struct {
	Output   string
	Error    string
	ExitCode int
}

// shellQuote properly quotes a string for shell usage
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	// Replace single quotes with '"'"'
	s = strings.ReplaceAll(s, "'", `'"'"'`)
	return "'" + s + "'"
}

func (e *CLIExecutor) Execute(ctx context.Context, command string, args []string) (*ExecuteResult, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	// Build command arguments directly (avoid shell quoting issues)
	var fullArgs []string
	fullArgs = append(fullArgs, command)

	// If command is "tell" add --bg for non-interactive background execution
	if command == "tell" {
		// Use --apply with tell to execute changes and avoid auto-context conflicts
		fullArgs = []string{"tell", "--apply"}
		fullArgs = append(fullArgs, args...)
	} else if command == "chat" {
		// chat supports passing prompt as arg
		fullArgs = []string{"chat"}
		fullArgs = append(fullArgs, args...)
	}

	cmd := exec.CommandContext(cmdCtx, e.plandexBinary, fullArgs...)
	cmd.Dir = e.workingDir

	// Set environment for non-interactive mode
	env := os.Environ()

	// Add API keys from configuration
	for key, value := range e.apiKeys {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	// Add custom environment variables
	for key, value := range e.environment {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	// Add non-interactive mode variables to disable TTY requirements
	env = append(env,
		"PLANDEX_DISABLE_TTY=1",
		"PLANDEX_NON_INTERACTIVE=1",
		"TERM=dumb",
		"NO_COLOR=1",
		"PLANDEX_SKIP_UPGRADE=1",
	)
	
	// Ensure OPENAI_API_KEY is explicitly set from config
	if openaiKey, exists := e.apiKeys["OPENAI_API_KEY"]; exists && openaiKey != "" {
		env = append(env, fmt.Sprintf("OPENAI_API_KEY=%s", openaiKey))
	}
	
	// Debug: Log environment setup for AI commands
	if command == "chat" || command == "tell" {
		fmt.Printf("DEBUG: Executing %s with args: %v\n", command, args)
		fmt.Printf("DEBUG: Working dir: %s\n", e.workingDir)
		for _, envVar := range env {
			if strings.Contains(envVar, "OPENAI_API_KEY") {
				fmt.Printf("DEBUG: API key found in env\n")
				break
			}
		}
	}

	cmd.Env = env

	var outBuilder, errBuilder strings.Builder
	cmd.Stdout = &outBuilder
	cmd.Stderr = &errBuilder

	runErr := cmd.Run()

	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	return &ExecuteResult{
		Output:   outBuilder.String(),
		Error:    errBuilder.String(),
		ExitCode: exitCode,
	}, runErr
}
