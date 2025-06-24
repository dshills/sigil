// Package memory provides Markdown-based memory storage for Sigil
package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dshills/sigil/internal/errors"
	"github.com/dshills/sigil/internal/logger"
	"github.com/dshills/sigil/internal/model"
)

const (
	memoryDir     = ".sigil/memory"
	sessionPrefix = "session_"
	contextPrefix = "context_"
	summaryPrefix = "summary_"
)

// Storage handles Markdown-based memory persistence
type Storage struct {
	baseDir string
}

// NewStorage creates a new memory storage instance
func NewStorage() (*Storage, error) {
	// Ensure memory directory exists
	memDir := filepath.Join(".", memoryDir)
	if err := os.MkdirAll(memDir, 0755); err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeFS, "NewStorage", "failed to create memory directory")
	}

	return &Storage{
		baseDir: memDir,
	}, nil
}

// StoreEntry stores a memory entry as a Markdown file
func (s *Storage) StoreEntry(entry MemoryEntry) error {
	filename := s.generateFilename(entry)
	filepath := filepath.Join(s.baseDir, filename)

	// Generate Markdown content
	content := s.formatEntryAsMarkdown(entry)

	// Write to file
	if err := os.WriteFile(filepath, []byte(content), 0600); err != nil {
		return errors.Wrap(err, errors.ErrorTypeFS, "StoreEntry",
			fmt.Sprintf("failed to write memory file %s", filename))
	}

	logger.Debug("stored memory entry", "file", filename, "type", entry.Type)
	return nil
}

// GetEntry retrieves a specific memory entry
func (s *Storage) GetEntry(id string) (*MemoryEntry, error) {
	// Find file by ID
	files, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeFS, "GetEntry", "failed to read memory directory")
	}

	for _, file := range files {
		if strings.Contains(file.Name(), id) && strings.HasSuffix(file.Name(), ".md") {
			content, err := os.ReadFile(filepath.Join(s.baseDir, file.Name()))
			if err != nil {
				return nil, errors.Wrap(err, errors.ErrorTypeFS, "GetEntry",
					fmt.Sprintf("failed to read memory file %s", file.Name()))
			}

			entry, err := s.parseMarkdownEntry(string(content))
			if err != nil {
				return nil, errors.Wrap(err, errors.ErrorTypeInput, "GetEntry",
					"failed to parse memory entry")
			}

			return entry, nil
		}
	}

	return nil, errors.New(errors.ErrorTypeInput, "GetEntry",
		fmt.Sprintf("memory entry %s not found", id))
}

// ListEntries lists memory entries with optional filtering
func (s *Storage) ListEntries(filter MemoryFilter) ([]MemoryEntry, error) {
	files, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeFS, "ListEntries", "failed to read memory directory")
	}

	var entries []MemoryEntry

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".md") {
			continue
		}

		content, err := os.ReadFile(filepath.Join(s.baseDir, file.Name()))
		if err != nil {
			logger.Warn("failed to read memory file", "file", file.Name(), "error", err)
			continue
		}

		entry, err := s.parseMarkdownEntry(string(content))
		if err != nil {
			logger.Warn("failed to parse memory file", "file", file.Name(), "error", err)
			continue
		}

		// Apply filter
		if s.matchesFilter(*entry, filter) {
			entries = append(entries, *entry)
		}
	}

	// Sort by timestamp (newest first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})

	// Apply limit
	if filter.Limit > 0 && len(entries) > filter.Limit {
		entries = entries[:filter.Limit]
	}

	return entries, nil
}

