package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
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
3. **Agent Priority**: Available Agents are listed in PRIORITY order. Highest priority agents (e.g. 'business-analyst') should typically lead the plan or be used for initial scoping.
4. **Precision First**: Review the 'Goal' carefully. If the User has already provided details (e.g. duration, audience, platform), **DO NOT** ask for them again. 
5. **Elicitation**: Only use 'Business Analyst' -> 'ask_questions' if critical information is missing to proceed. 
   - **Minimalism**: Ask a maximum of 3-5 high-impact questions. Do NOT provide long lists.
   - **No Duplicates**: Ensure every question is unique. Never repeat the same question.
   - **Format**: In the 'description', start with a summary, then state what is missing.
   - **Params**: Include 'questions' (list) and 'assumptions' (list).
6. **Agent Selection & Sequencing**:
   - Use 'business-analyst' (Priority 100) first to perform any elicitation or scoping. If the 'Goal' is missing key parameters (audience, duration, platform), use 'business-analyst' -> 'ask_questions'.
   - Use 'content-creator' (Priority 5) ONLY once the scope is clear to generate 'script_outline' or creative assets.
   - **CRITICAL**: 'business-analyst' must NEVER generate a 'script_outline'. That is the job of the 'content-creator'. Only move to 'content-creator' once all elicitation is complete.
7. **Structure Rules**:
   - **script_outline**: MUST be a JSON array of strings (scenes), NOT a single string and NOT an array of objects.
   - **Scene Format**: "<duration> <title>: <prompt>" (e.g. ["0:00-0:10 Intro: Kind ziet vuil water.", "0:10-0:40 Kern: Het zuiveringsproces."])
   - **Completeness**: Generate as much of the plan as possible in one go. If you have enough information to generate content (like a script outline), do it immediately in the same response.
   - Use 'id' from the 'Available Agents' list for 'agent_id'.

CRITICAL INSTRUCTION ON LANGUAGE:
The 'User Language' is defined above.
1. **Internal Logic**: 'agent_id', 'action', and base JSON keys MUST be in ENGLISH.
	- **agent_id**: Use the literal 'id' from the list.
	- **action**: MUST be a literal string selected from the 'Skills' list of that agent (e.g. 'copywriting', 'ask_questions', 'ensure_availability'). Do NOT invent action names or use the agent_id as the action.
2. **User Facing Content**: 'description', and ALL fields/values inside 'params' that contain human-readable text (questions, summaries, assumptions, script outlines, titles, etc.) MUST be in the USER LANGUAGE. Do NOT translate creative content to English.
3. **Questioning**: For 'ask_questions', you **MUST** include an 'assumptions' list in params (target language) matching the question count.
Example if User Language code is 'nl' (Dutch):
{
  "step_id": 1,
  "agent_id": "business-analyst",
  "action": "ask_questions", 
  "params": { 
     "questions": ["Wat is de visuele stijl van de video?"],
     "assumptions": ["Eenvoudige animatie geschikt voor kinderen"] 
  },
  "description": "Ik ga een video maken over waterzuivering voor kinderen. Om te beginnen moet ik de visuele stijl weten."
}

7. **Agent Selection**: Use the literal 'id' from the list.
- Use 'business-analyst' for any elicitation or project scoping.
- Use 'content-creator' for creative assets.

Break this down into execution steps.
Output JSON array of objects:
[
  { "step_id": 1, "agent_id": "LITERAL_ID_FROM_LIST", "action": "...", "params": {...}, "description": "..." }
]
`

func (p *Planner) cleanJSONResponse(resp string) string {
	clean := strings.TrimSpace(resp)
	if start := strings.Index(clean, "```"); start != -1 {
		if newline := strings.Index(clean[start:], "\n"); newline != -1 {
			start += newline + 1
		} else {
			start += 3
		}
		end := strings.LastIndex(clean, "```")
		if end > start {
			clean = clean[start:end]
		} else {
			clean = clean[start:]
		}
	}
	clean = strings.TrimSpace(clean)

	// Robustly close brackets and braces
	depthMap := map[rune]int{'{': 0, '[': 0}
	for _, r := range clean {
		switch r {
		case '{':
			depthMap['{']++
		case '}':
			depthMap['{']--
		case '[':
			depthMap['[']++
		case ']':
			depthMap['[']--
		}
	}
	// Append missing closers in reverse order? Simple check:
	for depthMap['['] > 0 {
		clean += "]"
		depthMap['[']--
	}
	for depthMap['{'] > 0 {
		clean += "}"
		depthMap['{']--
	}
	return clean
}

