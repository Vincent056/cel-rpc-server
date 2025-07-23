package main

import (
	"encoding/json" // still used for some operations like ImportRules fallback

	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	celv1 "github.com/Vincent056/cel-rpc-server/gen/cel/v1"

	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"
	"gopkg.in/yaml.v3"
)

// parseLegacyRuleJSON converts old rule JSON format (with InputType wrapper) to CELRule
func parseLegacyRuleJSON(data []byte) (*celv1.CELRule, error) {
	// Define minimal legacy structures to map the old JSON
	type legacyInputType struct {
		Kubernetes *celv1.KubernetesInput `json:"Kubernetes,omitempty"`
		File       *celv1.FileInput       `json:"File,omitempty"`
		Http       *celv1.HttpInput       `json:"Http,omitempty"`
	}
	type legacyRuleInput struct {
		Name      string          `json:"name"`
		InputType legacyInputType `json:"InputType"`
	}
	type legacyRule struct {
		Id             string                `json:"id"`
		Name           string                `json:"name"`
		Description    string                `json:"description"`
		Expression     string                `json:"expression"`
		Inputs         []legacyRuleInput     `json:"inputs"`
		Tags           []string              `json:"tags"`
		Category       string                `json:"category"`
		Severity       string                `json:"severity"`
		TestCases      []*celv1.RuleTestCase `json:"test_cases"`
		Metadata       *celv1.RuleMetadata   `json:"metadata"`
		CreatedAt      int64                 `json:"created_at"`
		UpdatedAt      int64                 `json:"updated_at"`
		CreatedBy      string                `json:"created_by"`
		LastModifiedBy string                `json:"last_modified_by"`
	}

	var lr legacyRule
	if err := json.Unmarshal(data, &lr); err != nil {
		return nil, err
	}

	// Convert to new structure
	cr := &celv1.CELRule{
		Id:             lr.Id,
		Name:           lr.Name,
		Description:    lr.Description,
		Expression:     lr.Expression,
		Tags:           lr.Tags,
		Category:       lr.Category,
		Severity:       lr.Severity,
		TestCases:      lr.TestCases,
		Metadata:       lr.Metadata,
		CreatedAt:      lr.CreatedAt,
		UpdatedAt:      lr.UpdatedAt,
		CreatedBy:      lr.CreatedBy,
		LastModifiedBy: lr.LastModifiedBy,
	}

	for _, li := range lr.Inputs {
		ri := celv1.RuleInput{Name: li.Name}
		if li.InputType.Kubernetes != nil {
			ri.InputType = &celv1.RuleInput_Kubernetes{Kubernetes: li.InputType.Kubernetes}
		} else if li.InputType.File != nil {
			ri.InputType = &celv1.RuleInput_File{File: li.InputType.File}
		} else if li.InputType.Http != nil {
			ri.InputType = &celv1.RuleInput_Http{Http: li.InputType.Http}
		}
		cr.Inputs = append(cr.Inputs, &ri)
	}

	return cr, nil
}

// RuleStore interface defines the operations for rule storage
type RuleStore interface {
	Save(rule *celv1.CELRule) error
	Get(id string) (*celv1.CELRule, error)
	List(filter *celv1.ListRulesFilter, pageSize int32, pageToken string, sortBy string, ascending bool) ([]*celv1.CELRule, string, int32, error)
	Update(rule *celv1.CELRule, updateFields []string) error
	Delete(id string) error
	ExportRules(ruleIDs []string, filter *celv1.ListRulesFilter, format celv1.ExportFormat, includeTestCases, includeMetadata bool) ([]byte, string, int32, error)
	ImportRules(data []byte, format celv1.ImportFormat, options *celv1.ImportOptions) (int32, int32, int32, []*celv1.ImportResult, error)
}

// FileRuleStore implements RuleStore using file system storage
type FileRuleStore struct {
	mu       sync.RWMutex
	basePath string
	rules    map[string]*celv1.CELRule
	index    *ruleIndex
}

type ruleIndex struct {
	byTag       map[string][]string
	byCategory  map[string][]string
	bySeverity  map[string][]string
	byFramework map[string][]string
	byResource  map[string][]string
}

// NewFileRuleStore creates a new file-based rule store
func NewFileRuleStore(basePath string) (*FileRuleStore, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create rule store directory: %w", err)
	}

	store := &FileRuleStore{
		basePath: basePath,
		rules:    make(map[string]*celv1.CELRule),
		index: &ruleIndex{
			byTag:       make(map[string][]string),
			byCategory:  make(map[string][]string),
			bySeverity:  make(map[string][]string),
			byFramework: make(map[string][]string),
			byResource:  make(map[string][]string),
		},
	}

	// Load existing rules
	if err := store.loadRules(); err != nil {
		return nil, fmt.Errorf("failed to load existing rules: %w", err)
	}

	return store, nil
}