// SearchEntries searches memory entries by content
func (s *Storage) SearchEntries(query string, limit int) ([]MemoryEntry, error) {
	filter := MemoryFilter{
		Query: query,
		Limit: limit,
	}

	entries, err := s.ListEntries(filter)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeFS, "SearchEntries", "failed to search entries")
	}

	// Additional content-based filtering
	var results []MemoryEntry
	queryLower := strings.ToLower(query)

	for _, entry := range entries {
		if strings.Contains(strings.ToLower(entry.Content), queryLower) ||
			strings.Contains(strings.ToLower(entry.Summary), queryLower) ||
			strings.Contains(strings.ToLower(entry.Tags), queryLower) {
			results = append(results, entry)
		}
	}

	return results, nil
}

// GetRecentContext gets recent memory entries for context
func (s *Storage) GetRecentContext(limit int) ([]model.MemoryEntry, error) {
	filter := MemoryFilter{
		Types: []string{"session", "context"},
		Limit: limit,
	}

	entries, err := s.ListEntries(filter)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrorTypeFS, "GetRecentContext", "failed to get recent context")
	}

	// Convert to model.MemoryEntry
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

// DeleteEntry deletes a memory entry
func (s *Storage) DeleteEntry(id string) error {
	// Find and delete file
	files, err := os.ReadDir(s.baseDir)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeFS, "DeleteEntry", "failed to read memory directory")
	}

	for _, file := range files {
		if strings.Contains(file.Name(), id) && strings.HasSuffix(file.Name(), ".md") {
			filePath := filepath.Join(s.baseDir, file.Name())
			if err := os.Remove(filePath); err != nil {
				return errors.Wrap(err, errors.ErrorTypeFS, "DeleteEntry",
					fmt.Sprintf("failed to delete memory file %s", file.Name()))
			}

			logger.Debug("deleted memory entry", "file", file.Name())
			return nil
		}
	}

	return errors.New(errors.ErrorTypeInput, "DeleteEntry",
		fmt.Sprintf("memory entry %s not found", id))
}

// CleanOldEntries removes old memory entries based on retention policy
func (s *Storage) CleanOldEntries(retention time.Duration) error {
	files, err := os.ReadDir(s.baseDir)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeFS, "CleanOldEntries", "failed to read memory directory")
	}

	cutoff := time.Now().Add(-retention)
	deletedCount := 0

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".md") {
			continue
		}

		info, err := file.Info()
		if err != nil {
			logger.Warn("failed to get file info", "file", file.Name(), "error", err)
			continue
		}

		if info.ModTime().Before(cutoff) {
			filePath := filepath.Join(s.baseDir, file.Name())
			if err := os.Remove(filePath); err != nil {
				logger.Warn("failed to delete old memory file", "file", file.Name(), "error", err)
				continue
			}

			deletedCount++
			logger.Debug("deleted old memory entry", "file", file.Name())
		}
	}

	if deletedCount > 0 {
		logger.Info("cleaned old memory entries", "count", deletedCount)
	}

	return nil
}

// generateFilename generates a filename for a memory entry
func (s *Storage) generateFilename(entry MemoryEntry) string {
	timestamp := entry.Timestamp.Format("2006-01-02_15-04-05")

	var prefix string
	switch entry.Type {
	case "session":
		prefix = sessionPrefix
	case "context":
		prefix = contextPrefix
	case "summary":
		prefix = summaryPrefix
	default:
		prefix = "entry_"
	}

	// Include first few words of content for readability
	contentWords := strings.Fields(entry.Content)
	var nameHint string
	if len(contentWords) > 0 {
		nameHint = strings.Join(contentWords[:min(3, len(contentWords))], "_")
		// Clean filename
		nameHint = strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
				return r
			}
			return '_'
		}, nameHint)
		nameHint = "_" + nameHint
	}

	return fmt.Sprintf("%s%s%s.md", prefix, timestamp, nameHint)
}

