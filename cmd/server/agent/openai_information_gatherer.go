package agent

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// OpenAISearchResult represents a search result from OpenAI Search API
type OpenAISearchResult struct {
	Title   string  `json:"title"`
	Content string  `json:"content"`
	URL     string  `json:"url"`
	Score   float64 `json:"score"`
	Source  string  `json:"source"`
}

// OpenAISearchResponse represents the response from OpenAI Search API
type OpenAISearchResponse struct {
	Results []OpenAISearchResult `json:"results"`
	Query   string               `json:"query"`
	Total   int                  `json:"total"`
}

// OpenAIInformationGatherer implements InformationGatherer using OpenAI Search API
type OpenAIInformationGatherer struct {
	apiKey     string
	llmClient  LLMClient
	httpClient *http.Client
	baseURL    string
}

// NewOpenAIInformationGatherer creates a new OpenAI-powered information gatherer
func NewOpenAIInformationGatherer(apiKey string, llmClient LLMClient) *OpenAIInformationGatherer {
	return &OpenAIInformationGatherer{
		apiKey:    apiKey,
		llmClient: llmClient,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: "https://api.openai.com/v1/search", // Note: This is a hypothetical endpoint
	}
}

// SearchDocumentation searches for documentation using OpenAI Search API
func (o *OpenAIInformationGatherer) SearchDocumentation(ctx context.Context, terms []string, domain string) ([]DocumentResult, error) {
	log.Printf("[OpenAIInformationGatherer] Searching for documentation: terms=%v, domain=%s", terms, domain)

	// Construct search query with domain context
	query := o.buildSearchQuery(terms, domain)

	// For now, we'll use a web search approach since OpenAI doesn't have a dedicated search API
	// In practice, you might want to use a combination of:
	// 1. Web search APIs (Google Custom Search, Bing Search API)
	// 2. Documentation-specific APIs (GitHub API for docs, official API docs)
	// 3. Vector databases with pre-indexed documentation

	// Simulate OpenAI-powered search by using the LLM to generate relevant documentation
	results, err := o.simulateDocumentationSearch(ctx, query, domain)
	if err != nil {
		return nil, fmt.Errorf("failed to search documentation: %w", err)
	}

	// Convert to DocumentResult format
	var documents []DocumentResult
	for _, result := range results {
		documents = append(documents, DocumentResult{
			Title:   result.Title,
			Content: result.Content,
			Source:  result.Source,
			URL:     result.URL,
		})
	}

	log.Printf("[OpenAIInformationGatherer] Found %d documents for query: %s", len(documents), query)
	return documents, nil
}

// buildSearchQuery constructs an optimized search query
func (o *OpenAIInformationGatherer) buildSearchQuery(terms []string, domain string) string {
	// Add domain-specific context to improve search results
	domainContext := map[string][]string{
		"kubernetes": {"kubernetes", "k8s", "kubectl", "pod", "deployment"},
		"openshift":  {"openshift", "oc", "cluster", "route", "project"},
		"security":   {"security", "rbac", "policy", "compliance", "cve"},
		"networking": {"network", "service", "ingress", "dns", "proxy"},
		"storage":    {"storage", "volume", "pv", "pvc", "storageclass"},
	}

	query := strings.Join(terms, " ")

	// Add domain-specific terms if available
	if contextTerms, exists := domainContext[domain]; exists {
		// Add most relevant context terms
		for i, term := range contextTerms {
			if i >= 2 { // Limit to 2 additional context terms
				break
			}
			if !strings.Contains(query, term) {
				query += " " + term
			}
		}
	}

	// Add documentation-specific terms to improve relevance
	query += " documentation guide tutorial"

	return query
}

// simulateDocumentationSearch uses LLM to generate relevant documentation content
// In a real implementation, this would call actual search APIs
func (o *OpenAIInformationGatherer) simulateDocumentationSearch(ctx context.Context, query string, domain string) ([]OpenAISearchResult, error) {
	prompt := fmt.Sprintf(`You are a documentation search engine. Generate realistic documentation content for the following search query.

Search Query: %s
Domain: %s

Generate 2-3 relevant documentation results that would typically be found when searching for this topic. Each result should include:
1. A realistic title for the documentation page
2. Comprehensive content that covers the topic in detail
3. A realistic source (e.g., "official_docs", "github_wiki", "community_guide")
4. A plausible URL

Focus on providing accurate, detailed information that would be found in real documentation for this topic.

Return the results in this JSON format:
{
  "results": [
    {
      "title": "Documentation Title",
      "content": "Detailed documentation content covering the topic...",
      "url": "https://docs.example.com/path/to/doc",
      "score": 0.95,
      "source": "official_docs"
    }
  ],
  "query": "%s",
  "total": 2
}`, query, domain, query)

	var searchResponse OpenAISearchResponse
	err := o.llmClient.Analyze(ctx, prompt, &searchResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to generate documentation search results: %w", err)
	}

	return searchResponse.Results, nil
}

