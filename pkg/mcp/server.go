package mcp

import (
	"context"
	"fmt"
	"log"

	celv1 "github.com/Vincent056/cel-rpc-server/gen/cel/v1"
	"github.com/Vincent056/cel-rpc-server/gen/cel/v1/celv1connect"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ToolHandler is the function signature for tool handlers
type ToolHandler func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)

// RuleStore interface defines the operations for rule storage
// This is a minimal interface matching what's needed from cmd/server/rulestore.go
type RuleStore interface {
	Save(rule *celv1.CELRule) error
	Get(id string) (*celv1.CELRule, error)
	List(filter *celv1.ListRulesFilter, pageSize int32, pageToken string, sortBy string, ascending bool) ([]*celv1.CELRule, string, int32, error)
	Update(rule *celv1.CELRule, updateFields []string) error
	Delete(id string) error
}

// MCPServer wraps the mcp-go server
type MCPServer struct {
	server       *server.MCPServer
	service      celv1connect.CELValidationServiceHandler
	ruleStore    RuleStore
	tools        map[string]mcp.Tool
	toolHandlers map[string]ToolHandler
}

// NewMCPServer creates a new MCP server using mcp-go
func NewMCPServer(service celv1connect.CELValidationServiceHandler, ruleStore RuleStore) (*MCPServer, error) {
	// Create the mcp-go server
	mcpServer := server.NewMCPServer("CEL RPC Server MCP Tools", "1.0.0")

	ms := &MCPServer{
		server:       mcpServer,
		service:      service,
		ruleStore:    ruleStore,
		tools:        make(map[string]mcp.Tool),
		toolHandlers: make(map[string]ToolHandler),
	}

	// Register all tools
	if err := ms.registerTools(); err != nil {
		return nil, fmt.Errorf("failed to register tools: %w", err)
	}

	return ms, nil
}

// GetTools returns all registered tools
func (ms *MCPServer) GetTools() []mcp.Tool {
	tools := make([]mcp.Tool, 0, len(ms.tools))
	for _, tool := range ms.tools {
		tools = append(tools, tool)
	}
	return tools
}

// GetToolHandlers returns all tool handlers
func (ms *MCPServer) GetToolHandlers() map[string]ToolHandler {
	return ms.toolHandlers
}

// registerTool registers a tool and stores it locally
func (ms *MCPServer) registerTool(tool mcp.Tool, handler ToolHandler) error {
	// Convert our ToolHandler to server.ToolHandlerFunc
	serverHandler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handler(ctx, request)
	}

	ms.server.AddTool(tool, serverHandler)
	ms.tools[tool.Name] = tool
	ms.toolHandlers[tool.Name] = handler
	log.Printf("[MCP] Registered tool: %s", tool.Name)
	return nil
}

// registerTools registers all available tools
func (ms *MCPServer) registerTools() error {
	// Register CEL verification tools
	if err := ms.registerVerifyCELTestCasesTool(); err != nil {
		return fmt.Errorf("failed to register verify_cel_with_tests tool: %w", err)
	}

	if err := ms.registerVerifyCELLiveTool(); err != nil {
		return fmt.Errorf("failed to register verify_cel_live_resources tool: %w", err)
	}

	// Register discovery tools
	if err := ms.registerDiscoverResourceTypesTool(); err != nil {
		return fmt.Errorf("failed to register discover_resource_types tool: %w", err)
	}

	if err := ms.registerCountResourcesTool(); err != nil {
		return fmt.Errorf("failed to register count_resources tool: %w", err)
	}

	if err := ms.registerGetResourceSamplesTool(); err != nil {
		return fmt.Errorf("failed to register get_resource_samples tool: %w", err)
	}

	// Register rule management tools
	if ms.ruleStore != nil {
		if err := ms.registerAddRuleTool(); err != nil {
			return fmt.Errorf("failed to register add_rule tool: %w", err)
		}

		if err := ms.registerListRulesTool(); err != nil {
			return fmt.Errorf("failed to register list_rules tool: %w", err)
		}

		if err := ms.registerRemoveRuleTool(); err != nil {
			return fmt.Errorf("failed to register remove_rule tool: %w", err)
		}

		if err := ms.registerTestRuleTool(); err != nil {
			return fmt.Errorf("failed to register test_rule tool: %w", err)
		}

		if err := ms.registerGetRuleTool(); err != nil {
			return fmt.Errorf("failed to register get_rule tool: %w", err)
		}

		if err := ms.registerUpdateRuleTool(); err != nil {
			return fmt.Errorf("failed to register update_rule tool: %w", err)
		}

		log.Printf("[MCP] Rule management tools registered (add_rule, list_rules, remove_rule, test_rule, get_rule, update_rule)")
	} else {
		log.Printf("[MCP] Rule store not available, skipping rule management tools")
	}

	return nil
}

// GetServer returns the underlying mcp-go server
func (ms *MCPServer) GetServer() *server.MCPServer {
	return ms.server
}

// Start starts the MCP server (if needed for specific transport)
func (ms *MCPServer) Start(ctx context.Context) error {
	log.Println("[MCP] Server started with mcp-go library")
	// The actual serving is handled by the transport layer
	return nil
}
