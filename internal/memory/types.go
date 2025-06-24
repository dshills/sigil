// Package memory provides types for the memory system
package memory

import (
	"time"

	"github.com/dshills/sigil/internal/model"
)

// MemoryEntry represents a stored memory entry
type MemoryEntry struct {
	ID         string        `json:"id"`
	Type       string        `json:"type"` // "session", "context", "summary", "decision"
	Timestamp  time.Time     `json:"timestamp"`
	Command    string        `json:"command"`     // Command that generated this entry
	Model      string        `json:"model"`       // Model used
	Content    string        `json:"content"`     // Main content
	Summary    string        `json:"summary"`     // Brief summary
	Tags       string        `json:"tags"`        // Comma-separated tags
	TokensUsed int           `json:"tokens_used"` // Tokens used for this entry
	Duration   time.Duration `json:"duration"`    // Processing duration
}

// MemoryFilter defines filtering options for memory retrieval
type MemoryFilter struct {
	Types   []string  `json:"types,omitempty"`   // Filter by types
	Command string    `json:"command,omitempty"` // Filter by command
	After   time.Time `json:"after,omitempty"`   // After this timestamp
	Before  time.Time `json:"before,omitempty"`  // Before this timestamp
	Query   string    `json:"query,omitempty"`   // Text search query
	Limit   int       `json:"limit,omitempty"`   // Maximum results
}

// Manager provides high-level memory operations
type Manager interface {
	// Store operations
	StoreSession(command string, input string, output model.PromptOutput, duration time.Duration) error
	StoreContext(context string, source string) error
	StoreSummary(content string, summary string) error
	StoreDecision(decision string, reasoning string) error

	// Retrieval operations
	GetRecentContext(limit int) ([]model.MemoryEntry, error)
	GetRecentSessions(limit int) ([]MemoryEntry, error)
	SearchMemory(query string, limit int) ([]MemoryEntry, error)

	// Maintenance operations
	CleanOldEntries(retention time.Duration) error
	GetStats() (MemoryStats, error)
}

// Repository provides low-level memory storage operations
type Repository interface {
	StoreEntry(entry MemoryEntry) error
	GetEntry(id string) (*MemoryEntry, error)
	ListEntries(filter MemoryFilter) ([]MemoryEntry, error)
	SearchEntries(query string, limit int) ([]MemoryEntry, error)
	DeleteEntry(id string) error
	CleanOldEntries(retention time.Duration) error
}

// MemoryStats provides statistics about memory usage
type MemoryStats struct {
	TotalEntries   int            `json:"total_entries"`
	EntriesByType  map[string]int `json:"entries_by_type"`
	OldestEntry    time.Time      `json:"oldest_entry"`
	NewestEntry    time.Time      `json:"newest_entry"`
	TotalSizeBytes int64          `json:"total_size_bytes"`
	AverageSize    int64          `json:"average_size"`
}

// Context holds memory context for commands
type Context struct {
	RecentEntries []model.MemoryEntry `json:"recent_entries"`
	SearchResults []MemoryEntry       `json:"search_results,omitempty"`
	Summary       string              `json:"summary,omitempty"`
}

// EntryBuilder helps build memory entries
type EntryBuilder struct {
	entry MemoryEntry
}

// NewEntryBuilder creates a new entry builder
func NewEntryBuilder() *EntryBuilder {
	return &EntryBuilder{
		entry: MemoryEntry{
			ID:        generateID(),
			Timestamp: time.Now(),
		},
	}
}

// WithType sets the entry type
func (b *EntryBuilder) WithType(entryType string) *EntryBuilder {
	b.entry.Type = entryType
	return b
}

// WithCommand sets the command
func (b *EntryBuilder) WithCommand(command string) *EntryBuilder {
	b.entry.Command = command
	return b
}

// WithModel sets the model
func (b *EntryBuilder) WithModel(model string) *EntryBuilder {
	b.entry.Model = model
	return b
}

// WithContent sets the content
func (b *EntryBuilder) WithContent(content string) *EntryBuilder {
	b.entry.Content = content
	return b
}

// WithSummary sets the summary
func (b *EntryBuilder) WithSummary(summary string) *EntryBuilder {
	b.entry.Summary = summary
	return b
}

// WithTags sets the tags
func (b *EntryBuilder) WithTags(tags string) *EntryBuilder {
	b.entry.Tags = tags
	return b
}

// WithTokens sets the token usage
func (b *EntryBuilder) WithTokens(tokens int) *EntryBuilder {
	b.entry.TokensUsed = tokens
	return b
}

// WithDuration sets the duration
func (b *EntryBuilder) WithDuration(duration time.Duration) *EntryBuilder {
	b.entry.Duration = duration
	return b
}

// Build returns the built entry
func (b *EntryBuilder) Build() MemoryEntry {
	return b.entry
}

// generateID generates a unique ID for memory entries
func generateID() string {
	return time.Now().Format("20060102-150405") + "-" + randomString(6)
}

// randomString generates a random string of given length
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}