func (p *Planner) selectRelevantAgents(ctx context.Context, intent model.Intent, agents []model.AgentDefinition) []string {
	var detailedList []string
	for _, a := range agents {
		detailedList = append(detailedList, fmt.Sprintf("%s: %s", a.ID, a.Description))
	}
	prompt := fmt.Sprintf("Goal: %s\nAvailable Agents:\n%v\n\nTask: Return exactly one JSON array of strings containing Agent IDs. Be extremely restrictive.\nGuidelines:\n- For creative tasks (videos, blogs), use 'content-creator'.\n- For research/data tasks, use 'data-scientist'.\n- For infrastructure/ops, use 'infrastructure-engineer'.\n- For task refinement or if the goal is vague, ALWAYS include 'business-analyst'.\nExample: [\"business-analyst\", \"content-creator\"]", intent.Prompt, detailedList)
	resp, err := p.llm.Generate(ctx, "Select Agents", prompt)
	if err != nil {
		return nil
	}

	clean := p.cleanJSONResponse(resp)
	var selected []string
	if err := json.Unmarshal([]byte(clean), &selected); err != nil {
		// Try parsing as a wrapped object
		var wrapped struct {
			SelectedAgents []string `json:"selected_agents"`
		}
		if err2 := json.Unmarshal([]byte(clean), &wrapped); err2 == nil && len(wrapped.SelectedAgents) > 0 {
			selected = wrapped.SelectedAgents
		} else {
			if p.Debug {
				fmt.Printf("[Planner] Failed to parse agent selection: %v. Raw: %s\n", err, resp)
			}
			return nil
		}
	}
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
		for _, a := range allAgents {
			for _, sel := range selectedIDs {
				if a.ID == sel { // Case sensitive? specific ID matching
					activeAgents = append(activeAgents, a)
					break
				}
			}
		}
	} else {
		// Fallback to all if selection failed or nothing selected
		activeAgents = allAgents
		selectedIDs = make([]string, 0, len(allAgents))
		for _, a := range allAgents {
			selectedIDs = append(selectedIDs, a.ID)
		}
	}

	// Sort agents by priority (descending)
	sort.Slice(activeAgents, func(i, j int) bool {
		return activeAgents[i].Priority > activeAgents[j].Priority
	})

	sortedIDs := make([]string, 0, len(activeAgents))
	agentList := make([]string, 0, len(activeAgents))
	for _, a := range activeAgents {
		sortedIDs = append(sortedIDs, a.ID)
		// Format: ID (Name) - Skills - Tools - Description
		agentList = append(agentList, fmt.Sprintf("%s (%s)\n  Skills: %v\n  Tools: %v\n  Description: %s", a.ID, a.Name, a.Skills, a.Tools, a.Description))
	}
	fmt.Printf("[Planner - Agents] %v\n", sortedIDs)

	// 2. Prompt LLM
	sysPrompt := fmt.Sprintf(systemPromptTmpl, intent.Prompt, intent.Action, intent.Language, blockNames, agentList)

	resp, err := p.llm.Generate(ctx, "Generate plan data", sysPrompt)
	if err != nil {
		return model.ExecutionPlan{}, err
	}

	// 3. Parse Response
	cleanResp := p.cleanJSONResponse(resp)

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

	// Ensure all steps have a status
	for i := range steps {
		if steps[i].Status == "" {
			steps[i].Status = "pending"
		}
	}

	// 4. Construct Plan
	plan := model.ExecutionPlan{
		ID:             fmt.Sprintf("plan-%d", time.Now().Unix()),
		Intent:         intent,
		Status:         "created",
		Steps:          steps,
		SelectedAgents: selectedIDs,
	}

	// Persistent Logging
	if p.Store != nil {
		_ = p.Store.SavePlan(plan)
		planJSON, _ := json.MarshalIndent(plan, "", "  ")
		_ = p.Store.LogInteraction(plan.ID, "Planner Create", sysPrompt, resp+"\n\nRESULTING PLAN:\n"+string(planJSON))
	}

	return plan, nil
}

