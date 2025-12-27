package agent

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// WorkflowStep represents a single step in a workflow
type WorkflowStep interface {
	Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)
	Name() string
}

// WorkflowStatus represents the execution status of a workflow step
type WorkflowStatus struct {
	Executed  bool
	StartTime time.Time
	EndTime   time.Time
	Error     error
	Input     map[string]interface{}
	Output    map[string]interface{}
	Duration  time.Duration
}

// Workflow represents a DAG-based workflow engine
type Workflow struct {
	name         string
	steps        map[string]WorkflowStep
	dependencies map[string][]string // step -> list of dependencies
	mu           sync.RWMutex
	status       map[string]*WorkflowStatus
}

// NewWorkflow creates a new workflow instance
func NewWorkflow(name string) *Workflow {
	return &Workflow{
		name:         name,
		steps:        make(map[string]WorkflowStep),
		dependencies: make(map[string][]string),
		status:       make(map[string]*WorkflowStatus),
	}
}

// Name returns the workflow name
func (w *Workflow) Name() string {
	return w.name
}

// AddStep adds a step to the workflow
func (w *Workflow) AddStep(step WorkflowStep) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.steps[step.Name()] = step
}

// AddDependency creates a dependency between two steps
func (w *Workflow) AddDependency(step, dependsOn string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.dependencies[step] == nil {
		w.dependencies[step] = make([]string, 0)
	}
	w.dependencies[step] = append(w.dependencies[step], dependsOn)
}

// Execute runs the workflow with the given input
func (w *Workflow) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Clear previous status
	w.status = make(map[string]*WorkflowStatus)

	// Check for circular dependencies
	if err := w.detectCircularDependencies(); err != nil {
		return nil, err
	}

	// Create execution plan
	executionPlan := w.createExecutionPlan()

	// Execute steps according to plan
	results := make(map[string]interface{})

	for _, stepNames := range executionPlan {
		// Execute steps in parallel within the same level
		var wg sync.WaitGroup
		var mu sync.Mutex
		levelResults := make(map[string]interface{})
		var levelErrors []error

		for _, stepName := range stepNames {
			wg.Add(1)
			go func(name string) {
				defer wg.Done()

				step := w.steps[name]
				stepInput := w.prepareStepInput(name, input, results)

				status := &WorkflowStatus{
					StartTime: time.Now(),
					Input:     stepInput,
					Executed:  false,
				}

				// Lock status update
				mu.Lock()
				w.status[name] = status
				mu.Unlock()

				output, err := w.executeStep(ctx, step, stepInput)
				status.EndTime = time.Now()
				status.Duration = status.EndTime.Sub(status.StartTime)
				status.Output = output
				status.Error = err

				mu.Lock()
				if err != nil {
					levelErrors = append(levelErrors, fmt.Errorf("step %s failed: %w", name, err))
				} else {
					status.Executed = true
					levelResults[name] = output
				}
				mu.Unlock()
			}(stepName)
		}

		// Wait for all steps in this level to complete
		wg.Wait()

		// If any step failed, return error
		if len(levelErrors) > 0 {
			return nil, levelErrors[0]
		}

		// Merge level results
		for name, result := range levelResults {
			// result is already a map[string]interface{} from step execution
			if resultMap, ok := result.(map[string]interface{}); ok {
				for k, v := range resultMap {
					results[fmt.Sprintf("%s_%s", name, k)] = v
				}
			} else {
				// If it's not a map, store it directly
				results[name] = result
			}
		}
	}

	return results, nil
}

// executeStep executes a single step with timeout handling
func (w *Workflow) executeStep(ctx context.Context, step WorkflowStep, input map[string]interface{}) (map[string]interface{}, error) {
	// Create a sub-context with timeout if needed
	stepCtx := ctx

	// Execute the step
	output, err := step.Execute(stepCtx, input)
	if err != nil {
		return nil, err
	}

	return output, nil
}

// prepareStepInput prepares input for a step by merging workflow input and dependent step outputs
func (w *Workflow) prepareStepInput(stepName string, workflowInput map[string]interface{}, results map[string]interface{}) map[string]interface{} {
	input := make(map[string]interface{})

	// Copy workflow input
	for k, v := range workflowInput {
		input[k] = v
	}

	// Add outputs from dependencies
	dependencies := w.dependencies[stepName]
	for _, dep := range dependencies {
		for key, value := range results {
			// Look for results that contain the dependency name
			if key == dep || fmt.Sprintf("%s_result", dep) == key {
				// If it's a map (from dependent step), merge it
				if depMap, ok := value.(map[string]interface{}); ok {
					for k, v := range depMap {
						input[fmt.Sprintf("%s_%s", dep, k)] = v
					}
				} else {
					// Otherwise add the value directly
					input[dep] = value
				}
			}
		}
	}

	return input
}

// createExecutionPlan creates a level-based execution plan from the DAG
func (w *Workflow) createExecutionPlan() [][]string {
	// Create execution levels
	inDegree := make(map[string]int)

	// Calculate in-degrees
	for name := range w.steps {
		inDegree[name] = 0
	}

	// Count dependencies for each step
	for step, deps := range w.dependencies {
		for range deps {
			inDegree[step]++
		}
	}

	// Create levels using topological sort
	var plan [][]string
	remaining := make(map[string]bool)

	// Initialize with all steps
	for name := range w.steps {
		remaining[name] = true
	}

	for len(remaining) > 0 {
		var currentLevel []string

		// Find steps with no remaining dependencies
		for name := range remaining {
			if inDegree[name] == 0 {
				currentLevel = append(currentLevel, name)
			}
		}

		if len(currentLevel) == 0 {
			// This should not happen if circular dependency check passed
			break
		}

		// Remove current level from remaining and update degrees
		for _, name := range currentLevel {
			delete(remaining, name)

			// Decrease in-degree of steps that depend on this one
			for stepName, stepDeps := range w.dependencies {
				for _, stepDep := range stepDeps {
					if stepDep == name && remaining[stepName] {
						inDegree[stepName]--
					}
				}
			}
		}

		plan = append(plan, currentLevel)
	}

	return plan
}

// detectCircularDependencies checks for circular dependencies using DFS
func (w *Workflow) detectCircularDependencies() error {
	visited := make(map[string]bool)
	recursionStack := make(map[string]bool)

	var dfs func(string) error
	dfs = func(node string) error {
		if recursionStack[node] {
			return fmt.Errorf("circular dependency detected involving step: %s", node)
		}

		if visited[node] {
			return nil
		}

		visited[node] = true
		recursionStack[node] = true

		// Visit all steps that this node depends on
		for _, dep := range w.dependencies[node] {
			if err := dfs(dep); err != nil {
				return err
			}
		}

		recursionStack[node] = false
		return nil
	}

	// Check each step
	for stepName := range w.steps {
		if !visited[stepName] {
			if err := dfs(stepName); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetExecutionStatus returns the execution status of all steps
func (w *Workflow) GetExecutionStatus() map[string]*WorkflowStatus {
	w.mu.RLock()
	defer w.mu.RUnlock()

	statusCopy := make(map[string]*WorkflowStatus)
	for k, v := range w.status {
		statusCopy[k] = v
	}

	return statusCopy
}
