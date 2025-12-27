package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sjhoeksma/druppie/core/internal/llm"
	"github.com/sjhoeksma/druppie/core/internal/model"
	"github.com/sjhoeksma/druppie/core/internal/registry"
	"github.com/sjhoeksma/druppie/core/internal/store"
)

type Planner struct {
	llm      llm.Provider
	registry *registry.Registry
	Store    store.Store
	Debug    bool
}

func NewPlanner(llm llm.Provider, reg *registry.Registry, store store.Store, debug bool) *Planner {
	return &Planner{
		llm:      llm,
		registry: reg,
		Store:    store,
		Debug:    debug,
	}
}

const systemPromptTmpl = `You are a Planner Agent.
Goal: %s
Action: %s
User Language: %s
Available Tools (Building Blocks): %v
Available Agents: %v

Strategies:
1. **Reuse over Rebuild**: Check 'Available Tools'. If a block matches the need (e.g. 'ai-video-comfyui' for video), USE IT. Do NOT design generic architecture or provision generic clusters if a specific Block exists.
2. **Ensure Availability**: Before using a Service Block, create a step for 'Infrastructure Engineer' to 'ensure_availability' of that block. This step must check status. IMPORTANT: Include a param 'if_missing' describing the deployment action (e.g. "Deploy ai-video-comfyui from Building Block Library") to execute if the block is not found.
3. **Elicitation First**: If the Goal is vague (missing audience, style, etc.), your first step MUST be 'Business Analyst' -> 'ask_questions'.
   - **Format**: In the step 'description', start with a **Summary** of what you understood (e.g. "I understand you want a video about wind."), then state what is missing.
   - **Params**: Include 'summary' (string) and 'questions' (list).
   - Example: { "step_id": 1, "agent_id": "business-analyst", "action": "ask_questions", "params": { "summary": "Project: Video about Wind.", "questions": ["Target Audience?", "Style?"] }, "description": "I understand you want a video about wind. To proceed, I need to know the target audience and style." }

CRITICAL INSTRUCTION ON LANGUAGE:
The 'User Language' is defined above.
1. INTERNAL LOGIC (agent_id, action, parameter keys) MUST be in ENGLISH.
2. USER FACING TEXT (description, params.questions, params.summary, params.assumptions) MUST be in the USER LANGUAGE.
3. For 'ask_questions', you **MUST** include an 'assumptions' list in params (in USER LANGUAGE), describing what you will assume if the user simply 'accepts' without answering. The length of 'assumptions' MUST match the length of 'questions'.
Example if User Language code is 'nl' (Dutch):
{
  "step_id": 1,
  "agent_id": "business-analyst",
  "action": "ask_questions", 
  "params": { 
     "questions": ["Wat is de doelgroep?", "Wat is de stijl?"],
     "assumptions": ["Algemeen publiek (General Audience)", "Informatief (Informative)"] 
  },
  "description": "Ik begrijp dat u een video wilt. Ik heb meer details nodig."
}

Break this down into execution steps.
Output JSON array of objects:
[
  { "step_id": 1, "agent_id": "REPLACE_WITH_AGENT_ID", "action": "...", "params": {...}, "description": "..." }
]
`

func (p *Planner) selectRelevantAgents(ctx context.Context, intent model.Intent, agents []model.AgentDefinition) []string {
	var detailedList []string
	for _, a := range agents {
		detailedList = append(detailedList, fmt.Sprintf("%s: %s", a.ID, a.Description))
	}
	prompt := fmt.Sprintf("Goal: %s\nAvailable Agents:\n%v\n\nTask: Return a valid JSON array of strings containing ONLY the Agent IDs strictly necessary for this goal. Do not include all agents unless necessary.", intent.Summary, detailedList)
	resp, err := p.llm.Generate(ctx, "Select Agents", prompt)
	if err != nil {
		return nil
	}
	var selected []string
	_ = json.Unmarshal([]byte(resp), &selected)
	return selected
}

