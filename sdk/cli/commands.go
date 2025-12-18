package cli

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
)

// Command interface for CLI commands
type Command interface {
	Execute(ctx context.Context, orchestrator *Orchestrator, args []string) error
	Name() string
	Description() string
	Usage() string
}

// ListAgentsCommand lists all agents
type ListAgentsCommand struct{}

func (c *ListAgentsCommand) Name() string { return "list-agents" }
func (c *ListAgentsCommand) Description() string { return "List all registered agents" }
func (c *ListAgentsCommand) Usage() string { return "list-agents" }

func (c *ListAgentsCommand) Execute(ctx context.Context, orchestrator *Orchestrator, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("too many arguments")
	}

	orchestrator.mu.RLock()
	defer orchestrator.mu.RUnlock()

	if len(orchestrator.agents) == 0 {
		fmt.Println("No agents registered")
		return nil
	}

	fmt.Printf("Registered Agents (%d):\n", len(orchestrator.agents))
	fmt.Printf("%-36s %-20s %-15s %-30s %-15s\n", "ID", "Name", "Type", "Capabilities", "Status")
	fmt.Println(strings.Repeat("-", 120))

	for _, agent := range orchestrator.agents {
		capabilities := strings.Join(agent.Capabilities, ", ")
		if len(capabilities) > 28 {
			capabilities = capabilities[:28] + ".."
		}
		
		fmt.Printf("%-36s %-20s %-15s %-30s %-15s\n", 
			agent.ID, agent.Name, agent.Type, capabilities, agent.Status)
	}

	return nil
}

// CreateAgentCommand creates a new agent
type CreateAgentCommand struct{}

func (c *CreateAgentCommand) Name() string { return "create-agent" }
func (c *CreateAgentCommand) Description() string { return "Create a new agent" }
func (c *CreateAgentCommand) Usage() string { return "create-agent <name> <type> [capabilities...]" }

func (c *CreateAgentCommand) Execute(ctx context.Context, orchestrator *Orchestrator, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: %s", c.Usage())
	}

	name := args[0]
	agentType := args[1]
	capabilities := []string{}
	if len(args) > 2 {
		capabilities = args[2:]
	}

	// Create agent in storage
	dbAgent := orchestrator.storage.NewAgentWithDefaults(name, agentType, capabilities)
	dbAgent.Status = "idle"

	if err := orchestrator.storage.Agents.Create(ctx, dbAgent); err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// Create in-memory agent
	agent := &Agent{
		ID:           dbAgent.ID,
		Name:         dbAgent.Name,
		Type:         agentType,
		Capabilities: capabilities,
		Config:       map[string]interface{}{"type": agentType, "version": "1.0"},
		Status:       "idle",
		LastSeen:     time.Now(),
	}

	orchestrator.mu.Lock()
	orchestrator.agents[agent.ID] = agent
	orchestrator.mu.Unlock()

	log.Printf("Created agent: %s (%s)", agent.Name, agent.ID)
	fmt.Printf("✓ Created agent: %s (ID: %s)\n", agent.Name, agent.ID)
	fmt.Printf("  Type: %s\n", agent.Type)
	fmt.Printf("  Capabilities: %s\n", strings.Join(capabilities, ", "))

	return nil
}

// ListTeamsCommand lists all teams
type ListTeamsCommand struct{}

func (c *ListTeamsCommand) Name() string { return "list-teams" }
func (c *ListTeamsCommand) Description() string { return "List all teams" }
func (c *ListTeamsCommand) Usage() string { return "list-teams" }

func (c *ListTeamsCommand) Execute(ctx context.Context, orchestrator *Orchestrator, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("too many arguments")
	}

	orchestrator.mu.RLock()
	defer orchestrator.mu.RUnlock()

	if len(orchestrator.teams) == 0 {
		fmt.Println("No teams created")
		return nil
	}

	fmt.Printf("Teams (%d):\n", len(orchestrator.teams))
	fmt.Printf("%-36s %-20s %-15s %-30s\n", "ID", "Name", "Description", "Agents")
	fmt.Println(strings.Repeat("-", 105))

	for _, team := range orchestrator.teams {
		agentsCount := fmt.Sprintf("%d agents", len(team.Agents))
		if len(team.Description) > 28 {
			team.Description = team.Description[:28] + ".."
		}
		
		fmt.Printf("%-36s %-20s %-15s %-30s\n", 
			team.ID, team.Name, team.Description, agentsCount)
	}

	return nil
}

// CreateTeamCommand creates a new team
type CreateTeamCommand struct{}

func (c *CreateTeamCommand) Name() string { return "create-team" }
func (c *CreateTeamCommand) Description() string { return "Create a new team" }
func (c *CreateTeamCommand) Usage() string { return "create-team <name> [description]" }