// formatEntryAsMarkdown formats a memory entry as Markdown
func (s *Storage) formatEntryAsMarkdown(entry MemoryEntry) string {
	var content strings.Builder

	// Frontmatter-style metadata
	content.WriteString("---\n")
	content.WriteString(fmt.Sprintf("id: %s\n", entry.ID))
	content.WriteString(fmt.Sprintf("type: %s\n", entry.Type))
	content.WriteString(fmt.Sprintf("timestamp: %s\n", entry.Timestamp.Format(time.RFC3339)))
	if entry.Command != "" {
		content.WriteString(fmt.Sprintf("command: %s\n", entry.Command))
	}
	if entry.Model != "" {
		content.WriteString(fmt.Sprintf("model: %s\n", entry.Model))
	}
	if entry.Tags != "" {
		content.WriteString(fmt.Sprintf("tags: %s\n", entry.Tags))
	}
	content.WriteString("---\n\n")

	// Title
	if entry.Summary != "" {
		content.WriteString(fmt.Sprintf("# %s\n\n", entry.Summary))
	} else {
		content.WriteString(fmt.Sprintf("# %s Entry - %s\n\n",
			strings.Title(entry.Type), entry.Timestamp.Format("2006-01-02 15:04")))
	}

	// Content
	content.WriteString(entry.Content)
	content.WriteString("\n")

	// Additional metadata as section
	if entry.TokensUsed > 0 || entry.Duration > 0 {
		content.WriteString("\n## Metadata\n\n")
		if entry.TokensUsed > 0 {
			content.WriteString(fmt.Sprintf("- **Tokens Used**: %d\n", entry.TokensUsed))
		}
		if entry.Duration > 0 {
			content.WriteString(fmt.Sprintf("- **Duration**: %s\n", entry.Duration))
		}
	}

	return content.String()
}

// parseMarkdownEntry parses a Markdown memory entry
func (s *Storage) parseMarkdownEntry(content string) (*MemoryEntry, error) {
	lines := strings.Split(content, "\n")
	if len(lines) < 5 || lines[0] != "---" {
		return nil, fmt.Errorf("invalid memory entry format")
	}

	entry := &MemoryEntry{}
	inFrontmatter := true
	var contentLines []string

	for _, line := range lines[1:] {
		if inFrontmatter && line == "---" {
			inFrontmatter = false
			continue
		}

		if inFrontmatter {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}

			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			switch key {
			case "id":
				entry.ID = value
			case "type":
				entry.Type = value
			case "timestamp":
				if t, err := time.Parse(time.RFC3339, value); err == nil {
					entry.Timestamp = t
				}
			case "command":
				entry.Command = value
			case "model":
				entry.Model = value
			case "tags":
				entry.Tags = value
			}
		} else {
			contentLines = append(contentLines, line)
		}
	}

	// Extract content (skip title and metadata sections)
	var actualContent []string
	inMetadata := false

	for _, line := range contentLines {
		if strings.HasPrefix(line, "# ") {
			// Skip title
			continue
		}
		if strings.HasPrefix(line, "## Metadata") {
			inMetadata = true
			continue
		}
		if !inMetadata {
			actualContent = append(actualContent, line)
		}
	}

	entry.Content = strings.TrimSpace(strings.Join(actualContent, "\n"))

	return entry, nil
}

// matchesFilter checks if an entry matches the given filter
func (s *Storage) matchesFilter(entry MemoryEntry, filter MemoryFilter) bool {
	// Type filter
	if len(filter.Types) > 0 {
		found := false
		for _, t := range filter.Types {
			if entry.Type == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Time range filter
	if !filter.After.IsZero() && entry.Timestamp.Before(filter.After) {
		return false
	}
	if !filter.Before.IsZero() && entry.Timestamp.After(filter.Before) {
		return false
	}

	// Command filter
	if filter.Command != "" && entry.Command != filter.Command {
		return false
	}

	// Query filter (basic text search)
	if filter.Query != "" {
		queryLower := strings.ToLower(filter.Query)
		if !strings.Contains(strings.ToLower(entry.Content), queryLower) &&
			!strings.Contains(strings.ToLower(entry.Summary), queryLower) {
			return false
		}
	}

	return true
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
