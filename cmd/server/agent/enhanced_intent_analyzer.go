package agent

import (
	"context"
	"fmt"
	"log"
)

// InformationNeed represents a detected need for additional information
type InformationNeed struct {
	Type        string                 `json:"type"`         // "documentation", "api_spec", "compliance_standard", etc.
	Topic       string                 `json:"topic"`        // What information is needed
	Priority    int                    `json:"priority"`     // How critical this information is (1-10)
	SearchTerms []string               `json:"search_terms"` // Terms to search for
	Context     map[string]interface{} `json:"context"`      // Additional context for the search
}

// EnhancedIntent extends the basic Intent with information-seeking capabilities
type EnhancedIntent struct {
	*Intent
	InformationNeeds []InformationNeed `json:"information_needs"`
	RequiresResearch bool              `json:"requires_research"`
	ResearchPhase    string            `json:"research_phase"` // "needed", "in_progress", "completed"
}

// EnhancedAnalysisResult holds both the enhanced intent and analyzed context
type EnhancedAnalysisResult struct {
	EnhancedIntent  *EnhancedIntent  `json:"enhanced_intent"`
	AnalyzedContext *AnalyzedContext `json:"analyzed_context"`
}

// InformationGatherer handles fetching and analyzing external information
type InformationGatherer interface {
	SearchDocumentation(ctx context.Context, terms []string, domain string) ([]DocumentResult, error)
	AnalyzeDocuments(ctx context.Context, documents []DocumentResult, intent *EnhancedIntent) (*AnalyzedContext, error)
}

// DocumentResult represents a found document or information source
type DocumentResult struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Source  string `json:"source"`
	URL     string `json:"url,omitempty"`
}

// AnalyzedContext represents processed information ready for rule generation
type AnalyzedContext struct {
	Summary          string                 `json:"summary"`
	KeyRequirements  []string               `json:"key_requirements"`
	Constraints      []string               `json:"constraints"`
	RecommendedRules []RuleRecommendation   `json:"recommended_rules"`
	Context          map[string]interface{} `json:"context"`
}

