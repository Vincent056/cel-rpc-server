package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	mcp "github.com/mark3labs/mcp-go/mcp"
)

// HTTPTransport provides HTTP endpoints for MCP
type HTTPTransport struct {
	server *MCPServer
}

// NewHTTPTransport creates a new HTTP transport for MCP
func NewHTTPTransport(mcpServer *MCPServer) *HTTPTransport {
	return &HTTPTransport{
		server: mcpServer,
	}
}

// RegisterRoutes registers MCP HTTP routes
func (t *HTTPTransport) RegisterRoutes(router *mux.Router) {
	// MCP endpoints
	mcpRouter := router.PathPrefix("/mcp").Subrouter()

	// List tools endpoint
	mcpRouter.HandleFunc("/tools", t.handleListTools).Methods("GET")

	// Execute tool endpoint
	mcpRouter.HandleFunc("/tools/execute", t.handleExecuteTool).Methods("POST")

	// Health check endpoint
	mcpRouter.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	// MCP protocol endpoints (for future compatibility)
	mcpRouter.HandleFunc("/", t.handleMCPRequest).Methods("POST")

	log.Println("[MCP] Registered HTTP routes at /mcp/*")
}

// handleListTools handles the list tools request
func (t *HTTPTransport) handleListTools(w http.ResponseWriter, r *http.Request) {
	log.Println("[MCP] Listing tools via HTTP")

	// Get tools from our wrapper
	tools := t.server.GetTools()

	// Convert to a simpler format for HTTP API
	toolList := make([]map[string]interface{}, 0, len(tools))
	for _, tool := range tools {
		toolInfo := map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
		}

		// Add schema if present
		if tool.InputSchema.Properties != nil {
			toolInfo["schema"] = tool.InputSchema.Properties
		}

		toolList = append(toolList, toolInfo)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toolList)
}

// handleExecuteTool handles tool execution requests
func (t *HTTPTransport) handleExecuteTool(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var httpReq struct {
		Tool  string          `json:"tool"`
		Input json.RawMessage `json:"input"`
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read request body: %v", err), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if err := json.Unmarshal(body, &httpReq); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request format: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("[MCP] Tool call request: %s", httpReq.Tool)

	// Find the tool handler
	toolHandlers := t.server.GetToolHandlers()
	handler, exists := toolHandlers[httpReq.Tool]
	if !exists {
		http.Error(w, fmt.Sprintf("Tool not found: %s", httpReq.Tool), http.StatusNotFound)
		return
	}

	// Create MCP call tool request with the correct structure
	callReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      httpReq.Tool,
			Arguments: httpReq.Input,
		},
	}

	// Call the tool handler directly
	result, err := handler(context.Background(), callReq)
	if err != nil {
		log.Printf("[MCP] Tool execution failed: %v", err)
		http.Error(w, fmt.Sprintf("Tool execution failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Format response based on result type
	response := map[string]interface{}{
		"success": true,
	}

	if len(result.Content) > 0 {
		// Extract text content from the first content item
		if textContent, ok := result.Content[0].(mcp.TextContent); ok {
			response["result"] = textContent.Text
		} else {
			response["result"] = result.Content
		}
	}

	if result.IsError {
		response["success"] = false
		response["error"] = response["result"]
		delete(response, "result")

		// Return 400 for validation errors
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleMCPRequest handles standard MCP protocol requests
func (t *HTTPTransport) handleMCPRequest(w http.ResponseWriter, r *http.Request) {
	// This would handle full MCP protocol messages
	// For now, redirect to appropriate handlers based on method

	var req mcp.JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid MCP request: %v", err), http.StatusBadRequest)
		return
	}

	log.Printf("[MCP] Received MCP protocol request: %s", req.Method)

	// Route based on method
	switch req.Method {
	case "tools/list":
		// Get tools list
		tools := t.server.GetTools()
		toolList := make([]interface{}, 0, len(tools))

		for _, tool := range tools {
			toolInfo := map[string]interface{}{
				"name":        tool.Name,
				"description": tool.Description,
			}
			if tool.InputSchema.Properties != nil {
				toolInfo["schema"] = tool.InputSchema
			}
			toolList = append(toolList, toolInfo)
		}

		// Return JSON-RPC response
		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result": map[string]interface{}{
				"tools": toolList,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return

	case "tools/call":
		// Extract tool name and arguments from params
		paramsBytes, err := json.Marshal(req.Params)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid params: %v", err), http.StatusBadRequest)
			return
		}

		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(paramsBytes, &params); err != nil {
			http.Error(w, fmt.Sprintf("Invalid params: %v", err), http.StatusBadRequest)
			return
		}

		// Create a new request with the expected format
		toolReq := struct {
			Tool  string          `json:"tool"`
			Input json.RawMessage `json:"input"`
		}{
			Tool:  params.Name,
			Input: params.Arguments,
		}

		body, _ := json.Marshal(toolReq)
		r.Body = io.NopCloser(bytes.NewReader(body))
		t.handleExecuteTool(w, r)

	default:
		http.Error(w, fmt.Sprintf("Unknown method: %s", req.Method), http.StatusNotImplemented)
	}
}
