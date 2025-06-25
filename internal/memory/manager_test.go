package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dshills/sigil/internal/model"
)

// MockRepository implements Repository for testing
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) StoreEntry(entry MemoryEntry) error {
	args := m.Called(entry)
	return args.Error(0)
}

func (m *MockRepository) GetEntry(id string) (*MemoryEntry, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*MemoryEntry), args.Error(1)
}

func (m *MockRepository) ListEntries(filter MemoryFilter) ([]MemoryEntry, error) {
	args := m.Called(filter)
	return args.Get(0).([]MemoryEntry), args.Error(1)
}

func (m *MockRepository) SearchEntries(query string, limit int) ([]MemoryEntry, error) {
	args := m.Called(query, limit)
	return args.Get(0).([]MemoryEntry), args.Error(1)
}

func (m *MockRepository) DeleteEntry(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockRepository) CleanOldEntries(retention time.Duration) error {
	args := m.Called(retention)
	return args.Error(0)
}

func TestNewManager(t *testing.T) {
	// Save current directory
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(originalDir)

	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "sigil-manager-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	err = os.Chdir(tempDir)
	require.NoError(t, err)

	manager, err := NewManager()
	assert.NoError(t, err)
	assert.NotNil(t, manager)

	// Check that memory directory was created
	assert.DirExists(t, filepath.Join(".", memoryDir))
}

func TestDefaultManager_StoreSession(t *testing.T) {
	mockRepo := &MockRepository{}
	manager := &DefaultManager{storage: mockRepo}

	output := model.PromptOutput{
		Response:   "Test response",
		TokensUsed: 100,
		Model:      "gpt-4",
	}

	// Set up mock expectation
	mockRepo.On("StoreEntry", mock.MatchedBy(func(entry MemoryEntry) bool {
		return entry.Type == "session" &&
			entry.Command == "ask" &&
			entry.Model == "gpt-4" &&
			entry.TokensUsed == 100 &&
			strings.Contains(entry.Content, "Test response")
	})).Return(nil)

	err := manager.StoreSession("ask", "test input", output, 5*time.Second)
	assert.NoError(t, err)

	mockRepo.AssertExpectations(t)
}

func TestDefaultManager_StoreContext(t *testing.T) {
	mockRepo := &MockRepository{}
	manager := &DefaultManager{storage: mockRepo}

	// Set up mock expectation
	mockRepo.On("StoreEntry", mock.MatchedBy(func(entry MemoryEntry) bool {
		return entry.Type == "context" &&
			entry.Content == "test context" &&
			strings.Contains(entry.Summary, "user input") &&
			entry.Tags == "user input"
	})).Return(nil)

	err := manager.StoreContext("test context", "user input")
	assert.NoError(t, err)

	mockRepo.AssertExpectations(t)
}

func TestDefaultManager_StoreSummary(t *testing.T) {
	mockRepo := &MockRepository{}
	manager := &DefaultManager{storage: mockRepo}

	// Set up mock expectation
	mockRepo.On("StoreEntry", mock.MatchedBy(func(entry MemoryEntry) bool {
		return entry.Type == "summary" &&
			entry.Content == "detailed content" &&
			entry.Summary == "brief summary"
	})).Return(nil)

	err := manager.StoreSummary("detailed content", "brief summary")
	assert.NoError(t, err)

	mockRepo.AssertExpectations(t)
}

func TestDefaultManager_StoreDecision(t *testing.T) {
	mockRepo := &MockRepository{}
	manager := &DefaultManager{storage: mockRepo}

	// Set up mock expectation
	mockRepo.On("StoreEntry", mock.MatchedBy(func(entry MemoryEntry) bool {
		return entry.Type == "decision" &&
			strings.Contains(entry.Content, "use Go") &&
			strings.Contains(entry.Content, "performance benefits") &&
			entry.Summary == "use Go"
	})).Return(nil)

	err := manager.StoreDecision("use Go", "performance benefits")
	assert.NoError(t, err)

	mockRepo.AssertExpectations(t)
}

