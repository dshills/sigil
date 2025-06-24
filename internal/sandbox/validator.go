// Package sandbox provides validation and rule enforcement
package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dshills/sigil/internal/errors"
	"github.com/dshills/sigil/internal/logger"
	"gopkg.in/yaml.v3"
)

// Validator enforces rules and validates code changes
type Validator struct {
	rules Rules
}

// Rules defines validation rules
type Rules struct {
	FileRules     []FileRule    `yaml:"file_rules"`
	ContentRules  []ContentRule `yaml:"content_rules"`
	SizeRules     SizeRule      `yaml:"size_rules"`
	SecurityRules SecurityRule  `yaml:"security_rules"`
}

// FileRule defines rules for file operations
type FileRule struct {
	Name        string   `yaml:"name"`
	PathPattern string   `yaml:"path_pattern"`
	AllowedOps  []string `yaml:"allowed_operations"`
	BlockedOps  []string `yaml:"blocked_operations"`
	Required    bool     `yaml:"required"`
	Description string   `yaml:"description"`
}

// ContentRule defines rules for file content
type ContentRule struct {
	Name            string   `yaml:"name"`
	PathPattern     string   `yaml:"path_pattern"`
	Patterns        []string `yaml:"patterns"`
	BlockedPatterns []string `yaml:"blocked_patterns"`
	Required        bool     `yaml:"required"`
	Description     string   `yaml:"description"`
}

// SizeRule defines size limitations
type SizeRule struct {
	MaxFileSize  int64 `yaml:"max_file_size"`  // bytes
	MaxTotalSize int64 `yaml:"max_total_size"` // bytes
	MaxFiles     int   `yaml:"max_files"`
}

// SecurityRule defines security-related rules
type SecurityRule struct {
	BlockedExtensions  []string `yaml:"blocked_extensions"`
	BlockedPaths       []string `yaml:"blocked_paths"`
	RequireTests       bool     `yaml:"require_tests"`
	RequireLinting     bool     `yaml:"require_linting"`
	AllowNetworkAccess bool     `yaml:"allow_network_access"`
}

// DefaultRules returns default validation rules
func DefaultRules() Rules {
	return Rules{
		FileRules: []FileRule{
			{
				Name:        "Go source files",
				PathPattern: "*.go",
				AllowedOps:  []string{"create", "update"},
				Required:    false,
				Description: "Go source code files",
			},
			{
				Name:        "Configuration files",
				PathPattern: "*.{yml,yaml,json,toml}",
				AllowedOps:  []string{"create", "update"},
				Required:    false,
				Description: "Configuration files",
			},
			{
				Name:        "Documentation",
				PathPattern: "*.{md,txt,rst}",
				AllowedOps:  []string{"create", "update"},
				Required:    false,
				Description: "Documentation files",
			},
		},
		ContentRules: []ContentRule{
			{
				Name:        "No credentials",
				PathPattern: "*",
				BlockedPatterns: []string{
					`(?i)(password|passwd|pwd)\s*[:=]\s*['""][^'""]+['""]`,
					`(?i)(api_key|apikey|access_key)\s*[:=]\s*['""][^'""]+['""]`,
					`(?i)(secret|token)\s*[:=]\s*['""][^'""]+['""]`,
					`-----BEGIN\s+(RSA\s+)?PRIVATE\s+KEY-----`,
				},
				Required:    true,
				Description: "Prevent credential exposure",
			},
			{
				Name:        "No dangerous operations",
				PathPattern: "*.go",
				BlockedPatterns: []string{
					`os\.RemoveAll\(['""]\/`,
					`exec\.Command\(['""]rm`,
					`exec\.Command\(['""]sudo`,
					`os\.Exit\(`,
				},
				Required:    true,
				Description: "Prevent dangerous system operations",
			},
		},
		SizeRules: SizeRule{
			MaxFileSize:  1024 * 1024,      // 1MB per file
			MaxTotalSize: 10 * 1024 * 1024, // 10MB total
			MaxFiles:     100,
		},
		SecurityRules: SecurityRule{
			BlockedExtensions:  []string{".exe", ".dll", ".so", ".dylib", ".bin"},
			BlockedPaths:       []string{"/etc/", "/usr/", "/bin/", "/sbin/", "C:\\Windows\\"},
			RequireTests:       false,
			RequireLinting:     false,
			AllowNetworkAccess: false,
		},
	}
}