func (c *CreateTeamCommand) Execute(ctx context.Context, orchestrator *Orchestrator, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: %s", c.Usage())
	}

	name := args[0]
	description := ""
	if len(args) > 1 {
		description = strings.Join(args[1:], " ")
	}

	// Create team in storage
	dbTeam := orchestrator.storage.NewTeamWithDefaults(name, description)

	if err := orchestrator.storage.Teams.Create(ctx, dbTeam); err != nil {
		return fmt.Errorf("failed to create team: %w", err)
	}

	// Create in-memory team
	team := &Team{
		ID:          dbTeam.ID,
		Name:        dbTeam.Name,
		Description: dbTeam.Description,
		Agents:      []string{},
		Config:      map[string]interface{}{"version": "1.0"},
		Metadata:    map[string]interface{}{},
	}

	orchestrator.mu.Lock()
	orchestrator.teams[team.ID] = team
	orchestrator.mu.Unlock()

	log.Printf("Created team: %s (%s)", team.Name, team.ID)
	fmt.Printf("✓ Created team: %s (ID: %s)\n", team.Name, team.ID)
	if description != "" {
		fmt.Printf("  Description: %s\n", description)
	}

	return nil
}

// AddToTeamCommand adds agents to teams
type AddToTeamCommand struct{}

func (c *AddToTeamCommand) Name() string { return "add-to-team" }
func (c *AddToTeamCommand) Description() string { return "Add agents to a team" }
func (c *AddToTeamCommand) Usage() string { return "add-to-team <team-id> <agent-id> [role]" }

func (c *AddToTeamCommand) Execute(ctx context.Context, orchestrator *Orchestrator, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: %s", c.Usage())
	}

	teamID := args[0]
	agentID := args[1]
	role := "member"
	if len(args) > 2 {
		role = args[2]
	}

	// Check if team exists
	orchestrator.mu.RLock()
	team, teamExists := orchestrator.teams[teamID]
	agent, agentExists := orchestrator.agents[agentID]
	orchestrator.mu.RUnlock()

	if !teamExists {
		return fmt.Errorf("team not found: %s", teamID)
	}

	if !agentExists {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	// Add agent to team in storage
	if err := orchestrator.storage.Teams.AddMember(ctx, teamID, agentID, role); err != nil {
		return fmt.Errorf("failed to add agent to team: %w", err)
	}

	// Update in-memory team
	orchestrator.mu.Lock()
	team.Agents = append(team.Agents, agentID)
	orchestrator.mu.Unlock()

	log.Printf("Added agent %s to team %s with role %s", agentID, teamID, role)
	fmt.Printf("✓ Added agent %s to team %s with role %s\n", agent.Name, team.Name, role)

	return nil
}

// ListTasksCommand lists all tasks
type ListTasksCommand struct{}

func (c *ListTasksCommand) Name() string { return "list-tasks" }
func (c *ListTasksCommand) Description() string { return "List all tasks" }
func (c *ListTasksCommand) Usage() string { return "list-tasks [status]" }

func (c *ListTasksCommand) Execute(ctx context.Context, orchestrator *Orchestrator, args []string) error {
	statusFilter := ""
	if len(args) > 0 {
		statusFilter = args[0]
	}

	orchestrator.mu.RLock()
	defer orchestrator.mu.RUnlock()

	var filteredTasks []*Task
	for _, task := range orchestrator.tasks {
		if statusFilter == "" || task.Status == statusFilter {
			filteredTasks = append(filteredTasks, task)
		}
	}

	if len(filteredTasks) == 0 {
		if statusFilter != "" {
			fmt.Printf("No tasks with status: %s\n", statusFilter)
		} else {
			fmt.Println("No tasks found")
		}
		return nil
	}

	if statusFilter != "" {
		fmt.Printf("Tasks with status '%s' (%d):\n", statusFilter, len(filteredTasks))
	} else {
		fmt.Printf("All Tasks (%d):\n", len(filteredTasks))
	}
	
	fmt.Printf("%-36s %-15s %-15s %-20s %-15s\n", "ID", "Type", "Status", "Agent", "Created")
	fmt.Println(strings.Repeat("-", 105))

	for _, task := range filteredTasks {
		agentName := task.AgentID
		if agent, exists := orchestrator.agents[task.AgentID]; exists {
			agentName = agent.Name
		}
		
		createdTime := task.CreatedAt.Format("2006-01-02 15:04")
		
		fmt.Printf("%-36s %-15s %-15s %-20s %-15s\n", 
			task.ID, task.Type, task.Status, agentName, createdTime)
	}

	return nil
}

// StatusCommand shows system status
type StatusCommand struct{}