// UpdatePlan updates an existing plan based on user feedback or answers.
func (p *Planner) UpdatePlan(ctx context.Context, plan *model.ExecutionPlan, feedback string) (*model.ExecutionPlan, error) {
	// 0. Handle Feedback
	// Find the first pending step that would have triggered this feedback
	for i := range plan.Steps {
		if plan.Steps[i].Status == "pending" {
			// If it's a question or content creation step, mark it as completed
			action := plan.Steps[i].Action
			if action == "ask_questions" || action == "copywriting" || action == "video-design" || action == "content-creator" {
				plan.Steps[i].Status = "completed"
				plan.Steps[i].Result = feedback
				break
			}
		}
	}

	// 1. Construct Effective Goal (Context)
	effectiveGoal := plan.Intent.InitialPrompt
	for _, s := range plan.Steps {
		if s.Result != "" {
			effectiveGoal += fmt.Sprintf("\n- %s", s.Result)
		}
	}
	// Update the active prompt to reflect the full gathered context
	plan.Intent.Prompt = effectiveGoal

	// Truncate pending steps: we want the LLM to redefine the future of the plan
	// based on the new information/feedback.
	var completedSteps []model.Step
	for _, s := range plan.Steps {
		if s.Status == "completed" {
			completedSteps = append(completedSteps, s)
		}
	}
	plan.Steps = completedSteps

	// 2. Re-Prompt LLM for Next Steps
	// We gather the available agents/tools context again to ensure the LLM doesn't hallucinate IDs
	allAgents := p.registry.ListAgents()
	var activeAgents []model.AgentDefinition
	if len(plan.SelectedAgents) > 0 {
		for _, a := range allAgents {
			for _, sel := range plan.SelectedAgents {
				if a.ID == sel {
					activeAgents = append(activeAgents, a)
					break
				}
			}
		}
	} else {
		activeAgents = allAgents
	}

	// Sort agents by priority (descending)
	sort.Slice(activeAgents, func(i, j int) bool {
		return activeAgents[i].Priority > activeAgents[j].Priority
	})

	sortedIDs := make([]string, 0, len(activeAgents))
	agentList := make([]string, 0, len(activeAgents))
	for _, a := range activeAgents {
		sortedIDs = append(sortedIDs, a.ID)
		// Format: ID (Name) - Skills - Tools - Description
		agentList = append(agentList, fmt.Sprintf("%s (%s)\n  Skills: %v\n  Tools: %v\n  Description: %s", a.ID, a.Name, a.Skills, a.Tools, a.Description))
	}
	fmt.Printf("[Planner - Agents] %v\n", sortedIDs)

	blocks := p.registry.ListBuildingBlocks()
	blockNames := make([]string, 0, len(blocks))
	for _, b := range blocks {
		blockNames = append(blockNames, b.Name)
	}

	prompt := fmt.Sprintf(
		"You are a Planner Agent continuing a session.\n\n"+
			"--- CONTEXT ---\n"+
			"Objective: %s\n"+
			"User Language: %s\n"+
			"Available Agents: %v\n"+
			"Available Tools: %v\n\n"+
			"--- HISTORY & PROGRESS ---\n"+
			"Current Steps (with results): %v\n"+
			"Latest User Input: %s\n\n"+
			"--- TASK ---\n"+
			"1. REPLAN: Review 'Objective' and 'Current Steps'. If a step has a 'result', that info is now known.\n"+
			"2. PRECISION: **DO NOT** ask for information already present in the 'Objective' or in 'results'.\n"+
			"3. GENERATE: Provide NEXT steps (starting from id %d). If the goal is not fully realized, add steps for the next phase (e.g. Asset Generation, Infrastructure).\n"+
			"4. NO DUPLICATES: Review 'Current Steps'. If a task (like 'ask_questions' or 'script_outline' generation) has already been COMPLETED and has a 'result', **DO NOT** create a new step for it.\n"+
			"5. MINIMALISM: Ask a maximum of 3-5 high-impact questions only if absolutely necessary.\n"+
			"6. LANGUAGE: Use %s for all user-facing text. action/agent_id must be English.\n"+
			"7. **FORMATTING**: 'script_outline' MUST be a JSON array of strings in the format: \"<duration> <title>: <prompt>\".\n"+
			"8. **CONTENT**: When generating steps for a creative agent (e.g. content-creator), you MUST provide the actual content (like 'script_outline') in the 'params' field based on the gathered requirements.\n"+
			"9. **TRANSITION**: Once creative assets (like 'script_outline') are in the 'results', use technical agents (architect, infrastructure-engineer) and 'Available Tools' to move towards production/deployment.\n"+
			"10. OUTPUT: Return a JSON array of Step objects.",
		effectiveGoal,
		plan.Intent.Language,
		agentList,
		blockNames,
		plan.Steps,
		feedback,
		len(plan.Steps)+1,
		plan.Intent.Language,
	)

	resp, err := p.llm.Generate(ctx, "Refine Plan", prompt)
	if err != nil {
		return nil, err
	}

	// 3. Parse and Append
	cleanResp := p.cleanJSONResponse(resp)
	var newSteps []model.Step
	if err := json.Unmarshal([]byte(cleanResp), &newSteps); err != nil {
		// Try wrapped
		var wrapped struct {
			Steps []model.Step `json:"steps"`
		}
		if err2 := json.Unmarshal([]byte(cleanResp), &wrapped); err2 == nil {
			newSteps = wrapped.Steps
		} else {
			// Try single object
			var single model.Step
			if err3 := json.Unmarshal([]byte(cleanResp), &single); err3 == nil && single.Action != "" {
				newSteps = []model.Step{single}
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
		planJSON, _ := json.MarshalIndent(plan, "", "  ")
		_ = p.Store.LogInteraction(plan.ID, "Planner Update", prompt, resp+"\n\nRESULTING PLAN:\n"+string(planJSON))
	}

	return plan, nil
}