func TestDefaultManager_GetRecentContext(t *testing.T) {
	t.Run("with storage implementation", func(t *testing.T) {
		// Test with actual Storage implementation
		tempDir, err := os.MkdirTemp("", "sigil-context-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		storage := &Storage{baseDir: tempDir}
		manager := &DefaultManager{storage: storage}

		// Store some test entries
		entries := []MemoryEntry{
			{
				ID:        "session-1",
				Type:      "session",
				Timestamp: time.Now().Add(-1 * time.Hour),
				Content:   "Session content",
			},
			{
				ID:        "context-1",
				Type:      "context",
				Timestamp: time.Now(),
				Content:   "Context content",
			},
		}

		for _, entry := range entries {
			err := storage.StoreEntry(entry)
			require.NoError(t, err)
		}

		modelEntries, err := manager.GetRecentContext(10)
		assert.NoError(t, err)
		assert.Len(t, modelEntries, 2)

		// Check format conversion
		for _, entry := range modelEntries {
			assert.NotEmpty(t, entry.Timestamp)
			assert.NotEmpty(t, entry.Content)
			assert.NotEmpty(t, entry.Type)
		}
	})

	t.Run("with mock repository fallback", func(t *testing.T) {
		mockRepo := &MockRepository{}
		manager := &DefaultManager{storage: mockRepo}

		entries := []MemoryEntry{
			{
				ID:        "session-1",
				Type:      "session",
				Timestamp: time.Now(),
				Content:   "Session content",
			},
		}

		mockRepo.On("ListEntries", mock.MatchedBy(func(filter MemoryFilter) bool {
			return len(filter.Types) == 3 &&
				filter.Limit == 5
		})).Return(entries, nil)

		modelEntries, err := manager.GetRecentContext(5)
		assert.NoError(t, err)
		assert.Len(t, modelEntries, 1)
		assert.Equal(t, "session", modelEntries[0].Type)

		mockRepo.AssertExpectations(t)
	})
}

func TestDefaultManager_GetRecentSessions(t *testing.T) {
	mockRepo := &MockRepository{}
	manager := &DefaultManager{storage: mockRepo}

	entries := []MemoryEntry{
		{
			ID:        "session-1",
			Type:      "session",
			Timestamp: time.Now(),
			Content:   "Session 1",
		},
		{
			ID:        "session-2",
			Type:      "session",
			Timestamp: time.Now().Add(-1 * time.Hour),
			Content:   "Session 2",
		},
	}

	mockRepo.On("ListEntries", mock.MatchedBy(func(filter MemoryFilter) bool {
		return len(filter.Types) == 1 &&
			filter.Types[0] == "session" &&
			filter.Limit == 10
	})).Return(entries, nil)

	result, err := manager.GetRecentSessions(10)
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "session-1", result[0].ID)
	assert.Equal(t, "session-2", result[1].ID)

	mockRepo.AssertExpectations(t)
}

func TestDefaultManager_SearchMemory(t *testing.T) {
	mockRepo := &MockRepository{}
	manager := &DefaultManager{storage: mockRepo}

	entries := []MemoryEntry{
		{
			ID:      "match-1",
			Type:    "session",
			Content: "Contains golang code",
		},
	}

	mockRepo.On("SearchEntries", "golang", 5).Return(entries, nil)

	result, err := manager.SearchMemory("golang", 5)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "match-1", result[0].ID)

	mockRepo.AssertExpectations(t)
}

func TestDefaultManager_CleanOldEntries(t *testing.T) {
	mockRepo := &MockRepository{}
	manager := &DefaultManager{storage: mockRepo}

	retention := 24 * time.Hour
	mockRepo.On("CleanOldEntries", retention).Return(nil)

	err := manager.CleanOldEntries(retention)
	assert.NoError(t, err)

	mockRepo.AssertExpectations(t)
}

func TestDefaultManager_GetStats(t *testing.T) {
	mockRepo := &MockRepository{}
	manager := &DefaultManager{storage: mockRepo}

	entries := []MemoryEntry{
		{
			ID:        "session-1",
			Type:      "session",
			Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Content:   "Session content with some text",
			Summary:   "Session summary",
		},
		{
			ID:        "context-1",
			Type:      "context",
			Timestamp: time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
			Content:   "Context content",
			Summary:   "Context summary",
		},
		{
			ID:        "session-2",
			Type:      "session",
			Timestamp: time.Date(2024, 1, 3, 12, 0, 0, 0, time.UTC),
			Content:   "Another session",
			Summary:   "Another summary",
		},
	}

	mockRepo.On("ListEntries", MemoryFilter{}).Return(entries, nil)

	stats, err := manager.GetStats()
	assert.NoError(t, err)

	assert.Equal(t, 3, stats.TotalEntries)
	assert.Equal(t, 2, stats.EntriesByType["session"])
	assert.Equal(t, 1, stats.EntriesByType["context"])
	assert.Equal(t, time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC), stats.OldestEntry)
	assert.Equal(t, time.Date(2024, 1, 3, 12, 0, 0, 0, time.UTC), stats.NewestEntry)
	assert.Greater(t, stats.TotalSizeBytes, int64(0))
	assert.Greater(t, stats.AverageSize, int64(0))

	mockRepo.AssertExpectations(t)
}

func TestDefaultManager_GetStats_EmptyEntries(t *testing.T) {
	mockRepo := &MockRepository{}
	manager := &DefaultManager{storage: mockRepo}

	entries := []MemoryEntry{}
	mockRepo.On("ListEntries", MemoryFilter{}).Return(entries, nil)

	stats, err := manager.GetStats()
	assert.NoError(t, err)

	assert.Equal(t, 0, stats.TotalEntries)
	assert.Len(t, stats.EntriesByType, 0)
	assert.Equal(t, int64(0), stats.TotalSizeBytes)
	assert.Equal(t, int64(0), stats.AverageSize)

	mockRepo.AssertExpectations(t)
}

