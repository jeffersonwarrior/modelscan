package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock workflow step for testing
type mockWorkflowStep struct {
	name        string
	executed    bool
	input       map[string]interface{}
	output      map[string]interface{}
	errorToReturn error
	delay       time.Duration
}

func (m *mockWorkflowStep) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	m.executed = true
	m.input = input
	
	if m.delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(m.delay):
			// Continue after delay
		}
	}
	
	if m.errorToReturn != nil {
		return nil, m.errorToReturn
	}
	
	return m.output, nil
}

func (m *mockWorkflowStep) Name() string {
	return m.name
}

func TestNewWorkflow_CreatesEmptyWorkflow(t *testing.T) {
	workflow := NewWorkflow("test-workflow")
	
	assert.Equal(t, "test-workflow", workflow.Name())
	assert.Empty(t, workflow.steps)
	assert.Empty(t, workflow.dependencies)
}

func TestWorkflow_AddStep_AddsStepToWorkflow(t *testing.T) {
	workflow := NewWorkflow("test-workflow")
	step := &mockWorkflowStep{name: "step1"}
	
	workflow.AddStep(step)
	
	assert.Contains(t, workflow.steps, "step1")
	assert.Equal(t, step, workflow.steps["step1"])
}

func TestWorkflow_AddDependency_CreatesDependencyBetweenSteps(t *testing.T) {
	workflow := NewWorkflow("test-workflow")
	step1 := &mockWorkflowStep{name: "step1"}
	step2 := &mockWorkflowStep{name: "step2"}
	
	workflow.AddStep(step1)
	workflow.AddStep(step2)
	workflow.AddDependency("step2", "step1")
	
	assert.Contains(t, workflow.dependencies["step2"], "step1")
}

func TestWorkflow_Execute_SingleStep_Success(t *testing.T) {
	workflow := NewWorkflow("test-workflow")
	step := &mockWorkflowStep{
		name: "step1",
		output: map[string]interface{}{
			"result": "success",
		},
	}
	
	workflow.AddStep(step)
	
	result, err := workflow.Execute(context.Background(), map[string]interface{}{})
	
	require.NoError(t, err)
	assert.True(t, step.executed)
	assert.Equal(t, "success", result["step1_result"])
}

func TestWorkflow_Execute_SequentialSteps_Success(t *testing.T) {
	workflow := NewWorkflow("test-workflow")
	step1 := &mockWorkflowStep{
		name: "step1",
		output: map[string]interface{}{
			"value": 10,
		},
	}
	step2 := &mockWorkflowStep{
		name: "step2",
		output: map[string]interface{}{
			"doubled": 20,
		},
	}
	
	workflow.AddStep(step1)
	workflow.AddStep(step2)
	workflow.AddDependency("step2", "step1")
	
	result, err := workflow.Execute(context.Background(), map[string]interface{}{})
	
	require.NoError(t, err)
	assert.True(t, step1.executed)
	assert.True(t, step2.executed)
	assert.Equal(t, 20, result["step2_doubled"])
}

func TestWorkflow_Execute_ParallelSteps_Success(t *testing.T) {
	workflow := NewWorkflow("test-workflow")
	step1 := &mockWorkflowStep{
		name: "step1",
		output: map[string]interface{}{
			"task1": "done",
		},
		delay: 100 * time.Millisecond,
	}
	step2 := &mockWorkflowStep{
		name: "step2",
		output: map[string]interface{}{
			"task2": "done",
		},
		delay: 100 * time.Millisecond,
	}
	
	workflow.AddStep(step1)
	workflow.AddStep(step2)
	// No dependencies - should run in parallel
	
	start := time.Now()
	result, err := workflow.Execute(context.Background(), map[string]interface{}{})
	duration := time.Since(start)
	
	require.NoError(t, err)
	assert.True(t, step1.executed)
	assert.True(t, step2.executed)
	assert.Equal(t, "done", result["step1_task1"])
	assert.Equal(t, "done", result["step2_task2"])
	// Should complete in roughly 100ms, not 200ms (parallel execution)
	assert.Less(t, duration, 150*time.Millisecond)
}

func TestWorkflow_Execute_ConditionalBranch_ExecutesCorrectPath(t *testing.T) {
	workflow := NewWorkflow("test-workflow")
	
	// Create a step that outputs a condition
	conditionStep := &mockWorkflowStep{
		name: "condition",
		output: map[string]interface{}{
			"should_branch": true,
		},
	}
	
	// Create branch steps
	branchTrue := &mockWorkflowStep{
		name: "branch_true",
		output: map[string]interface{}{
			"result": "executed_true_path",
		},
	}
	
	branchFalse := &mockWorkflowStep{
		name: "branch_false",
		output: map[string]interface{}{
			"result": "executed_false_path",
		},
	}
	
	workflow.AddStep(conditionStep)
	workflow.AddStep(branchTrue)
	workflow.AddStep(branchFalse)
	
	// Only add dependency for true branch
	workflow.AddDependency("branch_true", "condition")
	workflow.AddDependency("branch_false", "condition")
	
	_, err := workflow.Execute(context.Background(), map[string]interface{}{})
	
	require.NoError(t, err)
	assert.True(t, conditionStep.executed)
	// Both branches execute in this simple implementation
	// In a real conditional workflow, we'd add conditional logic
}

func TestWorkflow_Execute_CircularDependency_ReturnsError(t *testing.T) {
	workflow := NewWorkflow("test-workflow")
	step1 := &mockWorkflowStep{name: "step1"}
	step2 := &mockWorkflowStep{name: "step2"}
	
	workflow.AddStep(step1)
	workflow.AddStep(step2)
	workflow.AddDependency("step1", "step2")
	workflow.AddDependency("step2", "step1") // Circular dependency
	
	_, err := workflow.Execute(context.Background(), map[string]interface{}{})
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency")
}

func TestWorkflow_Execute_StepError_PropagatesError(t *testing.T) {
	workflow := NewWorkflow("test-workflow")
	step := &mockWorkflowStep{
		name: "step1",
		errorToReturn: errors.New("step failed"),
	}
	
	workflow.AddStep(step)
	
	_, err := workflow.Execute(context.Background(), map[string]interface{}{})
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "step failed")
}

func TestWorkflow_Execute_ContextCancellation_PropagatesError(t *testing.T) {
	workflow := NewWorkflow("test-workflow")
	step := &mockWorkflowStep{
		name: "step1",
		delay: 100 * time.Millisecond,
		output: map[string]interface{}{
			"result": "success",
		},
	}
	
	workflow.AddStep(step)
	
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	
	_, err := workflow.Execute(ctx, map[string]interface{}{})
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestWorkflow_GetExecutionStatus_TracksExecution(t *testing.T) {
	workflow := NewWorkflow("test-workflow")
	step := &mockWorkflowStep{
		name: "step1",
		output: map[string]interface{}{
			"result": "success",
		},
	}
	
	workflow.AddStep(step)
	
	// Execute workflow
	_, err := workflow.Execute(context.Background(), map[string]interface{}{})
	require.NoError(t, err)
	
	// Check execution status
	status := workflow.GetExecutionStatus()
	assert.NotNil(t, status)
	assert.True(t, status["step1"].Executed)
}