package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// CLI represents the command-line interface
type CLI struct {
	rootCmd      *cobra.Command
	orchestrator *Orchestrator
	commands     map[string]Command
	initialized  bool
}

// NewCLI creates a new CLI instance
func NewCLI() *CLI {
	cli := &CLI{
		commands: make(map[string]Command),
	}

	// Create root command
	cli.rootCmd = &cobra.Command{
		Use:   "modelscan",
		Short: "ModelScan - Multi-Agent LLM Framework",
		Long: `ModelScan CLI is a powerful multi-agent orchestration system for LLM applications.
It manages agents, teams, tasks, and provides coordination capabilities.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.runOrchestrator(args)
		},
	}

	// Register built-in commands
	cli.registerBuiltinCommands()

	// Set up command line flags
	cli.setupFlags()

	return cli
}

// registerBuiltinCommands registers all built-in commands
func (cli *CLI) registerBuiltinCommands() {
	// Create command instances
	commands := []Command{
		&ListAgentsCommand{},
		&CreateAgentCommand{},
		&ListTeamsCommand{},
		&CreateTeamCommand{},
		&AddToTeamCommand{},
		&ListTasksCommand{},
		&StatusCommand{},
		&CleanupCommand{},
	}

	// Register commands
	for _, cmd := range commands {
		cli.registerCommand(cmd)
	}

	// Register help command
	cli.registerCommand(NewHelpCommand(cli.commands))
}

// registerCommand registers a command with the CLI
func (cli *CLI) registerCommand(cmd Command) {
	// Store command
	cli.commands[cmd.Name()] = cmd

	// Create cobra command
	cobraCmd := &cobra.Command{
		Use:   cmd.Name(),
		Short: cmd.Description(),
		Args:  cobra.ArbitraryArgs,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			if !cli.initialized {
				if err := cli.initializeOrchestrator(); err != nil {
					return err
				}
			}

			return cmd.Execute(context.Background(), cli.orchestrator, args)
		},
	}

	// Add to root command
	cli.rootCmd.AddCommand(cobraCmd)
}

// setupFlags sets up command line flags
func (cli *CLI) setupFlags() {
	cli.rootCmd.PersistentFlags().String("database", "", "Database file path")
	cli.rootCmd.PersistentFlags().String("log-level", "info", "Log level (debug, info, warn, error)")
	cli.rootCmd.PersistentFlags().Duration("retention", 0, "Data retention period")
	cli.rootCmd.PersistentFlags().Int("max-concurrency", 10, "Maximum concurrent operations")
}

// initializeOrchestrator initializes the orchestrator
func (cli *CLI) initializeOrchestrator() error {
	if cli.initialized {
		return nil
	}

	// Load configuration
	config := cli.loadConfig()

	// Create orchestrator
	orchestrator, err := NewOrchestrator(config)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	// Start orchestrator
	if err := orchestrator.Start(); err != nil {
		return fmt.Errorf("failed to start orchestrator: %w", err)
	}

	cli.orchestrator = orchestrator
	cli.initialized = true

	return nil
}

// loadConfig loads configuration from flags and defaults
func (cli *CLI) loadConfig() *Config {
	config := DefaultConfig()

	// Override with flags
	if dbPath, err := cli.rootCmd.PersistentFlags().GetString("database"); err == nil && dbPath != "" {
		config.DatabasePath = dbPath
	}

	if logLevel, err := cli.rootCmd.PersistentFlags().GetString("log-level"); err == nil && logLevel != "" {
		config.LogLevel = logLevel
	}

	if retention, err := cli.rootCmd.PersistentFlags().GetDuration("retention"); err == nil && retention > 0 {
		config.DataRetention = retention
	}

	if maxConcurrency, err := cli.rootCmd.PersistentFlags().GetInt("max-concurrency"); err == nil && maxConcurrency > 0 {
		config.MaxConcurrency = maxConcurrency
	}

	return config
}

// runOrchestrator runs the orchestrator in standalone mode
func (cli *CLI) runOrchestrator(args []string) error {
	if !cli.initialized {
		if err := cli.initializeOrchestrator(); err != nil {
			return err
		}
	}

	if len(args) == 0 {
		// Show status if no subcommand provided
		return cli.commands["status"].Execute(context.Background(), cli.orchestrator, []string{})
	}

	// Try to execute as command line
	input := strings.Join(args, " ")
	return cli.executeCommandLine(input)
}

// executeCommandLine executes a command line string
func (cli *CLI) executeCommandLine(input string) error {
	if !cli.initialized {
		return fmt.Errorf("orchestrator not initialized")
	}

	// Parse input
	parts := strings.Fields(strings.TrimSpace(input))
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}

	cmdName := parts[0]
	args := parts[1:]

	// Find command
	cmd, exists := cli.commands[cmdName]
	if !exists {
		return fmt.Errorf("unknown command: %s", cmdName)
	}

	// Execute command
	return cmd.Execute(context.Background(), cli.orchestrator, args)
}

// Execute executes the CLI
func (cli *CLI) Execute() error {
	return cli.rootCmd.Execute()
}

// Shutdown shuts down the CLI
func (cli *CLI) Shutdown() error {
	if cli.orchestrator != nil {
		return cli.orchestrator.Stop()
	}
	return nil
}

// IsInitialized returns true if the orchestrator is initialized
func (cli *CLI) IsInitialized() bool {
	return cli.initialized
}

// GetOrchestrator returns the orchestrator instance
func (cli *CLI) GetOrchestrator() *Orchestrator {
	return cli.orchestrator
}

// AddCommand adds a new command to the CLI
func (cli *CLI) AddCommand(cmd Command) {
	cli.registerCommand(cmd)
}

// GetCommands returns all registered commands
func (cli *CLI) GetCommands() map[string]Command {
	result := make(map[string]Command)
	for k, v := range cli.commands {
		result[k] = v
	}
	return result
}