func TestDefaultManager_ExportMemory(t *testing.T) {
	mockRepo := &MockRepository{}
	manager := &DefaultManager{storage: mockRepo}

	entries := []MemoryEntry{
		{
			ID:        "export-1",
			Type:      "session",
			Timestamp: time.Now(),
			Content:   "Export test content",
			Summary:   "Export test",
			Command:   "ask",
			Model:     "gpt-4",
		},
	}

	mockRepo.On("ListEntries", MemoryFilter{}).Return(entries, nil)

	// Test markdown export
	tempFile := filepath.Join(os.TempDir(), "test-export.md")
	defer os.Remove(tempFile)

	err := manager.ExportMemory("markdown", tempFile)
	assert.NoError(t, err)

	// Check file was created and contains expected content
	content, err := os.ReadFile(tempFile)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "# Sigil Memory Export")
	assert.Contains(t, string(content), "Export test content")
	assert.Contains(t, string(content), "**Type**: session")

	mockRepo.AssertExpectations(t)
}

func TestDefaultManager_ExportMemory_UnsupportedFormat(t *testing.T) {
	mockRepo := &MockRepository{}
	manager := &DefaultManager{storage: mockRepo}

	entries := []MemoryEntry{}
	mockRepo.On("ListEntries", MemoryFilter{}).Return(entries, nil)

	err := manager.ExportMemory("xml", "/tmp/test.xml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported export format")

	mockRepo.AssertExpectations(t)
}

func TestDefaultManager_ExportMemory_JSON(t *testing.T) {
	mockRepo := &MockRepository{}
	manager := &DefaultManager{storage: mockRepo}

	entries := []MemoryEntry{}
	mockRepo.On("ListEntries", MemoryFilter{}).Return(entries, nil)

	err := manager.ExportMemory("json", "/tmp/test.json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "JSON export not yet implemented")

	mockRepo.AssertExpectations(t)
}

func TestGetMemoryDirectory(t *testing.T) {
	dir := GetMemoryDirectory()
	assert.Contains(t, dir, ".sigil/memory")
	assert.True(t, strings.HasSuffix(dir, memoryDir))
}

func TestInitializeMemory(t *testing.T) {
	// Save current directory
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(originalDir)

	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "sigil-init-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	err = os.Chdir(tempDir)
	require.NoError(t, err)

	err = InitializeMemory()
	assert.NoError(t, err)

	// Check that directory was created
	memDir := GetMemoryDirectory()
	assert.DirExists(t, memDir)
}

func TestDefaultManager_Integration(t *testing.T) {
	// Save current directory
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(originalDir)

	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "sigil-integration-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	err = os.Chdir(tempDir)
	require.NoError(t, err)

	// Create manager
	manager, err := NewManager()
	require.NoError(t, err)

	// Store various types of entries
	output := model.PromptOutput{
		Response:   "Test response",
		TokensUsed: 50,
		Model:      "gpt-4",
	}

	err = manager.StoreSession("ask", "test input", output, 2*time.Second)
	assert.NoError(t, err)

	err = manager.StoreContext("user context", "user")
	assert.NoError(t, err)

	err = manager.StoreSummary("detailed info", "brief summary")
	assert.NoError(t, err)

	err = manager.StoreDecision("use testing", "improves code quality")
	assert.NoError(t, err)

	// Test retrieval
	sessions, err := manager.GetRecentSessions(10)
	assert.NoError(t, err)
	assert.Len(t, sessions, 1)
	assert.Equal(t, "session", sessions[0].Type)

	context, err := manager.GetRecentContext(10)
	assert.NoError(t, err)
	assert.Len(t, context, 2) // session and context types

	// Test search
	results, err := manager.SearchMemory("testing", 10)
	assert.NoError(t, err)
	assert.Len(t, results, 1) // Should find the decision entry

	// Test stats
	stats, err := manager.GetStats()
	assert.NoError(t, err)
	assert.Equal(t, 4, stats.TotalEntries)
	assert.Equal(t, 1, stats.EntriesByType["session"])
	assert.Equal(t, 1, stats.EntriesByType["context"])
	assert.Equal(t, 1, stats.EntriesByType["summary"])
	assert.Equal(t, 1, stats.EntriesByType["decision"])
}

func TestDefaultManager_ErrorHandling(t *testing.T) {
	mockRepo := &MockRepository{}
	manager := &DefaultManager{storage: mockRepo}

	// Test storage errors are properly wrapped
	mockRepo.On("StoreEntry", mock.Anything).Return(assert.AnError)

	output := model.PromptOutput{Response: "test", Model: "gpt-4"}
	err := manager.StoreSession("ask", "input", output, time.Second)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to store session")

	err = manager.StoreContext("context", "source")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to store context")

	err = manager.StoreSummary("content", "summary")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to store summary")

	err = manager.StoreDecision("decision", "reasoning")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to store decision")
}