// RuleRecommendation suggests specific rules based on analyzed information
type RuleRecommendation struct {
	Type        string                 `json:"type"`
	Description string                 `json:"description"`
	CELPattern  string                 `json:"cel_pattern,omitempty"`
	Priority    int                    `json:"priority"`
	Rationale   string                 `json:"rationale"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// EnhancedIntentAnalyzer provides advanced intent analysis with information-seeking
type EnhancedIntentAnalyzer struct {
	basicAnalyzer       *IntentAnalyzer
	informationGatherer InformationGatherer
	llmClient           LLMClient
}

// NewEnhancedIntentAnalyzer creates a new enhanced intent analyzer
func NewEnhancedIntentAnalyzer(llmClient LLMClient, gatherer InformationGatherer) *EnhancedIntentAnalyzer {
	return &EnhancedIntentAnalyzer{
		basicAnalyzer:       NewIntentAnalyzer(llmClient),
		informationGatherer: gatherer,
		llmClient:           llmClient,
	}
}

// AnalyzeIntentWithResearch performs comprehensive intent analysis with information gathering
func (e *EnhancedIntentAnalyzer) AnalyzeIntentWithResearch(ctx context.Context, message string, context map[string]interface{}) (*EnhancedAnalysisResult, error) {
	log.Printf("[EnhancedIntentAnalyzer] Starting comprehensive intent analysis for: %s", message)

	// Step 1: Basic intent analysis
	basicIntent, err := e.basicAnalyzer.AnalyzeIntent(ctx, message, context)
	if err != nil {
		return nil, fmt.Errorf("basic intent analysis failed: %w", err)
	}

	// Step 2: Detect information needs
	informationNeeds, err := e.detectInformationNeeds(ctx, message, basicIntent, context)
	if err != nil {
		log.Printf("[EnhancedIntentAnalyzer] Warning: Failed to detect information needs: %v", err)
		// Continue without information needs detection
		informationNeeds = []InformationNeed{}
	}

	enhancedIntent := &EnhancedIntent{
		Intent:           basicIntent,
		InformationNeeds: informationNeeds,
		RequiresResearch: len(informationNeeds) > 0,
		ResearchPhase:    "needed",
	}

	// Initialize result with enhanced intent
	result := &EnhancedAnalysisResult{
		EnhancedIntent:  enhancedIntent,
		AnalyzedContext: nil, // Will be set if research is performed
	}

	// Step 3: If research is needed, gather information
	if enhancedIntent.RequiresResearch && e.informationGatherer != nil {
		log.Printf("[EnhancedIntentAnalyzer] Research required, gathering information...")
		enhancedIntent.ResearchPhase = "in_progress"

		analyzedContext, err := e.gatherAndAnalyzeInformation(ctx, enhancedIntent)
		if err != nil {
			log.Printf("[EnhancedIntentAnalyzer] Warning: Information gathering failed: %v", err)
			// Continue without research results
		} else {
			// Store the analyzed context in the result
			result.AnalyzedContext = analyzedContext
			// Enhance the intent with research results
			e.enrichIntentWithResearch(enhancedIntent, analyzedContext)
			enhancedIntent.ResearchPhase = "completed"
			log.Printf("[EnhancedIntentAnalyzer] Enhanced intent with %d rule recommendations and research context", len(analyzedContext.RecommendedRules))
			log.Printf("[EnhancedIntentAnalyzer] Research completed successfully")
		}
	}

	log.Printf("[EnhancedIntentAnalyzer] Enhanced intent analysis completed. Research required: %v, Phase: %s",
		enhancedIntent.RequiresResearch, enhancedIntent.ResearchPhase)

	return result, nil
}

// detectInformationNeeds identifies what additional information is required
func (e *EnhancedIntentAnalyzer) detectInformationNeeds(ctx context.Context, message string, intent *Intent, context map[string]interface{}) ([]InformationNeed, error) {
	prompt := fmt.Sprintf(`Analyze this request and determine what additional information is needed before proceeding with rule generation.

Consider these scenarios where research is needed:
1. Platform-specific features (OpenShift, Kubernetes distributions, cloud providers)
2. Configuration guidelines for specific technologies
3. Security best practices for specific technologies
4. API specifications or configuration format, API Schema.
5. Domain-specific knowledge (networking, storage, authentication, etc.)

IMPORTANT: You MUST return a JSON array, even if there's only one item or no items.

For each information need, provide:
- type: "documentation", "api_spec", "best_practices", "configuration_guide"
- topic: Brief description of what information is needed
- priority: 1-10 (10 = critical, cannot proceed without it)
- search_terms: Array of specific terms to search for
- context: Additional context for the search

Return empty array [] if no additional information is needed.

Request: %s
Current Intent: %v
Context: %v

Response format (MUST be a JSON array):
[
  {
    "type": "documentation",
    "topic": "OpenShift kubeadmin user management",
    "priority": 9,
    "search_terms": ["how to disable kubeadmin on openshift", "how setup tls on log forwarding", "What is api schema for Kubernetes Pod resource"],
    "context": {"platform": "openshift"}
  }
]

If no research is needed, return: []`, message, intent, context)

	var needs []InformationNeed
	err := e.llmClient.Analyze(ctx, prompt, &needs)
	if err != nil {
		// Try to handle case where AI returns a single object instead of array
		log.Printf("[EnhancedIntentAnalyzer] Array parsing failed, trying single object: %v", err)

		var singleNeed InformationNeed
		err2 := e.llmClient.Analyze(ctx, prompt, &singleNeed)
		if err2 != nil {
			log.Printf("[EnhancedIntentAnalyzer] Single object parsing also failed: %v", err2)
			return nil, fmt.Errorf("failed to detect information needs: %w", err)
		}

		// Successfully parsed as single object, convert to array
		needs = []InformationNeed{singleNeed}
		log.Printf("[EnhancedIntentAnalyzer] Successfully converted single object to array")
	}

	log.Printf("[EnhancedIntentAnalyzer] Detected %d information needs", len(needs))
	for _, need := range needs {
		log.Printf("[EnhancedIntentAnalyzer] Need: %s (priority %d) - %s", need.Type, need.Priority, need.Topic)
	}

	return needs, nil
}

// gatherAndAnalyzeInformation collects and processes required information
func (e *EnhancedIntentAnalyzer) gatherAndAnalyzeInformation(ctx context.Context, intent *EnhancedIntent) (*AnalyzedContext, error) {
	var allDocuments []DocumentResult

	// Gather information for each identified need
	for _, need := range intent.InformationNeeds {
		if need.Priority < 5 {
			log.Printf("[EnhancedIntentAnalyzer] Skipping low-priority information need: %s", need.Topic)
			continue
		}

		log.Printf("[EnhancedIntentAnalyzer] Gathering information for: %s", need.Topic)

		domain := "general"
		if domainVal, ok := need.Context["domain"]; ok {
			if domainStr, ok := domainVal.(string); ok {
				domain = domainStr
			}
		}

		documents, err := e.informationGatherer.SearchDocumentation(ctx, need.SearchTerms, domain)
		if err != nil {
			log.Printf("[EnhancedIntentAnalyzer] Warning: Failed to gather information for %s: %v", need.Topic, err)
			continue
		}

		allDocuments = append(allDocuments, documents...)
		log.Printf("[EnhancedIntentAnalyzer] Found %d documents for %s", len(documents), need.Topic)
	}

	if len(allDocuments) == 0 {
		return nil, fmt.Errorf("no information could be gathered")
	}

	// Analyze gathered documents
	log.Printf("[EnhancedIntentAnalyzer] Analyzing %d gathered documents", len(allDocuments))
	analyzedContext, err := e.informationGatherer.AnalyzeDocuments(ctx, allDocuments, intent)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze gathered documents: %w", err)
	}

	return analyzedContext, nil
}

// enrichIntentWithResearch enhances the intent with research findings
func (e *EnhancedIntentAnalyzer) enrichIntentWithResearch(intent *EnhancedIntent, analyzedContext *AnalyzedContext) {
	// Add research context to the intent
	if intent.Context == nil {
		intent.Context = make(map[string]interface{})
	}

	intent.Context["research_summary"] = analyzedContext.Summary
	intent.Context["key_requirements"] = analyzedContext.KeyRequirements
	intent.Context["constraints"] = analyzedContext.Constraints
	intent.Context["has_research_context"] = true

	// Convert rule recommendations to suggested tasks
	for _, rec := range analyzedContext.RecommendedRules {
		taskSuggestion := TaskSuggestion{
			Type:        TaskType(rec.Type),
			Priority:    rec.Priority,
			Description: rec.Description,
			Parameters:  rec.Parameters,
		}
		intent.SuggestedTasks = append(intent.SuggestedTasks, taskSuggestion)
	}

	// Update required steps to include research-informed steps
	if len(analyzedContext.KeyRequirements) > 0 {
		intent.SuggestedTasks = append([]TaskSuggestion{{
			Type:        TaskType("apply_research_context"),
			Priority:    10,
			Description: "Apply research context to the intent",
			Parameters:  map[string]interface{}{},
		}}, intent.SuggestedTasks...)
	}

	log.Printf("[EnhancedIntentAnalyzer] Enhanced intent with %d rule recommendations and research context",
		len(analyzedContext.RecommendedRules))
}
