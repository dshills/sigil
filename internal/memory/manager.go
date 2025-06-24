// Package memory provides memory management functionality
package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dshills/sigil/internal/errors"
	"github.com/dshills/sigil/internal/logger"
	"github.com/dshills/sigil/internal/model"
)

// DefaultManager implements the Manager interface
type DefaultManager struct {
	storage Repository
}

// NewManager creates a new memory manager
func NewManager() (Manager, error) {
	storage, err := NewStorage()
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeConfig, "NewManager", "failed to create storage")
	}

	return &DefaultManager{
		storage: storage,
	}, nil
}

// StoreSession stores a session memory entry
func (m *DefaultManager) StoreSession(command string, input string, output model.PromptOutput, duration time.Duration) error {
	// Build session content
	var content strings.Builder
	content.WriteString(fmt.Sprintf("**Command**: %s\n\n", command))

	if input != "" {
		content.WriteString("**Input**:\n")
		content.WriteString(input)
		content.WriteString("\n\n")
	}

	content.WriteString("**Output**:\n")
	content.WriteString(output.Response)
	content.WriteString("\n")

	// Create entry
	entry := NewEntryBuilder().
		WithType("session").
		WithCommand(command).
		WithModel(output.Model).
		WithContent(content.String()).
		WithSummary(fmt.Sprintf("%s session", command)).
		WithTokens(output.TokensUsed).
		WithDuration(duration).
		Build()

	if err := m.storage.StoreEntry(entry); err != nil {
		return errors.Wrap(err, errors.ErrorTypeFS, "StoreSession", "failed to store session")
	}

	logger.Debug("stored session memory", "command", command, "tokens", output.TokensUsed)
	return nil
}

// StoreContext stores a context memory entry
func (m *DefaultManager) StoreContext(context string, source string) error {
	entry := NewEntryBuilder().
		WithType("context").
		WithContent(context).
		WithSummary(fmt.Sprintf("Context from %s", source)).
		WithTags(source).
		Build()

	if err := m.storage.StoreEntry(entry); err != nil {
		return errors.Wrap(err, errors.ErrorTypeFS, "StoreContext", "failed to store context")
	}

	logger.Debug("stored context memory", "source", source)
	return nil
}

// StoreSummary stores a summary memory entry
func (m *DefaultManager) StoreSummary(content string, summary string) error {
	entry := NewEntryBuilder().
		WithType("summary").
		WithContent(content).
		WithSummary(summary).
		Build()

	if err := m.storage.StoreEntry(entry); err != nil {
		return errors.Wrap(err, errors.ErrorTypeFS, "StoreSummary", "failed to store summary")
	}

	logger.Debug("stored summary memory")
	return nil
}

// StoreDecision stores a decision memory entry
func (m *DefaultManager) StoreDecision(decision string, reasoning string) error {
	content := fmt.Sprintf("**Decision**: %s\n\n**Reasoning**:\n%s", decision, reasoning)

	entry := NewEntryBuilder().
		WithType("decision").
		WithContent(content).
		WithSummary(decision).
		Build()

	if err := m.storage.StoreEntry(entry); err != nil {
		return errors.Wrap(err, errors.ErrorTypeFS, "StoreDecision", "failed to store decision")
	}

	logger.Debug("stored decision memory", "decision", decision)
	return nil
}

// GetRecentContext gets recent memory entries for context
func (m *DefaultManager) GetRecentContext(limit int) ([]model.MemoryEntry, error) {
	if storage, ok := m.storage.(*Storage); ok {
		return storage.GetRecentContext(limit)
	}

	// Fallback implementation
	filter := MemoryFilter{
		Types: []string{"session", "context", "summary"},
		Limit: limit,
	}

	entries, err := m.storage.ListEntries(filter)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeFS, "GetRecentContext", "failed to get recent context")
	}

	modelEntries := make([]model.MemoryEntry, 0, len(entries))
	for _, entry := range entries {
		modelEntries = append(modelEntries, model.MemoryEntry{
			Timestamp: entry.Timestamp.Format(time.RFC3339),
			Content:   entry.Content,
			Type:      entry.Type,
		})
	}

	return modelEntries, nil
}

// GetRecentSessions gets recent session entries
func (m *DefaultManager) GetRecentSessions(limit int) ([]MemoryEntry, error) {
	filter := MemoryFilter{
		Types: []string{"session"},
		Limit: limit,
	}

	entries, err := m.storage.ListEntries(filter)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeFS, "GetRecentSessions", "failed to get recent sessions")
	}

	return entries, nil
}

// SearchMemory searches memory entries
func (m *DefaultManager) SearchMemory(query string, limit int) ([]MemoryEntry, error) {
	entries, err := m.storage.SearchEntries(query, limit)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeFS, "SearchMemory", "failed to search memory")
	}

	return entries, nil
}

