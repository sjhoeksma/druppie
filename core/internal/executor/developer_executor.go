package executor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sjhoeksma/druppie/core/internal/model"
)

// DeveloperExecutor handles "create_code" and "modify_code" actions
type DeveloperExecutor struct{}

func (e *DeveloperExecutor) CanHandle(action string) bool {
	return action == "create_code" || action == "modify_code"
}

func (e *DeveloperExecutor) Execute(ctx context.Context, step model.Step, outputChan chan<- string) error {
	outputChan <- "DeveloperExecutor: Processing request..."

	planID, ok := step.Params["_plan_id"].(string)
	if !ok || planID == "" {
		// Fallback if not injected, though it should be
		return fmt.Errorf("missing plan ID in context")
	}

	// Determine Project Root (mimic other executors)
	cwd, _ := os.Getwd()
	// Default to .druppie/plans/<plan-id>/src
	projectRoot := filepath.Join(cwd, ".druppie", "plans", planID, "src")

	// Ensure root exists
	if err := os.MkdirAll(projectRoot, 0755); err != nil {
		return fmt.Errorf("failed to create project root: %w", err)
	}

	if step.Action == "create_code" {
		var fileMap map[string]interface{}

		if f, ok := step.Params["files"].(map[string]interface{}); ok {
			fileMap = f
		} else if fList, ok := step.Params["files"].([]interface{}); ok {
			// Convert list of objects to map if possible, e.g. [{"path": "p", "content": "c"}]
			fileMap = make(map[string]interface{})
			for _, item := range fList {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if p, ok := itemMap["path"].(string); ok {
						if c, ok := itemMap["content"].(string); ok {
							fileMap[p] = c
						}
					}
				}
			}
		}

		// Debug params
		outputChan <- fmt.Sprintf("Debug: Received params for create_code: %+v", step.Params)

		if len(fileMap) == 0 {
			// Fallback: Check for single file in root params (filename/path + content/code)
			var path, content string

			if p, ok := step.Params["filename"].(string); ok {
				path = p
			}
			if p, ok := step.Params["path"].(string); ok {
				path = p
			}

			if c, ok := step.Params["content"].(string); ok {
				content = c
			}
			if c, ok := step.Params["code"].(string); ok {
				content = c
			}

			if path != "" && content != "" {
				fileMap = map[string]interface{}{path: content}
				outputChan <- fmt.Sprintf("Recovered single file from params: %s", path)
			} else {
				return fmt.Errorf("create_code requires 'files' parameter map or list of objects")
			}
		}

		for path, content := range fileMap {
			strContent, ok := content.(string)
			if !ok {
				outputChan <- fmt.Sprintf("Skipping %s: content is not string", path)
				continue
			}

			// Clean path to prevent traversal escape (simple check)
			// Do NOT strip "src/" prefix; let the planner define structure (e.g. src/app.js)

			cleanPath := filepath.Clean(path)
			if cleanPath == "." || cleanPath == "/" {
				continue
			}
			fullPath := filepath.Join(projectRoot, cleanPath)

			// Ensure dir exists
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				return fmt.Errorf("failed to create dir for %s: %w", path, err)
			}

			if err := os.WriteFile(fullPath, []byte(strContent), 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", path, err)
			}

			outputChan <- fmt.Sprintf("Created file: %s", path)
		}
	}

	outputChan <- "DeveloperExecutor: Completed."
	return nil
}