// NewValidator creates a new validator
func NewValidator() (*Validator, error) {
	validator := &Validator{
		rules: DefaultRules(),
	}

	// Try to load custom rules
	if err := validator.LoadRules(); err != nil {
		logger.Warn("failed to load custom rules, using defaults", "error", err)
	}

	logger.Debug("initialized validator", "file_rules", len(validator.rules.FileRules), "content_rules", len(validator.rules.ContentRules))
	return validator, nil
}

// LoadRules loads validation rules from .sigil/rules.yml
func (v *Validator) LoadRules() error {
	rulesPath := filepath.Join(".sigil", "rules.yml")

	if _, err := os.Stat(rulesPath); os.IsNotExist(err) {
		// No custom rules file, use defaults
		return nil
	}

	data, err := os.ReadFile(rulesPath)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeFS, "LoadRules", "failed to read rules file")
	}

	var rules Rules
	if err := yaml.Unmarshal(data, &rules); err != nil {
		return errors.Wrap(err, errors.ErrorTypeInput, "LoadRules", "failed to parse rules file")
	}

	v.rules = rules
	logger.Info("loaded custom validation rules", "path", rulesPath)
	return nil
}

// SaveRules saves current rules to .sigil/rules.yml
func (v *Validator) SaveRules() error {
	rulesPath := filepath.Join(".sigil", "rules.yml")

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(rulesPath), 0755); err != nil {
		return errors.Wrap(err, errors.ErrorTypeFS, "SaveRules", "failed to create rules directory")
	}

	data, err := yaml.Marshal(v.rules)
	if err != nil {
		return errors.Wrap(err, errors.ErrorTypeInternal, "SaveRules", "failed to marshal rules")
	}

	if err := os.WriteFile(rulesPath, data, 0600); err != nil {
		return errors.Wrap(err, errors.ErrorTypeFS, "SaveRules", "failed to write rules file")
	}

	logger.Info("saved validation rules", "path", rulesPath)
	return nil
}

// ValidateRequest validates an execution request against rules
func (v *Validator) ValidateRequest(request ExecutionRequest) error {
	logger.Debug("validating execution request", "id", request.ID, "files", len(request.Files))

	// Validate overall limits
	if err := v.validateSizeLimits(request); err != nil {
		return errors.Wrap(err, errors.ErrorTypeValidation, "ValidateRequest", "size limit validation failed")
	}

	// Validate each file
	for _, file := range request.Files {
		if err := v.validateFile(file); err != nil {
			return errors.Wrap(err, errors.ErrorTypeValidation, "ValidateRequest",
				fmt.Sprintf("file validation failed for %s", file.Path))
		}
	}

	// Validate security rules
	if err := v.validateSecurity(request); err != nil {
		return errors.Wrap(err, errors.ErrorTypeValidation, "ValidateRequest", "security validation failed")
	}

	logger.Debug("execution request validation passed", "id", request.ID)
	return nil
}

// validateSizeLimits validates size constraints
func (v *Validator) validateSizeLimits(request ExecutionRequest) error {
	if len(request.Files) > v.rules.SizeRules.MaxFiles {
		return fmt.Errorf("too many files: %d (max: %d)", len(request.Files), v.rules.SizeRules.MaxFiles)
	}

	var totalSize int64
	for _, file := range request.Files {
		fileSize := int64(len(file.Content))

		if fileSize > v.rules.SizeRules.MaxFileSize {
			return fmt.Errorf("file %s too large: %d bytes (max: %d)", file.Path, fileSize, v.rules.SizeRules.MaxFileSize)
		}

		totalSize += fileSize
	}

	if totalSize > v.rules.SizeRules.MaxTotalSize {
		return fmt.Errorf("total size too large: %d bytes (max: %d)", totalSize, v.rules.SizeRules.MaxTotalSize)
	}

	return nil
}