func (s *FileRuleStore) loadRules() error {
	files, err := ioutil.ReadDir(s.basePath)
	if err != nil {
		return err
	}

	log.Printf("[FileRuleStore] Loading rules from %s, found %d files", s.basePath, len(files))

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".json") {
			filePath := filepath.Join(s.basePath, file.Name())
			data, err := ioutil.ReadFile(filePath)
			if err != nil {
				log.Printf("[FileRuleStore] Failed to read file %s: %v", filePath, err)
				continue
			}

			var rule celv1.CELRule
			if err := protojson.Unmarshal(data, &rule); err != nil {
				// Attempt legacy JSON format fallback
				legacyRule, legacyErr := parseLegacyRuleJSON(data)
				if legacyErr != nil {
					log.Printf("[FileRuleStore] Failed to unmarshal rule from %s: %v (legacyErr=%v)", filePath, err, legacyErr)
					continue
				}
				rule = *legacyRule
			}

			log.Printf("[FileRuleStore] Loaded rule: ID=%s, Name=%s", rule.Id, rule.Name)
			s.rules[rule.Id] = &rule
			s.updateIndex(&rule)
		}
	}

	log.Printf("[FileRuleStore] Loaded %d rules total", len(s.rules))
	return nil
}

func (s *FileRuleStore) updateIndex(rule *celv1.CELRule) {
	// Update tag index
	for _, tag := range rule.Tags {
		s.index.byTag[tag] = append(s.index.byTag[tag], rule.Id)
	}

	// Update category index
	if rule.Category != "" {
		s.index.byCategory[rule.Category] = append(s.index.byCategory[rule.Category], rule.Id)
	}

	// Update severity index
	if rule.Severity != "" {
		s.index.bySeverity[rule.Severity] = append(s.index.bySeverity[rule.Severity], rule.Id)
	}

	// Update compliance framework index
	if rule.Metadata != nil && rule.Metadata.ComplianceFramework != "" {
		s.index.byFramework[rule.Metadata.ComplianceFramework] = append(s.index.byFramework[rule.Metadata.ComplianceFramework], rule.Id)
	}

	// Update resource type index
	for _, input := range rule.Inputs {
		if k8s := input.GetKubernetes(); k8s != nil {
			resourceType := k8s.Resource
			s.index.byResource[resourceType] = append(s.index.byResource[resourceType], rule.Id)
		}
	}
}

func (s *FileRuleStore) Save(rule *celv1.CELRule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if rule.Id == "" {
		rule.Id = uuid.New().String()
	}

	rule.CreatedAt = time.Now().Unix()
	rule.UpdatedAt = rule.CreatedAt

	// Save to file using protojson with indentation
	mo := protojson.MarshalOptions{UseProtoNames: true, Indent: "  "}
	data, err := mo.Marshal(rule)
	if err != nil {
		return fmt.Errorf("failed to marshal rule: %w", err)
	}

	filename := filepath.Join(s.basePath, rule.Id+".json")
	if err := ioutil.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write rule file: %w", err)
	}

	s.rules[rule.Id] = rule
	s.updateIndex(rule)

	return nil
}

func (s *FileRuleStore) Get(id string) (*celv1.CELRule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rule, exists := s.rules[id]
	if !exists {
		return nil, fmt.Errorf("rule not found: %s", id)
	}

	return rule, nil
}

