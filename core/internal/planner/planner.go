package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/drug-nl/druppie/core/internal/llm"
	"github.com/drug-nl/druppie/core/internal/model"
	"github.com/drug-nl/druppie/core/internal/registry"
)

type Planner struct {
	llm      llm.Provider
	registry *registry.Registry
}

func NewPlanner(llm llm.Provider, reg *registry.Registry) *Planner {
	return &Planner{
		llm:      llm,
		registry: reg,
	}
}

const systemPromptTmpl = `You are a Planner Agent.
Goal: %s
Action: %s
Available Tools (Building Blocks): %v
Available Agents: %v

Break this down into execution steps.
Assign each step to the most appropriate Agent from the "Available Agents" list.
If a step requires a specific building block, mention it in params.

Output JSON array of objects:
[
  { "step_id": 1, "agent_id": "REPLACE_WITH_AGENT_ID", "action": "...", "params": {...}, "description": "..." }
]
`

func (p *Planner) CreatePlan(ctx context.Context, intent model.Intent) (model.ExecutionPlan, error) {
	// 1. Gather Context from Registry
	blocks := p.registry.ListBuildingBlocks()
	blockNames := make([]string, 0, len(blocks))
	for _, b := range blocks {
		blockNames = append(blockNames, b.Name)
	}

	agents := p.registry.ListAgents()
	agentList := make([]string, 0, len(agents))
	for _, a := range agents {
		// Format: ID (Name) - Description
		agentList = append(agentList, fmt.Sprintf("%s (%s): %s", a.ID, a.Name, a.Description))
	}

	// 2. Prompt LLM
	sysPrompt := fmt.Sprintf(systemPromptTmpl, intent.Summary, intent.Action, blockNames, agentList)
	resp, err := p.llm.Generate(ctx, "Generate plan data", sysPrompt)
	if err != nil {
		return model.ExecutionPlan{}, err
	}

	// 3. Parse Response
	var steps []model.Step
	if err := json.Unmarshal([]byte(resp), &steps); err != nil {
		return model.ExecutionPlan{}, fmt.Errorf("failed to parse planner response: %w. Raw: %s", err, resp)
	}

	// 4. Construct Plan
	plan := model.ExecutionPlan{
		ID:     fmt.Sprintf("plan-%d", time.Now().Unix()),
		Intent: intent,
		Status: "created",
		Steps:  steps,
	}

	return plan, nil
}
