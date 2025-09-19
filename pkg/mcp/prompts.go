package mcp

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/mark3labs/mcp-go/mcp"
	"gopkg.in/yaml.v3"
)

//go:embed prompts/*.yaml
var promptFiles embed.FS

// PromptConfig represents the structure of a prompt YAML file
type PromptConfig struct {
	Name        string           `yaml:"name"`
	Description string           `yaml:"description"`
	Arguments   []PromptArgument `yaml:"arguments"`
	Template    string           `yaml:"template"`
}

// PromptArgument represents a prompt argument from YAML
type PromptArgument struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
}

// registerPrompts registers all MCP prompts for rule writing guidance
func (ms *MCPServer) registerPrompts() error {
	// Get list of prompt YAML files
	entries, err := promptFiles.ReadDir("prompts")
	if err != nil {
		return fmt.Errorf("failed to read prompts directory: %w", err)
	}

	// Load and register each prompt
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		promptPath := filepath.Join("prompts", entry.Name())
		if err := ms.loadAndRegisterPrompt(promptPath); err != nil {
			return fmt.Errorf("failed to load prompt %s: %w", entry.Name(), err)
		}
	}

	return nil
}

// loadAndRegisterPrompt loads a prompt from a YAML file and registers it
func (ms *MCPServer) loadAndRegisterPrompt(filePath string) error {
	// Read the YAML file
	data, err := promptFiles.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read prompt file %s: %w", filePath, err)
	}

	// Parse the YAML
	var config PromptConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse YAML in %s: %w", filePath, err)
	}

	// Create MCP prompt arguments
	var args []mcp.PromptOption
	if config.Description != "" {
		args = append(args, mcp.WithPromptDescription(config.Description))
	}

	for _, arg := range config.Arguments {
		argOptions := []mcp.ArgumentOption{
			mcp.ArgumentDescription(arg.Description),
		}
		if arg.Required {
			argOptions = append(argOptions, mcp.RequiredArgument())
		}
		args = append(args, mcp.WithArgument(arg.Name, argOptions...))
	}

	// Create the prompt
	prompt := mcp.NewPrompt(config.Name, args...)

	// Create the handler that uses the template
	handler := func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return ms.executePromptTemplate(config, request)
	}

	// Register the prompt
	return ms.registerPrompt(prompt, handler)
}

// executePromptTemplate executes the prompt template with the provided arguments
func (ms *MCPServer) executePromptTemplate(config PromptConfig, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	// Validate required arguments
	for _, arg := range config.Arguments {
		if arg.Required && request.Params.Arguments[arg.Name] == "" {
			return nil, fmt.Errorf("%s is required", arg.Name)
		}
	}

	// Parse and execute the template
	tmpl, err := template.New(config.Name).Funcs(template.FuncMap{
		"title": strings.Title,
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"eq": func(a interface{}, b ...interface{}) bool {
			for _, val := range b {
				if a == val {
					return true
				}
			}
			return false
		},
	}).Parse(config.Template)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, request.Params.Arguments); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}

	// Create the response
	messages := []mcp.PromptMessage{
		mcp.NewPromptMessage(
			mcp.RoleAssistant,
			mcp.NewTextContent(buf.String()),
		),
	}

	return mcp.NewGetPromptResult(config.Description, messages), nil
}

// registerPrompt registers a prompt with the MCP server
func (ms *MCPServer) registerPrompt(prompt mcp.Prompt, handler func(context.Context, mcp.GetPromptRequest) (*mcp.GetPromptResult, error)) error {
	ms.server.AddPrompt(prompt, handler)
	return nil
}