// validateFile validates a single file change
func (v *Validator) validateFile(file FileChange) error {
	// Check file rules
	for _, rule := range v.rules.FileRules {
		if matched, err := filepath.Match(rule.PathPattern, file.Path); err != nil {
			logger.Warn("invalid path pattern", "pattern", rule.PathPattern, "error", err)
			continue
		} else if matched {
			if err := v.validateFileRule(file, rule); err != nil {
				return err
			}
		}
	}

	// Check content rules
	for _, rule := range v.rules.ContentRules {
		if matched, err := filepath.Match(rule.PathPattern, file.Path); err != nil {
			logger.Warn("invalid path pattern", "pattern", rule.PathPattern, "error", err)
			continue
		} else if matched {
			if err := v.validateContentRule(file, rule); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateFileRule validates against a file rule
func (v *Validator) validateFileRule(file FileChange, rule FileRule) error {
	operation := string(file.Operation)

	// Check if operation is blocked
	for _, blocked := range rule.BlockedOps {
		if operation == blocked {
			return fmt.Errorf("operation %s not allowed for %s (rule: %s)", operation, file.Path, rule.Name)
		}
	}

	// Check if operation is allowed (if allowlist exists)
	if len(rule.AllowedOps) > 0 {
		allowed := false
		for _, allowedOp := range rule.AllowedOps {
			if operation == allowedOp {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("operation %s not allowed for %s (rule: %s)", operation, file.Path, rule.Name)
		}
	}

	return nil
}

// validateContentRule validates against a content rule
func (v *Validator) validateContentRule(file FileChange, rule ContentRule) error {
	content := file.Content

	// Check blocked patterns
	for _, pattern := range rule.BlockedPatterns {
		if matched, err := regexp.MatchString(pattern, content); err != nil {
			logger.Warn("invalid regex pattern", "pattern", pattern, "error", err)
			continue
		} else if matched {
			return fmt.Errorf("content in %s matches blocked pattern (rule: %s)", file.Path, rule.Name)
		}
	}

	// Check required patterns
	for _, pattern := range rule.Patterns {
		if matched, err := regexp.MatchString(pattern, content); err != nil {
			logger.Warn("invalid regex pattern", "pattern", pattern, "error", err)
			continue
		} else if !matched && rule.Required {
			return fmt.Errorf("content in %s missing required pattern (rule: %s)", file.Path, rule.Name)
		}
	}

	return nil
}

// validateSecurity validates security rules
func (v *Validator) validateSecurity(request ExecutionRequest) error {
	for _, file := range request.Files {
		// Check blocked extensions
		ext := strings.ToLower(filepath.Ext(file.Path))
		for _, blocked := range v.rules.SecurityRules.BlockedExtensions {
			if ext == blocked {
				return fmt.Errorf("file extension %s not allowed for %s", ext, file.Path)
			}
		}

		// Check blocked paths
		for _, blocked := range v.rules.SecurityRules.BlockedPaths {
			if strings.HasPrefix(file.Path, blocked) {
				return fmt.Errorf("path %s not allowed (blocked: %s)", file.Path, blocked)
			}
		}
	}

	return nil
}

// GetRules returns current validation rules
func (v *Validator) GetRules() Rules {
	return v.rules
}

// UpdateRules updates validation rules
func (v *Validator) UpdateRules(rules Rules) error {
	v.rules = rules
	return v.SaveRules()
}

// ValidateCode validates code content without execution
func (v *Validator) ValidateCode(path string, content string) error {
	file := FileChange{
		Path:      path,
		Content:   content,
		Operation: OperationUpdate,
	}

	return v.validateFile(file)
}

// GetRulesForPath returns applicable rules for a given path
func (v *Validator) GetRulesForPath(path string) ([]FileRule, []ContentRule) {
	var fileRules []FileRule
	var contentRules []ContentRule

	for _, rule := range v.rules.FileRules {
		if matched, err := filepath.Match(rule.PathPattern, path); err == nil && matched {
			fileRules = append(fileRules, rule)
		}
	}

	for _, rule := range v.rules.ContentRules {
		if matched, err := filepath.Match(rule.PathPattern, path); err == nil && matched {
			contentRules = append(contentRules, rule)
		}
	}

	return fileRules, contentRules
}
