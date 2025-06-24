// Package agent provides multi-agent orchestration for code transformations
package agent

import (
	"context"
	"time"

	"github.com/dshills/sigil/internal/model"
)

// Agent represents an intelligent agent that can perform code transformations
type Agent interface {
	// GetID returns the unique identifier for this agent
	GetID() string

	// GetRole returns the role of this agent (lead, reviewer, etc.)
	GetRole() AgentRole

	// GetCapabilities returns the capabilities of this agent
	GetCapabilities() []Capability

	// Execute performs the agent's primary function with the given task
	Execute(ctx context.Context, task Task) (*Result, error)

	// Review evaluates a proposal from another agent
	Review(ctx context.Context, proposal Proposal) (*ReviewResult, error)

	// GetModel returns the underlying LLM model
	GetModel() model.Model
}

// AgentRole defines the role of an agent in the multi-agent system
type AgentRole string

const (
	RoleLead     AgentRole = "lead"     // Primary agent that coordinates tasks
	RoleReviewer AgentRole = "reviewer" // Agent that reviews and validates proposals
	RoleExpert   AgentRole = "expert"   // Specialized agent for specific domains
)

// Capability defines what an agent can do
type Capability string

const (
	CapabilityCodeGeneration      Capability = "code_generation"
	CapabilityCodeReview          Capability = "code_review"
	CapabilityTesting             Capability = "testing"
	CapabilityDocumentation       Capability = "documentation"
	CapabilityRefactoring         Capability = "refactoring"
	CapabilitySecurityAnalysis    Capability = "security_analysis"
	CapabilityPerformanceAnalysis Capability = "performance_analysis"
	CapabilityArchitectureReview  Capability = "architecture_review"
)

// Specialization constants to avoid goconst warnings
const (
	SpecializationSecurity     = "security"
	SpecializationPerformance  = "performance"
	SpecializationArchitecture = "architecture"
	SpecializationTesting      = "testing"
)

