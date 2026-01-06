package executor

import (
	"context"
	"fmt"

	"github.com/sjhoeksma/druppie/core/internal/mcp"
	"github.com/sjhoeksma/druppie/core/internal/model"
)

// MCPExecutor handles execution of tools provided by MCP servers
type MCPExecutor struct {
	Manager *mcp.Manager
}

// CanHandle checks if the action corresponds to a known MCP tool
func (e *MCPExecutor) CanHandle(action string) bool {
	if e.Manager == nil {
		return false
	}
	// Direct tool name match
	_, ok := e.Manager.GetServerForTool(action)
	return ok
}

// Execute calls the tool via the MCP Manager
func (e *MCPExecutor) Execute(ctx context.Context, step model.Step, outputChan chan<- string) error {
	outputChan <- fmt.Sprintf("🔧 Executing MCP Tool: %s", step.Action)

	// Call the tool
	result, err := e.Manager.ExecuteTool(ctx, step.Action, step.Params)
	if err != nil {
		return fmt.Errorf("mcp tool execution failed: %w", err)
	}

	// Process Output
	if result.IsError {
		return fmt.Errorf("mcp tool returned error status")
	}

	var texts []string
	for _, content := range result.Content {
		switch content.Type {
		case "text":
			texts = append(texts, content.Text)
			outputChan <- content.Text
		case "image":
			outputChan <- "[Image Content Received]"
		default:
			outputChan <- fmt.Sprintf("[%s content]", content.Type)
		}
	}

	if len(texts) == 0 {
		outputChan <- "Tool executed successfully (no text output)."
	}

	return nil
}
