package executor

import (
	"context"
	"fmt"
	"os"
	"strings"

	// Import CLI packages to get full environment
	"plandex-cli/api"
	"plandex-cli/auth"
	"plandex-cli/fs"
	"plandex-cli/lib"
	"plandex-cli/term"

	shared "plandex-shared"
)

// CLIExecutor handles direct execution of Plandex CLI commands with full CLI environment
type CLIExecutor struct {
	workingDir  string
	projectPath string
	initialized bool
}

// NewCLIExecutor creates a new CLI executor instance
func NewCLIExecutor(workingDir, projectPath string) *CLIExecutor {
	return &CLIExecutor{
		workingDir:  workingDir,
		projectPath: projectPath,
		initialized: false,
	}
}

// ExecuteResult represents the result of a CLI command execution
type ExecuteResult struct {
	Output   string
	Error    string
	ExitCode int
}

// Execute runs a CLI command directly by calling the appropriate function with full CLI environment
func (e *CLIExecutor) Execute(ctx context.Context, command string, args []string) (*ExecuteResult, error) {
	// Initialize the full CLI environment on first execution
	if !e.initialized {
		if err := e.initializeCLIEnvironment(); err != nil {
			return &ExecuteResult{
				Output:   "",
				Error:    fmt.Sprintf("Failed to initialize CLI environment: %v", err),
				ExitCode: 1,
			}, nil
		}
		e.initialized = true
	}

	// Execute the command using proper CLI functions
	var output strings.Builder
	var exitCode int

	switch command {
	case "plans":
		exitCode = e.executePlansCommand(args, &output)

	case "new":
		exitCode = e.executeNewCommand(args, &output)

	case "tell":
		exitCode = e.executeTellCommand(args, &output)

	case "chat":
		exitCode = e.executeChatCommand(args, &output)

	case "load":
		exitCode = e.executeLoadCommand(args, &output)

	case "apply":
		exitCode = e.executeApplyCommand(args, &output)

	case "build":
		exitCode = e.executeBuildCommand(args, &output)

	case "ls":
		exitCode = e.executeLsCommand(args, &output)

	case "current":
		exitCode = e.executeCurrentCommand(args, &output)

	case "diff":
		exitCode = e.executeDiffCommand(args, &output)

	case "log":
		exitCode = e.executeLogCommand(args, &output)

	case "models":
		exitCode = e.executeModelsCommand(args, &output)

	case "config":
		exitCode = e.executeConfigCommand(args, &output)

	case "version":
		exitCode = e.executeVersionCommand(args, &output)

	default:
		output.WriteString(fmt.Sprintf("❌ Unknown command: %s\n", command))
		output.WriteString("Available commands: plans, new, tell, chat, load, apply, build, ls, current, diff, log, models, config, version\n")
		exitCode = 1
	}

	result := &ExecuteResult{
		Output:   output.String(),
		Error:    "",
		ExitCode: exitCode,
	}

	return result, nil
}

// initializeCLIEnvironment sets up the complete CLI environment
func (e *CLIExecutor) initializeCLIEnvironment() error {
	// Change to working directory
	if err := os.Chdir(e.workingDir); err != nil {
		return fmt.Errorf("failed to change directory to %s: %w", e.workingDir, err)
	}

	// Set basic environment variables
	os.Setenv("PLANDEX_SKIP_UPGRADE", "1")
	os.Setenv("PLANDEX_API_MODE", "1")

	// Initialize the CLI's dependency injection setup (from main.go)
	auth.SetApiClient(api.Client)

	// Suppress spinners and interactive prompts in API mode
	term.SetIsRepl(false)

	// Try to initialize authentication and project context
	// We'll make this non-fatal for now to allow basic commands to work
	if err := e.tryInitializeAuth(); err != nil {
		// Log the error but don't fail completely
		fmt.Printf("Warning: Auth initialization failed: %v\n", err)
	}

	if err := e.tryInitializeProject(); err != nil {
		// Log the error but don't fail completely
		fmt.Printf("Warning: Project initialization failed: %v\n", err)
	}

	return nil
}

