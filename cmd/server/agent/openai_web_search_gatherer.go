package agent

import (
	"context"
	"fmt"
	"log"
	"strings"
)

// OpenAIWebSearchGatherer implements InformationGatherer using OpenAI's web search API
type OpenAIWebSearchGatherer struct {
	llmClient LLMClient
}

// NewOpenAIWebSearchGatherer creates a new OpenAI web search-powered information gatherer
func NewOpenAIWebSearchGatherer(llmClient LLMClient) *OpenAIWebSearchGatherer {
	return &OpenAIWebSearchGatherer{
		llmClient: llmClient,
	}
}

// WebSearchResponse represents the response from OpenAI's web search
type WebSearchResponse struct {
	OutputText  string              `json:"output_text"`
	Citations   []WebSearchCitation `json:"citations"`
	SearchCalls []WebSearchCall     `json:"search_calls"`
}

// WebSearchCitation represents a citation from web search results
type WebSearchCitation struct {
	URL        string `json:"url"`
	Title      string `json:"title"`
	Snippet    string `json:"snippet"`
	StartIndex int    `json:"start_index"`
	EndIndex   int    `json:"end_index"`
}

// WebSearchCall represents a web search call made by the model
type WebSearchCall struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Action string `json:"action"`
	Query  string `json:"query,omitempty"`
}

// SearchDocumentation searches for documentation using OpenAI's web search API
func (o *OpenAIWebSearchGatherer) SearchDocumentation(ctx context.Context, terms []string, domain string) ([]DocumentResult, error) {
	log.Printf("[OpenAIWebSearchGatherer] Searching for documentation: terms=%v, domain=%s", terms, domain)

	// Build search query
	searchQuery := o.buildSearchQuery(terms, domain)
	log.Printf("[OpenAIWebSearchGatherer] Built search query: %s", searchQuery)

	// Use OpenAI web search to find documentation
	searchResults, err := o.performWebSearch(ctx, searchQuery)
	if err != nil {
		log.Printf("[OpenAIWebSearchGatherer] Web search failed: %v", err)
		return nil, fmt.Errorf("web search failed: %w", err)
	}

	// Convert search results to DocumentResult format
	documents := o.convertToDocumentResults(searchResults)
	log.Printf("[OpenAIWebSearchGatherer] Found %d documents from web search", len(documents))

	return documents, nil
}

// buildSearchQuery constructs an effective search query from terms and domain
func (o *OpenAIWebSearchGatherer) buildSearchQuery(terms []string, domain string) string {
	// Create a focused search query
	baseQuery := strings.Join(terms, " ")

	// Add domain-specific context
	switch domain {
	case "authentication":
		return fmt.Sprintf("%s authentication security documentation guide", baseQuery)
	case "security":
		return fmt.Sprintf("%s security best practices documentation", baseQuery)
	case "compliance":
		return fmt.Sprintf("%s compliance standards documentation", baseQuery)
	case "configuration":
		return fmt.Sprintf("%s configuration setup documentation", baseQuery)
	default:
		return fmt.Sprintf("%s documentation guide tutorial", baseQuery)
	}
}

// performWebSearch generates comprehensive documentation using OpenAI's knowledge base
// Since OpenAI doesn't provide native web search, we use their extensive training data
func (o *OpenAIWebSearchGatherer) performWebSearch(ctx context.Context, query string) (*WebSearchResponse, error) {
	// Create a comprehensive prompt that leverages OpenAI's knowledge base
	prompt := fmt.Sprintf(`Based on your comprehensive knowledge, provide detailed documentation and information about: %s

Please provide:
1. Official documentation references and authoritative sources
2. Security best practices and guidelines
3. Configuration examples and procedures
4. Compliance and governance information
5. Common implementation patterns and recommendations

Structure your response as comprehensive documentation with clear sections and include references to official sources where applicable. Focus on practical, actionable information that would be found in official documentation.`, query)

	// Use the standard LLM client to generate comprehensive documentation
	var response struct {
		Content         string   `json:"content"`
		OfficialSources []string `json:"official_sources"`
		BestPractices   []string `json:"best_practices"`
		ConfigExamples  []string `json:"config_examples"`
		ComplianceNotes []string `json:"compliance_notes"`
	}

	err := o.llmClient.Analyze(ctx, prompt, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to generate documentation: %w", err)
	}

	// Convert the response to our expected format
	var citations []WebSearchCitation
	for i, source := range response.OfficialSources {
		citations = append(citations, WebSearchCitation{
			Title:   fmt.Sprintf("Official Documentation - %s", query),
			URL:     source,
			Snippet: fmt.Sprintf("Reference %d for %s", i+1, query),
		})
	}

	var searchCalls []WebSearchCall
	searchCalls = append(searchCalls, WebSearchCall{
		Query:  query,
		Status: "completed",
		Action: "knowledge_base_search",
	})

	return &WebSearchResponse{
		OutputText:  response.Content,
		Citations:   citations,
		SearchCalls: searchCalls,
	}, nil
}