// CleanOldEntries removes old memory entries
func (m *DefaultManager) CleanOldEntries(retention time.Duration) error {
	if err := m.storage.CleanOldEntries(retention); err != nil {
		return errors.Wrap(err, errors.ErrorTypeFS, "CleanOldEntries", "failed to clean old entries")
	}

	return nil
}

// GetStats returns memory statistics
func (m *DefaultManager) GetStats() (MemoryStats, error) {
	stats := MemoryStats{
		EntriesByType: make(map[string]int),
	}

	// Get all entries
	filter := MemoryFilter{}
	entries, err := m.storage.ListEntries(filter)
	if err != nil {
		return stats, errors.Wrap(err, errors.ErrorTypeFS, "GetStats", "failed to get entries")
	}

	stats.TotalEntries = len(entries)

	if len(entries) == 0 {
		return stats, nil
	}

	// Calculate statistics
	var totalSize int64
	stats.OldestEntry = entries[0].Timestamp
	stats.NewestEntry = entries[0].Timestamp

	for _, entry := range entries {
		// Count by type
		stats.EntriesByType[entry.Type]++

		// Track timestamps
		if entry.Timestamp.Before(stats.OldestEntry) {
			stats.OldestEntry = entry.Timestamp
		}
		if entry.Timestamp.After(stats.NewestEntry) {
			stats.NewestEntry = entry.Timestamp
		}

		// Estimate size
		entrySize := int64(len(entry.Content) + len(entry.Summary) + 100) // Rough estimate
		totalSize += entrySize
	}

	stats.TotalSizeBytes = totalSize
	if stats.TotalEntries > 0 {
		stats.AverageSize = totalSize / int64(stats.TotalEntries)
	}

	return stats, nil
}

// GetMemoryDirectory returns the memory directory path
func GetMemoryDirectory() string {
	return filepath.Join(".", memoryDir)
}

// InitializeMemory ensures the memory directory exists
func InitializeMemory() error {
	memDir := GetMemoryDirectory()
	if err := os.MkdirAll(memDir, 0755); err != nil {
		return errors.Wrap(err, errors.ErrorTypeFS, "InitializeMemory", "failed to create memory directory")
	}

	logger.Debug("initialized memory directory", "path", memDir)
	return nil
}

// ExportMemory exports memory entries to a specific format
func (m *DefaultManager) ExportMemory(format string, outputPath string) error {
	filter := MemoryFilter{}
	entries, err := m.storage.ListEntries(filter)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeFS, "ExportMemory", "failed to get entries")
	}

	switch format {
	case "markdown":
		return m.exportAsMarkdown(entries, outputPath)
	case "json":
		return m.exportAsJSON(entries, outputPath)
	default:
		return errors.New(errors.ErrorTypeInput, "ExportMemory",
			fmt.Sprintf("unsupported export format: %s", format))
	}
}

// exportAsMarkdown exports entries as a single Markdown file
func (m *DefaultManager) exportAsMarkdown(entries []MemoryEntry, outputPath string) error {
	var content strings.Builder

	content.WriteString("# Sigil Memory Export\n\n")
	content.WriteString(fmt.Sprintf("Generated: %s\n", time.Now().Format(time.RFC3339)))
	content.WriteString(fmt.Sprintf("Total entries: %d\n\n", len(entries)))

	for _, entry := range entries {
		content.WriteString("---\n\n")
		content.WriteString(fmt.Sprintf("## %s\n\n", entry.Summary))
		content.WriteString(fmt.Sprintf("**Type**: %s  \n", entry.Type))
		content.WriteString(fmt.Sprintf("**Timestamp**: %s  \n", entry.Timestamp.Format(time.RFC3339)))
		if entry.Command != "" {
			content.WriteString(fmt.Sprintf("**Command**: %s  \n", entry.Command))
		}
		if entry.Model != "" {
			content.WriteString(fmt.Sprintf("**Model**: %s  \n", entry.Model))
		}
		content.WriteString("\n")
		content.WriteString(entry.Content)
		content.WriteString("\n\n")
	}

	if err := os.WriteFile(outputPath, []byte(content.String()), 0600); err != nil {
		return errors.Wrap(err, errors.ErrorTypeFS, "exportAsMarkdown", "failed to write export file")
	}

	return nil
}

// exportAsJSON exports entries as JSON
func (m *DefaultManager) exportAsJSON(entries []MemoryEntry, outputPath string) error {
	// This would use encoding/json to marshal entries
	// For now, return a placeholder error
	return errors.New(errors.ErrorTypeInternal, "exportAsJSON", "JSON export not yet implemented")
}
