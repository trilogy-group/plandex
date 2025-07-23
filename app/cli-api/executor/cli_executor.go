package executor

import (
	"context"
	"fmt"
	"io/ioutil"
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
	
	// Create temporary output file for non-interactive mode
	tmpFile, err := ioutil.TempFile("", "plandex-output-*")
	if err != nil {
		return &ExecuteResult{Output: "", Error: err.Error(), ExitCode: 1}, err
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)
	
	// Build command with properly quoted arguments
	var cmdParts []string
	cmdParts = append(cmdParts, shellQuote(command))
	for _, arg := range args {
		cmdParts = append(cmdParts, shellQuote(arg))
	}
	
	// Build full command with proper environment sourcing and argument quoting
	fullCmd := fmt.Sprintf("cd %s && echo 'n' | %s %s",
		shellQuote(e.workingDir),
		e.plandexBinary,
		strings.Join(cmdParts, " "))
	
	cmd := exec.CommandContext(cmdCtx, "bash", "-c", fullCmd)
	
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
	
	// Add non-interactive mode variables
	env = append(env,
		"PLANDEX_REPL=1",                           // Enable REPL/batch mode
		"PLANDEX_REPL_OUTPUT_FILE="+tmpPath,        // Output to file instead of TTY
		"PLANDEX_SKIP_UPGRADE=1",                   // Skip upgrade prompts
		"PLANDEX_DISABLE_SUGGESTIONS=1",            // Disable suggestions
		"PLANDEX_COLUMNS=120",                      // Set terminal width
	)
	
	cmd.Env = env
	
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	err = cmd.Run()
	
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = 1
		}
	}
	
	// Try to read from output file first (for interactive commands like chat)
	output := stdout.String()
	if fileContent, fileErr := ioutil.ReadFile(tmpPath); fileErr == nil && len(fileContent) > 0 {
		output = string(fileContent)
	}
	
	// Add stderr if any
	if stderr.String() != "" {
		if output != "" {
			output += "\n"
		}
		output += stderr.String()
	}
	
	return &ExecuteResult{
		Output:   output,
		Error:    stderr.String(),
		ExitCode: exitCode,
	}, nil
}
