package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// WebSearchResult represents a search result from web search APIs
type WebSearchResult struct {
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
	URL     string `json:"url"`
	Source  string `json:"source"`
}

// DuckDuckGoSearchResponse represents DuckDuckGo Instant Answer API response
type DuckDuckGoSearchResponse struct {
	Abstract     string `json:"Abstract"`
	AbstractText string `json:"AbstractText"`
	AbstractURL  string `json:"AbstractURL"`
	Answer       string `json:"Answer"`
	AnswerType   string `json:"AnswerType"`
	Definition   string `json:"Definition"`
	Entity       string `json:"Entity"`
	Heading      string `json:"Heading"`
	Image        string `json:"Image"`
	ImageHeight  string `json:"ImageHeight"`
	ImageWidth   string `json:"ImageWidth"`
	Infobox      string `json:"Infobox"`
	Redirect     string `json:"Redirect"`
	RelatedTopics []struct {
		FirstURL string `json:"FirstURL"`
		Icon     struct {
			Height string `json:"Height"`
			URL    string `json:"URL"`
			Width  string `json:"Width"`
		} `json:"Icon"`
		Result string `json:"Result"`
		Text   string `json:"Text"`
	} `json:"RelatedTopics"`
	Results []struct {
		FirstURL string `json:"FirstURL"`
		Icon     struct {
			Height string `json:"Height"`
			URL    string `json:"URL"`
			Width  string `json:"Width"`
		} `json:"Icon"`
		Result string `json:"Result"`
		Text   string `json:"Text"`
	} `json:"Results"`
	Type string `json:"Type"`
}

// WebSearchInformationGatherer implements InformationGatherer using web search APIs
type WebSearchInformationGatherer struct {
	llmClient  LLMClient
	httpClient *http.Client
}

