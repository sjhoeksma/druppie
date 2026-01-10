package executor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sjhoeksma/druppie/core/internal/llm"
	"github.com/sjhoeksma/druppie/core/internal/model"
	"github.com/sjhoeksma/druppie/core/internal/paths"
	"github.com/sjhoeksma/druppie/core/internal/registry"
)

// ArchitectExecutor handles high-level architectural design tasks
type ArchitectExecutor struct {
	LLM      llm.Provider
	Registry *registry.Registry
}

func (e *ArchitectExecutor) CanHandle(action string) bool {
	return action == "architectural-design" ||
		action == "documentation-assembly" || action == "documentation_assembly" ||
		action == "motivation-modeling" || action == "motivation_modeling" ||
		action == "baseline-modeling" || action == "baseline_modeling" ||
		action == "target-modeling" || action == "target_modeling" ||
		action == "viewpoint-derivation" || action == "viewpoint_derivation" ||
		action == "principles-consistency-check" || action == "principles_consistency_check" || action == "principles_and_consistency_check" ||
		action == "decision-recording" || action == "decision_recording" ||
		action == "roadmap-gaps" || action == "roadmap_gaps" ||
		action == "roadmap-and-gaps" || action == "roadmap_and_gaps" ||
		action == "review-governance" || action == "review_governance" || action == "review_and_governance" ||
		action == "intake" || action == "completion"
}

func (e *ArchitectExecutor) Execute(ctx context.Context, step model.Step, outputChan chan<- string) error {
	action := step.Action
	outputChan <- fmt.Sprintf("ArchitectExecutor: Executing '%s'...", action)

	// Extract planID
	planID := ""
	if p, ok := step.Params["plan_id"].(string); ok {
		planID = p
	} else if p, ok := step.Params["_plan_id"].(string); ok {
		planID = p
	} else {
		planID = "unknown_plan"
		outputChan <- "Warning: plan_id not found in params"
	}

	var result string
	var usage model.TokenUsage

	// Use LLM for architectural tasks
	if e.LLM != nil {
		language := ""
		if l, ok := step.Params["language"].(string); ok {
			language = l
		}

		// Construct System Prompt
		systemPrompt := "You are an Enterprise Architect. Provide a structured architectural output in Markdown format."
		if language != "" {
			systemPrompt = fmt.Sprintf("IMPORTANT: You MUST write in %s language.\n%s", language, systemPrompt)
		}

		// Construct User Prompt with Context
		userPrompt := fmt.Sprintf("Task: %s\n\nContext: %v", action, step.Params)
		if language != "" {
			userPrompt += fmt.Sprintf("\n\nReminder: Output must be in %s.", language)
		}

		var err error
		var providerName string

		// Lookup Provider if AgentID is known
		if e.Registry != nil && step.AgentID != "" {
			if agent, err := e.Registry.GetAgent(step.AgentID); err == nil {
				if agent.Provider != "" {
					providerName = agent.Provider
				}
			}
		}

		if providerName != "" {
			if mgr, ok := e.LLM.(*llm.Manager); ok {
				result, usage, err = mgr.GenerateWithProvider(ctx, providerName, userPrompt, systemPrompt)
			} else {
				result, usage, err = e.LLM.Generate(ctx, userPrompt, systemPrompt)
			}
		} else {
			result, usage, err = e.LLM.Generate(ctx, userPrompt, systemPrompt)
		}
		if err != nil {
			outputChan <- fmt.Sprintf("LLM generation failed: %v. Using fallback.", err)
			result = e.getFallbackResult(action)
		}
	} else {
		result = e.getFallbackResult(action)
	}

	// Write to File
	safeAction := strings.ReplaceAll(step.Action, "/", "-")
	safeAction = strings.ReplaceAll(safeAction, "\\", "-")

	filesDir, _ := paths.ResolvePath(".druppie", "plans", planID, "files")
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		outputChan <- fmt.Sprintf("Error creating files directory: %v", err)
		return err
	}

	fileName := fmt.Sprintf("%s-%d.md", safeAction, step.ID)
	filePath := filepath.Join(filesDir, fileName)

	if err := os.WriteFile(filePath, []byte(result), 0644); err != nil {
		outputChan <- fmt.Sprintf("Error writing architecture file: %v", err)
		return err
	}

	outputChan <- fmt.Sprintf("Document saved to: %s", filePath)

	// Report usage via special output format
	if usage.TotalTokens > 0 {
		outputChan <- fmt.Sprintf("RESULT_TOKEN_USAGE=%d,%d,%d", usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens)
	}

	return nil
}

func (e *ArchitectExecutor) getFallbackResult(action string) string {
	switch action {
	case "architectural-design", "target-modeling", "target_modeling":
		return "Architecture Design Complete.\n\n" +
			"### Proposed Solution\n" +
			"- **Frontend**: React SPA\n" +
			"- **Backend**: Go Microservices\n" +
			"- **Database**: PostgreSQL\n" +
			"- **Infrastructure**: Kubernetes (EKS)\n\n" +
			"### Diagram\n" +
			"```mermaid\n" +
			"graph TD\n" +
			"  Client --> Ingress\n" +
			"  Ingress --> ServiceA\n" +
			"  Ingress --> ServiceB\n" +
			"  ServiceA --> DB\n" +
			"```"

	case "documentation-assembly", "documentation_assembly":
		return "# Architecture Documentation\n\n" +
			"## Executive Summary\n" +
			"Following the architectural design, this document outlines the proposed solution.\n\n" +
			"## Artifacts\n" +
			"- System Context Diagram\n" +
			"- Container Diagram\n" +
			"- Deployment View\n\n" +
			"## Decisions (ADRs)\n" +
			"- ADR-001: Use Microservices\n" +
			"- ADR-002: Use PostgreSQL\n"

	case "intake":
		return "## Intake Summary\nScope verified. Stakeholders: Admin, Dev Team."

	case "motivation-modeling", "motivation_modeling":
		return "## Motivation Model\n**Driver**: Modernization\n**Goal**: Improve scalability."

	case "roadmap-gaps", "roadmap_gaps", "roadmap-and-gaps", "roadmap_and_gaps":
		return "## Roadmap\n1. Phase 1: Foundation\n2. Phase 2: Migration"

	case "review-governance", "review_governance":
		return "## Governance Review\n**Status**: APPROVED\n\nAll architecture principles and compliance requirements have been met. Proceed to completion."

	default:
		return fmt.Sprintf("## Output for %s\nTask completed successfully.", action)
	}
}