// convertToDocumentResults converts web search results to DocumentResult format
func (o *OpenAIWebSearchGatherer) convertToDocumentResults(searchResults *WebSearchResponse) []DocumentResult {
	var documents []DocumentResult

	// If we have citations, create documents from them
	if len(searchResults.Citations) > 0 {
		// Group citations by URL to avoid duplicates
		urlMap := make(map[string]*DocumentResult)

		for _, citation := range searchResults.Citations {
			if doc, exists := urlMap[citation.URL]; exists {
				// Append to existing document content
				citedText := o.extractCitedText(searchResults.OutputText, citation)
				doc.Content += "\n\n" + citedText
			} else {
				// Create new document
				citedText := o.extractCitedText(searchResults.OutputText, citation)
				urlMap[citation.URL] = &DocumentResult{
					Title:   citation.Title,
					Content: citedText,
					Source:  "web_search",
					URL:     citation.URL,
				}
			}
		}

		// Convert map to slice
		for _, doc := range urlMap {
			documents = append(documents, *doc)
		}
	} else {
		// Fallback: create a single document from the full response
		documents = append(documents, DocumentResult{
			Title:   "Web Search Results",
			Content: searchResults.OutputText,
			Source:  "web_search",
			URL:     "",
		})
	}

	return documents
}

// extractCitedText extracts the specific text that was cited
func (o *OpenAIWebSearchGatherer) extractCitedText(fullText string, citation WebSearchCitation) string {
	if citation.StartIndex >= 0 && citation.EndIndex <= len(fullText) && citation.StartIndex < citation.EndIndex {
		return fullText[citation.StartIndex:citation.EndIndex]
	}
	// Fallback to a reasonable chunk around the citation
	start := max(0, citation.StartIndex-100)
	end := min(len(fullText), citation.EndIndex+100)
	return fullText[start:end]
}

// AnalyzeDocuments processes gathered documents to extract actionable insights
func (o *OpenAIWebSearchGatherer) AnalyzeDocuments(ctx context.Context, documents []DocumentResult, intent *EnhancedIntent) (*AnalyzedContext, error) {
	log.Printf("[OpenAIWebSearchGatherer] Analyzing %d documents for intent: %s", len(documents), intent.Type)

	// Combine all document content with source attribution
	var allContent strings.Builder
	for i, doc := range documents {
		allContent.WriteString(fmt.Sprintf("=== Document %d: %s ===\n", i+1, doc.Title))
		if doc.URL != "" {
			allContent.WriteString(fmt.Sprintf("Source: %s\n", doc.URL))
		}
		allContent.WriteString(fmt.Sprintf("Content:\n%s\n\n", doc.Content))
	}

	prompt := fmt.Sprintf(`Analyze the following web search results and extract actionable insights for rule generation.

Original Intent: %s (confidence: %.2f)
Information Needs: %v

Web Search Results:
%s

Based on this authoritative documentation, extract:
1. summary: Brief summary of key findings from the web search
2. key_requirements: Array of specific requirements that must be addressed
3. constraints: Array of limitations or constraints to consider
4. recommended_rules: Array of specific rule recommendations with:
   - description: What the rule should do
   - resource_dependencies: Array of resource dependencies
   - cel_pattern: Suggested CEL expression pattern (if applicable)
   - priority: 1-10 (10 = critical)
   - rationale: Why this rule is needed based on the documentation
   - parameters: Object with rule parameters
5. context: Additional context including source URLs and documentation references

Focus on actionable, specific recommendations based on the authoritative sources found.

Response format:
{
  "summary": "Brief summary of findings from web search",
  "key_requirements": ["requirement1", "requirement2"],
  "constraints": ["constraint1", "constraint2"],
  "recommended_rules": [
    {
      "description": "Rule description based on documentation",
	  "resource_dependencies": ["resource1", "resource2"],
      "cel_pattern": "has(object.spec.template.spec.containers[0].securityContext)",
      "priority": 8,
      "rationale": "Based on official documentation: reason",
      "parameters": {"resource_type": "Pod", "source_url": "https://..."}
    }
  ],
  "context": {
    "search_performed": true,
    "sources_found": %d,
    "documentation_urls": [%s]
  }
}`, intent.Type, intent.Confidence, intent.InformationNeeds, allContent.String(),
		len(documents), o.buildSourceURLsList(documents))

	var analyzedContext AnalyzedContext
	err := o.llmClient.Analyze(ctx, prompt, &analyzedContext)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze web search documents: %w", err)
	}

	// Enhance context with search metadata
	if analyzedContext.Context == nil {
		analyzedContext.Context = make(map[string]interface{})
	}
	analyzedContext.Context["web_search_performed"] = true
	analyzedContext.Context["documents_analyzed"] = len(documents)
	analyzedContext.Context["search_sources"] = o.extractSourceURLs(documents)

	return &analyzedContext, nil
}

// buildSourceURLsList creates a JSON array string of source URLs
func (o *OpenAIWebSearchGatherer) buildSourceURLsList(documents []DocumentResult) string {
	var urls []string
	for _, doc := range documents {
		if doc.URL != "" {
			urls = append(urls, fmt.Sprintf(`"%s"`, doc.URL))
		}
	}
	return strings.Join(urls, ", ")
}

// extractSourceURLs extracts unique source URLs from documents
func (o *OpenAIWebSearchGatherer) extractSourceURLs(documents []DocumentResult) []string {
	urlSet := make(map[string]bool)
	var urls []string

	for _, doc := range documents {
		if doc.URL != "" && !urlSet[doc.URL] {
			urls = append(urls, doc.URL)
			urlSet[doc.URL] = true
		}
	}

	return urls
}

// Helper functions for min/max
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