// NewWebSearchInformationGatherer creates a new web search-powered information gatherer
func NewWebSearchInformationGatherer(llmClient LLMClient) *WebSearchInformationGatherer {
	return &WebSearchInformationGatherer{
		llmClient: llmClient,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SearchDocumentation searches for documentation using web search APIs
func (w *WebSearchInformationGatherer) SearchDocumentation(ctx context.Context, terms []string, domain string) ([]DocumentResult, error) {
	log.Printf("[WebSearchInformationGatherer] Searching for documentation: terms=%v, domain=%s", terms, domain)

	// Build search query with domain-specific context
	query := w.buildDocumentationQuery(terms, domain)
	
	// Try multiple search approaches
	var allResults []DocumentResult
	
	// 1. DuckDuckGo Instant Answer API (free, no API key required)
	ddgResults, err := w.searchDuckDuckGo(ctx, query)
	if err != nil {
		log.Printf("[WebSearchInformationGatherer] DuckDuckGo search failed: %v", err)
	} else {
		allResults = append(allResults, ddgResults...)
	}
	
	// 2. Generate documentation-style content using LLM as fallback/enhancement
	llmResults, err := w.generateDocumentationContent(ctx, query, domain)
	if err != nil {
		log.Printf("[WebSearchInformationGatherer] LLM documentation generation failed: %v", err)
	} else {
		allResults = append(allResults, llmResults...)
	}

	log.Printf("[WebSearchInformationGatherer] Found %d total documents for query: %s", len(allResults), query)
	return allResults, nil
}

// buildDocumentationQuery constructs an optimized search query for documentation
func (w *WebSearchInformationGatherer) buildDocumentationQuery(terms []string, domain string) string {
	baseQuery := strings.Join(terms, " ")
	
	// Add domain-specific documentation sites and terms
	domainSites := map[string][]string{
		"kubernetes": {"site:kubernetes.io", "site:github.com/kubernetes", "kubectl", "k8s"},
		"openshift":  {"site:docs.openshift.com", "site:access.redhat.com", "oc command"},
		"security":   {"site:owasp.org", "site:nist.gov", "security best practices"},
		"compliance": {"site:cisecurity.org", "site:nist.gov", "compliance framework"},
		"docker":     {"site:docs.docker.com", "dockerfile", "container"},
		"general":    {"documentation", "guide", "tutorial"},
	}
	
	// Add domain-specific terms
	if sites, exists := domainSites[domain]; exists {
		for _, site := range sites {
			baseQuery += " " + site
		}
	} else {
		// Default documentation terms
		baseQuery += " documentation guide tutorial"
	}
	
	return baseQuery
}

// searchDuckDuckGo searches using DuckDuckGo Instant Answer API
func (w *WebSearchInformationGatherer) searchDuckDuckGo(ctx context.Context, query string) ([]DocumentResult, error) {
	// DuckDuckGo Instant Answer API
	baseURL := "https://api.duckduckgo.com/"
	params := url.Values{}
	params.Add("q", query)
	params.Add("format", "json")
	params.Add("no_html", "1")
	params.Add("skip_disambig", "1")
	
	fullURL := baseURL + "?" + params.Encode()
	
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("User-Agent", "CEL-RPC-Server/1.0")
	
	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search API returned status %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	
	var searchResp DuckDuckGoSearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, fmt.Errorf("failed to parse search response: %w", err)
	}
	
	var results []DocumentResult
	
	// Extract abstract information
	if searchResp.Abstract != "" {
		results = append(results, DocumentResult{
			Title:   searchResp.Heading,
			Content: searchResp.Abstract,
			Source:  "duckduckgo_abstract",
			URL:     searchResp.AbstractURL,
		})
	}
	
	// Extract answer if available
	if searchResp.Answer != "" {
		results = append(results, DocumentResult{
			Title:   "Direct Answer: " + query,
			Content: searchResp.Answer,
			Source:  "duckduckgo_answer",
			URL:     "",
		})
	}
	
	// Extract related topics
	for i, topic := range searchResp.RelatedTopics {
		if i >= 3 { // Limit to 3 related topics
			break
		}
		if topic.Text != "" {
			results = append(results, DocumentResult{
				Title:   fmt.Sprintf("Related: %s", strings.Split(topic.Text, " - ")[0]),
				Content: topic.Text,
				Source:  "duckduckgo_related",
				URL:     topic.FirstURL,
			})
		}
	}
	
	log.Printf("[WebSearchInformationGatherer] DuckDuckGo returned %d results", len(results))
	return results, nil
}

// generateDocumentationContent uses LLM to generate comprehensive documentation content
func (w *WebSearchInformationGatherer) generateDocumentationContent(ctx context.Context, query string, domain string) ([]DocumentResult, error) {
	prompt := fmt.Sprintf(`You are a technical documentation expert. Generate comprehensive, accurate documentation content for the following query.

Query: %s
Domain: %s

Generate 2-3 realistic documentation sections that would be found in official documentation for this topic. Each section should:

1. Cover a specific aspect of the topic
2. Include practical examples and code snippets where appropriate
3. Follow documentation best practices
4. Be technically accurate and detailed
5. Include relevant configuration options, parameters, or commands

Focus on providing information that would help someone implement or understand the topic in a production environment.

Return the results in this JSON format:
{
  "documents": [
    {
      "title": "Specific Documentation Section Title",
      "content": "Comprehensive documentation content with examples, configurations, and detailed explanations...",
      "source": "generated_docs",
      "url": "https://docs.example.com/generated/path"
    }
  ]
}`, query, domain)

	var response struct {
		Documents []DocumentResult `json:"documents"`
	}
	
	err := w.llmClient.Analyze(ctx, prompt, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to generate documentation content: %w", err)
	}
	
	log.Printf("[WebSearchInformationGatherer] Generated %d documentation sections", len(response.Documents))
	return response.Documents, nil
}

// AnalyzeDocuments processes gathered documents to extract actionable insights
func (w *WebSearchInformationGatherer) AnalyzeDocuments(ctx context.Context, documents []DocumentResult, intent *EnhancedIntent) (*AnalyzedContext, error) {
	log.Printf("[WebSearchInformationGatherer] Analyzing %d documents for intent: %s", len(documents), intent.Intent.Type)

	// Combine all document content with source attribution
	var allContent strings.Builder
	allContent.WriteString("=== GATHERED DOCUMENTATION FROM WEB SEARCH ===\n\n")
	
	for i, doc := range documents {
		allContent.WriteString(fmt.Sprintf("=== DOCUMENT %d: %s ===\n", i+1, doc.Title))
		allContent.WriteString(fmt.Sprintf("Source: %s\n", doc.Source))
		if doc.URL != "" {
			allContent.WriteString(fmt.Sprintf("URL: %s\n", doc.URL))
		}
		allContent.WriteString(fmt.Sprintf("Content:\n%s\n\n", doc.Content))
	}

	prompt := fmt.Sprintf(`You are an expert compliance and security analyst. Analyze the following web-sourced documentation and extract actionable insights for generating CEL (Common Expression Language) rules.

ORIGINAL USER REQUEST CONTEXT:
- Intent Type: %s
- Confidence: %.2f
- Entities: %v
- Required Steps: %v
- Information Needs: %v

WEB-SOURCED DOCUMENTATION:
%s

Based on this documentation, extract:

1. **summary**: A concise summary of the key findings and how they relate to the original request
2. **key_requirements**: Specific technical requirements that must be addressed in any generated CEL rules
3. **constraints**: Important limitations, security considerations, or constraints to consider
4. **recommended_rules**: Specific CEL rule recommendations with detailed implementation guidance

For each recommended rule, provide:
- type: The type of rule (e.g., "security", "compliance", "validation", "policy", "resource_check")
- description: Clear description of what the CEL rule should accomplish
- cel_pattern: A specific CEL expression pattern or template that can be used directly
- priority: Priority level 1-10 (10 = critical)
- rationale: Detailed explanation of why this rule is necessary based on the documentation
- parameters: Specific parameters and configuration options for the CEL rule

Focus on practical, implementable CEL rules that directly address the original user request while following best practices found in the documentation.

Response format (valid JSON):
{
  "summary": "Brief summary of key findings from web documentation analysis",
  "key_requirements": ["requirement1", "requirement2", "requirement3"],
  "constraints": ["constraint1", "constraint2"],
  "recommended_rules": [
    {
      "type": "security",
      "description": "Detailed CEL rule description",
      "cel_pattern": "has(object.spec.template.spec.containers[0].securityContext) && object.spec.template.spec.containers[0].securityContext.runAsNonRoot == true",
      "priority": 8,
      "rationale": "Detailed explanation based on web documentation findings",
      "parameters": {
        "resource_type": "Pod",
        "field_path": "spec.template.spec.containers[0].securityContext.runAsNonRoot",
        "expected_value": true,
        "validation_message": "Container must run as non-root user"
      }
    }
  ]
}`, 
		intent.Intent.Type, 
		intent.Intent.Confidence, 
		intent.Intent.Entities, 
		intent.Intent.RequiredSteps,
		intent.InformationNeeds,
		allContent.String())

	var analyzedContext AnalyzedContext
	err := w.llmClient.Analyze(ctx, prompt, &analyzedContext)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze web-sourced documents: %w", err)
	}

	log.Printf("[WebSearchInformationGatherer] Analysis completed: %d requirements, %d constraints, %d recommended rules", 
		len(analyzedContext.KeyRequirements), len(analyzedContext.Constraints), len(analyzedContext.RecommendedRules))

	return &analyzedContext, nil
}