func (s *FileRuleStore) List(filter *celv1.ListRulesFilter, pageSize int32, pageToken string, sortBy string, ascending bool) ([]*celv1.CELRule, string, int32, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	log.Printf("[FileRuleStore] List called with pageSize=%d, total rules in store=%d", pageSize, len(s.rules))

	var filteredRules []*celv1.CELRule
	ruleSet := make(map[string]bool)

	// Apply filters
	if filter != nil {
		log.Printf("[FileRuleStore] Applying filters: tags=%v, category=%s, severity=%s", filter.Tags, filter.Category, filter.Severity)

		// Check if any filter criteria are specified
		hasFilterCriteria := len(filter.Tags) > 0 || filter.Category != "" || filter.Severity != "" ||
			filter.ComplianceFramework != "" || filter.ResourceType != "" || filter.SearchText != "" || filter.VerifiedOnly

		if !hasFilterCriteria {
			// No filter criteria specified, return all rules
			log.Printf("[FileRuleStore] No filter criteria specified, returning all rules")
			for _, rule := range s.rules {
				filteredRules = append(filteredRules, rule)
			}
		} else {
			// Apply specific filters
			// Filter by tags
			if len(filter.Tags) > 0 {
				for _, tag := range filter.Tags {
					for _, ruleID := range s.index.byTag[tag] {
						ruleSet[ruleID] = true
					}
				}
			}

			// Filter by category
			if filter.Category != "" {
				for _, ruleID := range s.index.byCategory[filter.Category] {
					ruleSet[ruleID] = true
				}
			}

			// Filter by severity
			if filter.Severity != "" {
				for _, ruleID := range s.index.bySeverity[filter.Severity] {
					ruleSet[ruleID] = true
				}
			}

			// Filter by compliance framework
			if filter.ComplianceFramework != "" {
				for _, ruleID := range s.index.byFramework[filter.ComplianceFramework] {
					ruleSet[ruleID] = true
				}
			}

			// Filter by resource type
			if filter.ResourceType != "" {
				for _, ruleID := range s.index.byResource[filter.ResourceType] {
					ruleSet[ruleID] = true
				}
			}

			// Apply text search
			if filter.SearchText != "" {
				searchLower := strings.ToLower(filter.SearchText)
				for _, rule := range s.rules {
					if strings.Contains(strings.ToLower(rule.Name), searchLower) ||
						strings.Contains(strings.ToLower(rule.Description), searchLower) ||
						strings.Contains(strings.ToLower(rule.Expression), searchLower) {
						ruleSet[rule.Id] = true
					}
				}
			}

			// Filter by verified status
			if filter.VerifiedOnly {
				for ruleID := range ruleSet {
					if rule, exists := s.rules[ruleID]; exists && !rule.IsVerified {
						delete(ruleSet, ruleID)
					}
				}
			}

			// Build filtered list
			for ruleID := range ruleSet {
				if rule, exists := s.rules[ruleID]; exists {
					filteredRules = append(filteredRules, rule)
				}
			}
		} // Close the else block for hasFilterCriteria
	} else {
		// No filter, return all rules
		for _, rule := range s.rules {
			filteredRules = append(filteredRules, rule)
		}
	}

	// TODO: Implement sorting and pagination
	totalCount := int32(len(filteredRules))

	// Simple pagination
	start := 0
	if pageToken != "" {
		// Parse page token (simplified)
		fmt.Sscanf(pageToken, "%d", &start)
	}

	end := start + int(pageSize)
	if end > len(filteredRules) {
		end = len(filteredRules)
	}

	var nextPageToken string
	if end < len(filteredRules) {
		nextPageToken = fmt.Sprintf("%d", end)
	}

	return filteredRules[start:end], nextPageToken, totalCount, nil
}

func (s *FileRuleStore) Update(rule *celv1.CELRule, updateFields []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, exists := s.rules[rule.Id]
	if !exists {
		return fmt.Errorf("rule not found: %s", rule.Id)
	}

	// Update only specified fields
	if len(updateFields) > 0 {
		// Create a copy of existing rule
		updated := *existing

		for _, field := range updateFields {
			switch field {
			case "name":
				updated.Name = rule.Name
			case "description":
				updated.Description = rule.Description
			case "expression":
				updated.Expression = rule.Expression
			case "inputs":
				updated.Inputs = rule.Inputs
			case "tags":
				updated.Tags = rule.Tags
			case "category":
				updated.Category = rule.Category
			case "severity":
				updated.Severity = rule.Severity
			case "test_cases":
				updated.TestCases = rule.TestCases
			case "metadata":
				updated.Metadata = rule.Metadata
			case "is_verified":
				updated.IsVerified = rule.IsVerified
			}
		}

		rule = &updated
	}

	rule.UpdatedAt = time.Now().Unix()

	// Save to file using protojson with indentation
	mo := protojson.MarshalOptions{UseProtoNames: true, Indent: "  "}
	data, err := mo.Marshal(rule)
	if err != nil {
		return fmt.Errorf("failed to marshal rule: %w", err)
	}

	filename := filepath.Join(s.basePath, rule.Id+".json")
	if err := ioutil.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write rule file: %w", err)
	}

	s.rules[rule.Id] = rule
	// TODO: Update index properly (remove old entries first)
	s.updateIndex(rule)

	return nil
}

func (s *FileRuleStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.rules[id]; !exists {
		return fmt.Errorf("rule not found: %s", id)
	}

	filename := filepath.Join(s.basePath, id+".json")
	if err := os.Remove(filename); err != nil {
		return fmt.Errorf("failed to delete rule file: %w", err)
	}

	delete(s.rules, id)
	// TODO: Clean up index

	return nil
}

