package mcp

import (
	"fmt"
	"strings"

	celv1 "github.com/Vincent056/cel-rpc-server/gen/cel/v1"
)

// Common input configuration types
type KubernetesInputConfig struct {
	Group        string `json:"group"`
	Version      string `json:"version"`
	Resource     string `json:"resource"`
	Namespace    string `json:"namespace"`
	ResourceName string `json:"resource_name"`
}

type FileInputConfig struct {
	Path   string `json:"path"`
	Format string `json:"format"`
}

type HTTPInputConfig struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers,omitempty"`
}

// formatValidationResponse formats the validation response for output
func formatValidationResponse(resp *celv1.ValidationResponse) string {
	var result strings.Builder

	if resp.Success {
		result.WriteString("✅ CEL expression validation succeeded\n\n")
	} else {
		result.WriteString("❌ CEL expression validation failed\n\n")
	}

	if resp.Error != "" {
		result.WriteString(fmt.Sprintf("**Error:** %s\n\n", resp.Error))
	}

	if len(resp.Results) > 0 {
		result.WriteString("## Validation Results\n\n")

		passedCount := 0
		for _, vr := range resp.Results {
			status := "✅ PASS"
			if !vr.Passed {
				status = "❌ FAIL"
			} else {
				passedCount++
			}

			testName := vr.TestCase
			if testName == "" && vr.ResourceInfo != nil {
				testName = formatResourceInfo(vr.ResourceInfo)
			}

			result.WriteString(fmt.Sprintf("### %s - %s\n", testName, status))

			if vr.Error != "" {
				result.WriteString(fmt.Sprintf("**Error:** %s\n", vr.Error))
			}

			if vr.Details != "" {
				result.WriteString(fmt.Sprintf("**Details:** %s\n", vr.Details))
			}

			if len(vr.EvaluationContext) > 0 {
				result.WriteString("\n**Evaluation Context:**\n")
				for k, v := range vr.EvaluationContext {
					result.WriteString(fmt.Sprintf("- %s: %s\n", k, v))
				}
			}

			result.WriteString("\n")
		}

		result.WriteString(fmt.Sprintf("**Total:** %d/%d passed\n", passedCount, len(resp.Results)))
	}

	if resp.Performance != nil {
		result.WriteString(fmt.Sprintf("\n**Performance:** Total: %dms, Average: %dms\n",
			resp.Performance.TotalTimeMs, resp.Performance.AverageTimeMs))
	}

	return result.String()
}

// formatResourceInfo formats resource information for display
func formatResourceInfo(info *celv1.ResourceInfo) string {
	if info == nil {
		return "Unknown Resource"
	}

	parts := []string{}

	if info.Kind != "" {
		parts = append(parts, info.Kind)
	}

	if info.Name != "" {
		if info.Namespace != "" {
			parts = append(parts, fmt.Sprintf("%s/%s", info.Namespace, info.Name))
		} else {
			parts = append(parts, info.Name)
		}
	}

	if len(parts) == 0 && info.ApiVersion != "" {
		parts = append(parts, info.ApiVersion)
	}

	if len(parts) == 0 {
		return "Resource"
	}

	return strings.Join(parts, " ")
}
