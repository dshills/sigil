// Package agent provides orchestration for multi-agent systems
package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dshills/sigil/internal/errors"
	"github.com/dshills/sigil/internal/logger"
)

// DefaultOrchestrator implements the Orchestrator interface
type DefaultOrchestrator struct {
	agents  map[string]Agent
	config  OrchestrationConfig
	metrics OrchestrationMetrics
	mu      sync.RWMutex
	eventCh chan OrchestrationEvent
	stopCh  chan struct{}
}

// OrchestrationEvent represents events in the orchestration process
type OrchestrationEvent struct {
	Type      EventType         `json:"type"`
	TaskID    string            `json:"task_id,omitempty"`
	AgentID   string            `json:"agent_id,omitempty"`
	Data      map[string]string `json:"data,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// EventType for orchestration events
type EventType string

const (
	EventTaskStarted      EventType = "task_started"
	EventTaskCompleted    EventType = "task_completed"
	EventTaskFailed       EventType = "task_failed"
	EventReviewStarted    EventType = "review_started"
	EventReviewCompleted  EventType = "review_completed"
	EventConsensusReached EventType = "consensus_reached"
	EventConflictDetected EventType = "conflict_detected"
)

// NewOrchestrator creates a new orchestrator
func NewOrchestrator(config OrchestrationConfig) *DefaultOrchestrator {
	return &DefaultOrchestrator{
		agents: make(map[string]Agent),
		config: config,
		metrics: OrchestrationMetrics{
			TotalTasks:       0,
			CompletedTasks:   0,
			FailedTasks:      0,
			AgentUtilization: make(map[string]float64),
			LastUpdated:      time.Now(),
		},
		eventCh: make(chan OrchestrationEvent, 100),
		stopCh:  make(chan struct{}),
	}
}

// RegisterAgent adds an agent to the orchestrator
func (o *DefaultOrchestrator) RegisterAgent(agent Agent) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if len(o.agents) >= o.config.MaxAgents {
		return errors.New(errors.ErrorTypeConfig, "RegisterAgent",
			fmt.Sprintf("maximum number of agents (%d) reached", o.config.MaxAgents))
	}

	agentID := agent.GetID()
	if _, exists := o.agents[agentID]; exists {
		return errors.New(errors.ErrorTypeInput, "RegisterAgent",
			fmt.Sprintf("agent %s already registered", agentID))
	}

	o.agents[agentID] = agent
	o.metrics.AgentUtilization[agentID] = 0.0

	logger.Info("registered agent", "agent_id", agentID, "role", agent.GetRole(),
		"capabilities", len(agent.GetCapabilities()))

	return nil
}

// GetAgents returns all registered agents
func (o *DefaultOrchestrator) GetAgents() []Agent {
	o.mu.RLock()
	defer o.mu.RUnlock()

	agents := make([]Agent, 0, len(o.agents))
	for _, agent := range o.agents {
		agents = append(agents, agent)
	}

	return agents
}

// GetAgentsByRole returns agents with a specific role
func (o *DefaultOrchestrator) GetAgentsByRole(role AgentRole) []Agent {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var agents []Agent
	for _, agent := range o.agents {
		if agent.GetRole() == role {
			agents = append(agents, agent)
		}
	}

	return agents
}

// GetAgentsByCapability returns agents with a specific capability
func (o *DefaultOrchestrator) GetAgentsByCapability(capability Capability) []Agent {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var agents []Agent
	for _, agent := range o.agents {
		for _, cap := range agent.GetCapabilities() {
			if cap == capability {
				agents = append(agents, agent)
				break
			}
		}
	}

	return agents
}

// ExecuteTask coordinates task execution across multiple agents
func (o *DefaultOrchestrator) ExecuteTask(ctx context.Context, task Task) (*OrchestrationResult, error) {
	logger.Info("orchestrating task execution", "task_id", task.ID, "task_type", task.Type)

	startTime := time.Now()
	o.emitEvent(EventTaskStarted, task.ID, "", nil)

	// Update metrics
	o.mu.Lock()
	o.metrics.TotalTasks++
	o.mu.Unlock()

	result := &OrchestrationResult{
		TaskID:    task.ID,
		Status:    StatusSuccess,
		Results:   []Result{},
		Timestamp: startTime,
	}

	// Find suitable lead agent
	leadAgent, err := o.selectLeadAgent(task)
	if err != nil {
		o.updateFailureMetrics()
		result.Status = StatusFailed
		result.Duration = time.Since(startTime)
		o.emitEvent(EventTaskFailed, task.ID, "", map[string]string{"error": err.Error()})
		return result, errors.Wrap(err, errors.ErrorTypeConfig, "ExecuteTask", "failed to select lead agent")
	}

	result.LeadAgent = leadAgent.GetID()

	// Create execution context with timeout
	execCtx, cancel := context.WithTimeout(ctx, o.config.TaskTimeout)
	defer cancel()

	// Execute task with lead agent
	leadResult, err := leadAgent.Execute(execCtx, task)
	if err != nil {
		o.updateFailureMetrics()
		result.Status = StatusFailed
		result.Duration = time.Since(startTime)
		o.emitEvent(EventTaskFailed, task.ID, leadAgent.GetID(), map[string]string{"error": err.Error()})
		return result, errors.Wrap(err, errors.ErrorTypeInternal, "ExecuteTask", "lead agent execution failed")
	}

	result.Results = append(result.Results, *leadResult)

	// If proposals were generated, coordinate review process
	if len(leadResult.Proposals) > 0 {
		for _, proposal := range leadResult.Proposals {
			consensus, err := o.ReviewProposal(execCtx, proposal)
			if err != nil {
				logger.Warn("proposal review failed", "proposal_id", proposal.ID, "error", err)
				continue
			}

			result.Consensus = consensus

			// Check if consensus approves the proposal
			if consensus.Decision == ConsensusApprove {
				result.FinalResult = leadResult
			}
		}
	} else {
		// No proposals, use lead result directly
		result.FinalResult = leadResult
	}

	// Update metrics
	result.Duration = time.Since(startTime)
	o.updateSuccessMetrics(result.Duration)

	logger.Info("task orchestration completed", "task_id", task.ID, "status", result.Status,
		"duration", result.Duration, "proposals", len(leadResult.Proposals))

	o.emitEvent(EventTaskCompleted, task.ID, leadAgent.GetID(), map[string]string{
		"status":    string(result.Status),
		"duration":  result.Duration.String(),
		"proposals": fmt.Sprintf("%d", len(leadResult.Proposals)),
	})

	return result, nil
}

// ReviewProposal coordinates proposal review across multiple agents
func (o *DefaultOrchestrator) ReviewProposal(ctx context.Context, proposal Proposal) (*ConsensusResult, error) {
	logger.Debug("orchestrating proposal review", "proposal_id", proposal.ID)

	startTime := time.Now()
	o.emitEvent(EventReviewStarted, "", "", map[string]string{"proposal_id": proposal.ID})

	result := &ConsensusResult{
		ProposalID:   proposal.ID,
		Decision:     ConsensusNoConsensus,
		Reviews:      []ReviewResult{},
		Participants: []string{},
		Timestamp:    startTime,
	}

	// Select reviewer agents
	reviewers := o.selectReviewers(proposal)
	if len(reviewers) == 0 {
		return result, errors.New(errors.ErrorTypeConfig, "ReviewProposal", "no suitable reviewers found")
	}

	// Ensure minimum reviewers requirement
	if len(reviewers) < o.config.QualityGate.MinReviewers {
		return result, errors.New(errors.ErrorTypeConfig, "ReviewProposal",
			fmt.Sprintf("insufficient reviewers: %d required, %d available",
				o.config.QualityGate.MinReviewers, len(reviewers)))
	}

	// Limit reviewers to maximum
	if len(reviewers) > o.config.QualityGate.MaxReviewers {
		reviewers = reviewers[:o.config.QualityGate.MaxReviewers]
	}

	// Create review context with timeout
	reviewCtx, cancel := context.WithTimeout(ctx, o.config.ReviewTimeout)
	defer cancel()

	// Execute reviews
	var reviews []ReviewResult
	if o.config.EnableParallelReview {
		reviews = o.executeParallelReviews(reviewCtx, proposal, reviewers)
	} else {
		reviews = o.executeSequentialReviews(reviewCtx, proposal, reviewers)
	}

	result.Reviews = reviews
	for _, review := range reviews {
		result.Participants = append(result.Participants, review.ReviewerID)
	}

	// Build consensus
	consensus := o.buildConsensus(reviews)
	result.Decision = consensus.decision
	result.Score = consensus.score
	result.Conflicts = consensus.conflicts

	if len(consensus.conflicts) > 0 {
		o.emitEvent(EventConflictDetected, "", "", map[string]string{
			"proposal_id": proposal.ID,
			"conflicts":   fmt.Sprintf("%d", len(consensus.conflicts)),
		})

		// Attempt conflict resolution
		resolution, err := o.resolveConflicts(consensus.conflicts, reviews)
		if err != nil {
			logger.Warn("conflict resolution failed", "proposal_id", proposal.ID, "error", err)
		} else {
			result.Resolution = resolution
			// Update decision based on resolution
			if resolution.Method == ResolutionVoting {
				result.Decision = consensus.decision
			}
		}
	}

	logger.Info("proposal review completed", "proposal_id", proposal.ID, "decision", result.Decision,
		"score", result.Score, "reviewers", len(reviewers), "conflicts", len(result.Conflicts))

	o.emitEvent(EventReviewCompleted, "", "", map[string]string{
		"proposal_id": proposal.ID,
		"decision":    string(result.Decision),
		"score":       fmt.Sprintf("%.2f", result.Score),
	})

	if result.Decision != ConsensusNoConsensus {
		o.emitEvent(EventConsensusReached, "", "", map[string]string{
			"proposal_id": proposal.ID,
			"decision":    string(result.Decision),
		})
	}

	return result, nil
}

// GetMetrics returns orchestration metrics
func (o *DefaultOrchestrator) GetMetrics() OrchestrationMetrics {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.metrics
}

// selectLeadAgent selects the most suitable lead agent for a task
func (o *DefaultOrchestrator) selectLeadAgent(_ Task) (Agent, error) {
	leadAgents := o.GetAgentsByRole(RoleLead)
	if len(leadAgents) == 0 {
		return nil, errors.New(errors.ErrorTypeConfig, "selectLeadAgent", "no lead agents available")
	}

	// For now, select the first available lead agent
	// In a more sophisticated implementation, you'd consider:
	// - Agent specialization
	// - Current load
	// - Task requirements
	// - Agent performance history

	return leadAgents[0], nil
}

// selectReviewers selects appropriate reviewer agents for a proposal
func (o *DefaultOrchestrator) selectReviewers(_ Proposal) []Agent {
	reviewers := o.GetAgentsByRole(RoleReviewer)
	if len(reviewers) == 0 {
		return []Agent{}
	}

	// Filter by required capabilities
	var suitableReviewers []Agent
	for _, reviewer := range reviewers {
		hasRequired := true
		for _, reqCap := range o.config.QualityGate.RequiredCapabilities {
			found := false
			for _, agentCap := range reviewer.GetCapabilities() {
				if agentCap == reqCap {
					found = true
					break
				}
			}
			if !found {
				hasRequired = false
				break
			}
		}
		if hasRequired {
			suitableReviewers = append(suitableReviewers, reviewer)
		}
	}

	// Add mandatory reviewers
	for _, mandatoryID := range o.config.QualityGate.MandatoryReviewers {
		if agent, exists := o.agents[mandatoryID]; exists {
			// Check if not already included
			found := false
			for _, existing := range suitableReviewers {
				if existing.GetID() == mandatoryID {
					found = true
					break
				}
			}
			if !found {
				suitableReviewers = append(suitableReviewers, agent)
			}
		}
	}

	return suitableReviewers
}

// executeParallelReviews executes reviews in parallel
func (o *DefaultOrchestrator) executeParallelReviews(ctx context.Context, proposal Proposal, reviewers []Agent) []ReviewResult {
	type reviewResult struct {
		result *ReviewResult
		err    error
	}

	resultCh := make(chan reviewResult, len(reviewers))

	// Start all reviews concurrently
	for _, reviewer := range reviewers {
		go func(agent Agent) {
			result, err := agent.Review(ctx, proposal)
			resultCh <- reviewResult{result: result, err: err}
		}(reviewer)
	}

	// Collect results
	var reviews []ReviewResult
	for i := 0; i < len(reviewers); i++ {
		select {
		case res := <-resultCh:
			if res.err != nil {
				logger.Warn("reviewer failed", "error", res.err)
			} else if res.result != nil {
				reviews = append(reviews, *res.result)
			}
		case <-ctx.Done():
			logger.Warn("review timeout", "proposal_id", proposal.ID)
			break
		}
	}

	return reviews
}

// executeSequentialReviews executes reviews sequentially
func (o *DefaultOrchestrator) executeSequentialReviews(ctx context.Context, proposal Proposal, reviewers []Agent) []ReviewResult {
	var reviews []ReviewResult

	for _, reviewer := range reviewers {
		select {
		case <-ctx.Done():
			logger.Warn("review timeout", "proposal_id", proposal.ID)
			break
		default:
			result, err := reviewer.Review(ctx, proposal)
			if err != nil {
				logger.Warn("reviewer failed", "reviewer_id", reviewer.GetID(), "error", err)
			} else if result != nil {
				reviews = append(reviews, *result)
			}
		}
	}

	return reviews
}

// consensusData holds consensus building data
type consensusData struct {
	decision  ConsensusDecision
	score     float64
	conflicts []Conflict
}

// buildConsensus analyzes reviews and builds consensus
func (o *DefaultOrchestrator) buildConsensus(reviews []ReviewResult) consensusData {
	if len(reviews) == 0 {
		return consensusData{
			decision: ConsensusNoConsensus,
			score:    0.0,
		}
	}

	// Count decisions
	decisionCounts := make(map[ReviewDecision]int)
	totalScore := 0.0
	totalConfidence := 0.0

	for _, review := range reviews {
		decisionCounts[review.Decision]++
		totalScore += review.Score
		totalConfidence += review.Confidence
	}

	avgScore := totalScore / float64(len(reviews))
	avgConfidence := totalConfidence / float64(len(reviews))

	// Determine consensus
	var conflicts []Conflict
	maxCount := 0
	var majorityDecision ReviewDecision

	for decision, count := range decisionCounts {
		if count > maxCount {
			maxCount = count
			majorityDecision = decision
		}
	}

	consensusThreshold := o.config.ConsensusThreshold
	consensusRatio := float64(maxCount) / float64(len(reviews))

	var finalDecision ConsensusDecision
	if consensusRatio >= consensusThreshold {
		// Strong consensus
		switch majorityDecision {
		case DecisionApprove:
			finalDecision = ConsensusApprove
		case DecisionReject:
			finalDecision = ConsensusReject
		case DecisionRequestChanges:
			finalDecision = ConsensusRequireChanges
		case DecisionNeedsMoreInfo:
			finalDecision = ConsensusNoConsensus
		default:
			finalDecision = ConsensusNoConsensus
		}
	} else {
		// No consensus, check for conflicts
		finalDecision = ConsensusNoConsensus

		// Identify conflicts
		if len(decisionCounts) > 1 {
			var conflictingAgents []string
			for _, review := range reviews {
				if review.Decision != majorityDecision {
					conflictingAgents = append(conflictingAgents, review.ReviewerID)
				}
			}

			if len(conflictingAgents) > 0 {
				conflict := Conflict{
					Type:   ConflictTypeDecision,
					Agents: conflictingAgents,
					Description: fmt.Sprintf("Disagreement on decision: %d for %s, %d for others",
						maxCount, majorityDecision, len(reviews)-maxCount),
					Severity: SeverityWarning,
				}
				conflicts = append(conflicts, conflict)
			}
		}
	}

	// Apply quality gate checks
	if avgConfidence < o.config.QualityGate.MinConfidence {
		finalDecision = ConsensusNoConsensus
		conflicts = append(conflicts, Conflict{
			Type:        ConflictTypeDecision,
			Description: fmt.Sprintf("Low confidence: %.2f < %.2f", avgConfidence, o.config.QualityGate.MinConfidence),
			Severity:    SeverityWarning,
		})
	}

	return consensusData{
		decision:  finalDecision,
		score:     avgScore,
		conflicts: conflicts,
	}
}

// resolveConflicts attempts to resolve conflicts between reviews
func (o *DefaultOrchestrator) resolveConflicts(conflicts []Conflict, reviews []ReviewResult) (*Resolution, error) {
	if len(conflicts) == 0 {
		return &Resolution{Method: o.config.ConflictResolution, Timestamp: time.Now()}, nil
	}

	resolution := &Resolution{
		Method:    o.config.ConflictResolution,
		Timestamp: time.Now(),
	}

	switch o.config.ConflictResolution {
	case ResolutionVoting:
		// Simple majority voting
		decisionCounts := make(map[ReviewDecision]int)
		for _, review := range reviews {
			decisionCounts[review.Decision]++
		}

		maxCount := 0
		var winningDecision ReviewDecision
		for decision, count := range decisionCounts {
			if count > maxCount {
				maxCount = count
				winningDecision = decision
			}
		}

		resolution.Description = fmt.Sprintf("Resolved by majority vote: %s (%d votes)", winningDecision, maxCount)
		resolution.Rationale = "Simple majority voting among reviewers"

	case ResolutionExpertRule:
		// Defer to expert agents or higher priority reviewers
		resolution.Description = "Resolved by expert opinion"
		resolution.Rationale = "Deferred to agents with higher expertise"

	case ResolutionCompromise:
		// Attempt to find middle ground
		resolution.Description = "Resolved through compromise solution"
		resolution.Rationale = "Found middle ground between conflicting opinions"

	case ResolutionArbitration:
		// Use lead agent as arbiter
		resolution.Description = "Resolved through arbitration"
		resolution.Rationale = "Lead agent made final decision"

	default:
		return nil, errors.New(errors.ErrorTypeConfig, "resolveConflicts",
			fmt.Sprintf("unsupported resolution method: %s", o.config.ConflictResolution))
	}

	resolution.ResolvedBy = "orchestrator"
	return resolution, nil
}

// updateSuccessMetrics updates metrics for successful task completion
func (o *DefaultOrchestrator) updateSuccessMetrics(duration time.Duration) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.metrics.CompletedTasks++

	// Update average task time
	if o.metrics.CompletedTasks == 1 {
		o.metrics.AverageTaskTime = duration
	} else {
		// Exponential moving average
		alpha := 0.1
		o.metrics.AverageTaskTime = time.Duration(float64(o.metrics.AverageTaskTime)*(1-alpha) + float64(duration)*alpha)
	}

	o.metrics.LastUpdated = time.Now()
}

// updateFailureMetrics updates metrics for failed task completion
func (o *DefaultOrchestrator) updateFailureMetrics() {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.metrics.FailedTasks++
	o.metrics.LastUpdated = time.Now()
}

// emitEvent emits an orchestration event
func (o *DefaultOrchestrator) emitEvent(eventType EventType, taskID, agentID string, data map[string]string) {
	event := OrchestrationEvent{
		Type:      eventType,
		TaskID:    taskID,
		AgentID:   agentID,
		Data:      data,
		Timestamp: time.Now(),
	}

	select {
	case o.eventCh <- event:
	default:
		// Event channel full, drop event
		logger.Warn("orchestration event dropped", "type", eventType)
	}
}

// Start starts the orchestrator background processes
func (o *DefaultOrchestrator) Start() {
	go o.eventProcessor()
}

// Stop stops the orchestrator
func (o *DefaultOrchestrator) Stop() {
	close(o.stopCh)
}

// eventProcessor processes orchestration events
func (o *DefaultOrchestrator) eventProcessor() {
	for {
		select {
		case event := <-o.eventCh:
			logger.Debug("orchestration event", "type", event.Type, "task_id", event.TaskID, "agent_id", event.AgentID)
			// Process event (could emit to external systems, update metrics, etc.)

		case <-o.stopCh:
			return
		}
	}
}