func (s *FileRuleStore) ExportRules(ruleIDs []string, filter *celv1.ListRulesFilter, format celv1.ExportFormat, includeTestCases, includeMetadata bool) ([]byte, string, int32, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var rulesToExport []*celv1.CELRule

	if len(ruleIDs) > 0 {
		// Export specific rules
		for _, id := range ruleIDs {
			if rule, exists := s.rules[id]; exists {
				rulesToExport = append(rulesToExport, rule)
			}
		}
	} else if filter != nil {
		// Export filtered rules
		rules, _, _, err := s.List(filter, 10000, "", "", true)
		if err != nil {
			return nil, "", 0, err
		}
		rulesToExport = rules
	} else {
		// Export all rules
		for _, rule := range s.rules {
			rulesToExport = append(rulesToExport, rule)
		}
	}

	// Prepare rules for export
	exportRules := make([]*celv1.CELRule, len(rulesToExport))
	for i, rule := range rulesToExport {
		exportRule := &celv1.CELRule{
			Id:          rule.Id,
			Name:        rule.Name,
			Description: rule.Description,
			Expression:  rule.Expression,
			Inputs:      rule.Inputs,
			Tags:        rule.Tags,
			Category:    rule.Category,
			Severity:    rule.Severity,
		}

		if includeTestCases {
			exportRule.TestCases = rule.TestCases
		}

		if includeMetadata {
			exportRule.Metadata = rule.Metadata
		}

		exportRules[i] = exportRule
	}

	var data []byte
	var contentType string
	var err error

	switch format {
	case celv1.ExportFormat_EXPORT_FORMAT_JSON:
		mo := protojson.MarshalOptions{UseProtoNames: true, Indent: "  "}
		var parts []string
		for _, r := range exportRules {
			b, mErr := mo.Marshal(r)
			if mErr != nil {
				err = mErr
				break
			}
			parts = append(parts, string(b))
		}
		if err == nil {
			data = []byte("[\n  " + strings.Join(parts, ",\n  ") + "\n]")
		}
		contentType = "application/json"
	case celv1.ExportFormat_EXPORT_FORMAT_YAML:
		data, err = yaml.Marshal(exportRules)
		contentType = "application/yaml"
	default:
		return nil, "", 0, fmt.Errorf("unsupported export format: %v", format)
	}

	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to marshal rules: %w", err)
	}

	return data, contentType, int32(len(exportRules)), nil
}

func (s *FileRuleStore) ImportRules(data []byte, format celv1.ImportFormat, options *celv1.ImportOptions) (int32, int32, int32, []*celv1.ImportResult, error) {
	var rules []*celv1.CELRule
	var err error

	// Auto-detect format if needed
	if format == celv1.ImportFormat_IMPORT_FORMAT_AUTO_DETECT {
		if json.Valid(data) {
			format = celv1.ImportFormat_IMPORT_FORMAT_JSON
		} else {
			format = celv1.ImportFormat_IMPORT_FORMAT_YAML
		}
	}

	// Parse data
	switch format {
	case celv1.ImportFormat_IMPORT_FORMAT_JSON:
		err = json.Unmarshal(data, &rules)
	case celv1.ImportFormat_IMPORT_FORMAT_YAML:
		err = yaml.Unmarshal(data, &rules)
	default:
		return 0, 0, 0, nil, fmt.Errorf("unsupported import format: %v", format)
	}

	if err != nil {
		return 0, 0, 0, nil, fmt.Errorf("failed to unmarshal rules: %w", err)
	}

	var imported, skipped, failed int32
	var results []*celv1.ImportResult

	for _, rule := range rules {
		result := &celv1.ImportResult{
			RuleName: rule.Name,
		}

		// Check if rule exists
		_, exists := s.rules[rule.Id]
		if exists && !options.OverwriteExisting {
			result.Success = false
			result.Error = "rule already exists"
			skipped++
		} else {
			// Apply options
			if options.CategoryOverride != "" {
				rule.Category = options.CategoryOverride
			}

			if len(options.TagsToAdd) > 0 {
				rule.Tags = append(rule.Tags, options.TagsToAdd...)
			}

			// Save rule
			if exists {
				err = s.Update(rule, nil)
			} else {
				err = s.Save(rule)
			}

			if err != nil {
				result.Success = false
				result.Error = err.Error()
				failed++

				if !options.SkipOnError {
					return imported, skipped, failed, results, err
				}
			} else {
				result.Success = true
				result.RuleId = rule.Id
				imported++
			}
		}

		results = append(results, result)
	}

	return imported, skipped, failed, results, nil
}