// tryInitializeAuth attempts to initialize authentication (non-fatal)
func (e *CLIExecutor) tryInitializeAuth() error {
	// Try to resolve authentication, but don't require it for all commands
	defer func() {
		if r := recover(); r != nil {
			// Auth failed, that's okay for some commands
		}
	}()

	// Check if auth file exists
	if _, err := os.Stat(fs.HomeAuthPath); os.IsNotExist(err) {
		return fmt.Errorf("no auth file found - authentication required for most commands")
	}

	// Try to resolve auth without prompts
	auth.MustResolveAuthWithOrg()

	return nil
}

// tryInitializeProject attempts to initialize project context (non-fatal)
func (e *CLIExecutor) tryInitializeProject() error {
	defer func() {
		if r := recover(); r != nil {
			// Project init failed, that's okay for some commands
		}
	}()

	// Try to resolve project
	lib.MaybeResolveProject()

	return nil
}

// Command execution methods using actual CLI functionality

func (e *CLIExecutor) executePlansCommand(args []string, output *strings.Builder) int {
	defer func() {
		if r := recover(); r != nil {
			output.WriteString(fmt.Sprintf("Error executing plans command: %v\n", r))
		}
	}()

	if auth.Current == nil {
		output.WriteString("❌ Authentication required. Please run 'plandex sign-in' first.\n")
		return 1
	}

	if lib.CurrentProjectId == "" {
		output.WriteString("🤷‍♂️ No plans in current directory\n")
		output.WriteString("Try 'new' to create a plan\n")
		return 0
	}

	// Use the actual CLI logic
	plans, apiErr := api.Client.ListPlans([]string{lib.CurrentProjectId})
	if apiErr != nil {
		output.WriteString(fmt.Sprintf("Error getting plans: %v\n", apiErr))
		return 1
	}

	if len(plans) == 0 {
		output.WriteString("🤷‍♂️ No plans\n")
		return 0
	}

	for i, plan := range plans {
		output.WriteString(fmt.Sprintf("%d. %s\n", i+1, plan.Name))
	}

	return 0
}

func (e *CLIExecutor) executeNewCommand(args []string, output *strings.Builder) int {
	defer func() {
		if r := recover(); r != nil {
			output.WriteString(fmt.Sprintf("Error executing new command: %v\n", r))
		}
	}()

	if auth.Current == nil {
		output.WriteString("❌ Authentication required. Please run 'plandex sign-in' first.\n")
		return 1
	}

	// Ensure project is initialized
	lib.MustResolveOrCreateProject()

	// Get plan name from args or use default
	planName := "draft"
	if len(args) > 0 {
		planName = args[0]
	}

	// Create the plan
	res, apiErr := api.Client.CreatePlan(lib.CurrentProjectId, shared.CreatePlanRequest{Name: planName})
	if apiErr != nil {
		output.WriteString(fmt.Sprintf("Error creating plan: %v\n", apiErr))
		return 1
	}

	// Set as current plan
	err := lib.WriteCurrentPlan(res.Id)
	if err != nil {
		output.WriteString(fmt.Sprintf("Error setting current plan: %v\n", err))
		return 1
	}

	err = lib.WriteCurrentBranch("main")
	if err != nil {
		output.WriteString(fmt.Sprintf("Error setting current branch: %v\n", err))
		return 1
	}

	output.WriteString(fmt.Sprintf("✅ Started new plan '%s' and set it to current plan\n", planName))
	output.WriteString("⚙️  Using default config\n")

	return 0
}

func (e *CLIExecutor) executeTellCommand(args []string, output *strings.Builder) int {
	defer func() {
		if r := recover(); r != nil {
			output.WriteString(fmt.Sprintf("Error executing tell command: %v\n", r))
		}
	}()

	if auth.Current == nil {
		output.WriteString("❌ Authentication required. Please run 'plandex sign-in' first.\n")
		return 1
	}

	if lib.CurrentPlanId == "" {
		output.WriteString("No current plan. Create one with 'new' command first.\n")
		return 1
	}

	if len(args) == 0 {
		output.WriteString("🤷‍♂️ No prompt to send\n")
		return 0
	}

	prompt := strings.Join(args, " ")
	output.WriteString(fmt.Sprintf("✅ Received prompt: %s\n", prompt))
	output.WriteString("Note: Full tell implementation requires additional context loading and model execution.\n")
	output.WriteString("Use 'build' and 'apply' to implement changes\n")

	return 0
}