// Task represents a high-level task to be performed by an agent
type Task struct {
	ID          string            `json:"id"`
	Type        TaskType          `json:"type"`
	Description string            `json:"description"`
	Context     TaskContext       `json:"context"`
	Constraints []Constraint      `json:"constraints"`
	Priority    Priority          `json:"priority"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	Deadline    *time.Time        `json:"deadline,omitempty"`
}

// TaskType defines the type of task
type TaskType string

const (
	TaskTypeEdit     TaskType = "edit"
	TaskTypeGenerate TaskType = "generate"
	TaskTypeRefactor TaskType = "refactor"
	TaskTypeDocument TaskType = "document"
	TaskTypeTest     TaskType = "test"
	TaskTypeReview   TaskType = "review"
	TaskTypeOptimize TaskType = "optimize"
	TaskTypeAnalyze  TaskType = "analyze"
)

// TaskContext provides context for task execution
type TaskContext struct {
	Files        []FileContext     `json:"files"`
	Dependencies []string          `json:"dependencies"`
	Requirements []string          `json:"requirements"`
	Examples     []Example         `json:"examples,omitempty"`
	ProjectInfo  ProjectInfo       `json:"project_info"`
	Memory       []MemoryEntry     `json:"memory,omitempty"`
	Environment  map[string]string `json:"environment,omitempty"`
}

// FileContext provides information about a file in the task context
type FileContext struct {
	Path        string `json:"path"`
	Content     string `json:"content"`
	Language    string `json:"language"`
	Purpose     string `json:"purpose"`
	IsTarget    bool   `json:"is_target"`    // Whether this file should be modified
	IsReference bool   `json:"is_reference"` // Whether this file is for reference only
}

// Example provides an example of what the agent should do
type Example struct {
	Description string `json:"description"`
	Input       string `json:"input"`
	Output      string `json:"output"`
	Explanation string `json:"explanation,omitempty"`
}

// ProjectInfo provides information about the project
type ProjectInfo struct {
	Language    string            `json:"language"`
	Framework   string            `json:"framework,omitempty"`
	Version     string            `json:"version,omitempty"`
	Style       string            `json:"style,omitempty"`
	Conventions map[string]string `json:"conventions,omitempty"`
}

// MemoryEntry represents a piece of information from memory
type MemoryEntry struct {
	Type    string  `json:"type"`
	Content string  `json:"content"`
	Source  string  `json:"source"`
	Weight  float64 `json:"weight,omitempty"`
}

// Constraint defines a constraint on task execution
type Constraint struct {
	Type        ConstraintType    `json:"type"`
	Description string            `json:"description"`
	Parameters  map[string]string `json:"parameters,omitempty"`
	Severity    Severity          `json:"severity"`
}

// ConstraintType defines the type of constraint
type ConstraintType string

const (
	ConstraintTypeStyle         ConstraintType = "style"
	ConstraintTypeSecurity      ConstraintType = "security"
	ConstraintTypePerformance   ConstraintType = "performance"
	ConstraintTypeCompatibility ConstraintType = "compatibility"
	ConstraintTypeResource      ConstraintType = "resource"
	ConstraintTypeTesting       ConstraintType = "testing"
)

// Priority defines task priority
type Priority string

const (
	PriorityLow      Priority = "low"
	PriorityMedium   Priority = "medium"
	PriorityHigh     Priority = "high"
	PriorityCritical Priority = "critical"
)

// Severity defines constraint severity
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
)

// Result represents the result of task execution
type Result struct {
	TaskID     string            `json:"task_id"`
	AgentID    string            `json:"agent_id"`
	Status     ResultStatus      `json:"status"`
	Proposals  []Proposal        `json:"proposals"`
	Artifacts  []Artifact        `json:"artifacts"`
	Reasoning  string            `json:"reasoning"`
	Confidence float64           `json:"confidence"` // 0.0 to 1.0
	Duration   time.Duration     `json:"duration"`
	Timestamp  time.Time         `json:"timestamp"`
	Error      string            `json:"error,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// ResultStatus defines the status of a result
type ResultStatus string

const (
	StatusSuccess    ResultStatus = "success"
	StatusPartial    ResultStatus = "partial"
	StatusFailed     ResultStatus = "failed"
	StatusIncomplete ResultStatus = "incomplete"
)

// Proposal represents a proposed change
type Proposal struct {
	ID          string            `json:"id"`
	AgentID     string            `json:"agent_id"`
	Type        ProposalType      `json:"type"`
	Description string            `json:"description"`
	Changes     []Change          `json:"changes"`
	Reasoning   string            `json:"reasoning"`
	Confidence  float64           `json:"confidence"`
	Impact      Impact            `json:"impact"`
	Tests       []TestCase        `json:"tests,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ProposalType defines the type of proposal
type ProposalType string

const (
	ProposalTypeFileChange   ProposalType = "file_change"
	ProposalTypeFileCreation ProposalType = "file_creation"
	ProposalTypeFileDeletion ProposalType = "file_deletion"
	ProposalTypeRefactoring  ProposalType = "refactoring"
	ProposalTypeArchitecture ProposalType = "architecture"
)

// Change represents a specific change in a proposal
type Change struct {
	Type        ChangeType `json:"type"`
	Path        string     `json:"path"`
	OldContent  string     `json:"old_content,omitempty"`
	NewContent  string     `json:"new_content"`
	StartLine   int        `json:"start_line,omitempty"`
	EndLine     int        `json:"end_line,omitempty"`
	Description string     `json:"description"`
}

// ChangeType defines the type of change
type ChangeType string

const (
	ChangeTypeCreate ChangeType = "create"
	ChangeTypeUpdate ChangeType = "update"
	ChangeTypeDelete ChangeType = "delete"
	ChangeTypeMove   ChangeType = "move"
	ChangeTypeRename ChangeType = "rename"
)

// Impact describes the impact of a proposal
type Impact struct {
	Scope           ImpactScope `json:"scope"`
	Risk            Risk        `json:"risk"`
	Benefits        []string    `json:"benefits"`
	Drawbacks       []string    `json:"drawbacks,omitempty"`
	Dependencies    []string    `json:"dependencies,omitempty"`
	BreakingChanges []string    `json:"breaking_changes,omitempty"`
}

// ImpactScope defines the scope of impact
type ImpactScope string

const (
	ScopeLocal     ImpactScope = "local"     // Single file or function
	ScopeModule    ImpactScope = "module"    // Single module or package
	ScopeProject   ImpactScope = "project"   // Entire project
	ScopeEcosystem ImpactScope = "ecosystem" // External dependencies
)

// Risk defines the risk level
type Risk string

const (
	RiskLow      Risk = "low"
	RiskMedium   Risk = "medium"
	RiskHigh     Risk = "high"
	RiskCritical Risk = "critical"
)

// TestCase represents a test case for validating a proposal
type TestCase struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Type        TestType          `json:"type"`
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Expected    string            `json:"expected,omitempty"`
	Timeout     time.Duration     `json:"timeout,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
}

// TestType defines the type of test
type TestType string

const (
	TestTypeUnit        TestType = "unit"
	TestTypeIntegration TestType = "integration"
	TestTypeLint        TestType = "lint"
	TestTypeBuild       TestType = "build"
	TestTypeCustom      TestType = "custom"
)

// Artifact represents an output artifact from task execution
type Artifact struct {
	Name      string            `json:"name"`
	Type      ArtifactType      `json:"type"`
	Path      string            `json:"path,omitempty"`
	Content   string            `json:"content,omitempty"`
	Size      int64             `json:"size,omitempty"`
	Checksum  string            `json:"checksum,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

// ArtifactType defines the type of artifact
type ArtifactType string

const (
	ArtifactTypeFile          ArtifactType = "file"
	ArtifactTypeReport        ArtifactType = "report"
	ArtifactTypeLog           ArtifactType = "log"
	ArtifactTypeDocumentation ArtifactType = "documentation"
	ArtifactTypeTest          ArtifactType = "test"
	ArtifactTypeConfiguration ArtifactType = "configuration"
)

// ReviewResult represents the result of reviewing a proposal
type ReviewResult struct {
	ProposalID  string            `json:"proposal_id"`
	ReviewerID  string            `json:"reviewer_id"`
	Decision    ReviewDecision    `json:"decision"`
	Score       float64           `json:"score"`      // 0.0 to 1.0
	Confidence  float64           `json:"confidence"` // 0.0 to 1.0
	Comments    []ReviewComment   `json:"comments"`
	Suggestions []Suggestion      `json:"suggestions,omitempty"`
	Tests       []TestResult      `json:"tests,omitempty"`
	Reasoning   string            `json:"reasoning"`
	Timestamp   time.Time         `json:"timestamp"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ReviewDecision defines the decision from a review
type ReviewDecision string

const (
	DecisionApprove        ReviewDecision = "approve"
	DecisionRequestChanges ReviewDecision = "request_changes"
	DecisionReject         ReviewDecision = "reject"
	DecisionNeedsMoreInfo  ReviewDecision = "needs_more_info"
)

// ReviewComment represents a specific comment in a review
type ReviewComment struct {
	Type       CommentType `json:"type"`
	Severity   Severity    `json:"severity"`
	Path       string      `json:"path,omitempty"`
	Line       int         `json:"line,omitempty"`
	Message    string      `json:"message"`
	Suggestion string      `json:"suggestion,omitempty"`
	Context    string      `json:"context,omitempty"`
	References []string    `json:"references,omitempty"`
}

// CommentType defines the type of review comment
type CommentType string

const (
	CommentTypeGeneral     CommentType = "general"
	CommentTypeSyntax      CommentType = "syntax"
	CommentTypeLogic       CommentType = "logic"
	CommentTypeStyle       CommentType = "style"
	CommentTypePerformance CommentType = "performance"
	CommentTypeSecurity    CommentType = "security"
	CommentTypeDesign      CommentType = "design"
	CommentTypeTesting     CommentType = "testing"
)

// Suggestion represents an improvement suggestion
type Suggestion struct {
	Type        SuggestionType `json:"type"`
	Description string         `json:"description"`
	Change      *Change        `json:"change,omitempty"`
	Priority    Priority       `json:"priority"`
	Rationale   string         `json:"rationale"`
}

// SuggestionType defines the type of suggestion
type SuggestionType string

const (
	SuggestionTypeImprovement  SuggestionType = "improvement"
	SuggestionTypeAlternative  SuggestionType = "alternative"
	SuggestionTypeOptimization SuggestionType = "optimization"
	SuggestionTypeFix          SuggestionType = "fix"
)

// TestResult represents the result of running a test
type TestResult struct {
	TestCase TestCase      `json:"test_case"`
	Status   TestStatus    `json:"status"`
	Output   string        `json:"output,omitempty"`
	Error    string        `json:"error,omitempty"`
	Duration time.Duration `json:"duration"`
}

// TestStatus defines the status of a test
type TestStatus string

const (
	TestStatusPassed  TestStatus = "passed"
	TestStatusFailed  TestStatus = "failed"
	TestStatusSkipped TestStatus = "skipped"
	TestStatusError   TestStatus = "error"
)

// Orchestrator manages multiple agents and coordinates their work
type Orchestrator interface {
	// RegisterAgent adds an agent to the orchestrator
	RegisterAgent(agent Agent) error

	// GetAgents returns all registered agents
	GetAgents() []Agent

	// GetAgentsByRole returns agents with a specific role
	GetAgentsByRole(role AgentRole) []Agent

	// GetAgentsByCapability returns agents with a specific capability
	GetAgentsByCapability(capability Capability) []Agent

	// ExecuteTask coordinates task execution across multiple agents
	ExecuteTask(ctx context.Context, task Task) (*OrchestrationResult, error)

	// ReviewProposal coordinates proposal review across multiple agents
	ReviewProposal(ctx context.Context, proposal Proposal) (*ConsensusResult, error)

	// GetMetrics returns orchestration metrics
	GetMetrics() OrchestrationMetrics
}

// OrchestrationResult represents the result of orchestrated task execution
type OrchestrationResult struct {
	TaskID      string            `json:"task_id"`
	Status      ResultStatus      `json:"status"`
	LeadAgent   string            `json:"lead_agent"`
	Results     []Result          `json:"results"`
	Consensus   *ConsensusResult  `json:"consensus,omitempty"`
	FinalResult *Result           `json:"final_result,omitempty"`
	Duration    time.Duration     `json:"duration"`
	Timestamp   time.Time         `json:"timestamp"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ConsensusResult represents the result of consensus building
type ConsensusResult struct {
	ProposalID   string            `json:"proposal_id"`
	Decision     ConsensusDecision `json:"decision"`
	Score        float64           `json:"score"`
	Reviews      []ReviewResult    `json:"reviews"`
	Conflicts    []Conflict        `json:"conflicts,omitempty"`
	Resolution   *Resolution       `json:"resolution,omitempty"`
	Participants []string          `json:"participants"`
	Timestamp    time.Time         `json:"timestamp"`
}

// ConsensusDecision defines the consensus decision
type ConsensusDecision string

const (
	ConsensusApprove        ConsensusDecision = "approve"
	ConsensusReject         ConsensusDecision = "reject"
	ConsensusRequireChanges ConsensusDecision = "require_changes"
	ConsensusNoConsensus    ConsensusDecision = "no_consensus"
)

// Conflict represents a conflict between reviews
type Conflict struct {
	Type        ConflictType `json:"type"`
	Agents      []string     `json:"agents"`
	Description string       `json:"description"`
	Severity    Severity     `json:"severity"`
	Context     string       `json:"context,omitempty"`
}

// ConflictType defines the type of conflict
type ConflictType string

const (
	ConflictTypeDecision       ConflictType = "decision"
	ConflictTypeApproach       ConflictType = "approach"
	ConflictTypeImplementation ConflictType = "implementation"
	ConflictTypePriority       ConflictType = "priority"
)

// Resolution represents the resolution of a conflict
type Resolution struct {
	Method      ResolutionMethod `json:"method"`
	Description string           `json:"description"`
	Rationale   string           `json:"rationale"`
	ResolvedBy  string           `json:"resolved_by"`
	Timestamp   time.Time        `json:"timestamp"`
}

// ResolutionMethod defines how a conflict was resolved
type ResolutionMethod string

const (
	ResolutionVoting      ResolutionMethod = "voting"
	ResolutionExpertRule  ResolutionMethod = "expert_rule"
	ResolutionCompromise  ResolutionMethod = "compromise"
	ResolutionArbitration ResolutionMethod = "arbitration"
)

// OrchestrationMetrics provides metrics about orchestration performance
type OrchestrationMetrics struct {
	TotalTasks       int64              `json:"total_tasks"`
	CompletedTasks   int64              `json:"completed_tasks"`
	FailedTasks      int64              `json:"failed_tasks"`
	AverageTaskTime  time.Duration      `json:"average_task_time"`
	AgentUtilization map[string]float64 `json:"agent_utilization"`
	ConsensusRate    float64            `json:"consensus_rate"`
	ConflictRate     float64            `json:"conflict_rate"`
	QualityScore     float64            `json:"quality_score"`
	LastUpdated      time.Time          `json:"last_updated"`
}

// Configuration for agent orchestration
type OrchestrationConfig struct {
	MaxAgents            int                    `yaml:"max_agents"`
	ConsensusThreshold   float64                `yaml:"consensus_threshold"`
	ConflictResolution   ResolutionMethod       `yaml:"conflict_resolution"`
	TaskTimeout          time.Duration          `yaml:"task_timeout"`
	ReviewTimeout        time.Duration          `yaml:"review_timeout"`
	MaxRetries           int                    `yaml:"max_retries"`
	EnableParallelReview bool                   `yaml:"enable_parallel_review"`
	QualityGate          QualityGateConfig      `yaml:"quality_gate"`
	AgentProfiles        map[string]AgentConfig `yaml:"agent_profiles"`
}

// QualityGateConfig defines quality gate settings
type QualityGateConfig struct {
	MinConfidence        float64      `yaml:"min_confidence"`
	RequiredCapabilities []Capability `yaml:"required_capabilities"`
	MandatoryReviewers   []string     `yaml:"mandatory_reviewers"`
	MinReviewers         int          `yaml:"min_reviewers"`
	MaxReviewers         int          `yaml:"max_reviewers"`
}

// AgentConfig defines configuration for a specific agent
type AgentConfig struct {
	Role           AgentRole    `yaml:"role"`
	Model          string       `yaml:"model"`
	Capabilities   []Capability `yaml:"capabilities"`
	Priority       int          `yaml:"priority"`
	MaxConcurrency int          `yaml:"max_concurrency"`
	Specialization string       `yaml:"specialization,omitempty"`
	Enabled        bool         `yaml:"enabled"`
}

// DefaultOrchestrationConfig returns default orchestration configuration
func DefaultOrchestrationConfig() OrchestrationConfig {
	return OrchestrationConfig{
		MaxAgents:            5,
		ConsensusThreshold:   0.7,
		ConflictResolution:   ResolutionVoting,
		TaskTimeout:          10 * time.Minute,
		ReviewTimeout:        5 * time.Minute,
		MaxRetries:           3,
		EnableParallelReview: true,
		QualityGate: QualityGateConfig{
			MinConfidence:        0.8,
			RequiredCapabilities: []Capability{CapabilityCodeReview},
			MinReviewers:         2,
			MaxReviewers:         4,
		},
		AgentProfiles: map[string]AgentConfig{
			"lead": {
				Role:           RoleLead,
				Model:          "claude-3-5-sonnet-20241022",
				Capabilities:   []Capability{CapabilityCodeGeneration, CapabilityRefactoring},
				Priority:       1,
				MaxConcurrency: 1,
				Enabled:        true,
			},
			"reviewer": {
				Role:           RoleReviewer,
				Model:          "gpt-4",
				Capabilities:   []Capability{CapabilityCodeReview, CapabilityTesting},
				Priority:       2,
				MaxConcurrency: 3,
				Enabled:        true,
			},
		},
	}
}