func (p *Planner) CreatePlan(ctx context.Context, intent model.Intent) (model.ExecutionPlan, error) {
	// 1. Gather Context from Registry
	blocks := p.registry.ListBuildingBlocks()
	blockNames := make([]string, 0, len(blocks))
	for _, b := range blocks {
		blockNames = append(blockNames, b.Name)
	}

	allAgents := p.registry.ListAgents()

	// Filter Agents
	selectedIDs := p.selectRelevantAgents(ctx, intent, allAgents)
	var activeAgents []model.AgentDefinition
	if len(selectedIDs) > 0 {
		fmt.Printf("[Planner] Selected Agents: %v\n", selectedIDs)
		for _, a := range allAgents {
			for _, sel := range selectedIDs {
				if a.ID == sel { // Case sensitive? specific ID matching
					activeAgents = append(activeAgents, a)
					break
				}
			}
		}
	} else {
		// Fallback to all if selection failed
		activeAgents = allAgents
	}

	agentList := make([]string, 0, len(activeAgents))
	for _, a := range activeAgents {
		// Format: ID (Name) - Skills - Description
		agentList = append(agentList, fmt.Sprintf("%s (%s)\n  Skills: %v\n  Description: %s", a.ID, a.Name, a.Skills, a.Description))
	}

	// 2. Prompt LLM
	sysPrompt := fmt.Sprintf(systemPromptTmpl, intent.Summary, intent.Action, intent.Language, blockNames, agentList)

	// Persistent Logging
	logFile := ".logs/ai_interaction.log"
	f, fileErr := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if fileErr == nil {
		defer f.Close()
		timestamp := time.Now().Format(time.RFC3339)
		f.WriteString(fmt.Sprintf("--- [Planner] %s ---\nINPUT:\n%s\n", timestamp, sysPrompt))
	}

	resp, err := p.llm.Generate(ctx, "Generate plan data", sysPrompt)
	if err != nil {
		return model.ExecutionPlan{}, err
	}

	if fileErr == nil {
		f.WriteString(fmt.Sprintf("OUTPUT:\n%s\n\n", resp))
	}

	// 3. Parse Response
	cleanResp := strings.TrimSpace(resp)
	// Extract JSON from markdown code blocks if present
	if start := strings.Index(cleanResp, "```"); start != -1 {
		// Find end of line for start tag (e.g. ```json)
		if newline := strings.Index(cleanResp[start:], "\n"); newline != -1 {
			start += newline + 1
		} else {
			start += 3
		}
		end := strings.LastIndex(cleanResp, "```")
		if end > start {
			cleanResp = cleanResp[start:end]
		} else {
			cleanResp = cleanResp[start:] // No end tag found, maybe truncated
		}
	}
	cleanResp = strings.TrimSpace(cleanResp)

	// Auto-repair common truncation
	if strings.HasPrefix(cleanResp, "[") && !strings.HasSuffix(cleanResp, "]") {
		cleanResp += "]"
	}
	if strings.HasPrefix(cleanResp, "{") && !strings.HasSuffix(cleanResp, "}") {
		cleanResp += "}"
	}

	var steps []model.Step

	// Attempt 1: Direct Array
	if err := json.Unmarshal([]byte(cleanResp), &steps); err != nil {
		// Attempt 2: Wrapped Object {"steps": [...]}
		var wrapped struct {
			Steps []model.Step `json:"steps"`
		}
		if err2 := json.Unmarshal([]byte(cleanResp), &wrapped); err2 == nil && len(wrapped.Steps) > 0 {
			steps = wrapped.Steps
		} else {
			// Attempt 3: Single Object
			var singleStep model.Step
			if err3 := json.Unmarshal([]byte(cleanResp), &singleStep); err3 == nil && singleStep.Action != "" {
				steps = []model.Step{singleStep}
			} else {
				// Attempt 4: Error Object
				var errorResp struct {
					Error string `json:"error"`
				}
				if err4 := json.Unmarshal([]byte(cleanResp), &errorResp); err4 == nil && errorResp.Error != "" {
					steps = []model.Step{
						{
							ID:          1,
							AgentID:     "business-analyst",
							Action:      "ask_questions",
							Params:      map[string]interface{}{"details_needed": errorResp.Error},
							Description: errorResp.Error,
							Status:      "pending",
						},
					}
				} else {
					return model.ExecutionPlan{}, fmt.Errorf("failed to parse planner response: %w. Raw: %s", err, cleanResp)
				}
			}
		}
	}

	// 4. Construct Plan
	plan := model.ExecutionPlan{
		ID:     fmt.Sprintf("plan-%d", time.Now().Unix()),
		Intent: intent,
		Status: "created",
		Steps:  steps,
	}

	// Save Initial Plan
	if p.Store != nil {
		_ = p.Store.SavePlan(plan)
	}

	return plan, nil
}

