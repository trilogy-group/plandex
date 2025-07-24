package executor

import (
	"fmt"
	"os"
	"plandex-cli/auth"
	"plandex-cli/lib"
	"plandex-cli/plan_exec"
	"plandex-cli/types"
	shared "plandex-shared"
	"strings"
)

type CLIExecutor struct {
	workingDir    string
	projectPath   string
	apiKeys       map[string]string
	environment   map[string]string
}

func NewCLIExecutor(workingDir string, apiKeys map[string]string, environment map[string]string) *CLIExecutor {
	return &CLIExecutor{
		workingDir:    workingDir,
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

func (e *CLIExecutor) Execute(command string, args []string) ExecuteResult {
	// Initialize Plandex environment (must be done before calling functions)
	err := e.initializePlandexEnvironment()
	if err != nil {
		return ExecuteResult{
			Output:   "",
			Error:    err.Error(),
			ExitCode: 1,
		}
	}

	// Handle different commands by calling Plandex Go functions directly
	switch command {
	case "tell":
		return e.executeTell(args)
	case "chat":
		return e.executeChat(args)
	case "models":
		return e.executeModels(args)
	case "plans":
		return e.executePlans(args)
	default:
		return ExecuteResult{
			Output:   "",
			Error:    "Unknown command: " + command,
			ExitCode: 1,
		}
	}
}

func (e *CLIExecutor) initializePlandexEnvironment() error {
	// Set up environment variables
	for key, value := range e.apiKeys {
		os.Setenv(key, value)
	}
	for key, value := range e.environment {
		os.Setenv(key, value)
	}

	// Change to working directory
	if err := os.Chdir(e.workingDir); err != nil {
		return fmt.Errorf("failed to change directory: %v", err)
	}

	// Initialize Plandex auth and project
	auth.MustResolveAuthWithOrg()
	lib.MustResolveProject()

	return nil
}

func (e *CLIExecutor) executeTell(args []string) ExecuteResult {
	if len(args) == 0 {
		return ExecuteResult{
			Output:   "",
			Error:    "No prompt provided for tell command",
			ExitCode: 1,
		}
	}

	prompt := strings.Join(args, " ")

	// Call the actual Plandex TellPlan function
	plan_exec.TellPlan(plan_exec.ExecParams{
		CurrentPlanId: lib.CurrentPlanId,
		CurrentBranch: lib.CurrentBranch,
		AuthVars:      lib.MustVerifyAuthVars(auth.Current.IntegratedModelsMode),
		CheckOutdatedContext: func(maybeContexts []*shared.Context, projectPaths *types.ProjectPaths) (bool, bool, error) {
			// Auto-approve context updates for API usage
			return lib.CheckOutdatedContextWithOutput(true, true, maybeContexts, projectPaths)
		},
	}, prompt, types.TellFlags{
		AutoContext:     true,
		ExecEnabled:     false, // Disable execution for safety
		SkipChangesMenu: true,  // Skip interactive menus
	})

	return ExecuteResult{
		Output:   "Tell command executed successfully",
		Error:    "",
		ExitCode: 0,
	}
}

func (e *CLIExecutor) executeChat(args []string) ExecuteResult {
	if len(args) == 0 {
		return ExecuteResult{
			Output:   "",
			Error:    "No prompt provided for chat command",
			ExitCode: 1,
		}
	}

	prompt := strings.Join(args, " ")

	// Call TellPlan with IsChatOnly flag for chat mode
	plan_exec.TellPlan(plan_exec.ExecParams{
		CurrentPlanId: lib.CurrentPlanId,
		CurrentBranch: lib.CurrentBranch,
		AuthVars:      lib.MustVerifyAuthVars(auth.Current.IntegratedModelsMode),
		CheckOutdatedContext: func(maybeContexts []*shared.Context, projectPaths *types.ProjectPaths) (bool, bool, error) {
			return lib.CheckOutdatedContextWithOutput(true, true, maybeContexts, projectPaths)
		},
	}, prompt, types.TellFlags{
		IsChatOnly:      true, // Chat mode - no file changes
		AutoContext:     true,
		SkipChangesMenu: true,
	})

	return ExecuteResult{
		Output:   "Chat command executed successfully",
		Error:    "",
		ExitCode: 0,
	}
}

func (e *CLIExecutor) executeModels(args []string) ExecuteResult {
	// For now, return a simple response - we can enhance this later
	return ExecuteResult{
		Output:   "Models command - using direct Go function calls",
		Error:    "",
		ExitCode: 0,
	}
}

func (e *CLIExecutor) executePlans(args []string) ExecuteResult {
	// For now, return a simple response - we can enhance this later  
	return ExecuteResult{
		Output:   "Plans command - using direct Go function calls",
		Error:    "",
		ExitCode: 0,
	}
}