// AnalyzeDocuments processes gathered documents to extract actionable insights
func (o *OpenAIInformationGatherer) AnalyzeDocuments(ctx context.Context, documents []DocumentResult, intent *EnhancedIntent) (*AnalyzedContext, error) {
	log.Printf("[OpenAIInformationGatherer] Analyzing %d documents for intent: %s", len(documents), intent.PrimaryIntent)

	// Combine all document content with source attribution
	var allContent strings.Builder
	allContent.WriteString("=== GATHERED DOCUMENTATION ===\n\n")

	for i, doc := range documents {
		allContent.WriteString(fmt.Sprintf("=== DOCUMENT %d: %s ===\n", i+1, doc.Title))
		allContent.WriteString(fmt.Sprintf("Source: %s\n", doc.Source))
		if doc.URL != "" {
			allContent.WriteString(fmt.Sprintf("URL: %s\n", doc.URL))
		}
		allContent.WriteString(fmt.Sprintf("Content:\n%s\n\n", doc.Content))
	}

	prompt := fmt.Sprintf(`You are an expert system analyst. Analyze the following documentation and extract actionable insights for generating compliance and security rules.

ORIGINAL USER REQUEST CONTEXT:
- Intent Type: %s
- Intent Summary: %s
- Confidence: %.2f
- Required Steps: %v
- Information Needs: %v

GATHERED DOCUMENTATION:
%s

Based on this documentation, extract:

1. **summary**: A concise summary of the key findings and how they relate to the original request
2. **key_requirements**: Specific technical requirements that must be addressed in any generated rules
3. **constraints**: Important limitations, security considerations, or constraints to consider
4. **recommended_rules**: Specific rule recommendations with detailed implementation guidance

For each recommended rule, provide:
- type: The type of rule (e.g., "security", "compliance", "validation", "policy")
- description: Clear description of what the rule should accomplish
- cel_pattern: A suggested CEL expression pattern or template (if applicable)
- priority: Priority level 1-10 (10 = critical)
- rationale: Detailed explanation of why this rule is necessary based on the documentation
- parameters: Specific parameters and configuration options

Focus on practical, implementable rules that directly address the original user request while following best practices found in the documentation.

Response format (valid JSON):
{
  "summary": "Brief summary of key findings from documentation analysis",
  "key_requirements": ["requirement1", "requirement2", "requirement3"],
  "constraints": ["constraint1", "constraint2"],
  "recommended_rules": [
    {
      "type": "security",
      "description": "Detailed rule description",
      "cel_pattern": "has(object.spec.template.spec.containers[0].securityContext) && object.spec.template.spec.containers[0].securityContext.runAsNonRoot == true",
      "priority": 8,
      "rationale": "Detailed explanation based on documentation findings",
      "parameters": {
        "resource_type": "Pod",
        "field_path": "spec.template.spec.containers[0].securityContext.runAsNonRoot",
        "expected_value": true
      }
    }
  ]
}`,
		intent.PrimaryIntent,
		intent.IntentSummary,
		intent.Confidence,
		intent.Context,
		intent.SuggestedTasks,
		intent.InformationNeeds,
		allContent.String())

	var analyzedContext AnalyzedContext
	err := o.llmClient.Analyze(ctx, prompt, &analyzedContext)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze documents: %w", err)
	}

	log.Printf("[OpenAIInformationGatherer] Analysis completed: %d requirements, %d constraints, %d recommended rules",
		len(analyzedContext.KeyRequirements), len(analyzedContext.Constraints), len(analyzedContext.RecommendedRules))

	return &analyzedContext, nil
}

// SearchWebDocumentation performs web-based documentation search (future enhancement)
func (o *OpenAIInformationGatherer) SearchWebDocumentation(ctx context.Context, query string) ([]DocumentResult, error) {
	// This would integrate with actual web search APIs in production:
	// - Google Custom Search API
	// - Bing Search API
	// - DuckDuckGo API
	// - Specialized documentation search engines

	// For now, return empty results as this is a placeholder for future implementation
	log.Printf("[OpenAIInformationGatherer] Web search not yet implemented for query: %s", query)
	return []DocumentResult{}, nil
}

// SearchGitHubDocumentation searches GitHub repositories for documentation (future enhancement)
func (o *OpenAIInformationGatherer) SearchGitHubDocumentation(ctx context.Context, query string, repos []string) ([]DocumentResult, error) {
	// This would integrate with GitHub API to search specific repositories:
	// - kubernetes/kubernetes
	// - openshift/origin
	// - ComplianceAsCode/content
	// - etc.

	log.Printf("[OpenAIInformationGatherer] GitHub search not yet implemented for query: %s", query)
	return []DocumentResult{}, nil
}

// SearchComplianceStandards searches for compliance standard documentation (future enhancement)
func (o *OpenAIInformationGatherer) SearchComplianceStandards(ctx context.Context, standard string, topic string) ([]DocumentResult, error) {
	// This would integrate with compliance databases:
	// - CIS Benchmarks
	// - NIST frameworks
	// - PCI-DSS standards
	// - SOC2 requirements

	log.Printf("[OpenAIInformationGatherer] Compliance standards search not yet implemented for standard: %s, topic: %s", standard, topic)
	return []DocumentResult{}, nil
}