// UpdatePlan updates an existing plan based on user feedback or answers.
func (p *Planner) UpdatePlan(ctx context.Context, plan *model.ExecutionPlan, feedback string) (*model.ExecutionPlan, error) {
	// 0. Handle Feedback
	if len(plan.Steps) > 0 {
		lastIdx := len(plan.Steps) - 1
		// If it's a question step, attach the feedback as result
		if plan.Steps[lastIdx].Action == "ask_questions" {
			plan.Steps[lastIdx].Status = "completed"
			plan.Steps[lastIdx].Result = feedback
		}
	}

	// 1. Construct Effective Goal (Context)
	effectiveGoal := plan.Intent.Summary
	for _, s := range plan.Steps {
		if s.Result != "" {
			effectiveGoal += fmt.Sprintf("\n[Completed Step %d Result]: %s", s.ID, s.Result)
		}
	}

	// 2. Re-Prompt LLM for Next Steps
	// We construct a prompt that includes the current plan and the new feedback
	prompt := fmt.Sprintf(
		"You are continuing a planning session.\n"+
			"Current Goal: %s\n"+
			"Language: %s\n"+
			"Current Plan Steps: %v\n"+
			"User Just Said: %s\n\n"+
			"Task: Generate the next set of execution steps to proceed towards the goal. "+
			"Use %s as the User Language.\n"+
			"CRITICAL: Keep 'action' and 'agent_id' in ENGLISH. Write 'description', 'questions', and 'assumptions' in %s.\n"+
			"Must include 'assumptions' list for 'ask_questions'.\n"+
			"If you have enough info, generate the actual implementation steps. "+
			"If you still need info, output another 'ask_questions' step.\n"+
			"Output valid JSON array of steps.",
		effectiveGoal,
		plan.Intent.Language,
		plan.Steps,
		feedback,
		plan.Intent.Language,
		plan.Intent.Language,
	)

	resp, err := p.llm.Generate(ctx, "Refine Plan", prompt)
	if err != nil {
		return nil, err
	}

	// 3. Parse and Append
	// Reuse parsing logic (simplified here)
	var newSteps []model.Step
	if err := json.Unmarshal([]byte(resp), &newSteps); err != nil {
		// Try wrapped
		var wrapped struct {
			Steps []model.Step `json:"steps"`
		}
		if err2 := json.Unmarshal([]byte(resp), &wrapped); err2 == nil {
			newSteps = wrapped.Steps
		} else {
			// Try single object
			var single model.Step
			if err3 := json.Unmarshal([]byte(resp), &single); err3 == nil {
				newSteps = []model.Step{single}
			} else {
				// Ignore error for now, maybe LLM just chatted
				// return nil, fmt.Errorf("failed to parse update response: %w", err)
			}
		}
	}

	// Adjust IDs
	startID := 0
	if len(plan.Steps) > 0 {
		startID = plan.Steps[len(plan.Steps)-1].ID
	}
	for i := range newSteps {
		newSteps[i].ID = startID + i + 1
		newSteps[i].Status = "pending"
	}

	// Append
	plan.Steps = append(plan.Steps, newSteps...)

	// Save to Store
	if p.Store != nil {
		_ = p.Store.SavePlan(*plan)
	}

	return plan, nil
}
