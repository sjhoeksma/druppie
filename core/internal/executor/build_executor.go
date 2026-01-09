package executor

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/sjhoeksma/druppie/core/internal/builder"
	"github.com/sjhoeksma/druppie/core/internal/model"
	"github.com/sjhoeksma/druppie/core/internal/paths"
)

// BuildExecutor handles "build_code" actions
type BuildExecutor struct {
	Builder builder.BuildEngine
}

func (e *BuildExecutor) CanHandle(action string) bool {
	return action == "build_code"
}

func (e *BuildExecutor) Execute(ctx context.Context, step model.Step, outputChan chan<- string) error {
	outputChan <- fmt.Sprintf("Starting build for Step %d...", step.ID)

	// Extract params
	repoURL, _ := step.Params["repo_url"].(string)
	commitHash, _ := step.Params["commit_hash"].(string)

	// Fallback: if repo_url is missing, maybe "source_path" or "path"
	if repoURL == "" {
		if p, ok := step.Params["path"].(string); ok {
			repoURL = p
		} else if p, ok := step.Params["source_path"].(string); ok {
			repoURL = p
		}
	}

	// Default to plan directory if internal context is present
	planID := ""
	if p, ok := step.Params["plan_id"].(string); ok {
		planID = p
	} else if p, ok := step.Params["_plan_id"].(string); ok {
		planID = p
	}

	var warning string
	var err error
	repoURL, warning, err = paths.ResolveRepoURL(repoURL, planID)
	if err != nil {
		return err
	}
	if warning != "" {
		outputChan <- warning
	}

	// Check if source directory exists and is not empty
	if _, err := os.Stat(repoURL); os.IsNotExist(err) {
		return fmt.Errorf("source directory '%s' does not exist. You must call 'create_repo' before 'build_code'", repoURL)
	}
	if entries, err := os.ReadDir(repoURL); err != nil || len(entries) == 0 {
		return fmt.Errorf("source directory '%s' is empty. You must call 'create_repo' with content before 'build_code'", repoURL)
	}

	// Define Log Path
	var logPath string

	if planID != "" {
		logPath, _ = paths.ResolvePath(".druppie", "plans", planID, "logs", "execution.log")
	}

	// Create Log Stream Adapter
	// Adapter that writes lines to outputChan
	// Create Log Stream Adapter
	// Adapter that writes lines to outputChan
	pr, pw := io.Pipe()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			outputChan <- scanner.Text()
		}
	}()
	// Note: We need to Close pw eventually?
	// TriggerBuild should close it? No, TriggerBuild takes Writer.
	// We can't easily wait for TriggerBuild to finish writing before returning here?
	// TriggerBuild is synchronous in LocalClient.
	// So we can defer close.

	// Trigger Build
	buildID, err := e.Builder.TriggerBuild(ctx, repoURL, commitHash, logPath, pw)
	if err != nil {
		return err
	}

	// Ensure logs are flushed
	pw.Close()
	wg.Wait()

	outputChan <- fmt.Sprintf("Build triggered successfully. Build ID: %s", buildID)
	outputChan <- fmt.Sprintf("RESULT_BUILD_ID=%s", buildID)

	// Construct artifact path for next steps
	// Convention: repoURL/../builds/buildID
	// But repoURL might be relative. The builder knows the absolute path.
	// Ideally Builder returns more info.
	// For now, let's assume the standard convention.
	// "We built code into .druppie/plans/plan-<id>/builds/<build-id>"
	// So if repoURL was ".druppie/plans/plan-123/src", result is in "../builds/<id>"

	// Just pass buildID as main result. Run Agent needs to know how to use it.

	return nil
}
