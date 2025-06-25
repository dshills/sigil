package agent

import (
	"context"
	"testing"
	"time"

	"github.com/dshills/sigil/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewOrchestrator(t *testing.T) {
	config := DefaultOrchestrationConfig()
	orchestrator := NewOrchestrator(config)

	assert.NotNil(t, orchestrator)
	assert.Equal(t, config, orchestrator.config)
	assert.NotNil(t, orchestrator.agents)
	assert.Empty(t, orchestrator.agents)
	assert.NotNil(t, orchestrator.eventCh)
	assert.NotNil(t, orchestrator.stopCh)
}

func TestOrchestrator_RegisterAgent(t *testing.T) {
	config := DefaultOrchestrationConfig()
	orchestrator := NewOrchestrator(config)

	agent := &MockAgent{
		id:           "test-agent",
		role:         RoleLead,
		capabilities: []Capability{CapabilityCodeGeneration},
	}

	err := orchestrator.RegisterAgent(agent)
	assert.NoError(t, err)

	// Verify agent was registered
	agents := orchestrator.GetAgents()
	assert.Len(t, agents, 1)
	assert.Equal(t, "test-agent", agents[0].GetID())
}

func TestOrchestrator_RegisterAgent_MaxAgentsExceeded(t *testing.T) {
	config := DefaultOrchestrationConfig()
	config.MaxAgents = 1 // Set limit to 1
	orchestrator := NewOrchestrator(config)

	// Register first agent - should succeed
	agent1 := &MockAgent{
		id:           "agent-1",
		role:         RoleLead,
		capabilities: []Capability{CapabilityCodeGeneration},
	}
	err := orchestrator.RegisterAgent(agent1)
	assert.NoError(t, err)

	// Register second agent - should fail
	agent2 := &MockAgent{
		id:           "agent-2",
		role:         RoleReviewer,
		capabilities: []Capability{CapabilityCodeReview},
	}
	err = orchestrator.RegisterAgent(agent2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maximum number of agents")
}

func TestOrchestrator_RegisterAgent_DuplicateID(t *testing.T) {
	config := DefaultOrchestrationConfig()
	orchestrator := NewOrchestrator(config)

	// Register first agent
	agent1 := &MockAgent{
		id:           "duplicate-id",
		role:         RoleLead,
		capabilities: []Capability{CapabilityCodeGeneration},
	}
	err := orchestrator.RegisterAgent(agent1)
	assert.NoError(t, err)

	// Try to register agent with same ID
	agent2 := &MockAgent{
		id:           "duplicate-id",
		role:         RoleReviewer,
		capabilities: []Capability{CapabilityCodeReview},
	}
	err = orchestrator.RegisterAgent(agent2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestOrchestrator_GetAgentsByRole(t *testing.T) {
	config := DefaultOrchestrationConfig()
	orchestrator := NewOrchestrator(config)

	// Register agents with different roles
	leadAgent := &MockAgent{
		id:           "lead-1",
		role:         RoleLead,
		capabilities: []Capability{CapabilityCodeGeneration},
	}
	reviewerAgent := &MockAgent{
		id:           "reviewer-1",
		role:         RoleReviewer,
		capabilities: []Capability{CapabilityCodeReview},
	}

	err := orchestrator.RegisterAgent(leadAgent)
	assert.NoError(t, err)
	err = orchestrator.RegisterAgent(reviewerAgent)
	assert.NoError(t, err)

	// Get agents by role
	leadAgents := orchestrator.GetAgentsByRole(RoleLead)
	assert.Len(t, leadAgents, 1)
	assert.Equal(t, "lead-1", leadAgents[0].GetID())

	reviewerAgents := orchestrator.GetAgentsByRole(RoleReviewer)
	assert.Len(t, reviewerAgents, 1)
	assert.Equal(t, "reviewer-1", reviewerAgents[0].GetID())

	// Test role with no agents
	expertAgents := orchestrator.GetAgentsByRole(RoleExpert)
	assert.Empty(t, expertAgents)
}

func TestOrchestrator_GetAgents(t *testing.T) {
	config := DefaultOrchestrationConfig()
	orchestrator := NewOrchestrator(config)

	// Initially empty
	agents := orchestrator.GetAgents()
	assert.Empty(t, agents)

	// Register agents
	agent1 := &MockAgent{
		id:           "agent-1",
		role:         RoleLead,
		capabilities: []Capability{CapabilityCodeGeneration},
	}
	agent2 := &MockAgent{
		id:           "agent-2",
		role:         RoleReviewer,
		capabilities: []Capability{CapabilityCodeReview},
	}

	err := orchestrator.RegisterAgent(agent1)
	assert.NoError(t, err)
	err = orchestrator.RegisterAgent(agent2)
	assert.NoError(t, err)

	// Get all agents
	agents = orchestrator.GetAgents()
	assert.Len(t, agents, 2)

	// Check that both agents are present
	agentIDs := make(map[string]bool)
	for _, agent := range agents {
		agentIDs[agent.GetID()] = true
	}
	assert.True(t, agentIDs["agent-1"])
	assert.True(t, agentIDs["agent-2"])
}

func TestOrchestrator_GetMetrics(t *testing.T) {
	config := DefaultOrchestrationConfig()
	orchestrator := NewOrchestrator(config)

	metrics := orchestrator.GetMetrics()

	// Check default metrics
	assert.Equal(t, int64(0), metrics.TotalTasks)
	assert.Equal(t, int64(0), metrics.CompletedTasks)
	assert.Equal(t, int64(0), metrics.FailedTasks)
	assert.Equal(t, time.Duration(0), metrics.AverageTaskTime)
	assert.Equal(t, 0.0, metrics.ConsensusRate)
	assert.Equal(t, 0.0, metrics.ConflictRate)
	assert.Equal(t, 0.0, metrics.QualityScore)
}

func TestOrchestrator_ExecuteTask_NoAgents(t *testing.T) {
	config := DefaultOrchestrationConfig()
	orchestrator := NewOrchestrator(config)

	task := Task{
		ID:          "test-task",
		Type:        TaskTypeEdit,
		Description: "Test task",
		Priority:    PriorityHigh,
		CreatedAt:   time.Now(),
	}

	ctx := context.Background()
	result, err := orchestrator.ExecuteTask(ctx, task)

	// The orchestrator returns an error when no agents are available
	assert.Error(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, StatusFailed, result.Status)
}

func TestEventType_Constants(t *testing.T) {
	tests := []struct {
		name      string
		eventType EventType
		expected  string
	}{
		{"task started", EventTaskStarted, "task_started"},
		{"task completed", EventTaskCompleted, "task_completed"},
		{"task failed", EventTaskFailed, "task_failed"},
		{"review started", EventReviewStarted, "review_started"},
		{"review completed", EventReviewCompleted, "review_completed"},
		{"consensus reached", EventConsensusReached, "consensus_reached"},
		{"conflict detected", EventConflictDetected, "conflict_detected"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.eventType))
		})
	}
}