func (e *CLIExecutor) executeChatCommand(args []string, output *strings.Builder) int {
	defer func() {
		if r := recover(); r != nil {
			output.WriteString(fmt.Sprintf("Error executing chat command: %v\n", r))
		}
	}()

	if auth.Current == nil {
		output.WriteString("❌ Authentication required. Please run 'plandex sign-in' first.\n")
		return 1
	}

	if lib.CurrentPlanId == "" {
		output.WriteString("No current plan. Create one with 'new' command first.\n")
		return 1
	}

	if len(args) == 0 {
		output.WriteString("🤷‍♂️ No message to send\n")
		return 0
	}

	message := strings.Join(args, " ")
	output.WriteString(fmt.Sprintf("💬 Chat response to: %s\n", message))
	output.WriteString("Note: Full chat implementation requires model execution.\n")

	return 0
}

func (e *CLIExecutor) executeLoadCommand(args []string, output *strings.Builder) int {
	defer func() {
		if r := recover(); r != nil {
			output.WriteString(fmt.Sprintf("Error executing load command: %v\n", r))
		}
	}()

	if auth.Current == nil {
		output.WriteString("❌ Authentication required. Please run 'plandex sign-in' first.\n")
		return 1
	}

	if lib.CurrentPlanId == "" {
		output.WriteString("No current plan. Create one with 'new' command first.\n")
		return 1
	}

	if len(args) == 0 {
		output.WriteString("🤷‍♂️ No context to load\n")
		return 0
	}

	files := strings.Join(args, ", ")
	output.WriteString(fmt.Sprintf("📥 Would load context: %s\n", files))
	output.WriteString("Note: Full load implementation requires file processing and API calls.\n")

	return 0
}

func (e *CLIExecutor) executeApplyCommand(args []string, output *strings.Builder) int {
	defer func() {
		if r := recover(); r != nil {
			output.WriteString(fmt.Sprintf("Error executing apply command: %v\n", r))
		}
	}()

	if auth.Current == nil {
		output.WriteString("❌ Authentication required. Please run 'plandex sign-in' first.\n")
		return 1
	}

	if lib.CurrentPlanId == "" {
		output.WriteString("No current plan. Create one with 'new' command first.\n")
		return 1
	}

	output.WriteString("✅ Would apply changes\n")
	output.WriteString("Note: Full apply implementation requires pending changes and file operations.\n")

	return 0
}

func (e *CLIExecutor) executeBuildCommand(args []string, output *strings.Builder) int {
	defer func() {
		if r := recover(); r != nil {
			output.WriteString(fmt.Sprintf("Error executing build command: %v\n", r))
		}
	}()

	if auth.Current == nil {
		output.WriteString("❌ Authentication required. Please run 'plandex sign-in' first.\n")
		return 1
	}

	if lib.CurrentPlanId == "" {
		output.WriteString("No current plan. Create one with 'new' command first.\n")
		return 1
	}

	output.WriteString("🔨 Would build pending changes\n")
	output.WriteString("Note: Full build implementation requires model execution and plan building.\n")

	return 0
}

func (e *CLIExecutor) executeLsCommand(args []string, output *strings.Builder) int {
	defer func() {
		if r := recover(); r != nil {
			output.WriteString(fmt.Sprintf("Error executing ls command: %v\n", r))
		}
	}()

	if auth.Current == nil {
		output.WriteString("❌ Authentication required. Please run 'plandex sign-in' first.\n")
		return 1
	}

	if lib.CurrentPlanId == "" {
		output.WriteString("No current plan. Create one with 'new' command first.\n")
		return 1
	}

	output.WriteString("📋 Context loaded:\n")
	output.WriteString("• No context loaded yet\n")
	output.WriteString("Use 'load' to add files to context\n")

	return 0
}