func (c *StatusCommand) Name() string { return "status" }
func (c *StatusCommand) Description() string { return "Show system status" }
func (c *StatusCommand) Usage() string { return "status" }

func (c *StatusCommand) Execute(ctx context.Context, orchestrator *Orchestrator, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("too many arguments")
	}

	// Get storage stats
	stats, err := orchestrator.storage.GetStorageStats(ctx)
	if err != nil {
		return fmt.Errorf("failed to get storage stats: %w", err)
	}

	// Get in-memory counts
	orchestrator.mu.RLock()
	activeAgents := 0
	idleAgents := 0
	for _, agent := range orchestrator.agents {
		if agent.Status == "active" {
			activeAgents++
		} else {
			idleAgents++
		}
	}
	
	pendingTasks := 0
	runningTasks := 0
	for _, task := range orchestrator.tasks {
		if task.Status == "pending" {
			pendingTasks++
		} else if task.Status == "running" {
			runningTasks++
		}
	}
	orchestrator.mu.RUnlock()

	fmt.Println("ModelScan CLI Status")
	fmt.Println("====================")
	fmt.Printf("System Status: %s\n", func() string {
		if orchestrator.IsRunning() {
			return "Running ✓"
		}
		return "Stopped ✗"
	}())
	fmt.Printf("Database: %s\n", orchestrator.config.DatabasePath)
	fmt.Printf("Log Level: %s\n", orchestrator.config.LogLevel)
	fmt.Printf("Data Retention: %v\n", orchestrator.config.DataRetention)

	fmt.Println("\nStorage Statistics:")
	fmt.Printf("  Total Agents: %v\n", stats["agents"])
	fmt.Printf("  Total Tasks: %v\n", stats["tasks"])
	fmt.Printf("  Total Messages: %v\n", stats["messages"])
	fmt.Printf("  Total Teams: %v\n", stats["teams"])
	fmt.Printf("  Tool Executions: %v\n", stats["tool_executions"])
	if dbSize, ok := stats["database_size_bytes"].(int64); ok && dbSize > 0 {
		fmt.Printf("  Database Size: %.2f MB\n", float64(dbSize)/(1024*1024))
	}

	fmt.Println("\nRuntime Status:")
	fmt.Printf("  Active Agents: %d\n", activeAgents)
	fmt.Printf("  Idle Agents: %d\n", idleAgents)
	fmt.Printf("  Pending Tasks: %d\n", pendingTasks)
	fmt.Printf("  Running Tasks: %d\n", runningTasks)

	return nil
}

// CleanupCommand performs cleanup
type CleanupCommand struct{}

func (c *CleanupCommand) Name() string { return "cleanup" }
func (c *CleanupCommand) Description() string { return "Perform cleanup of old data" }
func (c *CleanupCommand) Usage() string { return "cleanup" }

func (c *CleanupCommand) Execute(ctx context.Context, orchestrator *Orchestrator, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("too many arguments")
	}

	fmt.Println("Performing cleanup...")
	
	if err := orchestrator.storage.CleanupOldData(ctx); err != nil {
		return fmt.Errorf("cleanup failed: %w", err)
	}

	fmt.Println("✓ Cleanup completed successfully")
	return nil
}

// HelpCommand shows help
type HelpCommand struct {
	commands map[string]Command
}

func (c *HelpCommand) Name() string { return "help" }
func (c *HelpCommand) Description() string { return "Show help" }
func (c *HelpCommand) Usage() string { return "help [command]" }

func (c *HelpCommand) Execute(ctx context.Context, orchestrator *Orchestrator, args []string) error {
	if len(args) == 0 {
		// Show all commands in sorted order
		fmt.Println("ModelScan CLI Commands")
		fmt.Println("=======================")
		fmt.Println()
		
		// Get sorted command names
		commandNames := make([]string, 0, len(c.commands))
		for name := range c.commands {
			commandNames = append(commandNames, name)
		}
		sort.Strings(commandNames)
		
		for _, name := range commandNames {
			cmd := c.commands[name]
			fmt.Printf("%-20s %s\n", cmd.Name(), cmd.Description())
		}
		
		fmt.Println()
		fmt.Println("Use 'help <command>' for detailed usage information")
		
		return nil
	}

	// Show specific command help
	cmdName := args[0]
	cmd, exists := c.commands[cmdName]
	if !exists {
		return fmt.Errorf("unknown command: %s", cmdName)
	}

	fmt.Printf("Command: %s\n", cmd.Name())
	fmt.Printf("Description: %s\n", cmd.Description())
	fmt.Printf("Usage: %s\n", cmd.Usage())
	
	return nil
}

func NewHelpCommand(commands map[string]Command) *HelpCommand {
	return &HelpCommand{commands: commands}
}