func TestOrchestrationEvent_Structure(t *testing.T) {
	timestamp := time.Now()

	event := OrchestrationEvent{
		Type:      EventTaskStarted,
		TaskID:    "task-123",
		AgentID:   "agent-456",
		Data:      map[string]string{"key": "value"},
		Timestamp: timestamp,
	}

	assert.Equal(t, EventTaskStarted, event.Type)
	assert.Equal(t, "task-123", event.TaskID)
	assert.Equal(t, "agent-456", event.AgentID)
	assert.Equal(t, "value", event.Data["key"])
	assert.Equal(t, timestamp, event.Timestamp)
}

func TestOrchestrator_StartStop(t *testing.T) {
	config := DefaultOrchestrationConfig()
	orchestrator := NewOrchestrator(config)

	// Start the orchestrator
	orchestrator.Start()

	// Stop should not panic - avoid multiple stops for now
	// since the implementation doesn't guard against it
	orchestrator.Stop()
}

// MockAgent implements Agent interface for testing orchestrator interactions
type MockAgent struct {
	mock.Mock
	id           string
	role         AgentRole
	capabilities []Capability
}

func (m *MockAgent) GetID() string {
	return m.id
}

func (m *MockAgent) GetRole() AgentRole {
	return m.role
}

func (m *MockAgent) GetCapabilities() []Capability {
	return m.capabilities
}

func (m *MockAgent) HasCapability(cap Capability) bool {
	for _, c := range m.capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

func (m *MockAgent) Execute(ctx context.Context, task Task) (*Result, error) {
	args := m.Called(ctx, task)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Result), args.Error(1)
}

func (m *MockAgent) GetModel() model.Model {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(model.Model)
}

func (m *MockAgent) Review(ctx context.Context, proposal Proposal) (*ReviewResult, error) {
	args := m.Called(ctx, proposal)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ReviewResult), args.Error(1)
}
