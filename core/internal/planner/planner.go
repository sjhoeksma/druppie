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
- **Goal**: %goal%
- **Action**: %action%
- **User Language**: %language%
- **Available Tools (Building Blocks)**: %tools%
- **Available Agents**: %agents%

Strategies:
1. **Reuse over Rebuild**: Check 'Available Tools'. If a block matches the need (e.g. 'ai-video-comfyui' for video), USE IT. Do NOT design generic architecture or provision generic clusters if a specific Block exists.
2. **Ensure Availability**: Before using a Service Block, create a step for 'Infrastructure Engineer' to 'ensure_availability' of that block. This step must check status. IMPORTANT: Include a param 'if_missing' describing the deployment action (e.g. "Deploy ai-video-comfyui from Building Block Library") to execute if the block is not found.
3. **Agent Priority**: Available Agents are listed in PRIORITY order. Highest priority agents (e.g. 'business-analyst') should typically lead the plan or be used for initial scoping.
4. **Precision First**: Review the 'Goal' carefully. If the User has already provided details (e.g. duration, audience, platform), **DO NOT** ask for them again. 
5. **Elicitation**: Only use 'Business Analyst' -> 'ask_questions' if critical information is missing to proceed. 
   - **Minimalism**: Ask a maximum of 3-5 high-impact questions. Do NOT provide long lists.
   - **No Duplicates**: Ensure every question is unique. Never repeat the same question.
   - **Params**: Include 'questions' (list) and 'assumptions' (list).
6. **Agent Selection & Sequencing**:
   - Use 'business-analyst' (Priority 100) first to perform any elicitation or scoping. If the 'Goal' is missing key parameters (audience, duration, platform), use 'business-analyst' -> 'ask_questions'.
   - Use 'content-creator' -> 'content-review' (Priority 5) ONLY once the scope is clear to generate 'script_outline' or creative assets.
   - **CRITICAL**: 'business-analyst' must NEVER generate a 'script_outline'. That is the job of the 'content-creator'. Only move to 'content-creator' once all elicitation is complete.
7. **Structure Rules**:
   - **script_outline**: MUST be a JSON array of OBJECTS. Each object MUST have fields: 'duration', 'title', 'image_prompt' (for starting frame), and 'video_prompt' (for motion/action).
   - **Scene Format**: e.g. [{"duration": "10s", "title": "Intro", "image_prompt": "...", "video_prompt": "..."}]
   - **Completeness**: Generate as much of the plan as possible in one go. If you have enough information to generate content (like a script outline), do it immediately in the same response.
   - **Parallelism**: Use 'depends_on' (list of integers) to define dependencies. Steps with the EXACT same dependency requirements can run in parallel.
   - Use 'id' from the 'Available Agents' list for 'agent_id'.

8. **Production Workflow (Phase 2 & 3)**:
   - **Content Trigger**: If 'ask_questions' (Phase 1) is COMPLETED and 'script_outline' is missing in steps, Generate 'content-review' (Phase 2) with 'script_outline' params.
   - **Production Trigger**: If 'content-review' step is COMPLETED (with 'script_outline' in **PARAMS**, not Result), Transition to Phase 3.
     1. Create 'ensure_availability' step for required tools.
     2. Create parallel 'scene-creator' steps for EACH scene.
        - **Params**: Must be flat: {'scene_id': '...', 'image_prompt': '...', 'video_prompt': '...'}. 
        - **DependsOn**: All scene steps depend on 'ensure_availability'.
   - **Duplicate Guard**: Do NOT regenerate steps that are already completed.

CRITICAL INSTRUCTION ON LANGUAGE:
The 'User Language' is defined above.
1. **Internal Logic**: 'agent_id', 'action', and base JSON keys MUST be in ENGLISH.
   - **agent_id**: Use the literal 'id' from the list.
   - **action**: MUST be a literal string selected from the 'Skills' list of that agent (e.g. 'copywriting', 'ask_questions', 'ensure_availability'). Do NOT invent action names or use the agent_id as the action.