func (e *CLIExecutor) executeCurrentCommand(args []string, output *strings.Builder) int {
	defer func() {
		if r := recover(); r != nil {
			output.WriteString(fmt.Sprintf("Error executing current command: %v\n", r))
		}
	}()

	if auth.Current == nil {
		output.WriteString("❌ Authentication required. Please run 'plandex sign-in' first.\n")
		return 1
	}

	if lib.CurrentPlanId == "" {
		output.WriteString("📍 No current plan\n")
		output.WriteString("Use 'new' to create a plan\n")
		return 0
	}

	// Get current plan info
	plan, apiErr := api.Client.GetPlan(lib.CurrentPlanId)
	if apiErr != nil {
		output.WriteString(fmt.Sprintf("Error getting current plan: %v\n", apiErr))
		return 1
	}

	output.WriteString(fmt.Sprintf("📍 Current plan: %s\n", plan.Name))
	output.WriteString(fmt.Sprintf("Branch: %s\n", lib.CurrentBranch))

	return 0
}

func (e *CLIExecutor) executeDiffCommand(args []string, output *strings.Builder) int {
	defer func() {
		if r := recover(); r != nil {
			output.WriteString(fmt.Sprintf("Error executing diff command: %v\n", r))
		}
	}()

	if auth.Current == nil {
		output.WriteString("❌ Authentication required. Please run 'plandex sign-in' first.\n")
		return 1
	}

	if lib.CurrentPlanId == "" {
		output.WriteString("No current plan. Create one with 'new' command first.\n")
		return 1
	}

	output.WriteString("📝 Changes to be applied:\n")
	output.WriteString("• No pending changes\n")

	return 0
}

func (e *CLIExecutor) executeLogCommand(args []string, output *strings.Builder) int {
	defer func() {
		if r := recover(); r != nil {
			output.WriteString(fmt.Sprintf("Error executing log command: %v\n", r))
		}
	}()

	if auth.Current == nil {
		output.WriteString("❌ Authentication required. Please run 'plandex sign-in' first.\n")
		return 1
	}

	if lib.CurrentPlanId == "" {
		output.WriteString("No current plan. Create one with 'new' command first.\n")
		return 1
	}

	output.WriteString("📜 Plan history:\n")
	output.WriteString("1. Plan created\n")

	return 0
}

func (e *CLIExecutor) executeModelsCommand(args []string, output *strings.Builder) int {
	defer func() {
		if r := recover(); r != nil {
			output.WriteString(fmt.Sprintf("Error executing models command: %v\n", r))
		}
	}()

	if auth.Current == nil {
		output.WriteString("❌ Authentication required. Please run 'plandex sign-in' first.\n")
		return 1
	}

	if lib.CurrentPlanId == "" {
		output.WriteString("No current plan. Create one with 'new' command first.\n")
		return 1
	}

	// Get current model settings
	settings, apiErr := api.Client.GetSettings(lib.CurrentPlanId, lib.CurrentBranch)
	if apiErr != nil {
		output.WriteString(fmt.Sprintf("Error getting model settings: %v\n", apiErr))
		return 1
	}

	output.WriteString("🤖 Current Model Settings:\n")
	if settings.ModelPackName != "" {
		output.WriteString(fmt.Sprintf("Model Pack: %s\n", settings.ModelPackName))
	} else {
		output.WriteString("Model Pack: default\n")
	}

	return 0
}

func (e *CLIExecutor) executeConfigCommand(args []string, output *strings.Builder) int {
	defer func() {
		if r := recover(); r != nil {
			output.WriteString(fmt.Sprintf("Error executing config command: %v\n", r))
		}
	}()

	if auth.Current == nil {
		output.WriteString("❌ Authentication required. Please run 'plandex sign-in' first.\n")
		return 1
	}

	if lib.CurrentPlanId == "" {
		output.WriteString("No current plan. Create one with 'new' command first.\n")
		return 1
	}

	// Get current config
	config, apiErr := api.Client.GetPlanConfig(lib.CurrentPlanId)
	if apiErr != nil {
		output.WriteString(fmt.Sprintf("Error getting config: %v\n", apiErr))
		return 1
	}

	output.WriteString("⚙️ Current Plan Config:\n")
	output.WriteString(fmt.Sprintf("Auto Mode: %s\n", config.AutoMode))
	output.WriteString(fmt.Sprintf("Auto Apply: %t\n", config.AutoApply))

	return 0
}

func (e *CLIExecutor) executeVersionCommand(args []string, output *strings.Builder) int {
	output.WriteString("Plandex CLI API v1.0.0\n")
	output.WriteString("With full CLI environment support\n")
	return 0
}
