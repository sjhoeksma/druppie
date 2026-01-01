package executor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sjhoeksma/druppie/core/internal/model"
)

type FileReaderExecutor struct{}

func (e *FileReaderExecutor) CanHandle(action string) bool {
	return strings.EqualFold(action, "read_file") || strings.EqualFold(action, "analyze_file")
}

func (e *FileReaderExecutor) Execute(ctx context.Context, step model.Step, outputChan chan<- string) error {
	outputChan <- fmt.Sprintf("📂 [File Reader] Processing: %v", step.Params)

	// Get filename from params
	fileNameRaw, ok := step.Params["filename"]
	if !ok {
		// Try 'file'
		fileNameRaw, ok = step.Params["file"]
	}

	if !ok || fileNameRaw == "" {
		return fmt.Errorf("missing 'filename' parameter")
	}

	filename := fmt.Sprintf("%v", fileNameRaw)

	// Resolve Path
	// Context: The Executor runs in the context of the core.
	// We need the Plan ID to find the file.
	// HACK: The Step struct doesn't strictly carry the PlanID, but we can assume files are in a known location relative to root?
	// Or we need the Task Manager to pass context.
	// Wait, the Store uses `.druppie/files/<plan_id>`.
	// We don't have PlanID here easily unless we pass it.
	// BUT, the TaskManager IS passing context. Let's see if we can get it or if we should standardise passing PlanID in Params?
	// The planner should verify file existence, but for now let's try to find it.

	// CURRENT LIMITATION: Step doesn't know PlanID.
	// FIX: We will modify TaskManager to inject "plan_id" into parameters before execution?
	// OR: We iterate subdirectories in .druppie/files/ to find the file? (Risky for name collision)
	// BETTER: The Planner should invoke this with a full path? No, security.

	// Let's assume for now the TaskManager injects `_plan_id` or we assume only one active plan?
	// Let's check `task_manager.go`. The task has the plan.
	// We should update TaskManager to inject `_context_plan_id` into params.

	// For first version, let's assume `_plan_id` is passed in params.
	planID, _ := step.Params["_plan_id"].(string)

	rootDir, err := os.Getwd()
	if err != nil {
		return err
	}

	// Adjust for running from core/cmd vs root
	if strings.HasSuffix(rootDir, "core") {
		rootDir = filepath.Dir(rootDir)
	} else if strings.HasSuffix(rootDir, "cmd") {
		rootDir = filepath.Dir(filepath.Dir(rootDir))
	}

	// If planID is missing, we might fail to find the file if we rely on it.
	// Fallback strategy: Search in .druppie/files/*/<filename>
	path := ""
	if planID != "" {
		path = filepath.Join(rootDir, ".druppie", "files", planID, filename)
	} else {
		// Search
		glob := filepath.Join(rootDir, ".druppie", "files", "*", filename)
		matches, _ := filepath.Glob(glob)
		if len(matches) > 0 {
			path = matches[0] // Pick first
		}
	}

	if path == "" {
		return fmt.Errorf("file not found in store: %s", filename)
	}

	// Read
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	content := string(data)
	if len(content) > 2000 {
		content = content[:2000] + "\n... (truncated)"
	}

	outputChan <- fmt.Sprintf("✅ [File Reader] Read %d bytes.", len(data))
	outputChan <- fmt.Sprintf("RESULT_FILE_CONTENT=%s", content)

	return nil
}