2. **User Facing Content**: ALL fields/values inside 'params' that contain human-readable text (questions, summaries, assumptions, script outlines, titles, etc.) MUST be in the USER LANGUAGE. Do NOT translate creative content to English.
3. **Questioning**: For 'ask_questions', you **MUST** include an 'assumptions' list in params (target language) matching the question count.
Example if User Language code is 'nl' (Dutch):
{
  "step_id": 1,
  "agent_id": "business-analyst",
  "action": "ask_questions", 
  "params": { 
     "questions": ["Wat is de visuele stijl van de video?"],
     "assumptions": ["Eenvoudige animatie geschikt voor kinderen"] 
  }
}

Break this down into execution steps.
Output JSON array of objects:
[
  { "step_id": 1, "agent_id": "...", "action": "...", "params": {...}, "depends_on": [IDs] }
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
	prompt := fmt.Sprintf("Goal: %s\nAvailable Agents:\n%v\n\nTask: Return exactly one JSON array of strings containing Agent IDs. Be extremely restrictive.\nGuidelines:\n- For creative tasks (videos, blogs), use 'content-creator' AND 'scene-creator'.\n- For research/data tasks, use 'data-scientist'.\n- For infrastructure/ops, use 'infrastructure-engineer'.\n- For task refinement or if the goal is vague, ALWAYS include 'business-analyst'.\nExample: [\"business-analyst\", \"content-creator\", \"scene-creator\"]", intent.Prompt, detailedList)
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
		agentList = append(agentList, fmt.Sprintf(
			"ID: %s\n  Name: %s\n  Type: %s\n  Condition: %s\n  Sub-Agents: %v\n  Skills: %v\n  Priority: %.1f\n  Description: %s",
			a.ID, a.Name, a.Type, a.Condition, a.SubAgents, a.Skills, a.Priority, a.Description,
		))
	}
	fmt.Printf("[Planner - Agents] %v\n", sortedIDs)

	// 2. Prompt LLM
	sysTemplate := systemPromptTmpl
	if plannerAgent, err := p.registry.GetAgent("planner"); err == nil && plannerAgent.Instructions != "" {
		sysTemplate = plannerAgent.Instructions
	}

	replacer := strings.NewReplacer(
		"%goal%", intent.Prompt,
		"%action%", intent.Action,
		"%language%", intent.Language,
		"%tools%", fmt.Sprintf("%v", blockNames),
		"%agents%", fmt.Sprintf("%v", agentList),
	)
	sysPrompt := replacer.Replace(sysTemplate)

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
							ID:      1,
							AgentID: "business-analyst",
							Action:  "ask_questions",
							Params:  map[string]interface{}{"details_needed": errorResp.Error},
							Status:  "pending",
						},
					}
				} else {
					if p.Debug {
						fmt.Printf("[Planner] JSON Parse Error. Raw: %s\n", cleanResp)
					}
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
			if action == "ask_questions" || action == "copywriting" || action == "video-design" || action == "content-review" {
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
	// We use all agents so the planner can dynamically select new ones (e.g. content-creator, scene-creator)
	activeAgents := p.registry.ListAgents()

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
	//fmt.Printf("[Planner - Agents] All Registered: %v\n", sortedIDs)
	fmt.Printf("[Planner - Plan Context] Selected Agents: %v\n", plan.SelectedAgents)

	blocks := p.registry.ListBuildingBlocks()
	blockNames := make([]string, 0, len(blocks))
	for _, b := range blocks {
		blockNames = append(blockNames, b.Name)
	}

	allAgents := p.registry.ListAgents()
	// Sort by priority
	sort.Slice(allAgents, func(i, j int) bool {
		return allAgents[i].Priority > allAgents[j].Priority
	})

	updatedAgentList := make([]string, 0, len(allAgents))
	for _, a := range allAgents {
		updatedAgentList = append(updatedAgentList, fmt.Sprintf(
			"ID: %s\n  Name: %s\n  Type: %s\n  Condition: %s\n  Sub-Agents: %v\n  Skills: %v\n  Priority: %.1f\n  Description: %s",
			a.ID, a.Name, a.Type, a.Condition, a.SubAgents, a.Skills, a.Priority, a.Description,
		))
	}

	// --- AUTO-STOP LOGIC ---
	// Check if we have fulfilled the script outline
	var scriptLength int
	var sceneCount int
	for _, s := range plan.Steps {
		// Detect script in params
		if outline, ok := s.Params["script_outline"]; ok {
			if list, ok := outline.([]interface{}); ok {
				scriptLength = len(list)
			}
		}
		// Count executed scenes
		if s.Action == "scene-creator" || s.AgentID == "scene-creator" {
			sceneCount++
		}
	}

	if scriptLength > 0 && sceneCount >= scriptLength {
		// All scenes are accounted for. Stop planning.
		if p.Debug {
			fmt.Printf("[Planner] Script length %d, Scenes completed %d. Stopping plan generation.\n", scriptLength, sceneCount)
		}
		return plan, nil
	}
	// -----------------------

	// Load System Prompt from Agent Definition
	sysTemplate := systemPromptTmpl
	if plannerAgent, err := p.registry.GetAgent("planner"); err == nil && plannerAgent.Instructions != "" {
		sysTemplate = plannerAgent.Instructions
	}

	replacer := strings.NewReplacer(
		"%goal%", plan.Intent.Prompt,
		"%action%", plan.Intent.Action,
		"%language%", plan.Intent.Language,
		"%tools%", fmt.Sprintf("%v", blockNames),
		"%agents%", fmt.Sprintf("%v", updatedAgentList),
	)
	baseSystemPrompt := replacer.Replace(sysTemplate)

	startID := 0
	if len(plan.Steps) > 0 {
		startID = plan.Steps[len(plan.Steps)-1].ID
	}

	taskPrompt := fmt.Sprintf(
		"--- HISTORY & PROGRESS ---\n"+
			"Current Steps (with results): %v\n"+
			"Latest User Input: %s\n\n"+
			"--- TASK ---\n"+
			"1. REPLAN: Review 'Objective' and 'Current Steps'. If a step has a 'result', that info is now known.\n"+
			"2. STATUS CHECK: Review 'Current Steps'. If 'content-review' is pending, WAITING for user feedback. If 'scene-creator' is pending, WAITING for execution.\n"+
			"3. GENERATE: Provide NEXT steps (starting from id %d). Follow the Strategies defined above.\n"+
			"4. OUTPUT: Return a JSON array of Step objects.",
		plan.Steps,
		feedback,
		startID+1, // Start ID for new steps
	)

	fullPrompt := baseSystemPrompt + "\n\n" + taskPrompt

	resp, err := p.llm.Generate(ctx, "Refine Plan", fullPrompt)
	if err != nil {
		return nil, err
	}

	// 3. Parse and Append
	cleanResp := p.cleanJSONResponse(resp)

	// Temporary struct to handle string dependencies from LLM
	// Explicit struct to avoid embedding issues and handle flexible dependencies
	type ParsingStep struct {
		StepID       int                    `json:"step_id"`
		AgentID      string                 `json:"agent_id"`
		Action       string                 `json:"action"`
		Params       map[string]interface{} `json:"params"`
		DependsOnRaw interface{}            `json:"depends_on"`
	}
	var parsingSteps []ParsingStep

	if err := json.Unmarshal([]byte(cleanResp), &parsingSteps); err != nil {
		// Try wrapped
		var wrapped struct {
			Steps []ParsingStep `json:"steps"`
		}
		if err2 := json.Unmarshal([]byte(cleanResp), &wrapped); err2 == nil && len(wrapped.Steps) > 0 {
			parsingSteps = wrapped.Steps
		} else {
			// Try single object
			var single ParsingStep
			if err3 := json.Unmarshal([]byte(cleanResp), &single); err3 == nil {
				if single.Action != "" {
					parsingSteps = []ParsingStep{single}
				} else if p.Debug {
					fmt.Printf("[Planner] Single object parsed but Action is empty. Struct: %+v. RAW JSON: %s\n", single, cleanResp)
				}
			} else {
				if p.Debug {
					fmt.Printf("[Planner] Single object parse failed: %v\n", err3)
				}
				// Attempt 4: Error Object
				var errorResp struct {
					Error string `json:"error"`
				}
				if err4 := json.Unmarshal([]byte(cleanResp), &errorResp); err4 == nil && errorResp.Error != "" {
					if p.Debug {
						fmt.Printf("[Planner] LLM returned error: %s\n", errorResp.Error)
					}
				}
			}
		}
	}

	var newSteps []model.Step
	for _, ps := range parsingSteps {
		s := model.Step{
			ID:      ps.StepID,
			AgentID: ps.AgentID,
			Action:  ps.Action,
			Params:  ps.Params,
		}

		// Resolve Dependencies
		if ps.DependsOnRaw != nil {
			if arr, ok := ps.DependsOnRaw.([]interface{}); ok {
				for _, item := range arr {
					if f, ok := item.(float64); ok {
						s.DependsOn = append(s.DependsOn, int(f))
					}
				}
			} else if f, ok := ps.DependsOnRaw.(float64); ok {
				s.DependsOn = append(s.DependsOn, int(f))
			}
		}

		newSteps = append(newSteps, s)
	}

	if len(newSteps) == 0 && p.Debug {
		fmt.Printf("[Planner] No new steps generated. Raw response:\n%s\n", resp)
	}

	// Adjust IDs using existing startID
	// startID is already calculated above

	for i := range newSteps {
		newSteps[i].ID = startID + i + 1
		newSteps[i].Status = "pending"

		// Self-Correction: Fix missing AgentID by looking up Skill
		if newSteps[i].AgentID == "" {
			action := strings.ToLower(newSteps[i].Action)
			for _, agent := range allAgents {
				for _, skill := range agent.Skills {
					if strings.EqualFold(skill, action) {
						newSteps[i].AgentID = agent.ID
						break
					}
				}
				if newSteps[i].AgentID != "" {
					break
				}
			}
			// If still empty, we could fallback or leave it empty (which might cause issues)
			if newSteps[i].AgentID == "" && p.Debug {
				fmt.Printf("[Planner] WARNING: Could not resolve AgentID for action '%s'\n", newSteps[i].Action)
			}
		}

		// Resolve Dependencies (String -> Int)
		ps := parsingSteps[i]
		if ps.DependsOnRaw != nil {
			// Helper to resolve one dependency item
			resolveDep := func(dep interface{}) {
				if idFloat, ok := dep.(float64); ok {
					newSteps[i].DependsOn = append(newSteps[i].DependsOn, int(idFloat))
				} else if depStr, ok := dep.(string); ok {
					// Look for matching action in *newly created* steps (preceding this one)
					for j := 0; j < i; j++ {
						if newSteps[j].Action == depStr {
							newSteps[i].DependsOn = append(newSteps[i].DependsOn, newSteps[j].ID)
							break
						}
					}
				}
			}

			switch v := ps.DependsOnRaw.(type) {
			case []interface{}:
				for _, d := range v {
					resolveDep(d)
				}
			case string:
				resolveDep(v) // Handle single string case
			case float64:
				resolveDep(v)
			}
		}
	}

	// Filter out duplicate steps (LLMGuard)
	var filteredSteps []model.Step
	for _, newStep := range newSteps {
		isDuplicate := false
		// Check against existing history
		for _, existing := range plan.Steps {
			if existing.Action == newStep.Action && existing.Status == "completed" {
				// We generally don't want to repeat 'ask_questions' or 'content-review'
				if newStep.Action == "ask_questions" || newStep.Action == "content-review" {
					isDuplicate = true
					if p.Debug {
						fmt.Printf("[Planner] Dropping duplicate step: %s (already completed)\n", newStep.Action)
					}
					break
				}
			}
		}
		if !isDuplicate {
			filteredSteps = append(filteredSteps, newStep)
		}
	}

	// Append
	plan.Steps = append(plan.Steps, filteredSteps...)

	// Save to Store
	if p.Store != nil {
		_ = p.Store.SavePlan(*plan)
		planJSON, _ := json.MarshalIndent(plan, "", "  ")
		_ = p.Store.LogInteraction(plan.ID, "Planner Update", fullPrompt, resp+"\n\nRESULTING PLAN:\n"+string(planJSON))
	}

	return plan, nil
}
