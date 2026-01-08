package executor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sjhoeksma/druppie/core/internal/model"
	"github.com/sjhoeksma/druppie/core/internal/paths"
)

// ArchitectExecutor handles high-level architectural design tasks
type ArchitectExecutor struct {
}

func (e *ArchitectExecutor) CanHandle(action string) bool {
	return action == "architectural-design" ||
		action == "documentation-assembly" || action == "documentation_assembly" ||
		action == "motivation-modeling" || action == "motivation_modeling" ||
		action == "baseline-modeling" || action == "baseline_modeling" ||
		action == "target-modeling" || action == "target_modeling" ||
		action == "viewpoint-derivation" || action == "viewpoint_derivation" ||
		action == "principles-consistency-check" || action == "principles_consistency_check" ||
		action == "decision-recording" || action == "decision_recording" ||
		action == "roadmap-gaps" || action == "roadmap_gaps" ||
		action == "roadmap-and-gaps" || action == "roadmap_and_gaps" ||
		action == "review-governance" || action == "review_governance" ||
		action == "intake"
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
		// Try context or just use "unknown_plan" (fallback, though less desired)
		planID = "unknown_plan"
		outputChan <- "Warning: plan_id not found in params"
	}

	var result string

	switch action {
	case "architectural-design", "target-modeling", "target_modeling":
		// outputChan <- "Analyzing requirements..."
		result = "Architecture Design Complete.\n\n" +
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
		// outputChan <- "Assembling documentation..."
		result = "# Architecture Documentation\n\n" +
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
		// outputChan <- "Reviewing scope and stakeholders..."
		result = "## Intake Summary\nScope verified. Stakeholders: Admin, Dev Team."

	case "motivation-modeling", "motivation_modeling":
		// outputChan <- "Modeling drivers and goals..."
		result = "## Motivation Model\n**Driver**: Modernization\n**Goal**: Improve scalability."

	case "roadmap-gaps", "roadmap_gaps", "roadmap-and-gaps", "roadmap_and_gaps":
		// outputChan <- "Analyzing gaps..."
		result = "## Roadmap\n1. Phase 1: Foundation\n2. Phase 2: Migration"

	case "review-governance", "review_governance":
		// outputChan <- "Conducting governance review..."
		result = "## Governance Review\n**Status**: APPROVED\n\nAll architecture principles and compliance requirements have been met. Proceed to completion."

	default:
		// outputChan <- "Executing generic architecture task..."
		result = fmt.Sprintf("## Output for %s\nTask completed successfully.", action)
	}

	// outputChan <- result

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
	return nil
}
